package router

import (
	"github.com/gin-gonic/gin"
	"myblog/internal/handler"
)

func registerPublicRoutes(engine *gin.Engine, routes handler.PublicRouteHandlers) {
	engine.GET("/", routes.Index)
	engine.GET("/index", routes.Index)
	engine.GET("/page/:page", routes.IndexPage)
	engine.GET("/article/:id", routes.Article)
	engine.GET("/article/:id/preview", routes.ArticlePreview)
	engine.POST("/comment", routes.Comment)
	engine.GET("/category/:keyword", routes.Category)
	engine.GET("/category/:keyword/:page", routes.Category)
	engine.GET("/tag/:name", routes.Tag)
	engine.GET("/tag/:name/:page", routes.Tag)
	engine.GET("/search/:keyword", routes.Search)
	engine.GET("/search/:keyword/:page", routes.Search)
	engine.GET("/archives", routes.Archives)
	engine.GET("/links", routes.Links)
	engine.GET("/logout", routes.Logout)
}
