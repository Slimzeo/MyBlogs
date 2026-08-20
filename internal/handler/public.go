package handler

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"myblog/internal/middleware"
	"myblog/internal/model"
	"myblog/internal/notes"
	"myblog/internal/service"
	"myblog/internal/util"

	"github.com/gin-gonic/gin"
)

func (server *Server) index(context *gin.Context) {
	server.renderIndex(context, 1)
}

func (server *Server) indexPage(context *gin.Context) {
	server.renderIndex(context, pathInt(context, "page", 1))
}

func (server *Server) renderIndex(context *gin.Context, page int) {
	page = clampPage(page)
	limit := clampLimit(queryInt(context, "limit", 12), 12, 100)
	includeEncrypted := server.canAccessEncrypted(context)
	data := server.baseData(context, "", "")
	data.Articles = server.service.GetContents(page, limit, includeEncrypted)
	if page > 1 {
		data.Title = "第" + strconv.Itoa(page) + "页"
	}
	server.render(context, http.StatusOK, "index", data)
}

func (server *Server) article(context *gin.Context) {
	server.renderArticle(context, false)
}

func (server *Server) articlePreview(context *gin.Context) {
	if server.sessions.User(context) == nil {
		context.Redirect(http.StatusFound, "/admin/login")
		return
	}
	server.renderArticle(context, true)
}

// articleDocument serves complete HTML articles inside the sandboxed iframe
// used by the article page. It repeats the normal article authorization check
// so encrypted and private content cannot be fetched through this endpoint.
func (server *Server) articleDocument(context *gin.Context) {
	content, err := server.service.GetContentByID(strings.TrimSuffix(context.Param("id"), ".html"))
	preview := context.Query("preview") == "1"
	if err != nil || content == nil || content.Type != model.TypeArticle ||
		content.ContentFormat != model.ContentHTML || !server.canViewArticle(context, content, preview) {
		context.Status(http.StatusNotFound)
		return
	}
	context.Header("Cache-Control", "private, no-store")
	context.Header("Vary", "Cookie")
	context.Header("Referrer-Policy", "no-referrer")
	context.Header("Content-Security-Policy",
		"sandbox allow-scripts; default-src 'none'; base-uri 'none'; object-src 'none'; form-action 'none'; "+
			"frame-ancestors 'self'; img-src 'self' data: https:; media-src 'self' data: https:; "+
			"font-src data: https:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'none'")
	context.Data(http.StatusOK, "text/html; charset=utf-8", []byte(content.Content))
}

func (server *Server) renderArticle(context *gin.Context, preview bool) {
	contentID := strings.TrimSuffix(context.Param("id"), ".html")
	content, err := server.service.GetContentByID(contentID)
	if err != nil || content == nil || !server.canViewArticle(context, content, preview) {
		server.render(context, http.StatusNotFound, "error_404", PageData{})
		return
	}
	server.hitCounter.Observe(content.Cid, content.Hits)
	server.hitCounter.Incr(content.Cid)
	data := server.baseData(context, content.Title, "")
	data.Keywords = content.Tags
	data.IsPost = true
	data.Article = content
	data.Hits = server.hitCounter.Current(content.Cid)
	if content.AllowComment {
		page := clampPage(queryInt(context, "cp", 1))
		data.Comments = server.service.GetComments(content.Cid, page, 6)
	}
	server.render(context, http.StatusOK, "post", data)
}

func (server *Server) canViewArticle(context *gin.Context, content *model.Content, preview bool) bool {
	if preview {
		return server.sessions.User(context) != nil
	}
	if content.Status == model.TypePublish {
		return true
	}
	if content.Status == model.TypeEncrypted {
		if server.sessions.User(context) != nil {
			log.Printf(
				"encrypted article access cid=%d mode=admin client_ip=%s",
				content.Cid,
				util.ClientIP(context.Request),
			)
			return true
		}
		if server.sessions.ArticleAccessExpiry(context.Request, server.config.AccessKey) > 0 {
			log.Printf(
				"encrypted article access cid=%d mode=access_key client_ip=%s",
				content.Cid,
				util.ClientIP(context.Request),
			)
			return true
		}
		return false
	}
	return content.Status == model.TypePrivate && server.sessions.User(context) != nil
}

