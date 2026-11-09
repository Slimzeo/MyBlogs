package router

import (
	"os"
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
	root := applicationHandler.ReferenceRoot()
	if info, err := os.Stat(filepath.Join(root, "dist")); err == nil && info.IsDir() {
		for _, entry := range []string{"site", "media", "fonts"} {
			engine.GET("/assets/"+entry+"/*filepath", handler.ReferenceAsset(filepath.Join(root, "dist", "assets", entry)))
		}
		for _, entry := range []string{"campaign-actions.js", "works.json"} {
			engine.StaticFile("/assets/"+entry, filepath.Join(root, "dist", "assets", entry))
		}
		engine.GET("/next/assets/*filepath", handler.ReferenceAsset(filepath.Join(root, "dist", "next", "assets")))
		engine.GET("/assets/blog/*filepath", handler.ReferenceAsset(filepath.Join(root, "blog", "assets")))
	}

	routes := applicationHandler.RouteHandlers()
	engine.GET("/healthz", routes.Health)
	engine.GET("/readyz", routes.Ready)
	engine.Use(routes.IssueCSRF)

	registerPublicRoutes(engine, routes.Public, routes.AdminCSRF)
	registerAPIRoutes(engine, routes.API, routes.AgentAuth)
	registerAdminRoutes(engine, routes.Admin, routes.AdminAuth, routes.AdminCSRF)
	engine.NoRoute(routes.NotFound)
	return engine
}
