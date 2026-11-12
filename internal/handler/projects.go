package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (server *Server) projects(context *gin.Context) {
	data := server.baseData(context, "Projects", "projects")
	data.Description = "Hypnos 的个人项目简介与项目地址。"
	projects, err := server.service.Projects()
	if err != nil {
		log.Printf("load project cards: %v", err)
		server.render(context, http.StatusInternalServerError, "error_500", PageData{Message: "项目列表读取失败"})
		return
	}
	data.Projects = projects
	server.render(context, http.StatusOK, "projects", data)
}

func (server *Server) projectRedirect(context *gin.Context) {
	project, err := server.service.ProjectBySlug(context.Param("slug"))
	if err != nil {
		log.Printf("load project %q: %v", context.Param("slug"), err)
		server.render(context, http.StatusInternalServerError, "error_500", PageData{Message: "项目读取失败"})
		return
	}
	if project == nil {
		server.render(context, http.StatusNotFound, "error_404", PageData{})
		return
	}
	context.Redirect(http.StatusFound, "/projects#"+projectAnchor(project.Slug))
}

// Preserve bookmarks to the original two cards. Namespace new fragment IDs so
// editable slugs such as "board" never collide with the page header or controls.
func projectAnchor(slug string) string {
	if slug == "seekino" || slug == "catendar" {
		return slug
	}
	return "project:" + slug
}