func (server *Server) comment(context *gin.Context) {
	referer := context.GetHeader("Referer")
	token := context.PostForm("_csrf_token")
	if referer == "" || token == "" {
		respondFail(context, model.BadRequest)
		return
	}
	if !server.sessions.ValidateCSRFToken(token) {
		respondFail(context, model.BadRequest)
		return
	}
	cid, err := strconv.Atoi(context.PostForm("cid"))
	if err != nil || cid <= 0 {
		respondFail(context, "请输入完整后评论")
		return
	}
	content, _ := server.service.GetContentByID(strconv.Itoa(cid))
	if content == nil || !server.canViewArticle(context, content, false) {
		respondFail(context, "文章不存在或不可访问")
		return
	}
	parent, _ := strconv.Atoi(context.PostForm("coid"))
	author := strings.TrimSpace(context.PostForm("author"))
	mail := strings.TrimSpace(context.PostForm("mail"))
	commentURL := strings.TrimSpace(context.PostForm("url"))
	text := strings.TrimSpace(context.PostForm("text"))
	if text == "" {
		respondFail(context, "请输入完整后评论")
		return
	}
	if len([]rune(author)) > 50 {
		respondFail(context, "姓名过长")
		return
	}
	if mail != "" && !util.IsEmail(mail) {
		respondFail(context, "请输入正确的邮箱格式")
		return
	}
	if commentURL != "" {
		parsedURL, parseErr := url.ParseRequestURI(commentURL)
		if parseErr != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
			respondFail(context, "请输入正确的URL格式")
			return
		}
	}
	if len([]rune(text)) > 2000 {
		respondFail(context, "请输入2000个字符以内的评论")
		return
	}

	clientKey := util.ClientIP(context.Request) + ":" + strconv.Itoa(cid)
	if !server.service.Cache().HSetNX(model.TypeCommentsFreq, clientKey, 1, 60) {
		respondFail(context, "您发表评论太快了，请过会再试")
		return
	}
	comment := &model.Comment{
		Cid:     cid,
		Author:  util.CleanXSS(author),
		Mail:    mail,
		URL:     commentURL,
		IP:      util.ClientIP(context.Request),
		Agent:   context.GetHeader("User-Agent"),
		Content: util.CleanXSS(text),
		Parent:  parent,
	}
	if err := server.service.InsertComment(comment); err != nil {
		server.service.Cache().HDel(model.TypeCommentsFreq, clientKey)
		if message, ok := serviceMessage(err); ok {
			respondFail(context, message)
			return
		}
		respondFail(context, "评论发布失败")
		return
	}
	middleware.SetCookie(context, "tale_remember_author", url.QueryEscape(author), 7*24*60*60, false)
	middleware.SetCookie(context, "tale_remember_mail", url.QueryEscape(mail), 7*24*60*60, false)
	if commentURL != "" {
		middleware.SetCookie(context, "tale_remember_url", url.QueryEscape(commentURL), 7*24*60*60, false)
	}
	respondOK(context)
}

func (server *Server) category(context *gin.Context) {
	server.metaArticles(context, model.TypeCategory, context.Param("keyword"), "分类")
}

func (server *Server) tag(context *gin.Context) {
	name := strings.ReplaceAll(context.Param("name"), "+", " ")
	server.metaArticles(context, model.TypeTag, name, "标签")
}

func (server *Server) metaArticles(context *gin.Context, metaType, name, displayType string) {
	meta := server.service.GetMeta(metaType, name)
	if meta == nil {
		server.render(context, http.StatusNotFound, "error_404", PageData{})
		return
	}
	page := clampPage(pathInt(context, "page", 1))
	limit := clampLimit(queryInt(context, "limit", 12), 12, 100)
	data := server.baseData(context, name, "")
	data.Meta = meta
	data.Articles = server.service.GetArticlesByMeta(
		meta.Mid,
		page,
		limit,
		server.canAccessEncrypted(context),
	)
	data.Type = displayType
	data.Keyword = name
	server.render(context, http.StatusOK, "page-category", data)
}

func (server *Server) search(context *gin.Context) {
	keyword := context.Param("keyword")
	page := clampPage(pathInt(context, "page", 1))
	limit := clampLimit(queryInt(context, "limit", 12), 12, 100)
	data := server.baseData(context, keyword, "")
	data.Articles = server.service.SearchArticles(
		keyword,
		page,
		limit,
		server.canAccessEncrypted(context),
	)
	data.Type = "搜索"
	data.Keyword = keyword
	server.render(context, http.StatusOK, "page-category", data)
}

func (server *Server) topics(context *gin.Context) {
	view := context.DefaultQuery("view", "categories")
	if view != "categories" && view != "tags" {
		view = "categories"
	}
	data := server.baseData(context, "学习目录", "")
	data.TopicView = view
	includeEncrypted := server.canAccessEncrypted(context)
	data.Categories = server.service.GetPublishedMetaList(model.TypeCategory, 100, includeEncrypted)
	data.Tags = server.service.GetPublishedMetaList(model.TypeTag, 100, includeEncrypted)
	data.TopicGroups = server.service.GetPublishedTopicGroups(100, 20, includeEncrypted)
	server.render(context, http.StatusOK, "topics", data)
}

