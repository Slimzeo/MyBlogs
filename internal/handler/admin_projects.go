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
	server.render(context, http.StatusOK, "admin/project_edit", server.baseData(context, "新建项目卡片", "projects"))
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
	data := server.baseData(context, "编辑项目卡片", "projects")
	data.Project = project
	server.render(context, http.StatusOK, "admin/project_edit", data)
}

func (server *Server) adminProjectSave(context *gin.Context) {
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
	project := &model.Project{
		ID:         id,
		Slug:       context.PostForm("slug"),
		Name:       context.PostForm("name"),
		Summary:    context.PostForm("summary"),
		Notes:      context.PostForm("notes"),
		CoverURL:   context.PostForm("cover_url"),
		CoverAlt:   context.PostForm("cover_alt"),
		ProjectURL: context.PostForm("project_url"),
		Sort:       sortValue,
	}
	var err error
	if id == 0 {
		err = server.service.CreateProject(project)
	} else {
		err = server.service.UpdateProject(project)
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
