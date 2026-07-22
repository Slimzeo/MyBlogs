package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"myblog/internal/model"
	"myblog/internal/service"
	"myblog/internal/util"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func (server *Server) adminLogin(context *gin.Context) {
	if server.sessions.User(context) != nil {
		context.Redirect(http.StatusFound, "/admin/index")
		return
	}
	server.render(context, http.StatusOK, "admin/login", server.baseData(context, "登录", ""))
}

func (server *Server) adminDoLogin(context *gin.Context) {
	clientIP := util.ClientIP(context.Request)
	failureKey := "login_error_count:" + clientIP
	if failures, exists := server.service.Cache().GetInt(failureKey); exists && failures >= 3 {
		respondFail(context, "您输入密码已经错误超过3次，请10分钟后尝试")
		return
	}
	user, err := server.service.Login(context.PostForm("username"), context.PostForm("password"))
	if err != nil {
		failures := server.service.Cache().Incr(failureKey, 10*60)
		log.Printf("admin login failed username=%q client_ip=%s failures=%d", context.PostForm("username"), clientIP, failures)
		if failures >= 3 {
			respondFail(context, "您输入密码已经错误超过3次，请10分钟后尝试")
			return
		}
		if message, ok := serviceMessage(err); ok {
			respondFail(context, message)
			return
		}
		respondFail(context, "登录失败")
		return
	}
	server.service.Cache().Del(failureKey)
	server.sessions.Login(context, user, context.PostForm("remeber_me") != "")
	server.service.InsertLog(model.LogLogin, "", clientIP, user.Uid)
	respondOK(context)
}

func (server *Server) adminLogout(context *gin.Context) {
	server.sessions.Logout(context)
	context.Redirect(http.StatusFound, "/admin/login")
}

func (server *Server) adminIndex(context *gin.Context) {
	data := server.baseData(context, "管理首页", "index")
	data.RecentComments = server.service.RecentComments(5)
	data.RecentArticles = server.service.RecentContents(5)
	data.Statistics = server.service.GetStatistics()
	data.Logs = server.service.GetLogs(1, 5)
	server.render(context, http.StatusOK, "admin/index", data)
}

func (server *Server) adminProfile(context *gin.Context) {
	server.render(context, http.StatusOK, "admin/profile", server.baseData(context, "个人设置", "profile"))
}

func (server *Server) adminSaveProfile(context *gin.Context) {
	user := server.sessions.User(context)
	username := strings.TrimSpace(context.PostForm("username"))
	screenName := strings.TrimSpace(context.PostForm("screenName"))
	email := strings.TrimSpace(context.PostForm("email"))
	if username == "" || screenName == "" || !util.IsEmail(email) {
		respondFail(context, "请确认用户名、昵称和邮箱格式正确")
		return
	}
	update := &model.User{Uid: user.Uid, Username: username, ScreenName: screenName, Email: email}
	if err := server.service.UpdateUserByUID(update); err != nil {
		respondServiceError(context, err, "保存个人信息失败")
		return
	}
	user.Username = username
	user.ScreenName = screenName
	user.Email = email
	server.sessions.Login(context, user, false)
	server.service.InsertLog(model.LogUpInfo, marshalLog(update), util.ClientIP(context.Request), user.Uid)
	respondOK(context)
}

func (server *Server) adminChangePassword(context *gin.Context) {
	user := server.sessions.User(context)
	oldPassword := context.PostForm("oldPassword")
	password := context.PostForm("password")
	if !passwordMatches(user, oldPassword) {
		respondFail(context, "旧密码错误")
		return
	}
	if length := len([]rune(password)); length < 6 || length > 14 {
		respondFail(context, "请输入6-14位密码")
		return
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		respondFail(context, "密码修改失败")
		return
	}
	if err := server.service.UpdateUserByUID(&model.User{
		Uid:      user.Uid,
		Password: string(passwordHash),
	}); err != nil {
		respondFail(context, "密码修改失败")
		return
	}
	server.service.InsertLog(model.LogUpPwd, "", util.ClientIP(context.Request), user.Uid)
	server.sessions.Logout(context)
	respondOK(context)
}

