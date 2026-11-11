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
	has(editPage, `value="seekino"`, "Seekino", "封面图片 URL")
	get("/admin/projects/999999", http.StatusNotFound)

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

	created := postAdminForm(t, client, testServer.URL, "/admin/projects/save", "/admin/projects/new", url.Values{
		"name":        {"Gallery Demo"},
		"slug":        {"gallery-demo"},
		"summary":     {"一张示例卡片。"},
		"notes":       {"## 笔记\n\n正文。"},
		"cover_url":   {"https://github.com/example/demo/blob/main/docs/cover.png"},
		"cover_alt":   {"示例截图"},
		"project_url": {"https://github.com/example/demo"},
		"sort":        {"5"},
	})
	if !created.Success {
		t.Fatalf("create project failed: %s", created.Msg)
	}
	publicPage := get("/projects", http.StatusOK)
	has(publicPage, "Gallery Demo", `href="/projects/gallery-demo"`,
		`src="https://raw.githubusercontent.com/example/demo/main/docs/cover.png"`)
	has(get("/projects/gallery-demo", http.StatusOK), "笔记", `class="fluid-markdown"`)

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
		"id":          {strconv.Itoa(seekino.ID)},
		"name":        {"Seekino"},
		"slug":        {"seekino"},
		"summary":     {"Valid summary."},
		"project_url": {"javascript:alert(1)"},
	})
	if invalidUpdate.Success {
		t.Fatal("invalid project URL update was accepted")
	}
	validUpdate := postAdminForm(t, client, testServer.URL, "/admin/projects/save", "/admin/projects/"+strconv.Itoa(seekino.ID), url.Values{
		"id":          {strconv.Itoa(seekino.ID)},
		"name":        {"Seekino Updated"},
		"slug":        {"seekino"},
		"summary":     {"更新后的项目简介。"},
		"cover_url":   {seekino.CoverURL},
		"project_url": {seekino.ProjectURL},
		"sort":        {"10"},
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
