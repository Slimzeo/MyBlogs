package handler_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"myblog/config"
	"myblog/internal/cache"
	"myblog/internal/db"
	"myblog/internal/handler"
	"myblog/internal/model"
	"myblog/internal/router"
	"myblog/internal/service"
)

func TestReferenceHomeFallbackAndAssets(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "reference")
	t.Setenv("BLOG_REFERENCE_DIR", root)
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: filepath.Join(directory, "blog.db"),
		DBMaxOpenConns: 1, DBMaxIdleConns: 1, UploadDir: filepath.Join(directory, "upload"),
		SessionSecret: "test-home-secret-01234567890123456789",
		RateLimitRPS:  1000, RateLimitBurst: 1000, HitFlushEvery: 100,
		AdminUsername: "test-admin", AdminEmail: "test@example.com", AdminInitialPassword: "test-home-password"}
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := database.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	// A single run exercises both shells against the same journal. Give the
	// entries display dates later than the seed welcome post.
	stamp := int(time.Now().Unix()) + 3600
	for i, entry := range []struct{ slug, title, status string }{
		{"old-reading", "Earlier reading", model.TypePublish},
		{"lab-note", "Lab note", model.TypePublish},
		{"daily-log", "Newer diary", model.TypePublish},
		{"latest-diary", "Latest <diary>", model.TypePublish},
		{"private-diary", "never-list-private", model.TypePrivate},
		{"draft-diary", "never-list-draft", model.TypeDraft},
		{"locked-diary", "never-list-locked", model.TypeEncrypted},
	} {
		post := model.Content{Slug: entry.slug, Title: entry.title, Status: entry.status,
			Type: model.TypeArticle, ContentFormat: "markdown", DisplayTime: stamp + i,
			Content: "Visible excerpt.<!--more--> Long body only on the article page."}
		if err := database.Create(&post).Error; err != nil {
			t.Fatal(err)
		}
	}
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
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "fluid-page-main") || !strings.Contains(response.Body.String(), `id="blogs"`) {
		t.Fatalf("missing reference must render theme index: status=%d", response.Code)
	}
	for _, entry := range []string{"dist/assets/site", "blog"} {
		if err := os.MkdirAll(filepath.Join(root, entry), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for file, label := range map[string]string{"index.html": "intro-marker", "journal.html": "list-only-marker"} {
		shell := "<html><head></head><body>" + label + "<!--BLOG_TOPBAR--><!--BLOG_DIALOGS--><!--BLOG_FEED--></body></html>"
		if err := os.WriteFile(filepath.Join(root, "blog", file), []byte(shell), 0o644); err != nil {
			t.Fatal(err)
		}
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
	}
	for _, test := range []struct {
		path         string
		want, absent []string
	}{
		{"/?limit=2", []string{"intro-marker", "Latest &lt;diary&gt;", "Newer diary", `href="/page/2?limit=2#blogs"`}, []string{"Lab note", "Earlier reading"}},
		{"/page/2?limit=2", []string{"list-only-marker", "Lab note", "Earlier reading", `href="/?limit=2#blogs"`}, []string{"intro-marker", "Latest &lt;diary&gt;", "Newer diary"}},
	} {
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		body := response.Body.String()
		if response.Code != http.StatusOK || strings.Count(body, `class="journal-item"`) != 2 {
			t.Fatalf("GET %s did not return one server-rendered page of two entries", test.path)
		}
		for _, yes := range append(test.want, "Visible excerpt.") {
			if !strings.Contains(body, yes) {
				t.Errorf("%s missing %q", test.path, yes)
			}
		}
		for _, no := range append(test.absent, "never-list-", "Long body only on the article page.", "<!--BLOG_FEED-->") {
			if strings.Contains(body, no) {
				t.Errorf("%s unexpectedly contains %q", test.path, no)
			}
		}
	}
	response = httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/site/", nil))
	if response.Code != http.StatusNotFound {
		t.Errorf("directory listing: status=%d", response.Code)
	}
}
