package middleware

import (
	"errors"
	"net/http"
	"strings"

	"myblog/internal/model"
	"myblog/internal/service"

	"github.com/gin-gonic/gin"
)

const apiTokenContextKey = "api_token"

func RequireAPIToken(services *service.Service, requiredScope string) gin.HandlerFunc {
	return func(context *gin.Context) {
		header := strings.TrimSpace(context.GetHeader("Authorization"))
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			abortAPITokenRequest(context, http.StatusUnauthorized, "unauthorized", "请提供有效的 Bearer Token")
			return
		}
		token, err := services.AuthenticateAPIToken(parts[1], requiredScope)
		if errors.Is(err, service.ErrAPITokenForbidden) {
			abortAPITokenRequest(context, http.StatusForbidden, "forbidden", "密钥没有所需权限")
			return
		}
		if err != nil {
			abortAPITokenRequest(context, http.StatusUnauthorized, "unauthorized", "Bearer Token 无效、过期或已撤销")
			return
		}
		context.Set(apiTokenContextKey, token)
		context.Next()
	}
}

func APIToken(context *gin.Context) *model.APIToken {
	value, exists := context.Get(apiTokenContextKey)
	if !exists {
		return nil
	}
	token, _ := value.(*model.APIToken)
	return token
}

func abortAPITokenRequest(context *gin.Context, status int, code, message string) {
	context.Header("WWW-Authenticate", `Bearer realm="myblogs"`)
	context.AbortWithStatusJSON(status, gin.H{
		"success": false,
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
