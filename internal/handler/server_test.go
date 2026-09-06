package handler_test

import (
	"archive/zip"
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"myblog/config"
	"myblog/internal/cache"
	"myblog/internal/db"
	"myblog/internal/handler"
	"myblog/internal/model"
	"myblog/internal/router"
	"myblog/internal/service"
	"myblog/internal/util"

	"github.com/gin-gonic/gin"
)

var csrfPattern = regexp.MustCompile(`(?:name="_csrf_token" value="|name="csrf-token" content=")([^"]+)`)

func TestPublicAdminAndConcurrentArticleFlow(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	tempDirectory := t.TempDir()
	testUsername := "test-admin"
	testPassword := randomTestPassword(t)
	notesDirectory := filepath.Join(tempDirectory, "notes")
	if err := os.MkdirAll(filepath.Join(notesDirectory, "Go", "Concurrency"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notesDirectory, "README.md"), []byte("# Notes\n\nRoot notes."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(notesDirectory, "Go", "README.md"), []byte("# Go\n\nGo notes."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(notesDirectory, "Go", "Concurrency", "goroutines.md"),
		[]byte("# Goroutines\n\nA note about goroutines."),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	runtimeConfig := &config.Config{
		DBDriver:             "sqlite",
		DBDSN:                filepath.Join(tempDirectory, "blog.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)",
		DBMaxOpenConns:       20,
		DBMaxIdleConns:       10,
		DBConnMaxLifetime:    30 * time.Minute,
		SessionSecret:        "integration-test-secret-0123456789abcdef",
		AccessKey:            "integration-reader-key",
		UploadDir:            filepath.Join(tempDirectory, "upload"),
		HitFlushEvery:        100,
		RateLimitRPS:         100_000,
		RateLimitBurst:       200_000,
		AdminUsername:        testUsername,
		AdminEmail:           "test@example.com",
		AdminInitialPassword: testPassword,
		NotesDir:             notesDirectory,
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

	for _, path := range []string{"/", "/healthz", "/readyz", "/article/welcome", "/article/welcome.html", "/topics", "/notes", "/notes/Go", "/notes/Go/Concurrency/goroutines", "/archives", "/links", "/about"} {
		response, requestErr := http.Get(testServer.URL + path)
		if requestErr != nil {
			t.Fatalf("GET %s: %v", path, requestErr)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", path, response.StatusCode)
		}
	}
	homeResponse, err := http.Get(testServer.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	homeHTML, err := io.ReadAll(homeResponse.Body)
	_ = homeResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if value := homeResponse.Header.Get("Content-Security-Policy"); value == "" {
		t.Fatal("home page is missing Content-Security-Policy")
	}
	if value := homeResponse.Header.Get("Cache-Control"); value != "private, no-store" {
		t.Fatalf("access-controlled home cache control = %q, want private, no-store", value)
	}
	if value := homeResponse.Header.Get("Vary"); !strings.Contains(value, "Cookie") {
		t.Fatalf("access-controlled home vary = %q, want Cookie", value)
	}
	staticResponse, err := http.Get(testServer.URL + "/user/css/fluid.css")
	if err != nil {
		t.Fatal(err)
	}
	_ = staticResponse.Body.Close()
	if value := staticResponse.Header.Get("Cache-Control"); value == "" {
		t.Fatal("public static asset is missing Cache-Control")
	}
	for _, marker := range []string{
		`href="/user/css/fluid.css?v=5"`,
		`href="/user/css/markdown.css"`,
		`lxgw-wenkai-webfont@1.7.0/lxgwwenkai-regular.css`,
		`lxgw-wenkai-webfont@1.7.0/lxgwwenkai-bold.css`,
		`class="fluid-theme fluid-font-wenkai"`,
		`class="fluid-banner fluid-banner-home"`,
		`rel="preload" as="image"`,
		`class="fluid-banner-image fluid-banner-image-priority"`,
		`src="/user/img/forest.jpg"`,
		`fluid-home-quote`,
		`如果这个`,
		`fluid-quote-space`,
		`是注定的，<br/>`,
		`<strong><em>最重要的</em></strong>`,
		`fluid-leaf-canvas`,
		`class="fluid-board fluid-index-board"`,
		`id="color-toggle"`,
		`id="access-key-toggle"`,
		`id="access-key-modal"`,
		`导入访问密钥`,
		`var leafPalette = [`,
		`var spawnDepth = Math.min(120, viewportHeight * 0.16);`,
		`document.addEventListener('DOMContentLoaded', mountPage, {once: true});`,
		`>Blogs</a>`,
		`>Archieve</a>`,
		`>Friends</a>`,
		`>About</a>`,
	} {
		if !strings.Contains(string(homeHTML), marker) {
			t.Fatalf("home page missing UI marker %q", marker)
		}
	}
	if !strings.Contains(string(homeHTML), "fluid-index-card-no-image") {
		t.Fatal("home page is missing image-less article card")
	}
	if strings.Contains(string(homeHTML), "highlight.js/9.9.0/styles/xcode.min.css") {
		t.Fatal("home page should not load article highlight styles")
	}
	notesResponse, err := http.Get(testServer.URL + "/notes")
	if err != nil {
		t.Fatal(err)
	}
	notesHTML, err := io.ReadAll(notesResponse.Body)
	_ = notesResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		`href="/notes/Go"`,
		`href="/topics"`,
		`fluid-notes-explorer`,
		`fluid-notes-tabs`,
	} {
		if !strings.Contains(string(notesHTML), marker) {
			t.Fatalf("notes page missing UI marker %q", marker)
		}
	}
	noteDocumentResponse, err := http.Get(testServer.URL + "/notes/Go/Concurrency/goroutines")
	if err != nil {
		t.Fatal(err)
	}
	noteDocumentHTML, err := io.ReadAll(noteDocumentResponse.Body)
	_ = noteDocumentResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(noteDocumentHTML), "A note about goroutines.") {
		t.Fatal("notes markdown document was not rendered")
	}
	unsafeNotesResponse, err := http.Get(testServer.URL + "/notes/../.env")
	if err != nil {
		t.Fatal(err)
	}
	_ = unsafeNotesResponse.Body.Close()
	if unsafeNotesResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("unsafe notes path status = %d, want 404", unsafeNotesResponse.StatusCode)
	}
	aboutResponse, err := http.Get(testServer.URL + "/about")
	if err != nil {
		t.Fatal(err)
	}
	aboutHTML, err := io.ReadAll(aboutResponse.Body)
	_ = aboutResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		"Hi, 这里是Hypnos",
		`class="fluid-about-snapshot"`,
		`class="fluid-about-screenshot"`,
		`https://github.com/Slimzeo`,
		`src="/upload/about/github-profile.png"`,
		"努力看论文，学习看源码，成为 Agent 大王",
	} {
		if !strings.Contains(string(aboutHTML), marker) {
			t.Fatalf("about page is missing profile marker %q", marker)
		}
	}
	aboutDocument := string(aboutHTML)
	screenshotPosition := strings.Index(aboutDocument, `class="fluid-about-screenshot"`)
	introPosition := strings.Index(aboutDocument, "Hi, 这里是Hypnos")
	mottoPosition := strings.Index(aboutDocument, "努力看论文，学习看源码，成为 Agent 大王")
	if screenshotPosition < 0 || introPosition < screenshotPosition || mottoPosition < introPosition {
		t.Fatal("about page order must be GitHub screenshot, introduction, then motto")
	}
	invalidLoginResponse := postLogin(t, testServer.URL, "wrong-user", "wrong-password")
	if invalidLoginResponse.Msg != "用户名或密码错误" {
		t.Fatalf("invalid login message = %q, want generic credential error", invalidLoginResponse.Msg)
	}

	topicsResponse, err := http.Get(testServer.URL + "/topics")
	if err != nil {
		t.Fatal(err)
	}
	topicsHTML, err := io.ReadAll(topicsResponse.Body)
	_ = topicsResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		`class="fluid-topic-tabs"`,
		`href="/topics?view=categories"`,
		`href="/topics?view=tags"`,
		`href="/archives"`,
	} {
		if !strings.Contains(string(topicsHTML), marker) {
			t.Fatalf("topics page missing UI marker %q", marker)
		}
	}

	articleUIResponse, err := http.Get(testServer.URL + "/article/welcome")
	if err != nil {
		t.Fatal(err)
	}
	articleHTML, err := io.ReadAll(articleUIResponse.Body)
	_ = articleUIResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		`class="fluid-banner fluid-banner-post"`,
		`class="fluid-banner-image"`,
		`src="/user/img/forest.jpg"`,
		`class="fluid-post-layout"`,
		`class="fluid-board fluid-post-board"`,
		`id="article-toc"`,
		`highlight.js/9.9.0/styles/xcode.min.css`,
	} {
		if !strings.Contains(string(articleHTML), marker) {
			t.Fatalf("article page missing UI marker %q", marker)
		}
	}

	unauthenticatedClient := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	for _, path := range []string{"/admin/index", "/article/welcome/preview"} {
		protectedResponse, requestErr := unauthenticatedClient.Get(testServer.URL + path)
		if requestErr != nil {
			t.Fatalf("GET %s: %v", path, requestErr)
		}
		_ = protectedResponse.Body.Close()
		if protectedResponse.StatusCode != http.StatusFound ||
			protectedResponse.Header.Get("Location") != "/admin/login" {
			t.Fatalf(
				"GET %s status/location = %d/%q, want 302/%q",
				path,
				protectedResponse.StatusCode,
				protectedResponse.Header.Get("Location"),
				"/admin/login",
			)
		}
	}

	client := authenticatedClient(t, testServer.URL, testUsername, testPassword)
	settingResponse, err := client.Get(testServer.URL + "/admin/setting")
	if err != nil {
		t.Fatal(err)
	}
	settingHTML, err := io.ReadAll(settingResponse.Body)
	_ = settingResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		"前台模块",
		`name="module_notes_enabled"`,
		`name="module_archives_enabled"`,
		`name="module_links_enabled"`,
		`name="module_about_enabled"`,
		`name="about_motto"`,
	} {
		if !strings.Contains(string(settingHTML), marker) {
			t.Fatalf("setting page is missing module marker %q", marker)
		}
	}
	settingsHidden := postAdminForm(t, client, testServer.URL, "/admin/setting", "/admin/setting", url.Values{
		"module_notes_enabled":    {"0"},
		"module_archives_enabled": {"0"},
		"module_links_enabled":    {"0"},
		"module_about_enabled":    {"0"},
		"about_motto":             {"努力看论文，学习看源码，成为 Agent 大王"},
	})
	if !settingsHidden.Success {
		t.Fatalf("hiding public modules failed: %s", settingsHidden.Msg)
	}
	for _, path := range []string{"/notes", "/archives", "/links", "/about"} {
		hiddenResponse, requestErr := http.Get(testServer.URL + path)
		if requestErr != nil {
			t.Fatalf("GET hidden module %s: %v", path, requestErr)
		}
		_ = hiddenResponse.Body.Close()
		if hiddenResponse.StatusCode != http.StatusNotFound {
			t.Fatalf("hidden module %s status = %d, want 404", path, hiddenResponse.StatusCode)
		}
	}
	hiddenHomeResponse, err := http.Get(testServer.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	hiddenHomeHTML, err := io.ReadAll(hiddenHomeResponse.Body)
	_ = hiddenHomeResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, hiddenLink := range []string{`href="/notes"`, `href="/archives"`, `href="/links"`, `href="/about"`} {
		if strings.Contains(string(hiddenHomeHTML), hiddenLink) {
			t.Fatalf("hidden module link remains in navigation: %s", hiddenLink)
		}
	}
	settingsRestored := postAdminForm(t, client, testServer.URL, "/admin/setting", "/admin/setting", url.Values{
		"module_notes_enabled":    {"1"},
		"module_archives_enabled": {"1"},
		"module_links_enabled":    {"1"},
		"module_about_enabled":    {"1"},
	})
	if !settingsRestored.Success {
		t.Fatalf("restoring public modules failed: %s", settingsRestored.Msg)
	}
	response, err := client.Get(testServer.URL + "/admin/index")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		t.Fatalf("admin status = %d, want 200", response.StatusCode)
	}
	adminHTML, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, leakedValue := range []string{"默认账号", "默认密码"} {
		if strings.Contains(string(adminHTML), leakedValue) {
			t.Fatalf("admin page leaks credential text %q", leakedValue)
		}
	}
	articleEditorResponse, err := client.Get(testServer.URL + "/admin/article/publish")
	if err != nil {
		t.Fatal(err)
	}
	articleEditorHTML, err := io.ReadAll(articleEditorResponse.Body)
	_ = articleEditorResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		`class="article-editor-grid"`,
		`id="article-preview"`,
		`class="article-editor-preview fluid-markdown"`,
		`/user/css/markdown.css`,
		`syncPreviewScroll`,
		`previewBlockLines`,
		`buildSourceMap`,
		`insertEditorLineBreak`,
		`shiftKey`,
		`Enter 换段，Shift+Enter 半换行`,
		`id="markdown-toolbar"`,
		`id="content-format"`,
		`value="html"`,
		`id="html-file-input"`,
		`id="display-time"`,
		`/admin/article/preview`,
		`/admin/article/image`,
		`id="open-import"`,
		`id="import-form"`,
		`id="archive-file"`,
		`id="archive-file-name"`,
		`article-import-file-picker`,
		`data-meta-target="article-tags"`,
		`data-meta-target="article-categories"`,
		`data-meta-value="Go"`,
		`data-meta-value="默认分类"`,
		`value="encrypted"`,
		`/admin/article/import`,
		`clipboardData`,
		`data-action="image"`,
	} {
		if !strings.Contains(string(articleEditorHTML), marker) {
			t.Fatalf("article editor missing marker %q", marker)
		}
	}
	articleListResponse, err := client.Get(testServer.URL + "/admin/article")
	if err != nil {
		t.Fatal(err)
	}
	articleListHTML, err := io.ReadAll(articleListResponse.Body)
	_ = articleListResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		`class="article-import-trigger"`,
		`class="article-import-file-picker"`,
		`id="archive-file"`,
		`id="archive-file-name"`,
		`data-meta-target="import-tags"`,
		`data-meta-target="import-categories"`,
		`data-meta-value="Go"`,
		`data-meta-value="默认分类"`,
		`/admin/article/import`,
	} {
		if !strings.Contains(string(articleListHTML), marker) {
			t.Fatalf("article list missing shared import marker %q", marker)
		}
	}
	previewResult := postAdminForm(t, client, testServer.URL, "/admin/article/preview", "/admin/article/publish", url.Values{
		"content": {"# 一、起因\n\nmentor叫我写三个文档：\n\n- 项目复盘文档\n- 团队agent框架复盘文档\n- agent评测体系调研文档\n\n普通段落第一行\n普通段落第二行\n\n普通文字\n---\n后续文字"},
	})
	if !previewResult.Success {
		t.Fatalf("markdown preview failed: %s", previewResult.Msg)
	}
	previewPayload, ok := previewResult.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("markdown preview payload type = %T", previewResult.Payload)
	}
	previewHTML, ok := previewPayload["html"].(string)
	if !ok {
		t.Fatalf("markdown preview html type = %T", previewPayload["html"])
	}
	for _, marker := range []string{"<h1>一、起因</h1>", "<li>项目复盘文档</li>", "<hr>"} {
		if !strings.Contains(previewHTML, marker) {
			t.Fatalf("markdown preview missing marker %q: %s", marker, previewHTML)
		}
	}
	if !strings.Contains(previewHTML, "普通段落第一行<br>") {
		t.Fatalf("markdown soft line break was not rendered: %s", previewHTML)
	}
	if strings.Contains(previewHTML, "<h2>") {
		t.Fatalf("markdown preview unexpectedly parsed setext heading: %s", previewHTML)
	}
	blockLines, ok := previewPayload["blockLines"].([]interface{})
	if !ok || len(blockLines) < 4 {
		t.Fatalf("markdown preview block line map is missing or too short: %#v", previewPayload["blockLines"])
	}
	longContent := strings.TrimSuffix(strings.Repeat("长文段落第一行\n长文段落第二行\n\n", 2500), "\n\n")
	longPreviewResult := postAdminForm(t, client, testServer.URL, "/admin/article/preview", "/admin/article/publish", url.Values{
		"content": {longContent},
	})
	if !longPreviewResult.Success {
		t.Fatalf("long markdown preview failed: %s", longPreviewResult.Msg)
	}
	longPreviewPayload, ok := longPreviewResult.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("long markdown preview payload type = %T", longPreviewResult.Payload)
	}
	longPreviewHTML, ok := longPreviewPayload["html"].(string)
	if !ok || !strings.Contains(longPreviewHTML, "长文段落第二行") {
		t.Fatalf("long markdown preview was truncated: html type=%T length=%d", longPreviewPayload["html"], len(longPreviewHTML))
	}
	longBlockLines, ok := longPreviewPayload["blockLines"].([]interface{})
	if !ok || len(longBlockLines) != 2500 {
		t.Fatalf("long markdown block map length = %d, want 2500", len(longBlockLines))
	}
	profileResult := postAdminForm(t, client, testServer.URL, "/admin/profile", "/admin/profile", url.Values{
		"username":   {"renamed-admin"},
		"screenName": {"Renamed Admin"},
		"email":      {"renamed@example.com"},
	})
	if !profileResult.Success {
		t.Fatalf("profile update failed: %s", profileResult.Msg)
	}
	renamedProfile, err := client.Get(testServer.URL + "/admin/profile")
	if err != nil {
		t.Fatal(err)
	}
	renamedProfileBody, err := io.ReadAll(renamedProfile.Body)
	_ = renamedProfile.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(renamedProfileBody), `value="renamed-admin"`) {
		t.Fatal("profile update did not persist username")
	}
	categoryResult := postAdminForm(t, client, testServer.URL, "/admin/category/save", "/admin/category", url.Values{
		"type":  {"category"},
		"cname": {"integration-category"},
	})
	if !categoryResult.Success {
		t.Fatalf("category create failed: %s", categoryResult.Msg)
	}
	tagResult := postAdminForm(t, client, testServer.URL, "/admin/category/save", "/admin/category", url.Values{
		"type":  {"tag"},
		"cname": {"integration-tag"},
	})
	if !tagResult.Success {
		t.Fatalf("tag create failed: %s", tagResult.Msg)
	}
	categoryPage, err := client.Get(testServer.URL + "/admin/category")
	if err != nil {
		t.Fatal(err)
	}
	categoryPageBody, err := io.ReadAll(categoryPage.Body)
	_ = categoryPage.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(categoryPageBody), "integration-category") ||
		!strings.Contains(string(categoryPageBody), "integration-tag") {
		t.Fatal("category/tag create did not persist")
	}
	for _, marker := range []string{
		`id="tag-mid"`,
		`class="btn btn-default btn-sm rename-meta"`,
		`data-meta-type="tag"`,
		`新增/重命名标签`,
	} {
		if !strings.Contains(string(categoryPageBody), marker) {
			t.Fatalf("category/tag management missing marker %q", marker)
		}
	}

	normalizedMetaContent := &model.Content{
		Title:        "Normalized Meta Article",
		Content:      "Metadata normalization.",
		AuthorID:     1,
		Type:         model.TypeArticle,
		Status:       model.TypeDraft,
		Categories:   "默认分类，integration-category，integration-category",
		Tags:         "integration-tag，fresh-tag,integration-tag",
		AllowComment: true,
		AllowPing:    true,
		AllowFeed:    true,
	}
	if err := services.Publish(normalizedMetaContent); err != nil {
		t.Fatal(err)
	}
	if normalizedMetaContent.Categories != "默认分类,integration-category" ||
		normalizedMetaContent.Tags != "integration-tag,fresh-tag" {
		t.Fatalf(
			"metadata not normalized: categories=%q tags=%q",
			normalizedMetaContent.Categories,
			normalizedMetaContent.Tags,
		)
	}
	var normalizedRelationships int64
	if err := database.Model(&model.Relationship{}).
		Where("cid = ?", normalizedMetaContent.Cid).
		Count(&normalizedRelationships).Error; err != nil {
		t.Fatal(err)
	}
	if normalizedRelationships != 4 {
		t.Fatalf("normalized relationships = %d, want 4", normalizedRelationships)
	}

	var contentsBeforeImport int64
	var attachsBeforeImport int64
	if err := database.Model(&model.Content{}).Count(&contentsBeforeImport).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.Attach{}).Count(&attachsBeforeImport).Error; err != nil {
		t.Fatal(err)
	}
	uploadFilesBeforeImport := countFiles(t, runtimeConfig.UploadDir)
	invalidArchives := []struct {
		name string
		data []byte
	}{
		{name: "broken.zip", data: []byte("not a zip archive")},
		{
			name: "multiple-markdown.zip",
			data: buildTestImportArchive(t, map[string]string{
				"first.md":  "# first",
				"second.md": "# second",
			}),
		},
		{
			name: "unsafe-path.zip",
			data: buildTestImportArchive(t, map[string]string{
				"../escape.md": "# escape",
			}),
		},
	}
	for _, invalidArchive := range invalidArchives {
		result := postAdminMultipart(t, client, testServer.URL, "/admin/article/import", "/admin/article", "archive", invalidArchive.name, invalidArchive.data)
		if result.Success {
			t.Fatalf("invalid archive %s unexpectedly succeeded", invalidArchive.name)
		}
	}
	invalidImportArchive := buildTestImportArchive(t, map[string]string{
		"invalid.md":         "![截图](assets/diagram.png)",
		"assets/diagram.png": string([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}),
		"assets/unsafe.exe":  "not an allowed attachment",
	})
	invalidImportResponse := postAdminMultipart(t, client, testServer.URL, "/admin/article/import", "/admin/article", "archive", "invalid.zip", invalidImportArchive)
	if invalidImportResponse.Success {
		t.Fatal("invalid archive import unexpectedly succeeded")
	}
	var contentsAfterInvalidImport int64
	var attachsAfterInvalidImport int64
	if err := database.Model(&model.Content{}).Count(&contentsAfterInvalidImport).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.Attach{}).Count(&attachsAfterInvalidImport).Error; err != nil {
		t.Fatal(err)
	}
	uploadFilesAfterInvalidImport := countFiles(t, runtimeConfig.UploadDir)
	if contentsAfterInvalidImport != contentsBeforeImport ||
		attachsAfterInvalidImport != attachsBeforeImport ||
		uploadFilesAfterInvalidImport != uploadFilesBeforeImport {
		t.Fatalf(
			"invalid archive left changes: contents %d->%d attachments %d->%d files %d->%d",
			contentsBeforeImport,
			contentsAfterInvalidImport,
			attachsBeforeImport,
			attachsAfterInvalidImport,
			uploadFilesBeforeImport,
			uploadFilesAfterInvalidImport,
		)
	}

	importArchive := buildTestImportArchive(t, map[string]string{
		"商单灵感工具复盘.md":       "![截图](图片和附件/diagram.png)\n\n正文内容。",
		"图片和附件/diagram.png": string([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}),
	})
	importResponse := postAdminMultipart(t, client, testServer.URL, "/admin/article/import", "/admin/article", "archive", "notes.zip", importArchive)
	if !importResponse.Success {
		t.Fatalf("article import failed: %s", importResponse.Msg)
	}
	importedCID, ok := importResponse.Payload.(map[string]interface{})["cid"].(float64)
	if !ok || importedCID <= 0 {
		t.Fatalf("article import returned invalid cid: %#v", importResponse.Payload)
	}
	importedArticle, err := services.GetContentByID(strconv.Itoa(int(importedCID)))
	if err != nil {
		t.Fatal(err)
	}
	if importedArticle == nil || importedArticle.Title != "商单灵感工具复盘" ||
		importedArticle.Status != model.TypeDraft ||
		importedArticle.ContentFormat != model.ContentMarkdown ||
		!strings.Contains(importedArticle.Content, "/upload/") {
		t.Fatalf("article import result invalid: %#v", importedArticle)
	}

	htmlImportArchive := buildTestImportArchive(t, map[string]string{
		"design-demos/article.html": `<!doctype html><html><head><title>HTML Design Article</title><style>:root{--paper:#f2ede4}body{background-color:var(--paper);background-image:url('../assets/paper.png')}</style></head><body><img src="../assets/cover.png?size=large#hero"><script>document.body.dataset.ready='1'</script></body></html>`,
		"assets/cover.png":          string([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}),
		"assets/paper.png":          string([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}),
	})
	htmlImportResponse := postAdminMultipart(t, client, testServer.URL, "/admin/article/import", "/admin/article", "archive", "design.zip", htmlImportArchive)
	if !htmlImportResponse.Success {
		t.Fatalf("HTML article import failed: %s", htmlImportResponse.Msg)
	}
	htmlImportedCID, ok := htmlImportResponse.Payload.(map[string]interface{})["cid"].(float64)
	if !ok || htmlImportedCID <= 0 {
		t.Fatalf("HTML article import returned invalid cid: %#v", htmlImportResponse.Payload)
	}
	htmlArticle, err := services.GetContentByID(strconv.Itoa(int(htmlImportedCID)))
	if err != nil {
		t.Fatal(err)
	}
	if htmlArticle == nil || htmlArticle.Title != "HTML Design Article" ||
		htmlArticle.ContentFormat != model.ContentHTML || htmlArticle.DisplayTime != htmlArticle.Created ||
		htmlArticle.HTMLThemeColor != "#e9e4d6" || htmlArticle.HTMLThemeColorVersion != util.HTMLThemeColorVersion ||
		!strings.Contains(htmlArticle.Content, `data:image/png;base64,`) ||
		strings.Contains(htmlArticle.Content, `/upload/`) || strings.Contains(htmlArticle.Content, `../assets/`) {
		t.Fatalf("HTML article import result invalid: %#v", htmlArticle)
	}
	htmlArticle.Status = model.TypePublish
	if err := services.UpdateArticle(htmlArticle); err != nil {
		t.Fatal(err)
	}
	htmlShellResponse, err := http.Get(testServer.URL + "/article/" + strconv.Itoa(htmlArticle.Cid))
	if err != nil {
		t.Fatal(err)
	}
	htmlShellBody, err := io.ReadAll(htmlShellResponse.Body)
	_ = htmlShellResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(htmlShellBody), `class="fluid-html-article-frame"`) ||
		!strings.Contains(string(htmlShellBody), `class="fluid-html-frame-shell is-loading"`) ||
		!strings.Contains(string(htmlShellBody), `class="fluid-html-frame-viewport"`) ||
		!strings.Contains(string(htmlShellBody), `class="fluid-html-frame-loader"`) ||
		!strings.Contains(string(htmlShellBody), `role="status"`) ||
		!strings.Contains(string(htmlShellBody), `data-html-theme-color="#e9e4d6"`) ||
		!strings.Contains(string(htmlShellBody), `sandbox="allow-scripts"`) ||
		!strings.Contains(string(htmlShellBody), `scrolling="no"`) {
		t.Fatal("HTML article shell is missing the sandboxed iframe")
	}
	if !strings.Contains(string(htmlShellBody), `/user/js/html-article.js`) {
		t.Fatal("HTML article shell is missing the frame controller")
	}
	htmlDocumentResponse, err := http.Get(testServer.URL + "/article/" + strconv.Itoa(htmlArticle.Cid) + "/document")
	if err != nil {
		t.Fatal(err)
	}
	htmlDocumentBody, err := io.ReadAll(htmlDocumentResponse.Body)
	_ = htmlDocumentResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if htmlDocumentResponse.StatusCode != http.StatusOK || !strings.Contains(string(htmlDocumentBody), "HTML Design Article") {
		t.Fatalf("HTML document status/body invalid: %d", htmlDocumentResponse.StatusCode)
	}
	for _, marker := range []string{`name="viewport"`, `myblog:html-size`, `myblog:html-ready`, `ResizeObserver`} {
		if !strings.Contains(string(htmlDocumentBody), marker) {
			t.Fatalf("HTML document missing frame bridge marker %q", marker)
		}
	}
	for _, expected := range []string{"sandbox allow-scripts", "default-src 'none'", "connect-src 'none'"} {
		if !strings.Contains(htmlDocumentResponse.Header.Get("Content-Security-Policy"), expected) {
			t.Fatalf("HTML document CSP missing %q: %s", expected, htmlDocumentResponse.Header.Get("Content-Security-Policy"))
		}
	}
	if strings.Contains(htmlDocumentResponse.Header.Get("Content-Security-Policy"), "allow-same-origin") {
		t.Fatal("HTML document CSP unexpectedly allows same-origin access")
	}
	if htmlDocumentResponse.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("HTML document cache control = %q", htmlDocumentResponse.Header.Get("Cache-Control"))
	}

	content := &model.Content{
		Title:        "Concurrent Article",
		Slug:         "concurrent-article",
		Content:      "## Concurrent\n\nLoad test article.",
		AuthorID:     1,
		Type:         model.TypeArticle,
		Status:       model.TypePublish,
		Categories:   "默认分类",
		Tags:         "integration-tag",
		AllowComment: true,
		AllowPing:    true,
		AllowFeed:    true,
	}
	if err := services.Publish(content); err != nil {
		t.Fatal(err)
	}
	originalCreated := content.Created
	customDisplayTime := content.Created - 40*24*60*60
	content.DisplayTime = customDisplayTime
	if err := services.UpdateArticle(content); err != nil {
		t.Fatal(err)
	}
	var updatedTimeline model.Content
	if err := database.First(&updatedTimeline, content.Cid).Error; err != nil {
		t.Fatal(err)
	}
	if updatedTimeline.Created != originalCreated || updatedTimeline.DisplayTime != customDisplayTime {
		t.Fatalf("timeline update changed immutable creation time: created=%d want=%d display=%d want=%d", updatedTimeline.Created, originalCreated, updatedTimeline.DisplayTime, customDisplayTime)
	}
	newerDisplayArticle := &model.Content{
		Title:        "Display Time Ordered Article",
		Content:      "Display-time ordering.",
		DisplayTime:  content.Created + 24*60*60,
		AuthorID:     1,
		Type:         model.TypeArticle,
		Status:       model.TypePublish,
		Categories:   "默认分类",
		AllowComment: true,
		AllowPing:    true,
		AllowFeed:    true,
	}
	if err := services.Publish(newerDisplayArticle); err != nil {
		t.Fatal(err)
	}
	ordered := services.GetContents(1, 20, false)
	position := func(cid int) int {
		for index, article := range ordered.List {
			if article.Cid == cid {
				return index
			}
		}
		return -1
	}
	if newerPosition, olderPosition := position(newerDisplayArticle.Cid), position(content.Cid); newerPosition < 0 || olderPosition < 0 || newerPosition >= olderPosition {
		t.Fatalf("articles are not ordered by display time: newer=%d older=%d", newerPosition, olderPosition)
	}
	archiveMonth := util.FormatUnixCN(customDisplayTime)
	archiveFound := false
	for _, archive := range services.GetArchives(false) {
		if archive.Date != archiveMonth {
			continue
		}
		for _, article := range archive.Articles {
			if article.Cid == content.Cid {
				archiveFound = true
			}
		}
	}
	if !archiveFound {
		t.Fatalf("article was not archived under display month %s", archiveMonth)
	}
	expandedTopicsResponse, err := http.Get(testServer.URL + "/topics")
	if err != nil {
		t.Fatal(err)
	}
	expandedTopicsBody, err := io.ReadAll(expandedTopicsResponse.Body)
	_ = expandedTopicsResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		`<details class="fluid-topic-node">`,
		`Concurrent Article`,
		`href="/article/concurrent-article"`,
	} {
		if !strings.Contains(string(expandedTopicsBody), marker) {
			t.Fatalf("expanded topics missing marker %q", marker)
		}
	}
	tagMeta := services.GetMeta(model.TypeTag, "integration-tag")
	if tagMeta == nil {
		t.Fatal("integration tag was not created")
	}
	if err := services.SaveOrRenameCategory(model.TypeTag, "integration-tag-renamed", tagMeta.Mid); err != nil {
		t.Fatal(err)
	}
	var renamedArticle model.Content
	if err := database.First(&renamedArticle, content.Cid).Error; err != nil {
		t.Fatal(err)
	}
	if renamedArticle.Tags != "integration-tag-renamed" || renamedArticle.Categories != "默认分类" {
		t.Fatalf("tag rename changed wrong fields: tags=%q categories=%q", renamedArticle.Tags, renamedArticle.Categories)
	}
	privateContent := &model.Content{
		Title:        "Private Article",
		Slug:         "private-article",
		Content:      "Private content.",
		AuthorID:     1,
		Type:         model.TypeArticle,
		Status:       model.TypePrivate,
		Categories:   "默认分类",
		AllowComment: true,
		AllowPing:    true,
		AllowFeed:    true,
	}
	if err := services.Publish(privateContent); err != nil {
		t.Fatal(err)
	}
	configuredAccessKey := runtimeConfig.AccessKey
	runtimeConfig.AccessKey = ""
	disabledEncryptedContent := &model.Content{
		Title:      "Disabled encrypted article",
		Content:    "Must not persist without an access key.",
		AuthorID:   1,
		Type:       model.TypeArticle,
		Status:     model.TypeEncrypted,
		Categories: "disabled-encrypted-category",
	}
	if err := services.Publish(disabledEncryptedContent); err == nil {
		t.Fatal("encrypted article persisted while BLOG_ACCESS_KEY was disabled")
	}
	runtimeConfig.AccessKey = configuredAccessKey
	encryptedContent := &model.Content{
		Title:        "Encrypted Article",
		Slug:         "encrypted-article",
		Content:      "Encrypted content.",
		AuthorID:     1,
		Type:         model.TypeArticle,
		Status:       model.TypePublish,
		Categories:   "integration-encrypted-category",
		Tags:         "integration-encrypted-tag",
		AllowComment: true,
		AllowPing:    true,
		AllowFeed:    true,
	}
	if err := services.Publish(encryptedContent); err != nil {
		t.Fatal(err)
	}
	publicBeforeEncryption, err := http.Get(testServer.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	publicBeforeEncryptionBody, err := io.ReadAll(publicBeforeEncryption.Body)
	_ = publicBeforeEncryption.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(publicBeforeEncryptionBody), "Encrypted Article") {
		t.Fatal("published article did not appear before encryption transition")
	}
	encryptedContent.Status = model.TypeEncrypted
	if err := services.UpdateArticle(encryptedContent); err != nil {
		t.Fatal(err)
	}
	homeAfterPrivate, err := http.Get(testServer.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	homeAfterPrivateBody, err := io.ReadAll(homeAfterPrivate.Body)
	_ = homeAfterPrivate.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(homeAfterPrivateBody), "Private Article") {
		t.Fatal("private article appeared on the public home page")
	}
	if strings.Contains(string(homeAfterPrivateBody), "Encrypted Article") {
		t.Fatal("encrypted article appeared on the public home page without access")
	}
	searchAfterPrivate, err := http.Get(testServer.URL + "/search/Private")
	if err != nil {
		t.Fatal(err)
	}
	searchAfterPrivateBody, err := io.ReadAll(searchAfterPrivate.Body)
	_ = searchAfterPrivate.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(searchAfterPrivateBody), "Private Article") {
		t.Fatal("private article appeared in public search results")
	}
	encryptedPublicPaths := []string{
		"/search/Encrypted",
		"/archives",
		"/category/integration-encrypted-category",
		"/tag/integration-encrypted-tag",
		"/topics",
	}
	for _, path := range encryptedPublicPaths {
		publicResponse, requestErr := http.Get(testServer.URL + path)
		if requestErr != nil {
			t.Fatalf("GET %s: %v", path, requestErr)
		}
		publicBody, readErr := io.ReadAll(publicResponse.Body)
		_ = publicResponse.Body.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.Contains(string(publicBody), "Encrypted Article") {
			t.Fatalf("encrypted article leaked at %s without access", path)
		}
	}
	encryptedDirectResponse, err := http.Get(testServer.URL + "/article/encrypted-article")
	if err != nil {
		t.Fatal(err)
	}
	_ = encryptedDirectResponse.Body.Close()
	if encryptedDirectResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("encrypted article status = %d, want 404 without access", encryptedDirectResponse.StatusCode)
	}
	if value := encryptedDirectResponse.Header.Get("Cache-Control"); value != "private, no-store" {
		t.Fatalf("encrypted 404 cache control = %q, want private, no-store", value)
	}
	unauthorizedCommentClient := newCookieClient(t)
	unauthorizedCommentToken := fetchCSRFToken(t, unauthorizedCommentClient, testServer.URL+"/")
	unauthorizedCommentRequest, err := http.NewRequest(
		http.MethodPost,
		testServer.URL+"/comment",
		strings.NewReader(url.Values{
			"cid":         {strconv.Itoa(encryptedContent.Cid)},
			"author":      {"visitor"},
			"text":        {"unauthorized encrypted comment"},
			"_csrf_token": {unauthorizedCommentToken},
		}.Encode()),
	)
	if err != nil {
		t.Fatal(err)
	}
	unauthorizedCommentRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unauthorizedCommentRequest.Header.Set("Referer", testServer.URL+"/")
	unauthorizedCommentResponse, err := unauthorizedCommentClient.Do(unauthorizedCommentRequest)
	if err != nil {
		t.Fatal(err)
	}
	var unauthorizedCommentResult handler.RestResponse
	if err := json.NewDecoder(unauthorizedCommentResponse.Body).Decode(&unauthorizedCommentResult); err != nil {
		_ = unauthorizedCommentResponse.Body.Close()
		t.Fatal(err)
	}
	_ = unauthorizedCommentResponse.Body.Close()
	if unauthorizedCommentResult.Success {
		t.Fatal("unauthorized visitor commented on encrypted article")
	}
	accessClient := newCookieClient(t)
	invalidAccessResult, invalidCookies := postPublicAccessKey(
		t,
		accessClient,
		testServer.URL,
		"wrong-reader-key",
	)
	if invalidAccessResult.Success || len(invalidCookies) != 0 {
		t.Fatalf("invalid access key result/cookies = %#v/%#v", invalidAccessResult, invalidCookies)
	}
	validAccessResult, validCookies := postPublicAccessKey(
		t,
		accessClient,
		testServer.URL,
		runtimeConfig.AccessKey,
	)
	if !validAccessResult.Success {
		t.Fatalf("valid access key failed: %s", validAccessResult.Msg)
	}
	validAccessPayload, ok := validAccessResult.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("valid access payload type = %T", validAccessResult.Payload)
	}
	expiresAt, ok := validAccessPayload["expiresAt"].(float64)
	now := time.Now().Unix()
	if !ok || int64(expiresAt) < now+24*60*60-5 || int64(expiresAt) > now+24*60*60+5 {
		t.Fatalf("access expiry = %v, want about 24 hours from now", validAccessPayload["expiresAt"])
	}
	accessCookie := findCookie(validCookies, "BLOG_ARTICLE_ACCESS")
	if accessCookie == nil {
		t.Fatal("valid access key did not set access cookie")
	}
	if accessCookie.MaxAge != 24*60*60 || !accessCookie.HttpOnly ||
		accessCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("access cookie flags invalid: %#v", accessCookie)
	}
	if strings.Contains(accessCookie.Value, runtimeConfig.AccessKey) {
		t.Fatal("access cookie contains the plaintext access key")
	}
	for _, path := range append([]string{"/", "/article/encrypted-article"}, encryptedPublicPaths...) {
		authorizedResponse, requestErr := accessClient.Get(testServer.URL + path)
		if requestErr != nil {
			t.Fatalf("authorized GET %s: %v", path, requestErr)
		}
		authorizedBody, readErr := io.ReadAll(authorizedResponse.Body)
		_ = authorizedResponse.Body.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if authorizedResponse.StatusCode != http.StatusOK ||
			!strings.Contains(string(authorizedBody), "Encrypted Article") {
			t.Fatalf("authorized encrypted article missing at %s status=%d", path, authorizedResponse.StatusCode)
		}
		if !strings.Contains(string(authorizedBody), "fluid-encrypted-badge") {
			t.Fatalf("authorized encrypted article badge missing at %s", path)
		}
		if path == "/" && !strings.Contains(string(authorizedBody), "加密文章已解锁") {
			t.Fatal("authorized homepage is missing unlocked access state")
		}
		if value := authorizedResponse.Header.Get("Cache-Control"); value != "private, no-store" {
			t.Fatalf("authorized response cache control = %q, want private, no-store", value)
		}
	}
	anonymousAfterUnlock, err := http.Get(testServer.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	anonymousAfterUnlockBody, err := io.ReadAll(anonymousAfterUnlock.Body)
	_ = anonymousAfterUnlock.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(anonymousAfterUnlockBody), "Encrypted Article") {
		t.Fatal("authorized visitor leaked encrypted article into anonymous response cache")
	}
	freshAnonymousClient := newCookieClient(t)
	freshAnonymousResponse, err := freshAnonymousClient.Get(testServer.URL + "/article/encrypted-article")
	if err != nil {
		t.Fatal(err)
	}
	_ = freshAnonymousResponse.Body.Close()
	if freshAnonymousResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("fresh anonymous status after another visitor unlock = %d, want 404", freshAnonymousResponse.StatusCode)
	}
	keyVisitorPrivateResponse, err := accessClient.Get(testServer.URL + "/article/private-article")
	if err != nil {
		t.Fatal(err)
	}
	_ = keyVisitorPrivateResponse.Body.Close()
	if keyVisitorPrivateResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("key visitor private article status = %d, want 404", keyVisitorPrivateResponse.StatusCode)
	}
	expiredClient := newCookieClient(t)
	expiredURL, _ := url.Parse(testServer.URL)
	expiredClient.Jar.SetCookies(expiredURL, []*http.Cookie{
		signedAccessCookie(
			runtimeConfig.SessionSecret,
			runtimeConfig.AccessKey,
			"127.0.0.1",
			time.Now().Add(-time.Minute).Unix(),
		),
	})
	expiredResponse, err := expiredClient.Get(testServer.URL + "/article/encrypted-article")
	if err != nil {
		t.Fatal(err)
	}
	_ = expiredResponse.Body.Close()
	if expiredResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("expired access cookie status = %d, want 404", expiredResponse.StatusCode)
	}
	rotatedKeyClient := newCookieClient(t)
	rotatedKeyClient.Jar.SetCookies(expiredURL, []*http.Cookie{
		signedAccessCookie(
			runtimeConfig.SessionSecret,
			"previous-reader-key",
			"127.0.0.1",
			time.Now().Add(time.Hour).Unix(),
		),
	})
	rotatedKeyResponse, err := rotatedKeyClient.Get(testServer.URL + "/article/encrypted-article")
	if err != nil {
		t.Fatal(err)
	}
	_ = rotatedKeyResponse.Body.Close()
	if rotatedKeyResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("rotated access key cookie status = %d, want 404", rotatedKeyResponse.StatusCode)
	}
	tamperedClient := newCookieClient(t)
	tamperedCookie := signedAccessCookie(
		runtimeConfig.SessionSecret,
		runtimeConfig.AccessKey,
		"127.0.0.1",
		time.Now().Add(time.Hour).Unix(),
	)
	tamperedCookie.Value += "tampered"
	tamperedClient.Jar.SetCookies(expiredURL, []*http.Cookie{tamperedCookie})
	tamperedResponse, err := tamperedClient.Get(testServer.URL + "/article/encrypted-article")
	if err != nil {
		t.Fatal(err)
	}
	_ = tamperedResponse.Body.Close()
	if tamperedResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("tampered access cookie status = %d, want 404", tamperedResponse.StatusCode)
	}
	ipBoundCookie := signedAccessCookie(
		runtimeConfig.SessionSecret,
		runtimeConfig.AccessKey,
		"203.0.113.10",
		time.Now().Add(time.Hour).Unix(),
	)
	ipBoundRequest, err := http.NewRequest(
		http.MethodGet,
		testServer.URL+"/article/encrypted-article",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	ipBoundRequest.AddCookie(ipBoundCookie)
	ipBoundRequest.Header.Set("X-Real-IP", "203.0.113.11")
	ipBoundRequest.Header.Set("X-Forwarded-For", "203.0.113.10")
	ipBoundResponse, err := http.DefaultClient.Do(ipBoundRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = ipBoundResponse.Body.Close()
	if ipBoundResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("access cookie reused from another IP status = %d, want 404", ipBoundResponse.StatusCode)
	}
	sameIPRequest, err := http.NewRequest(
		http.MethodGet,
		testServer.URL+"/article/encrypted-article",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	sameIPRequest.AddCookie(ipBoundCookie)
	sameIPRequest.Header.Set("X-Real-IP", "203.0.113.10")
	sameIPRequest.Header.Set("X-Forwarded-For", "198.51.100.99")
	sameIPResponse, err := http.DefaultClient.Do(sameIPRequest)
	if err != nil {
		t.Fatal(err)
	}
	sameIPBody, err := io.ReadAll(sameIPResponse.Body)
	_ = sameIPResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if sameIPResponse.StatusCode != http.StatusOK ||
		!strings.Contains(string(sameIPBody), "Encrypted Article") {
		t.Fatalf("access cookie rejected on its bound IP status = %d", sameIPResponse.StatusCode)
	}
	adminEncryptedResponse, err := client.Get(testServer.URL + "/article/encrypted-article")
	if err != nil {
		t.Fatal(err)
	}
	adminEncryptedBody, err := io.ReadAll(adminEncryptedResponse.Body)
	_ = adminEncryptedResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if adminEncryptedResponse.StatusCode != http.StatusOK ||
		!strings.Contains(string(adminEncryptedBody), "Encrypted Article") {
		t.Fatalf("admin cannot view encrypted article: status=%d", adminEncryptedResponse.StatusCode)
	}
	adminHomeResponse, err := client.Get(testServer.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	adminHomeBody, err := io.ReadAll(adminHomeResponse.Body)
	_ = adminHomeResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(adminHomeBody), "Encrypted Article") {
		t.Fatal("admin homepage does not include encrypted article")
	}
	revokeResult := postPublicAccessRevoke(t, accessClient, testServer.URL)
	if !revokeResult.Success {
		t.Fatalf("revoke access failed: %s", revokeResult.Msg)
	}
	revokedResponse, err := accessClient.Get(testServer.URL + "/article/encrypted-article")
	if err != nil {
		t.Fatal(err)
	}
	_ = revokedResponse.Body.Close()
	if revokedResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("revoked access status = %d, want 404", revokedResponse.StatusCode)
	}
	privateResponse, err := unauthenticatedClient.Get(testServer.URL + "/article/private-article")
	if err != nil {
		t.Fatal(err)
	}
	if privateResponse.StatusCode != http.StatusNotFound {
		_ = privateResponse.Body.Close()
		t.Fatalf("private article status = %d, want 404", privateResponse.StatusCode)
	}
	_ = privateResponse.Body.Close()
	privatePreviewResponse, err := client.Get(testServer.URL + "/article/private-article")
	if err != nil {
		t.Fatal(err)
	}
	privatePreviewBody, err := io.ReadAll(privatePreviewResponse.Body)
	_ = privatePreviewResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if privatePreviewResponse.StatusCode != http.StatusOK || !strings.Contains(string(privatePreviewBody), "Private Article") {
		t.Fatalf("authenticated private article status/body invalid: %d", privatePreviewResponse.StatusCode)
	}
	backup, err := services.Backup("db", "", filepath.Join("..", "..", "templates", "theme"))
	if err != nil {
		t.Fatal(err)
	}
	backupArchive, err := zip.OpenReader(filepath.Join(
		runtimeConfig.UploadDir,
		strings.TrimPrefix(backup.SqlPath, "/upload/"),
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(backupArchive.File) != 1 {
		_ = backupArchive.Close()
		t.Fatalf("backup entries = %d, want 1", len(backupArchive.File))
	}
	backupEntry, err := backupArchive.File[0].Open()
	if err != nil {
		_ = backupArchive.Close()
		t.Fatal(err)
	}
	backupSQL, err := io.ReadAll(backupEntry)
	_ = backupEntry.Close()
	_ = backupArchive.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(backupSQL), "CREATE TABLE") ||
		!strings.Contains(string(backupSQL), "INSERT INTO `t_contents`") {
		t.Fatal("database backup does not contain schema and content data")
	}
	for index := range 2 {
		withoutSlug := &model.Content{
			Title:        "No Slug " + strconv.Itoa(index),
			Content:      "No slug content",
			AuthorID:     1,
			Type:         model.TypeArticle,
			Status:       model.TypePublish,
			Categories:   "默认分类",
			AllowComment: true,
			AllowPing:    true,
			AllowFeed:    true,
		}
		if err := services.Publish(withoutSlug); err != nil {
			t.Fatalf("publish empty slug %d: %v", index, err)
		}
	}

	articleResponse, err := http.Get(testServer.URL + "/article/concurrent-article")
	if err != nil {
		t.Fatal(err)
	}
	articleBody, err := io.ReadAll(articleResponse.Body)
	_ = articleResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	csrfMatch := csrfPattern.FindSubmatch(articleBody)
	if len(csrfMatch) != 2 {
		t.Fatal("article page has no CSRF token")
	}
	commentValues := url.Values{
		"cid":         {strconv.Itoa(content.Cid)},
		"coid":        {"0"},
		"author":      {"integration"},
		"mail":        {"integration@example.com"},
		"url":         {"https://example.com"},
		"text":        {"这是一条集成测试评论"},
		"_csrf_token": {string(csrfMatch[1])},
	}
	commentRequest, err := http.NewRequest(
		http.MethodPost,
		testServer.URL+"/comment",
		strings.NewReader(commentValues.Encode()),
	)
	if err != nil {
		t.Fatal(err)
	}
	commentRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	commentRequest.Header.Set("Referer", testServer.URL+"/article/concurrent-article")
	commentResponse, err := http.DefaultClient.Do(commentRequest)
	if err != nil {
		t.Fatal(err)
	}
	var commentResult handler.RestResponse
	if err := json.NewDecoder(commentResponse.Body).Decode(&commentResult); err != nil {
		_ = commentResponse.Body.Close()
		t.Fatal(err)
	}
	_ = commentResponse.Body.Close()
	if !commentResult.Success {
		t.Fatalf("comment failed: %s", commentResult.Msg)
	}

	const totalRequests = 1000
	const workers = 50
	jobs := make(chan struct{}, totalRequests)
	for range totalRequests {
		jobs <- struct{}{}
	}
	close(jobs)

	var failures atomic.Int64
	var waitGroup sync.WaitGroup
	httpClient := &http.Client{Timeout: 5 * time.Second}
	for range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for range jobs {
				result, requestErr := httpClient.Get(testServer.URL + "/article/concurrent-article")
				if requestErr != nil {
					failures.Add(1)
					continue
				}
				_, _ = io.Copy(io.Discard, result.Body)
				_ = result.Body.Close()
				if result.StatusCode != http.StatusOK {
					failures.Add(1)
				}
			}
		}()
	}
	waitGroup.Wait()
	if failures.Load() != 0 {
		t.Fatalf("concurrent request failures = %d", failures.Load())
	}

	server.Close()
	var hits int
	if err := database.Model(&model.Content{}).
		Select("hits").
		Where("cid = ?", content.Cid).
		Scan(&hits).Error; err != nil {
		t.Fatal(err)
	}
	expectedHits := totalRequests + 1 // one request fetched the comment CSRF token
	if hits != expectedHits {
		t.Fatalf("hits = %d, want %d", hits, expectedHits)
	}
}