func passwordMatches(user *model.User, password string) bool {
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) == nil {
		return true
	}
	return user.Password == util.MD5encode(user.Username+password)
}

func (server *Server) adminArticleList(context *gin.Context) {
	page := clampPage(queryInt(context, "page", 1))
	limit := clampLimit(queryInt(context, "limit", 15), 15, 100)
	data := server.baseData(context, "文章管理", "article")
	data.Articles = server.service.ArticlesByTypePaged(model.TypeArticle, page, limit)
	server.render(context, http.StatusOK, "admin/article_list", data)
}

func (server *Server) adminArticleNew(context *gin.Context) {
	data := server.baseData(context, "发表文章", "article")
	data.Links = server.service.GetMetas(model.TypeCategory)
	server.render(context, http.StatusOK, "admin/article_edit", data)
}

func (server *Server) adminArticleEdit(context *gin.Context) {
	content, _ := server.service.GetContentByID(context.Param("id"))
	if content == nil || content.Type != model.TypeArticle {
		server.render(context, http.StatusNotFound, "error_404", PageData{})
		return
	}
	data := server.baseData(context, "编辑文章", "article")
	data.Contents = content
	server.render(context, http.StatusOK, "admin/article_edit", data)
}

func (server *Server) adminArticlePublish(context *gin.Context) {
	content := server.articleFromForm(context)
	if content.Categories == "" {
		content.Categories = "默认分类"
	}
	if err := server.service.Publish(content); err != nil {
		respondServiceError(context, err, "文章发布失败")
		return
	}
	respondOK(context)
}

func (server *Server) adminArticleModify(context *gin.Context) {
	content := server.articleFromForm(context)
	content.Cid, _ = strconv.Atoi(context.PostForm("cid"))
	if err := server.service.UpdateArticle(content); err != nil {
		respondServiceError(context, err, "文章编辑失败")
		return
	}
	respondOK(context)
}

func (server *Server) adminArticleDelete(context *gin.Context) {
	cid, err := strconv.Atoi(context.PostForm("cid"))
	if err != nil {
		respondFail(context, "非法文章ID")
		return
	}
	if err := server.service.DeleteByCid(cid); err != nil {
		respondFail(context, "文章删除失败")
		return
	}
	user := server.sessions.User(context)
	server.service.InsertLog(model.LogDelArticle, strconv.Itoa(cid), util.ClientIP(context.Request), user.Uid)
	respondOK(context)
}

func (server *Server) adminArticleImageUpload(context *gin.Context) {
	if err := context.Request.ParseMultipartForm(int64(model.MaxFileSize) + (1 << 20)); err != nil {
		respondFail(context, "图片上传失败")
		return
	}
	header := firstMultipartFile(context, "file")
	if header == nil {
		respondFail(context, "请选择一张图片")
		return
	}
	if header.Size > model.MaxFileSize {
		respondFail(context, "图片不能超过1MB")
		return
	}
	fkey, fileType, err := server.saveUploadedFile(header)
	if err != nil || fileType != model.TypeImage {
		respondFail(context, "只支持 JPG、PNG、GIF、WEBP 或 BMP 图片")
		return
	}
	user := server.sessions.User(context)
	if err := server.service.SaveAttach(filepath.Base(header.Filename), fkey, fileType, user.Uid); err != nil {
		_ = os.Remove(filepath.Join(server.config.UploadDir, strings.TrimPrefix(fkey, "/upload/")))
		respondFail(context, "图片记录保存失败")
		return
	}
	respondOK(context, gin.H{"url": fkey})
}

