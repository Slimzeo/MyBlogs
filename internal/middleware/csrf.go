package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func ValidateCSRF(manager *SessionManager) gin.HandlerFunc {
	return func(context *gin.Context) {
		if context.Request.Method == http.MethodGet || context.Request.Method == http.MethodHead {
			context.Next()
			return
		}
		token := context.GetHeader("X-CSRF-Token")
		if token == "" {
			token = context.PostForm("_csrf_token")
		}
		if !manager.ValidateCSRFToken(token) {
			context.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"success": false,
				"msg":     "BAD REQUEST",
				"code":    -1,
			})
			return
		}
		context.Next()
	}
}
