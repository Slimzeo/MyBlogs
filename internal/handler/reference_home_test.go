package handler_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"myblog/config"
	"myblog/internal/cache"
	"myblog/internal/db"
	"myblog/internal/handler"
	"myblog/internal/router"
	"myblog/internal/service"
)

func TestReferenceHomeFallbackAndAssets(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "reference")
	t.Setenv("BLOG_REFERENCE_DIR", root)
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: filepath.Join(directory, "blog.db"),
		DBMaxOpenConns: 1, DBMaxIdleConns: 1, UploadDir: filepath.Join(directory, "upload"),
		NotesDir: filepath.Join(directory, "notes"), SessionSecret: "test-home-secret-01234567890123456789",
		RateLimitRPS: 1000, RateLimitBurst: 1000, HitFlushEvery: 100,
		AdminUsername: "test-admin", AdminEmail: "test@example.com", AdminInitialPassword: "test-home-password"}
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := database.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	applicationCache := cache.New()
	t.Cleanup(applicationCache.Close)
	server, err := handler.NewServer(cfg, service.New(database, applicationCache, cfg), filepath.Join("..", "..", "templates"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	engine := router.New(server, filepath.Join("..", "..", "static"))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "fluid-page-main") {
		t.Fatalf("missing reference must render theme index: status=%d", response.Code)
	}
	for _, entry := range []string{"dist/assets/site", "blog"} {
		if err := os.MkdirAll(filepath.Join(root, entry), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "blog/index.html"), []byte("<html><head><!--BLOG_HOME_DATA--></head><body>reference home</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dist/assets/site/example.js"), []byte("void 0;"), 0o644); err != nil {
		t.Fatal(err)
	}
	engine = router.New(server, filepath.Join("..", "..", "static"))
	adminPath := "/assets/admin/js/base.js"
	if _, err := os.Stat(filepath.Join("..", "..", "static", "admin", "js", "base.js")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/assets/site/example.js", adminPath} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Errorf("GET %s: status=%d", path, response.Code)
		}
		if path == "/" && !strings.Contains(response.Body.String(), "reference home") {
			t.Error("reference template not rendered")
		}
	}
	response = httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/site/", nil))
	if response.Code != http.StatusNotFound {
		t.Errorf("directory listing: status=%d", response.Code)
	}
}