func (server *Server) adminArticleImport(context *gin.Context) {
	if err := context.Request.ParseMultipartForm(int64(16<<20) + (1 << 20)); err != nil {
		respondFail(context, "压缩包上传失败")
		return
	}
	header := firstMultipartFile(context, "archive")
	if header == nil {
		respondFail(context, "请选择一个 ZIP 压缩包")
		return
	}
	if header.Size > 16<<20 {
		respondFail(context, "压缩包不能超过16MB")
		return
	}
	file, err := header.Open()
	if err != nil {
		respondFail(context, "压缩包无法读取")
		return
	}
	defer file.Close()
	archiveData, err := io.ReadAll(io.LimitReader(file, (16<<20)+1))
	if err != nil || len(archiveData) > 16<<20 {
		respondFail(context, "压缩包读取失败")
		return
	}
	user := server.sessions.User(context)
	content, err := server.service.ImportMarkdownArchive(archiveData, service.ImportOptions{
		AuthorID:   user.Uid,
		Tags:       context.PostForm("tags"),
		Categories: context.PostForm("categories"),
		Status:     model.TypeDraft,
	})
	if err != nil {
		if message, ok := service.AsTip(err); ok {
			respondFail(context, message)
			return
		}
		respondFail(context, "压缩包导入失败")
		return
	}
	respondOK(context, gin.H{
		"cid":    content.Cid,
		"title":  content.Title,
		"status": content.Status,
	})
}

func firstMultipartFile(context *gin.Context, field string) *multipart.FileHeader {
	if context.Request.MultipartForm == nil {
		return nil
	}
	headers := context.Request.MultipartForm.File[field]
	if len(headers) == 0 {
		return nil
	}
	return headers[0]
}

func (server *Server) articleFromForm(context *gin.Context) *model.Content {
	return &model.Content{
		Title:        strings.TrimSpace(context.PostForm("title")),
		Content:      context.PostForm("content"),
		Slug:         strings.TrimSpace(context.PostForm("slug")),
		Tags:         strings.TrimSpace(context.PostForm("tags")),
		Categories:   strings.TrimSpace(context.PostForm("categories")),
		Status:       defaultString(context.PostForm("status"), model.TypePublish),
		Type:         model.TypeArticle,
		AuthorID:     server.sessions.User(context).Uid,
		AllowComment: true,
		AllowPing:    true,
		AllowFeed:    true,
	}
}

func (server *Server) adminPageList(context *gin.Context) {
	data := server.baseData(context, "页面管理", "page")
	data.Articles = server.service.ArticlesByTypePaged(model.TypePage, 1, model.MaxPosts)
	server.render(context, http.StatusOK, "admin/page_list", data)
}

func (server *Server) adminPageNew(context *gin.Context) {
	server.render(context, http.StatusOK, "admin/page_edit", server.baseData(context, "新建页面", "page"))
}

func (server *Server) adminPageEdit(context *gin.Context) {
	content, _ := server.service.GetContentByID(context.Param("id"))
	if content == nil || content.Type != model.TypePage {
		server.render(context, http.StatusNotFound, "error_404", PageData{})
		return
	}
	data := server.baseData(context, "编辑页面", "page")
	data.Contents = content
	server.render(context, http.StatusOK, "admin/page_edit", data)
}

func (server *Server) adminPagePublish(context *gin.Context) {
	content := server.pageFromForm(context)
	if err := server.service.Publish(content); err != nil {
		respondServiceError(context, err, "页面发布失败")
		return
	}
	respondOK(context)
}

func (server *Server) adminPageModify(context *gin.Context) {
	content := server.pageFromForm(context)
	content.Cid, _ = strconv.Atoi(context.PostForm("cid"))
	if err := server.service.UpdateArticle(content); err != nil {
		respondServiceError(context, err, "页面编辑失败")
		return
	}
	respondOK(context)
}

