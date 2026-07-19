package router

import (
	"path/filepath"

	"myblog/internal/handler"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func New(applicationHandler *handler.Server, staticRoot string) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(applicationHandler.ApplicationMiddleware()...)
	engine.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{"/upload/"})))

	engine.Static("/user", filepath.Join(staticRoot, "user"))
	engine.Static("/assets/admin", filepath.Join(staticRoot, "admin"))
	engine.Static("/upload", applicationHandler.UploadDir())

	routes := applicationHandler.RouteHandlers()
	engine.GET("/healthz", routes.Health)
	engine.GET("/readyz", routes.Ready)
	engine.Use(routes.IssueCSRF)

	registerPublicRoutes(engine, routes.Public)
	registerAdminRoutes(engine, routes.Admin, routes.AdminAuth, routes.AdminCSRF)
	engine.NoRoute(routes.NotFound)
	return engine
}
