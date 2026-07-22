package middleware

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"myblog/internal/model"
	"myblog/internal/util"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen atomic.Int64
}

// IPLimiter bounds abusive clients without imposing a global request-path lock.
type IPLimiter struct {
	clients    sync.Map
	requests   rate.Limit
	burst      int
	lastPruned atomic.Int64
}

func NewIPLimiter(requestsPerSecond, burst int) *IPLimiter {
	limiter := &IPLimiter{requests: rate.Limit(requestsPerSecond), burst: burst}
	limiter.lastPruned.Store(time.Now().Unix())
	return limiter
}

func (limiter *IPLimiter) Middleware() gin.HandlerFunc {
	return func(context *gin.Context) {
		if limiter.requests <= 0 {
			context.Next()
			return
		}
		if !limiter.allow(util.ClientIP(context.Request)) {
			context.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"msg":     "请求过于频繁，请稍后再试",
			})
			return
		}
		context.Next()
	}
}

func (limiter *IPLimiter) allow(clientIP string) bool {
	now := time.Now()
	value, _ := limiter.clients.LoadOrStore(clientIP, &clientLimiter{
		limiter: rate.NewLimiter(limiter.requests, limiter.burst),
	})
	client := value.(*clientLimiter)
	client.lastSeen.Store(now.Unix())
	lastPruned := limiter.lastPruned.Load()
	if now.Unix()-lastPruned >= 60 && limiter.lastPruned.CompareAndSwap(lastPruned, now.Unix()) {
		limiter.clients.Range(func(key, value any) bool {
			candidate := value.(*clientLimiter)
			if now.Unix()-candidate.lastSeen.Load() > 600 {
				limiter.clients.Delete(key)
			}
			return true
		})
	}
	return client.limiter.Allow()
}

func SecurityHeaders() gin.HandlerFunc {
	return func(context *gin.Context) {
		header := context.Writer.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "SAMEORIGIN")
		header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		header.Set("Content-Security-Policy",
			"default-src 'self'; base-uri 'self'; form-action 'self'; "+
				"img-src 'self' data: https:; "+
				"media-src 'self' https:; "+
				"style-src 'self' 'unsafe-inline' https:; "+
				"font-src 'self' https: data:; "+
				"script-src 'self' 'unsafe-inline' https:; "+
				"connect-src 'self' https:; frame-ancestors 'self'")
		context.Next()
	}
}

func RequestLogger() gin.HandlerFunc {
	return func(context *gin.Context) {
		started := time.Now()
		context.Next()
		elapsed := time.Since(started)
		if context.Writer.Status() >= http.StatusInternalServerError || elapsed >= time.Second {
			log.Printf(
				"http method=%s path=%s status=%d latency=%s client_ip=%s",
				context.Request.Method,
				context.Request.URL.Path,
				context.Writer.Status(),
				elapsed,
				util.ClientIP(context.Request),
			)
		}
	}
}

func RequestBodyLimit() gin.HandlerFunc {
	return func(context *gin.Context) {
		if context.Request.Body == nil {
			context.Next()
			return
		}
		limit := int64(2 << 20)
		if strings.HasPrefix(context.Request.URL.Path, "/admin/attach/upload") {
			limit = int64(model.MaxFileSize*16) + (1 << 20)
		}
		if strings.HasPrefix(context.Request.URL.Path, "/admin/article/import") {
			limit = int64(16<<20) + (1 << 20)
		}
		context.Request.Body = http.MaxBytesReader(context.Writer, context.Request.Body, limit)
		context.Next()
	}
}

// StaticCacheHeaders lets browsers reuse versionless public assets briefly
// without caching dynamic HTML or admin responses.
func StaticCacheHeaders() gin.HandlerFunc {
	return func(context *gin.Context) {
		path := context.Request.URL.Path
		if context.Request.Method == http.MethodGet || context.Request.Method == http.MethodHead {
			if strings.HasPrefix(path, "/user/") || strings.HasPrefix(path, "/assets/admin/") {
				context.Header("Cache-Control", "public, max-age=3600, stale-while-revalidate=86400")
			} else if strings.HasPrefix(path, "/upload/") {
				context.Header("Cache-Control", "private, max-age=300")
			}
		}
		context.Next()
	}
}
