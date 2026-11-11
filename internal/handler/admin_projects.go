package handler

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"myblog/internal/model"
	"myblog/internal/util"

	"github.com/gin-gonic/gin"
)

func (server *Server) adminProjectCards(context *gin.Context) {
	data := server.baseData(context, "项目卡片", "projects")
	projects, err := server.service.Projects()
	if err != nil {
		log.Printf("admin load project cards: %v", err)
		server.render(context, http.StatusInternalServerError, "error_500", PageData{Message: "项目列表读取失败"})
		return
	}
	data.Projects = projects
	server.render(context, http.StatusOK, "admin/projects", data)
}

func (server *Server) adminProjectNew(context *gin.Context) {
	server.projectEditor(context, "新建项目卡片", nil)
}

func (server *Server) adminProjectEdit(context *gin.Context) {
	id, err := strconv.Atoi(context.Param("id"))
	if err != nil || id <= 0 {
		server.render(context, http.StatusNotFound, "error_404", PageData{})
		return
	}
	project, err := server.service.ProjectByID(id)
	if err != nil {
		log.Printf("admin load project id=%d: %v", id, err)
		server.render(context, http.StatusInternalServerError, "error_500", PageData{Message: "项目读取失败"})
		return
	}
	if project == nil {
		server.render(context, http.StatusNotFound, "error_404", PageData{})
		return
	}
	server.projectEditor(context, "编辑项目卡片", project)
}

func (server *Server) projectEditor(context *gin.Context, title string, project *model.Project) {
	articles, err := server.service.PublishedProjectArticles()
	if err != nil {
		log.Printf("admin load project articles: %v", err)
		server.render(context, http.StatusInternalServerError, "error_500", PageData{Message: "文章列表读取失败"})
		return
	}
	data := server.baseData(context, title, "projects")
	data.ProjectArticles = articles
	data.Project = project
	server.render(context, http.StatusOK, "admin/project_edit", data)
}

func (server *Server) adminProjectSave(context *gin.Context) {
	// A tab opened before the links migration still posts the old form.
	// Reject it before absent fields can clear the new destinations.
	if _, legacy := context.GetPostForm("project_url"); legacy {
		respondFail(context, "卡片编辑器已更新，请刷新页面后再保存")
		return
	}
	idValue := strings.TrimSpace(context.PostForm("id"))
	id := 0
	if idValue != "" {
		parsed, err := strconv.Atoi(idValue)
		if err != nil || parsed <= 0 {
			respondFail(context, "非法项目 ID")
			return
		}
		id = parsed
	}
	sortValue := 0
	if value := strings.TrimSpace(context.PostForm("sort")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			respondFail(context, "排序必须是整数")
			return
		}
		sortValue = parsed
	}
	articleID, err := strconv.Atoi(defaultString(strings.TrimSpace(context.PostForm("article_id")), "0"))
	if err != nil || articleID < 0 {
		respondFail(context, "所选文章无效")
		return
	}
	animation, animationSubmitted := context.GetPostForm("animation_url")
	project := &model.Project{
		ID:           id,
		Slug:         context.PostForm("slug"),
		Name:         context.PostForm("name"),
		Summary:      context.PostForm("summary"),
		CoverURL:     context.PostForm("cover_url"),
		AnimationURL: animation,
		CoverAlt:     context.PostForm("cover_alt"),
		PlatformURL:  context.PostForm("platform_url"),
		GitHubURL:    context.PostForm("github_url"),
		ArticleID:    articleID,
		Sort:         sortValue,
	}
	if id == 0 {
		err = server.service.CreateProject(project)
	} else {
		err = server.service.UpdateProject(project, animationSubmitted)
	}
	if err != nil {
		respondServiceError(context, err, "项目保存失败")
		return
	}
	user := server.sessions.User(context)
	server.service.InsertLog(
		model.LogSaveProject,
		marshalLog(map[string]any{"id": project.ID, "slug": project.Slug}),
		util.ClientIP(context.Request),
		user.Uid,
	)
	respondOK(context, gin.H{"id": project.ID, "slug": project.Slug})
}

func (server *Server) adminProjectDelete(context *gin.Context) {
	id, err := strconv.Atoi(context.PostForm("id"))
	if err != nil || id <= 0 {
		respondFail(context, "非法项目 ID")
		return
	}
	if err := server.service.DeleteProject(id); err != nil {
		respondServiceError(context, err, "项目删除失败")
		return
	}
	user := server.sessions.User(context)
	server.service.InsertLog(
		model.LogDeleteProject,
		strconv.Itoa(id),
		util.ClientIP(context.Request),
		user.Uid,
	)
	respondOK(context)
}