func (server *Server) pageFromForm(context *gin.Context) *model.Content {
	return &model.Content{
		Title:        strings.TrimSpace(context.PostForm("title")),
		Content:      context.PostForm("content"),
		Slug:         strings.TrimSpace(context.PostForm("slug")),
		Status:       defaultString(context.PostForm("status"), model.TypePublish),
		Type:         model.TypePage,
		AuthorID:     server.sessions.User(context).Uid,
		AllowComment: context.PostForm("allowComment") == "1",
		AllowPing:    context.PostForm("allowPing") == "1",
		AllowFeed:    true,
	}
}

func (server *Server) adminPageDelete(context *gin.Context) {
	cid, err := strconv.Atoi(context.PostForm("cid"))
	if err != nil {
		respondFail(context, "非法页面ID")
		return
	}
	if err := server.service.DeleteByCid(cid); err != nil {
		respondFail(context, "页面删除失败")
		return
	}
	user := server.sessions.User(context)
	server.service.InsertLog(model.LogDelPage, strconv.Itoa(cid), util.ClientIP(context.Request), user.Uid)
	respondOK(context)
}

func (server *Server) adminCommentList(context *gin.Context) {
	page := clampPage(queryInt(context, "page", 1))
	limit := clampLimit(queryInt(context, "limit", 15), 15, 100)
	data := server.baseData(context, "评论管理", "comment")
	data.AdminComments = server.service.GetCommentsExcludingAuthor(server.sessions.User(context).Uid, page, limit)
	server.render(context, http.StatusOK, "admin/comment_list", data)
}

func (server *Server) adminCommentDelete(context *gin.Context) {
	coid, err := strconv.Atoi(context.PostForm("coid"))
	if err != nil {
		respondFail(context, "不存在该评论")
		return
	}
	comment := server.service.GetCommentByID(coid)
	if comment == nil {
		respondFail(context, "不存在该评论")
		return
	}
	if err := server.service.DeleteComment(coid, comment.Cid); err != nil {
		respondFail(context, "评论删除失败")
		return
	}
	respondOK(context)
}

func (server *Server) adminCommentStatus(context *gin.Context) {
	coid, err := strconv.Atoi(context.PostForm("coid"))
	if err != nil {
		respondFail(context, "非法评论ID")
		return
	}
	server.service.UpdateComment(&model.Comment{Coid: coid, Status: context.PostForm("status")})
	respondOK(context)
}

func (server *Server) adminCommentReply(context *gin.Context) {
	coid, err := strconv.Atoi(context.PostForm("coid"))
	content := strings.TrimSpace(context.PostForm("content"))
	if err != nil || content == "" || len([]rune(content)) > 2000 {
		respondFail(context, "请输入2000个字符以内的回复")
		return
	}
	parent := server.service.GetCommentByID(coid)
	if parent == nil {
		respondFail(context, "不存在该评论")
		return
	}
	user := server.sessions.User(context)
	comment := &model.Comment{
		Cid:      parent.Cid,
		Author:   user.Username,
		AuthorID: user.Uid,
		Mail:     user.Email,
		URL:      user.HomeURL,
		IP:       util.ClientIP(context.Request),
		Agent:    context.GetHeader("User-Agent"),
		Content:  util.CleanXSS(content),
		Parent:   coid,
	}
	if err := server.service.InsertComment(comment); err != nil {
		respondServiceError(context, err, "回复失败")
		return
	}
	respondOK(context)
}

func (server *Server) adminCategory(context *gin.Context) {
	data := server.baseData(context, "分类标签", "category")
	data.Categories = server.service.GetMetaList(model.TypeCategory, "", model.MaxPosts)
	data.Tags = server.service.GetMetaList(model.TypeTag, "", model.MaxPosts)
	server.render(context, http.StatusOK, "admin/category", data)
}

func (server *Server) adminCategorySave(context *gin.Context) {
	mid, _ := strconv.Atoi(context.PostForm("mid"))
	metaType := strings.TrimSpace(context.PostForm("type"))
	if metaType != model.TypeCategory && metaType != model.TypeTag {
		respondFail(context, "项目类型不合法")
		return
	}
	if err := server.service.SaveOrRenameCategory(metaType, strings.TrimSpace(context.PostForm("cname")), mid); err != nil {
		respondServiceError(context, err, "分类或标签保存失败")
		return
	}
	respondOK(context)
}

