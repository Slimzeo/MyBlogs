package handler

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"myblog/config"
	"myblog/internal/middleware"
	"myblog/internal/model"
	"myblog/internal/notes"
	"myblog/internal/service"

	"github.com/gin-gonic/gin"
)

type Server struct {
	config      *config.Config
	service     *service.Service
	renderer    *Renderer
	siteConfig  *SiteConfig
	sessions    *middleware.SessionManager
	hitCounter  *service.HitCounter
	rateLimiter *middleware.IPLimiter
	notes       *notes.Store
}

func NewServer(config *config.Config, service *service.Service, templateRoot string) (*Server, error) {
	siteConfig := NewSiteConfig(service)
	renderer, err := NewRenderer(templateRoot, siteConfig)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(config.UploadDir, 0o755); err != nil {
		return nil, err
	}
	noteStore, err := notes.NewStore(config.NotesDir)
	if err != nil {
		return nil, err
	}
	return &Server{
		config:      config,
		service:     service,
		renderer:    renderer,
		siteConfig:  siteConfig,
		sessions:    middleware.NewSessionManager(service, config.SessionSecret, config.CookieSecure),
		hitCounter:  service.NewHitCounter(config.HitFlushEvery),
		rateLimiter: middleware.NewIPLimiter(config.RateLimitRPS, config.RateLimitBurst),
		notes:       noteStore,
	}, nil
}

func (server *Server) FlushHits() {
	server.hitCounter.FlushAll()
}

func (server *Server) Close() {
	server.hitCounter.Close()
}

func (server *Server) issueCSRFToken() gin.HandlerFunc {
	return func(context *gin.Context) {
		if context.Request.Method == http.MethodGet {
			context.Set("csrf_token", server.sessions.NewCSRFToken(context.Request.URL.Path))
		}
		context.Next()
	}
}

func (server *Server) render(context *gin.Context, status int, name string, data PageData) {
	loginUser := server.sessions.User(context)
	if data.LoginUser == nil {
		data.LoginUser = loginUser
	}
	if data.CsrfToken == "" {
		if token, ok := context.Get("csrf_token"); ok {
			data.CsrfToken, _ = token.(string)
		}
	}
	accessExpiry := server.sessions.ArticleAccessExpiry(context.Request, server.config.AccessKey)
	data.AccessKeyEnabled = server.config.AccessKey != ""
	data.EncryptedAccess = loginUser != nil || accessExpiry > 0
	data.AccessExpiresAt = accessExpiry
	if data.AccessKeyEnabled {
		context.Header("Cache-Control", "private, no-store")
		context.Header("Vary", "Cookie")
	}
	context.Status(status)
	context.Header("Content-Type", "text/html; charset=utf-8")
	if err := server.renderer.Render(context.Writer, name, data); err != nil {
		log.Printf("render template name=%s path=%s err=%v", name, context.Request.URL.Path, err)
		if !context.Writer.Written() {
			context.String(http.StatusInternalServerError, "template rendering failed")
		}
	}
}

func (server *Server) baseData(context *gin.Context, title, active string) PageData {
	accessExpiry := server.sessions.ArticleAccessExpiry(context.Request, server.config.AccessKey)
	loginUser := server.sessions.User(context)
	return PageData{
		Title:            title,
		Active:           active,
		LoginUser:        loginUser,
		CsrfToken:        csrfToken(context),
		AccessKeyEnabled: server.config.AccessKey != "",
		EncryptedAccess:  loginUser != nil || accessExpiry > 0,
		AccessExpiresAt:  accessExpiry,
	}
}

func (server *Server) canAccessEncrypted(context *gin.Context) bool {
	return server.sessions.User(context) != nil ||
		server.sessions.ArticleAccessExpiry(context.Request, server.config.AccessKey) > 0
}

func (server *Server) health(context *gin.Context) {
	context.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().Unix()})
}

func (server *Server) ready(context *gin.Context) {
	sqlDB, err := server.service.DB().DB()
	if err != nil || sqlDB.PingContext(context.Request.Context()) != nil {
		context.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	context.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func csrfToken(context *gin.Context) string {
	value, exists := context.Get("csrf_token")
	if !exists {
		return ""
	}
	token, _ := value.(string)
	return token
}

func queryInt(context *gin.Context, key string, defaultValue int) int {
	value, err := strconv.Atoi(context.DefaultQuery(key, strconv.Itoa(defaultValue)))
	if err != nil {
		return defaultValue
	}
	return value
}

func pathInt(context *gin.Context, key string, defaultValue int) int {
	value, err := strconv.Atoi(context.Param(key))
	if err != nil {
		return defaultValue
	}
	return value
}

func clampPage(page int) int {
	if page < 1 || page > model.MaxPage {
		return 1
	}
	return page
}

func clampLimit(limit, defaultValue, maximum int) int {
	if limit < 1 || limit > maximum {
		return defaultValue
	}
	return limit
}