func (server *Server) notesPage(context *gin.Context) {
	path := strings.TrimPrefix(context.Param("path"), "/")
	data := server.baseData(context, "Notes", "")
	data.NotesPath = path
	data.NotesTree, _ = server.notes.Tree()

	if path != "" {
		document, documentErr := server.notes.Document(path)
		if documentErr == nil {
			data.NotesDocument = document
			data.Title = document.Title
			server.render(context, http.StatusOK, "notes", data)
			return
		}
	}

	folder, folderErr := server.notes.Folder(path)
	if folderErr != nil {
		if errors.Is(folderErr, notes.ErrNotFound) {
			server.render(context, http.StatusNotFound, "error_404", PageData{})
			return
		}
		server.render(context, http.StatusInternalServerError, "error_500", PageData{Message: "读取 Notes 目录失败"})
		return
	}
	data.NotesFolder = folder
	data.NotesIsFolder = true
	readmePath := "README"
	if path != "" {
		readmePath = path + "/README"
	}
	if readme, readmeErr := server.notes.Document(readmePath); readmeErr == nil {
		data.NotesDocument = readme
	}
	server.render(context, http.StatusOK, "notes", data)
}

func (server *Server) archives(context *gin.Context) {
	data := server.baseData(context, "文章归档", "")
	data.Archives = server.service.GetArchives(server.canAccessEncrypted(context))
	for _, archive := range data.Archives {
		data.ArchiveCount += len(archive.Articles)
	}
	server.render(context, http.StatusOK, "archives", data)
}

func (server *Server) links(context *gin.Context) {
	data := server.baseData(context, "友情链接", "")
	data.Links = server.service.GetMetas(model.TypeLink)
	server.render(context, http.StatusOK, "links", data)
}

func (server *Server) publicLogout(context *gin.Context) {
	server.sessions.Logout(context)
	context.Redirect(http.StatusFound, "/")
}

func (server *Server) importAccessKey(context *gin.Context) {
	if server.config.AccessKey == "" {
		respondFail(context, "站点未启用访问密钥")
		return
	}
	failureKey := "access_key_error_count:" + util.ClientIP(context.Request)
	if failures, exists := server.service.Cache().GetInt(failureKey); exists && failures >= 5 {
		respondFail(context, "密钥错误次数过多，请10分钟后再试")
		return
	}
	if !accessKeyMatches(context.PostForm("accessKey"), server.config.AccessKey) {
		failures := server.service.Cache().Incr(failureKey, 10*60)
		if failures >= 5 {
			respondFail(context, "密钥错误次数过多，请10分钟后再试")
			return
		}
		respondFail(context, "访问密钥无效")
		return
	}
	server.service.Cache().Del(failureKey)
	expiry := server.sessions.GrantArticleAccess(context, server.config.AccessKey)
	log.Printf("encrypted article access granted client_ip=%s", util.ClientIP(context.Request))
	respondOK(context, gin.H{"expiresAt": expiry})
}

func (server *Server) revokeAccessKey(context *gin.Context) {
	server.sessions.RevokeArticleAccess(context)
	respondOK(context)
}

func accessKeyMatches(candidate, expected string) bool {
	candidateHash := sha256.Sum256([]byte(strings.TrimSpace(candidate)))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(candidateHash[:], expectedHash[:]) == 1
}

func (server *Server) customPageOrNotFound(context *gin.Context) {
	if context.Request.Method != http.MethodGet {
		server.render(context, http.StatusNotFound, "error_404", PageData{})
		return
	}
	path := strings.TrimPrefix(context.Request.URL.Path, "/")
	if path == "" || strings.Contains(path, "/") {
		server.render(context, http.StatusNotFound, "error_404", PageData{})
		return
	}
	content, err := server.service.GetContentByID(path)
	if err != nil || content == nil || content.Type != model.TypePage || content.Status == model.TypeDraft {
		server.render(context, http.StatusNotFound, "error_404", PageData{})
		return
	}
	server.hitCounter.Observe(content.Cid, content.Hits)
	server.hitCounter.Incr(content.Cid)
	data := server.baseData(context, content.Title, "")
	data.Article = content
	data.Hits = server.hitCounter.Current(content.Cid)
	if content.AllowComment {
		data.Comments = server.service.GetComments(content.Cid, clampPage(queryInt(context, "cp", 1)), 6)
	}
	server.render(context, http.StatusOK, "page", data)
}

func serviceMessage(err error) (string, bool) {
	return service.AsTip(err)
}