func (server *Server) adminMetaDelete(context *gin.Context) {
	mid, err := strconv.Atoi(context.PostForm("mid"))
	if err != nil {
		respondFail(context, "非法项目ID")
		return
	}
	if err := server.service.DeleteMeta(mid); err != nil {
		respondFail(context, "删除失败")
		return
	}
	respondOK(context)
}

func (server *Server) adminLinks(context *gin.Context) {
	data := server.baseData(context, "友链管理", "links")
	data.Links = server.service.GetMetas(model.TypeLink)
	server.render(context, http.StatusOK, "admin/links", data)
}

func (server *Server) adminLinkSave(context *gin.Context) {
	mid, _ := strconv.Atoi(context.PostForm("mid"))
	sortValue, _ := strconv.Atoi(context.PostForm("sort"))
	meta := &model.Meta{
		Mid:         mid,
		Name:        strings.TrimSpace(context.PostForm("title")),
		Slug:        strings.TrimSpace(context.PostForm("url")),
		Description: strings.TrimSpace(context.PostForm("logo")),
		Sort:        sortValue,
		Type:        model.TypeLink,
	}
	var err error
	if mid == 0 {
		err = server.service.SaveMeta(meta)
	} else {
		err = server.service.UpdateMeta(meta)
	}
	if err != nil {
		respondFail(context, "友链保存失败")
		return
	}
	respondOK(context)
}

func (server *Server) adminAttach(context *gin.Context) {
	page := clampPage(queryInt(context, "page", 1))
	limit := clampLimit(queryInt(context, "limit", 12), 12, 100)
	data := server.baseData(context, "附件管理", "attach")
	data.Attachs = server.service.GetAttachs(page, limit)
	data.MaxFileSize = model.MaxFileSize / 1024
	server.render(context, http.StatusOK, "admin/attach", data)
}

func (server *Server) adminAttachUpload(context *gin.Context) {
	context.Request.Body = http.MaxBytesReader(
		context.Writer,
		context.Request.Body,
		int64(model.MaxFileSize*16)+(1<<20),
	)
	if err := context.Request.ParseMultipartForm(int64(model.MaxFileSize) * 16); err != nil {
		respondFail(context, "上传文件解析失败")
		return
	}
	headers := context.Request.MultipartForm.File["file"]
	if len(headers) == 0 {
		respondFail(context, "请选择上传文件")
		return
	}
	user := server.sessions.User(context)
	var rejected []string
	for _, header := range headers {
		if header.Size > model.MaxFileSize {
			rejected = append(rejected, filepath.Base(header.Filename))
			continue
		}
		fkey, fileType, err := server.saveUploadedFile(header)
		if err != nil {
			rejected = append(rejected, filepath.Base(header.Filename))
			continue
		}
		if err := server.service.SaveAttach(filepath.Base(header.Filename), fkey, fileType, user.Uid); err != nil {
			_ = os.Remove(filepath.Join(server.config.UploadDir, strings.TrimPrefix(fkey, "/upload/")))
			rejected = append(rejected, filepath.Base(header.Filename))
		}
	}
	respondOK(context, rejected)
}

