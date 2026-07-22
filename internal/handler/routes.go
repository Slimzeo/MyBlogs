package handler

import (
	"myblog/internal/middleware"

	"github.com/gin-gonic/gin"
)

type PublicRouteHandlers struct {
	Index          gin.HandlerFunc
	IndexPage      gin.HandlerFunc
	Article        gin.HandlerFunc
	ArticlePreview gin.HandlerFunc
	Comment        gin.HandlerFunc
	Category       gin.HandlerFunc
	Tag            gin.HandlerFunc
	Search         gin.HandlerFunc
	Topics         gin.HandlerFunc
	Notes          gin.HandlerFunc
	Archives       gin.HandlerFunc
	Links          gin.HandlerFunc
	Logout         gin.HandlerFunc
}

type AdminRouteHandlers struct {
	Login              gin.HandlerFunc
	DoLogin            gin.HandlerFunc
	Index              gin.HandlerFunc
	IndexAlias         gin.HandlerFunc
	Logout             gin.HandlerFunc
	Profile            gin.HandlerFunc
	SaveProfile        gin.HandlerFunc
	ChangePassword     gin.HandlerFunc
	Article            gin.HandlerFunc
	NewArticle         gin.HandlerFunc
	EditArticle        gin.HandlerFunc
	PublishArticle     gin.HandlerFunc
	ModifyArticle      gin.HandlerFunc
	DeleteArticle      gin.HandlerFunc
	UploadArticleImage gin.HandlerFunc
	ImportArticle      gin.HandlerFunc
	Page               gin.HandlerFunc
	NewPage            gin.HandlerFunc
	EditPage           gin.HandlerFunc
	PublishPage        gin.HandlerFunc
	ModifyPage         gin.HandlerFunc
	DeletePage         gin.HandlerFunc
	Comments           gin.HandlerFunc
	ReplyComment       gin.HandlerFunc
	DeleteComment      gin.HandlerFunc
	UpdateComment      gin.HandlerFunc
	Category           gin.HandlerFunc
	SaveCategory       gin.HandlerFunc
	DeleteCategory     gin.HandlerFunc
	Links              gin.HandlerFunc
	SaveLink           gin.HandlerFunc
	DeleteLink         gin.HandlerFunc
	Attach             gin.HandlerFunc
	UploadAttach       gin.HandlerFunc
	DeleteAttach       gin.HandlerFunc
	Setting            gin.HandlerFunc
	SaveSetting        gin.HandlerFunc
	Backup             gin.HandlerFunc
}

type RouteHandlers struct {
	Health    gin.HandlerFunc
	Ready     gin.HandlerFunc
	NotFound  gin.HandlerFunc
	IssueCSRF gin.HandlerFunc
	AdminAuth gin.HandlerFunc
	AdminCSRF gin.HandlerFunc
	Public    PublicRouteHandlers
	Admin     AdminRouteHandlers
}

func (server *Server) RouteHandlers() RouteHandlers {
	return RouteHandlers{
		Health:    server.health,
		Ready:     server.ready,
		NotFound:  server.customPageOrNotFound,
		IssueCSRF: server.issueCSRFToken(),
		AdminAuth: server.sessions.RequireAdmin(),
		AdminCSRF: middleware.ValidateCSRF(server.sessions),
		Public: PublicRouteHandlers{
			Index:          server.index,
			IndexPage:      server.indexPage,
			Article:        server.article,
			ArticlePreview: server.articlePreview,
			Comment:        server.comment,
			Category:       server.category,
			Tag:            server.tag,
			Search:         server.search,
			Topics:         server.topics,
			Notes:          server.notesPage,
			Archives:       server.archives,
			Links:          server.links,
			Logout:         server.publicLogout,
		},
		Admin: AdminRouteHandlers{
			Login:              server.adminLogin,
			DoLogin:            server.adminDoLogin,
			Index:              server.adminIndex,
			IndexAlias:         server.adminIndex,
			Logout:             server.adminLogout,
			Profile:            server.adminProfile,
			SaveProfile:        server.adminSaveProfile,
			ChangePassword:     server.adminChangePassword,
			Article:            server.adminArticleList,
			NewArticle:         server.adminArticleNew,
			EditArticle:        server.adminArticleEdit,
			PublishArticle:     server.adminArticlePublish,
			ModifyArticle:      server.adminArticleModify,
			DeleteArticle:      server.adminArticleDelete,
			UploadArticleImage: server.adminArticleImageUpload,
			ImportArticle:      server.adminArticleImport,
			Page:               server.adminPageList,
			NewPage:            server.adminPageNew,
			EditPage:           server.adminPageEdit,
			PublishPage:        server.adminPagePublish,
			ModifyPage:         server.adminPageModify,
			DeletePage:         server.adminPageDelete,
			Comments:           server.adminCommentList,
			ReplyComment:       server.adminCommentReply,
			DeleteComment:      server.adminCommentDelete,
			UpdateComment:      server.adminCommentStatus,
			Category:           server.adminCategory,
			SaveCategory:       server.adminCategorySave,
			DeleteCategory:     server.adminMetaDelete,
			Links:              server.adminLinks,
			SaveLink:           server.adminLinkSave,
			DeleteLink:         server.adminMetaDelete,
			Attach:             server.adminAttach,
			UploadAttach:       server.adminAttachUpload,
			DeleteAttach:       server.adminAttachDelete,
			Setting:            server.adminSetting,
			SaveSetting:        server.adminSettingSave,
			Backup:             server.adminBackup,
		},
	}
}

func (server *Server) ApplicationMiddleware() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		middleware.RequestLogger(),
		middleware.SecurityHeaders(),
		middleware.RequestBodyLimit(),
		middleware.StaticCacheHeaders(),
		server.rateLimiter.Middleware(),
		server.sessions.Load(),
	}
}

func (server *Server) UploadDir() string {
	return server.config.UploadDir
}
