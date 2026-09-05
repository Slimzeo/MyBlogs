package handler

import (
	"errors"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"myblog/internal/middleware"
	"myblog/internal/model"
	"myblog/internal/service"
	"myblog/internal/util"

	"github.com/gin-gonic/gin"
)

const (
	apiImportProcessingTTL = 10 * 60
	apiImportResultTTL     = 24 * 60 * 60
)

var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)

func (server *Server) apiAuth(context *gin.Context) {
	context.Header("Cache-Control", "no-store")
	token := middleware.APIToken(context)
	if token == nil {
		respondAPIError(context, http.StatusInternalServerError, "internal_error", "认证上下文不可用")
		return
	}
	respondAPIOK(context, gin.H{
		"name":      token.Name,
		"scope":     token.Scope,
		"expiresAt": token.Expires,
	})
}

func (server *Server) apiImportArticle(context *gin.Context) {
	context.Header("Cache-Control", "no-store")
	token := middleware.APIToken(context)
	if token == nil {
		respondAPIError(context, http.StatusInternalServerError, "internal_error", "认证上下文不可用")
		return
	}
	idempotencyKey := strings.TrimSpace(context.GetHeader("Idempotency-Key"))
	if !idempotencyKeyPattern.MatchString(idempotencyKey) {
		respondAPIError(context, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key 应为8-128位字母、数字或 . _ : -")
		return
	}
	cacheKey := "api_article_import:" + token.TokenID + ":" + util.MD5encode(idempotencyKey)
	if server.replayAPIImport(context, cacheKey) {
		return
	}
	if !server.service.Cache().SetNX(cacheKey, 0, apiImportProcessingTTL) {
		if server.replayAPIImport(context, cacheKey) {
			return
		}
		respondAPIError(context, http.StatusConflict, "import_in_progress", "相同导入请求正在处理，请稍后重试")
		return
	}
	succeeded := false
	defer func() {
		if !succeeded {
			server.service.Cache().Del(cacheKey)
		}
	}()

	if err := context.Request.ParseMultipartForm(int64(16<<20) + (1 << 20)); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			respondAPIError(context, http.StatusRequestEntityTooLarge, "file_too_large", "导入文件不能超过16MB")
		} else {
			respondAPIError(context, http.StatusBadRequest, "invalid_multipart", "无法读取 multipart 导入请求")
		}
		return
	}
	if context.Request.MultipartForm != nil {
		defer context.Request.MultipartForm.RemoveAll()
	}
	header := firstMultipartFile(context, "archive")
	if header == nil {
		respondAPIError(context, http.StatusBadRequest, "missing_archive", "请在 archive 字段中提供 HTML 或 ZIP 文件")
		return
	}
	if header.Size <= 0 || header.Size > 16<<20 {
		respondAPIError(context, http.StatusRequestEntityTooLarge, "file_too_large", "导入文件必须大于0且不能超过16MB")
		return
	}
	extension := strings.ToLower(filepath.Ext(header.Filename))
	if extension != ".html" && extension != ".htm" && extension != ".zip" {
		respondAPIError(context, http.StatusUnprocessableEntity, "unsupported_file", "只支持 HTML 或 ZIP；Markdown 请与资源一起打包为 ZIP")
		return
	}
	file, err := header.Open()
	if err != nil {
		respondAPIError(context, http.StatusBadRequest, "unreadable_archive", "导入文件无法读取")
		return
	}
	data, readErr := io.ReadAll(io.LimitReader(file, (16<<20)+1))
	_ = file.Close()
	if readErr != nil || len(data) == 0 || len(data) > 16<<20 {
		respondAPIError(context, http.StatusRequestEntityTooLarge, "file_too_large", "导入文件读取失败或超过16MB")
		return
	}
	displayTime, err := util.ParseDateTimeLocal(strings.TrimSpace(context.PostForm("display_time")))
	if err != nil {
		respondAPIError(context, http.StatusUnprocessableEntity, "invalid_display_time", "display_time 应为 YYYY-MM-DDTHH:MM")
		return
	}
	options := service.ImportOptions{
		AuthorID:    token.AuthorID,
		Title:       context.PostForm("title"),
		Slug:        context.PostForm("slug"),
		Tags:        context.PostForm("tags"),
		Categories:  context.PostForm("categories"),
		Status:      model.TypeDraft,
		DisplayTime: displayTime,
	}
	var content *model.Content
	if extension == ".html" || extension == ".htm" {
		content, err = server.service.ImportHTMLDocument(data, header.Filename, options)
	} else {
		content, err = server.service.ImportArticleArchive(data, options)
	}
	if err != nil {
		if message, ok := service.AsTip(err); ok {
			respondAPIError(context, http.StatusUnprocessableEntity, "invalid_article", message)
			return
		}
		log.Printf("agent article import failed token_id=%s err=%v", token.TokenID, err)
		respondAPIError(context, http.StatusInternalServerError, "import_failed", "文章导入失败")
		return
	}
	succeeded = true
	server.service.Cache().Set(cacheKey, content.Cid, apiImportResultTTL)
	server.service.InsertLog(model.LogAgentImport, "cid="+strconv.Itoa(content.Cid)+" token_id="+token.TokenID, util.ClientIP(context.Request), token.AuthorID)
	respondAPIImport(context, content, false)
}

func (server *Server) replayAPIImport(context *gin.Context, cacheKey string) bool {
	cid, exists := server.service.Cache().GetInt(cacheKey)
	if !exists {
		return false
	}
	if cid == 0 {
		respondAPIError(context, http.StatusConflict, "import_in_progress", "相同导入请求正在处理，请稍后重试")
		return true
	}
	content, _ := server.service.GetContentByID(strconv.Itoa(cid))
	if content == nil {
		server.service.Cache().Del(cacheKey)
		return false
	}
	respondAPIImport(context, content, true)
	return true
}

func respondAPIImport(context *gin.Context, content *model.Content, replayed bool) {
	respondAPIOK(context, gin.H{
		"id":          content.Cid,
		"title":       content.Title,
		"status":      content.Status,
		"editPath":    "/admin/article/" + strconv.Itoa(content.Cid),
		"previewPath": "/article/" + strconv.Itoa(content.Cid) + "/preview",
		"replayed":    replayed,
	})
}

func respondAPIOK(context *gin.Context, data any) {
	context.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func respondAPIError(context *gin.Context, status int, code, message string) {
	context.JSON(status, gin.H{
		"success": false,
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