func (server *Server) saveUploadedFile(header *multipart.FileHeader) (string, string, error) {
	source, err := header.Open()
	if err != nil {
		return "", "", err
	}
	defer source.Close()

	buffer := make([]byte, 512)
	read, err := io.ReadFull(source, buffer)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", "", err
	}
	contentType := http.DetectContentType(buffer[:read])
	fileType := model.TypeFile
	if strings.HasPrefix(contentType, "image/") {
		fileType = model.TypeImage
	}
	extension := strings.ToLower(filepath.Ext(filepath.Base(header.Filename)))
	if !allowedUpload(extension, fileType) {
		return "", "", errors.New("invalid extension")
	}
	relativeDirectory := filepath.Join(timeNow().Format("2006"), timeNow().Format("01"))
	directory := filepath.Join(server.config.UploadDir, relativeDirectory)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", "", err
	}
	fileName := util.UU32() + extension
	destinationPath := filepath.Join(directory, fileName)
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", "", err
	}
	defer destination.Close()
	if _, err := destination.Write(buffer[:read]); err != nil {
		return "", "", err
	}
	if _, err := io.Copy(destination, source); err != nil {
		return "", "", err
	}
	return "/upload/" + filepath.ToSlash(filepath.Join(relativeDirectory, fileName)), fileType, nil
}

func allowedUpload(extension, fileType string) bool {
	imageExtensions := map[string]struct{}{
		".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {}, ".webp": {}, ".bmp": {},
	}
	fileExtensions := map[string]struct{}{
		".txt": {}, ".md": {}, ".pdf": {}, ".zip": {}, ".doc": {}, ".docx": {},
		".xls": {}, ".xlsx": {}, ".ppt": {}, ".pptx": {},
	}
	if fileType == model.TypeImage {
		_, allowed := imageExtensions[extension]
		return allowed
	}
	_, allowed := fileExtensions[extension]
	return allowed
}

func (server *Server) adminAttachDelete(context *gin.Context) {
	id, err := strconv.Atoi(context.PostForm("id"))
	if err != nil {
		respondFail(context, "不存在该附件")
		return
	}
	attachment := server.service.GetAttachByID(id)
	if attachment == nil {
		respondFail(context, "不存在该附件")
		return
	}
	filePath := filepath.Join(server.config.UploadDir, strings.TrimPrefix(attachment.Fkey, "/upload/"))
	if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		respondFail(context, "附件删除失败")
		return
	}
	server.service.DeleteAttach(id)
	respondOK(context)
}

func (server *Server) adminSetting(context *gin.Context) {
	data := server.baseData(context, "系统设置", "setting")
	data.Options = server.service.OptionsMap()
	server.render(context, http.StatusOK, "admin/setting", data)
}

func (server *Server) adminSettingSave(context *gin.Context) {
	if err := context.Request.ParseForm(); err != nil {
		respondFail(context, "保存设置失败")
		return
	}
	options := make(map[string]string, len(context.Request.PostForm))
	for key, values := range context.Request.PostForm {
		if key == "_csrf_token" {
			continue
		}
		options[key] = strings.Join(values, ",")
	}
	if err := server.service.SaveOptions(options); err != nil {
		respondFail(context, "保存设置失败")
		return
	}
	server.siteConfig.Refresh()
	user := server.sessions.User(context)
	server.service.InsertLog(model.LogSysSetting, marshalLog(options), util.ClientIP(context.Request), user.Uid)
	respondOK(context)
}

func (server *Server) adminBackup(context *gin.Context) {
	backupType := strings.TrimSpace(context.PostForm("bk_type"))
	if backupType == "" {
		respondFail(context, "请确认信息输入完整")
		return
	}
	result, err := server.service.Backup(backupType, strings.TrimSpace(context.PostForm("bk_path")), "templates/theme")
	if err != nil {
		respondServiceError(context, err, "备份失败")
		return
	}
	user := server.sessions.User(context)
	server.service.InsertLog(model.LogSysBackup, "", util.ClientIP(context.Request), user.Uid)
	respondOK(context, result)
}

func respondServiceError(context *gin.Context, err error, fallback string) {
	if message, ok := serviceMessage(err); ok {
		respondFail(context, message)
		return
	}
	respondFail(context, fallback)
}

func marshalLog(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	if len(data) > 2000 {
		data = data[:2000]
	}
	return string(data)
}

func defaultString(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

var timeNow = func() time.Time { return time.Now() }
