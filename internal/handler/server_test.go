package handler_test

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
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

	"github.com/gin-gonic/gin"
)

var csrfPattern = regexp.MustCompile(`(?:name="_csrf_token" value="|name="csrf-token" content=")([^"]+)`)

func TestPublicAdminAndConcurrentArticleFlow(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	tempDirectory := t.TempDir()
	testUsername := "test-admin"
	testPassword := randomTestPassword(t)
	runtimeConfig := &config.Config{
		DBDriver:             "sqlite",
		DBDSN:                filepath.Join(tempDirectory, "blog.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)",
		DBMaxOpenConns:       20,
		DBMaxIdleConns:       10,
		DBConnMaxLifetime:    30 * time.Minute,
		SessionSecret:        "integration-test-secret-0123456789abcdef",
		UploadDir:            filepath.Join(tempDirectory, "upload"),
		HitFlushEvery:        100,
		RateLimitRPS:         100_000,
		RateLimitBurst:       200_000,
		AdminUsername:        testUsername,
		AdminEmail:           "test@example.com",
		AdminInitialPassword: testPassword,
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

	for _, path := range []string{"/", "/healthz", "/readyz", "/article/welcome", "/article/welcome.html", "/archives", "/links", "/about"} {
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
	staticResponse, err := http.Get(testServer.URL + "/user/css/fluid.css")
	if err != nil {
		t.Fatal(err)
	}
	_ = staticResponse.Body.Close()
	if value := staticResponse.Header.Get("Cache-Control"); value == "" {
		t.Fatal("public static asset is missing Cache-Control")
	}
	for _, marker := range []string{
		`href="/user/css/fluid.css"`,
		`lxgw-wenkai-webfont@1.7.0/lxgwwenkai-regular.css`,
		`lxgw-wenkai-webfont@1.7.0/lxgwwenkai-bold.css`,
		`class="fluid-theme fluid-font-wenkai fluid-home-page"`,
		`class="fluid-banner fluid-banner-home"`,
		`background-image:url('/user/img/blog-banner.jpg')`,
		`rel="preload" as="image"`,
		`class="fluid-home-stage"`,
		`id="study-map"`,
		`学习地图`,
		`id="home-audio"`,
		`fluid-index-card-featured`,
		`class="fluid-board fluid-index-board"`,
		`id="color-toggle"`,
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
	invalidLoginResponse := postLogin(t, testServer.URL, "wrong-user", "wrong-password")
	if invalidLoginResponse.Msg != "用户名或密码错误" {
		t.Fatalf("invalid login message = %q, want generic credential error", invalidLoginResponse.Msg)
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
		`background-image:url('/user/img/blog-banner.jpg')`,
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
	if !result.Success {
		t.Fatalf("login failed: %s", result.Msg)
	}
	return client
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
