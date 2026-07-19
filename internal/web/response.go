package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RestResponse mirrors the Java RestResponseBo JSON contract.
type RestResponse struct {
	Payload   any    `json:"payload,omitempty"`
	Success   bool   `json:"success"`
	Msg       string `json:"msg,omitempty"`
	Code      int    `json:"code"`
	Timestamp int64  `json:"timestamp"`
}

func respondOK(context *gin.Context, payload ...any) {
	response := RestResponse{Success: true, Code: -1, Timestamp: time.Now().Unix()}
	if len(payload) > 0 {
		response.Payload = payload[0]
	}
	context.JSON(http.StatusOK, response)
}

func respondFail(context *gin.Context, message string) {
	context.JSON(http.StatusOK, RestResponse{
		Success:   false,
		Msg:       message,
		Code:      -1,
		Timestamp: time.Now().Unix(),
	})
}
