package router

import (
	"myblog/internal/handler"

	"github.com/gin-gonic/gin"
)

func registerAPIRoutes(engine *gin.Engine, routes handler.APIRouteHandlers, auth gin.HandlerFunc) {
	api := engine.Group("/api/v1")
	api.Use(auth)
	api.GET("/auth", routes.Auth)
	api.POST("/articles/import", routes.ImportArticle)
}