func authenticatedClient(t *testing.T, baseURL, username, password string) *http.Client {
	t.Helper()
	client := newCookieClient(t)
	response, err := client.Get(baseURL + "/admin/login")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	match := csrfPattern.FindSubmatch(body)
	if len(match) != 2 {
		t.Fatal("login page has no CSRF token")
	}
	values := url.Values{
		"username":    {username},
		"password":    {password},
		"_csrf_token": {string(match[1])},
	}
	request, err := http.NewRequest(
		http.MethodPost,
		baseURL+"/admin/login",
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginResponse, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer loginResponse.Body.Close()
	var result handler.RestResponse
	if err := json.NewDecoder(loginResponse.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("login failed: %s", result.Msg)
	}
	return client
}

func newCookieClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar, Timeout: 5 * time.Second}
}

func postPublicAccessKey(
	t *testing.T,
	client *http.Client,
	baseURL, accessKey string,
) (handler.RestResponse, []*http.Cookie) {
	t.Helper()
	csrf := fetchCSRFToken(t, client, baseURL+"/")
	request, err := http.NewRequest(
		http.MethodPost,
		baseURL+"/access-key",
		strings.NewReader(url.Values{
			"accessKey":   {accessKey},
			"_csrf_token": {csrf},
		}.Encode()),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Referer", baseURL+"/")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var result handler.RestResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result, response.Cookies()
}

func postPublicAccessRevoke(
	t *testing.T,
	client *http.Client,
	baseURL string,
) handler.RestResponse {
	t.Helper()
	csrf := fetchCSRFToken(t, client, baseURL+"/")
	request, err := http.NewRequest(
		http.MethodPost,
		baseURL+"/access-key/revoke",
		strings.NewReader(url.Values{"_csrf_token": {csrf}}.Encode()),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Referer", baseURL+"/")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var result handler.RestResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func fetchCSRFToken(t *testing.T, client *http.Client, target string) string {
	t.Helper()
	response, err := client.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	match := csrfPattern.FindSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("%s has no CSRF token", target)
	}
	return string(match[1])
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func signedAccessCookie(
	sessionSecret, accessKey, clientIP string,
	expiry int64,
) *http.Cookie {
	sessionKey := sha256.Sum256([]byte(sessionSecret))
	sign := func(payload string) string {
		mac := hmac.New(sha256.New, sessionKey[:])
		_, _ = mac.Write([]byte(payload))
		return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	}
	scope := sign("article-access|" + accessKey)
	ipScope := sign("article-access-ip|" + clientIP)
	payload := "v3|" + scope + "|" + ipScope + "|" + strconv.FormatInt(expiry, 10)
	value := base64.RawURLEncoding.EncodeToString([]byte(payload + "|" + sign(payload)))
	return &http.Cookie{
		Name:  "BLOG_ARTICLE_ACCESS",
		Value: value,
		Path:  "/",
	}
}

func postAdminForm(t *testing.T, client *http.Client, baseURL, path, csrfPath string, values url.Values) handler.RestResponse {
	t.Helper()
	response, err := client.Get(baseURL + csrfPath)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	match := csrfPattern.FindSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("%s has no CSRF token", csrfPath)
	}
	values.Set("_csrf_token", string(match[1]))
	request, err := http.NewRequest(http.MethodPost, baseURL+path, strings.NewReader(values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Referer", baseURL+path)
	resultResponse, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer resultResponse.Body.Close()
	var result handler.RestResponse
	if err := json.NewDecoder(resultResponse.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func postAdminMultipart(t *testing.T, client *http.Client, baseURL, path, csrfPath, field, filename string, content []byte) handler.RestResponse {
	t.Helper()
	response, err := client.Get(baseURL + csrfPath)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	match := csrfPattern.FindSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("%s has no CSRF token", csrfPath)
	}
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	if err := writer.WriteField("_csrf_token", string(match[1])); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, baseURL+path, &requestBody)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Referer", baseURL+path)
	resultResponse, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer resultResponse.Body.Close()
	var result handler.RestResponse
	if err := json.NewDecoder(resultResponse.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func buildTestImportArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func countFiles(t *testing.T, root string) int {
	t.Helper()
	count := 0
	err := filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func postLogin(t *testing.T, baseURL, username, password string) handler.RestResponse {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}
	response, err := client.Get(baseURL + "/admin/login")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	match := csrfPattern.FindSubmatch(body)
	if len(match) != 2 {
		t.Fatal("login page has no CSRF token")
	}
	values := url.Values{
		"username":    {username},
		"password":    {password},
		"_csrf_token": {string(match[1])},
	}
	request, err := http.NewRequest(
		http.MethodPost,
		baseURL+"/admin/login",
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginResponse, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer loginResponse.Body.Close()
	var result handler.RestResponse
	if err := json.NewDecoder(loginResponse.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func randomTestPassword(t *testing.T) string {
	t.Helper()
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		t.Fatalf("generate test password: %v", err)
	}
	return "test-" + hex.EncodeToString(buffer)
}
