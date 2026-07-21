package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"myblog/internal/middleware"
	"myblog/internal/model"
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
	data := server.baseData(context, "", "")
	data.Articles = server.service.GetContents(page, limit)
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
	data.Articles = server.service.GetArticlesByMeta(meta.Mid, page, limit)
	data.Type = displayType
	data.Keyword = name
	server.render(context, http.StatusOK, "page-category", data)
}

func (server *Server) search(context *gin.Context) {
	keyword := context.Param("keyword")
	page := clampPage(pathInt(context, "page", 1))
	limit := clampLimit(queryInt(context, "limit", 12), 12, 100)
	data := server.baseData(context, keyword, "")
	data.Articles = server.service.SearchArticles(keyword, page, limit)
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
	data.Categories = server.service.GetPublishedMetaList(model.TypeCategory, 100)
	data.Tags = server.service.GetPublishedMetaList(model.TypeTag, 100)
	data.TopicGroups = server.service.GetPublishedTopicGroups(100, 20)
	server.render(context, http.StatusOK, "topics", data)
}

func (server *Server) archives(context *gin.Context) {
	data := server.baseData(context, "文章归档", "")
	data.Archives = server.service.GetArchives()
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
