package handler_test

import (
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// Follow public URLs with actual category/tag relationships. Count and page
// after intersecting both filters and applying visibility, never before.
func TestBlogLibraryReadingPaths(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BLOG_REFERENCE_DIR", filepath.Join(root, "no-scene"))
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: filepath.Join(root, "blog.db"), DBMaxOpenConns: 1, DBMaxIdleConns: 1,
		UploadDir: filepath.Join(root, "uploads"), SessionSecret: "library-local-test-only-1234567890",
		AdminUsername: "test-admin", AdminEmail: "library@example.com", AdminInitialPassword: "test-library-password", RateLimitRPS: 1000, RateLimitBurst: 1000, HitFlushEvery: 100}
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := database.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	c := cache.New()
	t.Cleanup(c.Close)
	svc := service.New(database, c, cfg)
	server, err := handler.NewServer(cfg, svc, filepath.Join("..", "..", "templates"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	engine := router.New(server, filepath.Join("..", "..", "static"))
	const column = "读书 & 工程"
	const tag = "C++"
	stamp := int(time.Now().Unix()) + 3600
	add := func(slug, category, label, status, body string) {
		stamp++
		p := &model.Content{Slug: slug, Title: slug, Content: body, ContentFormat: model.ContentMarkdown, AuthorID: 1, Type: model.TypeArticle, Status: status, DisplayTime: stamp, Categories: category, Tags: label}
		if err := svc.Publish(p); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 13; i++ {
		add(fmt.Sprintf("overlap-%02d", i), column, tag, model.TypePublish, "A shared topic.")
	}
	add("column-alone", column, "设计", model.TypePublish, "Card without an image.")
	add("tag-from-elsewhere", "随笔", tag, model.TypePublish, "![figure](/upload/author-picture.png)\n\nLocal figure.")
	add("remote-cover", "随笔", "摄影", model.TypePublish, "![figure](https://example.net/do-not-hotlink.png)\n\nRemote figure.")
	add("hidden-post", "hidden-column", "hidden-tag", model.TypePrivate, "Private.")
	add("unpublished-post", column, tag, model.TypeDraft, "Draft.")
	get := func(path string, code int) string {
		t.Helper()
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != code {
			t.Fatalf("GET %s: %d, want %d", path, w.Code, code)
		}
		return w.Body.String()
	}
	has := func(body string, want ...string) {
		t.Helper()
		for _, s := range want {
			if !strings.Contains(body, s) {
				t.Fatalf("missing %q", s)
			}
		}
	}
	absent := func(body string, unwanted ...string) {
		t.Helper()
		for _, s := range unwanted {
			if strings.Contains(body, s) {
				t.Fatalf("unexpected %q", s)
			}
		}
	}
	feed := get("/blogs", 200)
	has(feed, `class="journal-card"`, `href="/categories">浏览专栏`, `href="/blogs?page=2"`)
	absent(feed, `class="library-shelves"`)
	dir := get("/categories", 200)
	has(dir, `data-column="读书 &amp; 工程"`, `data-column="随笔"`, "14 篇", "/article/column-alone")
	absent(dir, "hidden-column", "hidden-tag", "hidden-post", "unpublished-post")
	absent(dir, `class="tag-index"`, "A shared topic.")
	tagsPage := get("/tags", 200)
	has(tagsPage, `class="tag-index"`, "C%2B%2B", `data-tag="设计"`)
	absent(tagsPage, `class="column-index"`, "hidden-tag")
	old := httptest.NewRecorder()
	engine.ServeHTTP(old, httptest.NewRequest(http.MethodGet, "/notes?category="+url.QueryEscape(column)+"&tag=C%2B%2B&page=2", nil))
	if old.Code != http.StatusFound || old.Header().Get("Location") != "/categories?"+(url.Values{"category": {column}, "tag": {tag}, "page": {"2"}}).Encode() {
		t.Fatalf("old column/tag/page bookmark: %s", old.Header().Get("Location"))
	}
	archive := get("/archives", 200)
	has(archive, `class="dated-year"`, `rel="next" href="/archives?page=2"`, "tag-from-elsewhere")
	absent(archive, "hidden-post", "unpublished-post", "A shared topic.", `class="column-index"`)
	secondArchivePage := get("/archives?page=2", 200)
	has(secondArchivePage, "overlap-01", `rel="prev" href="/archives"`)
	absent(secondArchivePage, "tag-from-elsewhere")

	scope := "/categories?" + url.Values{"category": {column}, "tag": {tag}}.Encode()
	both := get(scope, 200)
	has(both, "13 篇文章", `href="/article/overlap-13"`, "在所有专栏查看")
	has(html.UnescapeString(both), "/categories?"+url.Values{"category": {column}, "tag": {tag}, "page": {"2"}}.Encode())
	absent(both, `href="/article/overlap-01"`, "tag-from-elsewhere", "column-alone", "unpublished-post")
	if count := strings.Count(both, `class="dated-link"`); count != 12 {
		t.Fatalf("intersection page 1 has %d results", count)
	}
	second := get(scope+"&page=2", 200)
	has(second, `href="/article/overlap-01"`, "13 篇文章")
	absent(second, `href="/article/overlap-13"`)
	if strings.Count(second, `class="dated-link"`) != 1 {
		t.Fatal("bad intersection page 2")
	}
	get(scope+"&page=3", 404)
	global := get("/tags?tag=C%2B%2B", 200)
	has(global, "14 篇文章", "tag-from-elsewhere")
	absent(global, "column-alone", "unpublished-post")
	columnOnly := get("/categories?category="+url.QueryEscape(column), 200)
	has(columnOnly, "14 篇文章", "column-alone")
	absent(columnOnly, "tag-from-elsewhere")
	empty := get("/categories?category="+url.QueryEscape(column)+"&tag="+url.QueryEscape("摄影"), 200)
	has(html.UnescapeString(empty), "没有符合当前条件的文章", "/categories?category="+url.QueryEscape(column))
	home := get("/", 200)
	has(home, `class="journal-card"`, `src="/upload/author-picture.png"`, `href="/tags?tag=C%2B%2B"`)
	absent(home, "do-not-hotlink.png", "hidden-post", "unpublished-post")
	if strings.Count(home, `class="journal-thumb"`) != 1 {
		t.Fatal("only the local cover should take up space")
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/topics?view=tags", nil))
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/tags" {
		t.Fatal("old tag bookmark lost its destination")
	}
	// Reassign a single column without changing its tags. Invalid edits must
	// leave the previous column and tag edges intact.
	entry := &model.Content{Slug: "three-tags", Title: "three-tags", AuthorID: 1, Type: model.TypeArticle, Status: model.TypePublish, Content: "Full entry", Tags: "C++,记忆,随笔", Categories: "旧专栏"}
	if err := svc.Publish(entry); err != nil {
		t.Fatal(err)
	}
	full := get("/article/three-tags", 200)
	has(full, "旧专栏", `id="article-tags"`, "记忆", "随笔", `/tags?tag=C%2B%2B`)
	entry.Categories = "旧专栏,新专栏"
	if err := svc.UpdateArticle(entry); err == nil {
		t.Fatal("multiple columns accepted on edit")
	}
	has(get("/categories?category="+url.QueryEscape("旧专栏"), 200), "three-tags")
	entry.Categories = "新专栏"
	if err := svc.UpdateArticle(entry); err != nil {
		t.Fatal(err)
	}
	absent(get("/categories?category="+url.QueryEscape("旧专栏"), 200), "/article/three-tags")
	current := get("/categories?category="+url.QueryEscape("新专栏")+"&tag=C%2B%2B", 200)
	has(current, `href="/article/three-tags"`)
	col := svc.GetMeta(model.TypeCategory, "新专栏")
	if col == nil {
		t.Fatal("missing assigned column")
	}
	if err := svc.DeleteMeta(col.Mid); err != nil {
		t.Fatal(err)
	}
	fallback := get("/categories?category="+url.QueryEscape(model.DefaultColumn)+"&tag=C%2B%2B", 200)
	has(fallback, `href="/article/three-tags"`)
	has(get("/article/three-tags", 200), "记忆", "随笔")
	// The archive's year boundary follows the blog's display timezone.
	stamp = int(time.Date(2023, 12, 31, 15, 59, 58, 0, time.UTC).Unix())
	add("last-of-2023", column, "", model.TypePublish, "December in Shanghai")
	add("first-of-2024", column, "", model.TypePublish, "January in Shanghai")
	archive = get("/archives?limit=100", 200)
	y2024 := strings.Index(archive, `class="dated-year">2024</h2>`)
	y2023 := strings.Index(archive, `class="dated-year">2023</h2>`)
	newYear := strings.Index(archive, `href="/article/first-of-2024"`)
	lastYear := strings.Index(archive, `href="/article/last-of-2023"`)
	if y2024 < 0 || !(y2024 < newYear && newYear < y2023 && y2023 < lastYear) {
		t.Fatal("archive titles are not grouped by the local display year")
	}

}
