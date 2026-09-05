package handler_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"myblog/config"
	"myblog/internal/cache"
	"myblog/internal/db"
	"myblog/internal/handler"
	"myblog/internal/model"
	"myblog/internal/router"
	"myblog/internal/service"

	"github.com/gin-gonic/gin"
)

func TestAgentAPIImportsDraftIdempotentlyAndRevokesToken(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	directory := t.TempDir()
	notesDirectory := filepath.Join(directory, "notes")
	if err := os.MkdirAll(notesDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	runtimeConfig := &config.Config{
		DBDriver:             "sqlite",
		DBDSN:                filepath.Join(directory, "blog.db"),
		DBMaxOpenConns:       5,
		DBMaxIdleConns:       2,
		DBConnMaxLifetime:    time.Minute,
		SessionSecret:        "agent-api-test-secret-0123456789abcdef",
		UploadDir:            filepath.Join(directory, "upload"),
		NotesDir:             notesDirectory,
		HitFlushEvery:        100,
		RateLimitRPS:         1000,
		RateLimitBurst:       2000,
		AdminUsername:        "agent-admin",
		AdminEmail:           "agent@example.com",
		AdminInitialPassword: "temporary-test-password",
	}
	database, err := db.Open(runtimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	applicationCache := cache.New()
	services := service.New(database, applicationCache, runtimeConfig)
	server, err := handler.NewServer(runtimeConfig, services, filepath.Join("..", "..", "templates"))
	if err != nil {
		t.Fatal(err)
	}
	testServer := httptest.NewServer(router.New(server, filepath.Join("..", "..", "static")))
	t.Cleanup(func() {
		testServer.Close()
		server.Close()
		applicationCache.Close()
		sqlDB, _ := database.DB()
		_ = sqlDB.Close()
	})

	unauthorized, err := http.Get(testServer.URL + "/api/v1/auth")
	if err != nil {
		t.Fatal(err)
	}
	_ = unauthorized.Body.Close()
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.StatusCode)
	}

	user := services.QueryUserByID(1)
	record, plaintext, err := services.CreateAPIToken("Codex local", model.ScopeArticleImport, user.Uid, 30)
	if err != nil {
		t.Fatal(err)
	}
	authRequest, _ := http.NewRequest(http.MethodGet, testServer.URL+"/api/v1/auth", nil)
	authRequest.Header.Set("Authorization", "Bearer "+plaintext)
	authResponse, err := http.DefaultClient.Do(authRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = authResponse.Body.Close()
	if authResponse.StatusCode != http.StatusOK {
		t.Fatalf("auth status = %d", authResponse.StatusCode)
	}

	archive := buildTestImportArchive(t, map[string]string{
		"draft.md":        "# Imported by agent\n\n![图](assets/tiny.png)",
		"assets/tiny.png": string([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}),
	})
	fields := url.Values{
		"title":        {"CLI Override"},
		"slug":         {"cli-agent-import"},
		"tags":         {"Agent,CLI"},
		"categories":   {"Tools"},
		"display_time": {"2026-09-05T21:30"},
		"status":       {"publish"},
	}
	first := postAgentImport(t, testServer.URL, plaintext, "same-import-request", archive, fields)
	if first.Status != http.StatusOK || !first.Success || first.Data.ID <= 0 || first.Data.Replayed {
		t.Fatalf("first import = %#v", first)
	}
	imported, err := services.GetContentByID(strconv.Itoa(first.Data.ID))
	if err != nil {
		t.Fatal(err)
	}
	if imported == nil || imported.Status != model.TypeDraft || imported.Title != "CLI Override" || imported.Slug != "cli-agent-import" {
		t.Fatalf("imported article = %#v", imported)
	}
	second := postAgentImport(t, testServer.URL, plaintext, "same-import-request", archive, fields)
	if second.Status != http.StatusOK || !second.Success || !second.Data.Replayed || second.Data.ID != first.Data.ID {
		t.Fatalf("replayed import = %#v", second)
	}
	var count int64
	if err := database.Model(&model.Content{}).Where("title = ?", "CLI Override").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("idempotent import count = %d", count)
	}

	if err := services.RevokeAPIToken(record.ID, user.Uid); err != nil {
		t.Fatal(err)
	}
	revokedRequest, _ := http.NewRequest(http.MethodGet, testServer.URL+"/api/v1/auth", nil)
	revokedRequest.Header.Set("Authorization", "Bearer "+plaintext)
	revokedResponse, err := http.DefaultClient.Do(revokedRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = revokedResponse.Body.Close()
	if revokedResponse.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked auth status = %d", revokedResponse.StatusCode)
	}
}

type agentImportResult struct {
	Status  int
	Success bool `json:"success"`
	Data    struct {
		ID       int  `json:"id"`
		Replayed bool `json:"replayed"`
	} `json:"data"`
}

func postAgentImport(t *testing.T, serverURL, token, idempotencyKey string, archive []byte, fields url.Values) agentImportResult {
	t.Helper()
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	part, err := writer.CreateFormFile("archive", "article.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(archive); err != nil {
		t.Fatal(err)
	}
	for name, values := range fields {
		for _, value := range values {
			if err := writer.WriteField(name, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, serverURL+"/api/v1/articles/import", &requestBody)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Idempotency-Key", idempotencyKey)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	result := agentImportResult{Status: response.StatusCode}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}
