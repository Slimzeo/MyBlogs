package router

import (
	"myblog/internal/handler"

	"github.com/gin-gonic/gin"
)

func registerAdminRoutes(engine *gin.Engine, routes handler.AdminRouteHandlers, auth, csrf gin.HandlerFunc) {
	engine.GET("/admin/login", routes.Login)
	engine.POST("/admin/login", csrf, routes.DoLogin)

	admin := engine.Group("/admin")
	admin.Use(auth, csrf)
	admin.GET("", routes.Index)
	admin.GET("/index", routes.IndexAlias)
	admin.GET("/logout", routes.Logout)
	admin.GET("/profile", routes.Profile)
	admin.POST("/profile", routes.SaveProfile)
	admin.POST("/password", routes.ChangePassword)

	admin.GET("/article", routes.Article)
	admin.GET("/article/publish", routes.NewArticle)
	admin.GET("/article/:id", routes.EditArticle)
	admin.POST("/article/publish", routes.PublishArticle)
	admin.POST("/article/modify", routes.ModifyArticle)
	admin.POST("/article/delete", routes.DeleteArticle)
	admin.POST("/article/image", routes.UploadArticleImage)

	admin.GET("/page", routes.Page)
	admin.GET("/page/new", routes.NewPage)
	admin.GET("/page/:id", routes.EditPage)
	admin.POST("/page/publish", routes.PublishPage)
	admin.POST("/page/modify", routes.ModifyPage)
	admin.POST("/page/delete", routes.DeletePage)

	admin.GET("/comments", routes.Comments)
	admin.POST("/comments", routes.ReplyComment)
	admin.POST("/comments/delete", routes.DeleteComment)
	admin.POST("/comments/status", routes.UpdateComment)

	admin.GET("/category", routes.Category)
	admin.POST("/category/save", routes.SaveCategory)
	admin.POST("/category/delete", routes.DeleteCategory)

	admin.GET("/links", routes.Links)
	admin.POST("/links/save", routes.SaveLink)
	admin.POST("/links/delete", routes.DeleteLink)

	admin.GET("/attach", routes.Attach)
	admin.POST("/attach/upload", routes.UploadAttach)
	admin.POST("/attach/delete", routes.DeleteAttach)

	admin.GET("/setting", routes.Setting)
	admin.POST("/setting", routes.SaveSetting)
	admin.POST("/setting/backup", routes.Backup)
}
