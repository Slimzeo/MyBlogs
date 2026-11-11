package handler_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
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

	"github.com/gin-gonic/gin"
)

func TestProjectCardsAdminEditsReachPublicPages(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	directory := t.TempDir()
	runtimeConfig := &config.Config{
		DBDriver:             "sqlite",
		DBDSN:                filepath.Join(directory, "blog.db"),
		DBMaxOpenConns:       5,
		DBMaxIdleConns:       2,
		DBConnMaxLifetime:    time.Minute,
		SessionSecret:        "project-admin-test-secret-0123456789abcdef",
		UploadDir:            filepath.Join(directory, "upload"),
		HitFlushEvery:        100,
		RateLimitRPS:         1000,
		RateLimitBurst:       2000,
		AdminUsername:        "project-admin",
		AdminEmail:           "project-admin@example.com",
		AdminInitialPassword: "temporary-project-password",
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

	client := authenticatedClient(t, testServer.URL, runtimeConfig.AdminUsername, runtimeConfig.AdminInitialPassword)
	get := func(path string, status int) string {
		t.Helper()
		response, requestErr := client.Get(testServer.URL + path)
		if requestErr != nil {
			t.Fatalf("GET %s: %v", path, requestErr)
		}
		body, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if response.StatusCode != status {
			t.Fatalf("GET %s status = %d, want %d", path, response.StatusCode, status)
		}
		return string(body)
	}
	has := func(body string, markers ...string) {
		t.Helper()
		for _, marker := range markers {
			if !strings.Contains(body, marker) {
				t.Fatalf("missing %q", marker)
			}
		}
	}

	adminList := get("/admin/projects", http.StatusOK)
	has(adminList, "项目卡片", `href="/admin/projects" class="active"`, "Seekino", "catendar")

	projects, err := services.Projects()
	if err != nil || len(projects) != 2 {
		t.Fatalf("seeded projects = %#v, %v", projects, err)
	}
	var seekino model.Project
	for _, project := range projects {
		if project.Slug == "seekino" {
			seekino = project
		}
	}
	if seekino.ID == 0 {
		t.Fatal("seeded Seekino card is missing")
	}
	editPage := get("/admin/projects/"+strconv.Itoa(seekino.ID), http.StatusOK)
	has(editPage, `value="seekino"`, "Seekino", `name="cover_url"`, `name="platform_url"`, `name="github_url"`, `name="article_id"`)
	get("/admin/projects/999999", http.StatusNotFound)

	stale := postAdminForm(t, client, testServer.URL, "/admin/projects/save", "/admin/projects/"+strconv.Itoa(seekino.ID), url.Values{
		"id": {strconv.Itoa(seekino.ID)}, "name": {"Seekino"}, "slug": {"seekino"}, "project_url": {""},
	})
	if stale.Success {
		t.Fatal("a legacy editor could erase the migrated card links")
	}

	invalidCreate := postAdminForm(t, client, testServer.URL, "/admin/projects/save", "/admin/projects/new", url.Values{
		"name":      {"Bad Card"},
		"slug":      {"bad-card"},
		"summary":   {"This must not be saved."},
		"cover_url": {"https://evil.example/cover.png"},
		"sort":      {"30"},
	})
	if invalidCreate.Success {
		t.Fatal("non-GitHub cover URL was accepted")
	}
	if body := get("/projects", http.StatusOK); strings.Contains(body, "Bad Card") {
		t.Fatal("invalid project appeared on the public page")
	}

	article := model.Content{Title: "Fixture article", Slug: "fixture-article", Type: model.TypeArticle, Status: model.TypePublish, Content: "Fixture."}
	if err := database.Create(&article).Error; err != nil {
		t.Fatal(err)
	}
	countArticles := func() int64 {
		t.Helper()
		var count int64
		if err := database.Model(&model.Content{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		return count
	}
	before := countArticles()
	form := url.Values{
		"name": {"Gallery Demo"}, "slug": {"gallery-demo"}, "article_id": {strconv.Itoa(article.Cid)},
		"cover_url": {"https://github.com/example/demo/blob/main/docs/cover.png"},
		"cover_alt": {"示例标志"}, "github_url": {"https://github.com/example/demo.git"}, "sort": {"5"},
	}
	save := func(path string) bool {
		t.Helper()
		return postAdminForm(t, client, testServer.URL, "/admin/projects/save", path, form).Success
	}
	if !save("/admin/projects/new") || countArticles() != before {
		t.Fatal("saving a card with no summary should link the selected article and create no content")
	}
	card, err := services.ProjectBySlug("gallery-demo")
	if err != nil || card == nil {
		t.Fatalf("created card missing: %v", err)
	}
	edit := "/admin/projects/" + strconv.Itoa(card.ID)
	public := func() string { return get("/projects", http.StatusOK) }
	has(public(), "Gallery Demo", `href="/article/fixture-article"`, `href="https://github.com/example/demo"`, `src="https://raw.githubusercontent.com/example/demo/main/docs/cover.png"`)
	has(get("/projects/gallery-demo", http.StatusOK), `id="project:gallery-demo"`)
	has(get(edit, http.StatusOK), ">Fixture article</option>")
	form.Set("id", strconv.Itoa(card.ID))
	for _, status := range []string{model.TypeDraft, model.TypePrivate, model.TypeEncrypted} {
		if err := database.Model(&article).Update("status", status).Error; err != nil {
			t.Fatal(err)
		}
		if strings.Contains(public(), "fixture-article") {
			t.Fatalf("link to %s article became public", status)
		}
		if strings.Contains(get("/admin/projects/new", http.StatusOK), ">Fixture article</option>") {
			t.Fatalf("the editor offered a %s article for a new association", status)
		}
		hidden := postAdminForm(t, client, testServer.URL, "/admin/projects/save", "/admin/projects/new", url.Values{
			"name": {"Invalid association"}, "slug": {"invalid-article"}, "article_id": {strconv.Itoa(article.Cid)},
		})
		if hidden.Success {
			t.Fatalf("a new card was linked to a %s article", status)
		}
	}
	has(get(edit, http.StatusOK), "原关联文章")
	if !save(edit) {
		t.Fatal("unrelated edit could not retain its previous hidden association")
	}
	if err := database.Model(&article).Updates(map[string]any{"status": model.TypePublish, "slug": "renamed-article"}).Error; err != nil {
		t.Fatal(err)
	}
	has(public(), `href="/article/renamed-article"`)
	form.Set("article_id", "999999")
	if save(edit) {
		t.Fatal("nonexistent article was accepted")
	}
	has(public(), `href="/article/renamed-article"`)
	if err := database.Delete(&article).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(public(), "renamed-article") {
		t.Fatal("deleted article still linked")
	}
	form.Set("article_id", "0")
	form.Set("github_url", "")
	form.Set("cover_url", "")
	if !save(edit) {
		t.Fatal("optional fields could not be cleared")
	}
	card, err = services.ProjectByID(card.ID)
	if err != nil || card == nil || card.ArticleID != 0 || card.GitHubURL != "" || card.CoverURL != "" {
		t.Fatalf("clearing optional fields did not persist: %#v, %v", card, err)
	}

	duplicate := postAdminForm(t, client, testServer.URL, "/admin/projects/save", "/admin/projects/"+strconv.Itoa(seekino.ID), url.Values{
		"id":      {strconv.Itoa(seekino.ID)},
		"name":    {"Overwritten"},
		"slug":    {"catendar"},
		"summary": {"Must not overwrite another card."},
	})
	if duplicate.Success {
		t.Fatal("duplicate slug update was accepted")
	}
	if unchanged, _ := services.ProjectBySlug("seekino"); unchanged == nil || unchanged.Name != "Seekino" {
		t.Fatal("duplicate slug update changed the original card")
	}

	invalidUpdate := postAdminForm(t, client, testServer.URL, "/admin/projects/save", "/admin/projects/"+strconv.Itoa(seekino.ID), url.Values{
		"id":           {strconv.Itoa(seekino.ID)},
		"name":         {"Seekino"},
		"slug":         {"seekino"},
		"summary":      {"Valid summary."},
		"platform_url": {"javascript:alert(1)"},
	})
	if invalidUpdate.Success {
		t.Fatal("invalid project URL update was accepted")
	}
	validUpdate := postAdminForm(t, client, testServer.URL, "/admin/projects/save", "/admin/projects/"+strconv.Itoa(seekino.ID), url.Values{
		"id":           {strconv.Itoa(seekino.ID)},
		"name":         {"Seekino Updated"},
		"slug":         {"seekino"},
		"summary":      {"更新后的项目简介。"},
		"cover_url":    {seekino.CoverURL},
		"platform_url": {seekino.PlatformURL},
		"github_url":   {seekino.GitHubURL},
		"sort":         {"10"},
	})
	if !validUpdate.Success {
		t.Fatalf("valid project update failed: %s", validUpdate.Msg)
	}
	has(get("/projects", http.StatusOK), "Seekino Updated", "更新后的项目简介。")

	projects, err = services.Projects()
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range projects {
		deleted := postAdminForm(t, client, testServer.URL, "/admin/projects/delete", "/admin/projects", url.Values{
			"id": {strconv.Itoa(project.ID)},
		})
		if !deleted.Success {
			t.Fatalf("delete project %d failed: %s", project.ID, deleted.Msg)
		}
	}
	emptyGallery := get("/projects", http.StatusOK)
	has(emptyGallery, "还没有项目卡片。")
	if remaining, _ := services.Projects(); len(remaining) != 0 {
		t.Fatalf("intentional empty gallery still has %d cards", len(remaining))
	}
	has(get("/admin/projects", http.StatusOK), "暂无项目卡片")
	if !strings.Contains(get("/projects/removed-card", http.StatusNotFound), "404") {
		t.Fatal("removed card detail did not return the project 404 page")
	}
}
