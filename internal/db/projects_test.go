package db

import (
	"path/filepath"
	"testing"
	"time"

	"myblog/config"
	"myblog/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestProjectSeedRunsOnceAndNeverRefillsADeletedGallery(t *testing.T) {
	runtimeConfig := &config.Config{
		DBDriver:             "sqlite",
		DBDSN:                filepath.Join(t.TempDir(), "projects.db"),
		DBMaxOpenConns:       1,
		DBMaxIdleConns:       1,
		DBConnMaxLifetime:    time.Minute,
		AdminUsername:        "project-admin",
		AdminEmail:           "projects@example.com",
		AdminInitialPassword: "temporary-project-password",
		UploadDir:            filepath.Join(t.TempDir(), "upload"),
	}
	openDatabase := func() *gorm.DB {
		t.Helper()
		database, err := Open(runtimeConfig)
		if err != nil {
			t.Fatal(err)
		}
		return database
	}
	closeDatabase := func(database *gorm.DB) {
		sqlDB, err := database.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}

	first := openDatabase()
	t.Cleanup(func() { closeDatabase(first) })
	var seeded []model.Project
	if err := first.Order("sort ASC, id ASC").Find(&seeded).Error; err != nil {
		t.Fatal(err)
	}
	if len(seeded) != 2 || seeded[0].Slug != "seekino" || seeded[1].Slug != "catendar" {
		t.Fatalf("seeded projects = %#v", seeded)
	}
	if seeded[0].CoverURL != seekinoCover || seeded[0].AnimationURL != seekinoAnimation || seeded[0].PlatformURL != seekinoPlatform || seeded[0].GitHubURL != seekinoRepo ||
		seeded[1].CoverURL != catendarCover || seeded[1].AnimationURL != "" || seeded[1].PlatformURL != "" || seeded[1].GitHubURL != catendarRepo ||
		seeded[0].ArticleID != 0 || seeded[1].ArticleID != 0 {
		t.Fatalf("incorrect default links or artwork: %#v", seeded)
	}
	var marker model.Option
	if err := first.Where("name = ?", projectSeedMarker).First(&marker).Error; err != nil {
		t.Fatal("project seed marker was not written")
	}

	if err := first.Model(&model.Project{}).Where("slug = ?", "seekino").
		Updates(map[string]any{"name": "Seekino Edited", "animation_url": ""}).Error; err != nil {
		t.Fatal(err)
	}
	closeDatabase(first)

	second := openDatabase()
	t.Cleanup(func() { closeDatabase(second) })
	var edited model.Project
	if err := second.Where("slug = ?", "seekino").First(&edited).Error; err != nil {
		t.Fatal(err)
	}
	if edited.Name != "Seekino Edited" || edited.AnimationURL != "" {
		t.Fatalf("reopening the database overwrote the edit: %#v", edited)
	}
	if err := second.Where("1 = 1").Delete(&model.Project{}).Error; err != nil {
		t.Fatal(err)
	}
	closeDatabase(second)

	third := openDatabase()
	t.Cleanup(func() { closeDatabase(third) })
	var count int64
	if err := third.Model(&model.Project{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("deleted gallery was reseeded with %d cards", count)
	}

	// A pre-existing project table without the marker is user data, not a
	// fresh install: mark it as seeded and leave the rows alone.
	if err := third.Where("name = ?", projectSeedMarker).Delete(&model.Option{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := third.Create(&model.Project{
		Slug: "custom-card", Name: "Custom", Summary: "Existing user data.", Sort: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	closeDatabase(third)

	fourth := openDatabase()
	t.Cleanup(func() { closeDatabase(fourth) })
	var existing []model.Project
	if err := fourth.Order("id ASC").Find(&existing).Error; err != nil {
		t.Fatal(err)
	}
	if len(existing) != 1 || existing[0].Slug != "custom-card" {
		t.Fatalf("existing project rows were replaced by the seed: %#v", existing)
	}
	var remadeMarker model.Option
	if err := fourth.Where("name = ?", projectSeedMarker).First(&remadeMarker).Error; err != nil {
		t.Fatal("existing database was not marked as seeded")
	}
}

func TestUpgradeLegacyProjectsPreservesAuthoredFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.AutoMigrate(&model.Option{}))
	must(db.Create(&model.Option{Name: projectSeedMarker, Value: "1"}).Error)
	// The previous release stored the generic destination in project_url and
	// kept the author's notes in a separate column.
	must(db.Exec(`CREATE TABLE t_projects (
		id integer PRIMARY KEY AUTOINCREMENT, slug text NOT NULL, name text NOT NULL, summary text NOT NULL,
		notes mediumtext, cover_url text, cover_alt text, project_url text,
		sort integer NOT NULL DEFAULT 0, created integer NOT NULL DEFAULT 0, modified integer NOT NULL DEFAULT 0)`).Error)
	insert := func(slug, url, cover, alt string) {
		t.Helper()
		must(db.Exec(`INSERT INTO t_projects (slug,name,summary,notes,project_url,cover_url,cover_alt)
			VALUES (?,?,?,?,?,?,?)`, slug, slug, "Edited caption", "Authored legacy text", url, cover, alt).Error)
	}
	insert("seekino", seekinoRepo, oldSeekinoCover, oldSeekinoAlt)
	insert("catendar", catendarRepo, oldCatendarCover, "Edited alt")
	insert("custom", "https://example.org/demo", "https://raw.githubusercontent.com/a/b/main/custom.svg", "Custom")
	insert("repo-only", "https://github.com/example/demo.git", "", "")
	insert("blank", "", "", "")
	must(db.AutoMigrate(&model.Project{}))
	must(seedProjects(db))
	load := func(slug string) model.Project {
		t.Helper()
		var card model.Project
		must(db.Where("slug = ?", slug).First(&card).Error)
		var notes string
		must(db.Table("t_projects").Select("notes").Where("id = ?", card.ID).Scan(&notes).Error)
		if card.Summary != "Edited caption" || notes != "Authored legacy text" || card.ArticleID != 0 {
			t.Fatalf("authored content changed: %#v", card)
		}
		return card
	}
	if c := load("seekino"); c.PlatformURL != seekinoPlatform || c.GitHubURL != seekinoRepo || c.CoverURL != seekinoCover || c.CoverAlt != seekinoAlt || c.AnimationURL != seekinoAnimation {
		t.Fatalf("seekino upgrade: %#v", c)
	}
	if c := load("catendar"); c.PlatformURL != "" || c.GitHubURL != catendarRepo || c.CoverURL != catendarCover || c.CoverAlt != "Edited alt" {
		t.Fatalf("catendar upgrade: %#v", c)
	}
	if c := load("custom"); c.PlatformURL != "https://example.org/demo" || c.GitHubURL != "" || c.CoverAlt != "Custom" || c.CoverURL != "https://raw.githubusercontent.com/a/b/main/custom.svg" {
		t.Fatalf("custom card changed: %#v", c)
	}
	if c := load("repo-only"); c.GitHubURL != "https://github.com/example/demo" || c.PlatformURL != "" || c.CoverURL != "" {
		t.Fatalf("old repository URL lost: %#v", c)
	}
	if c := load("blank"); c.GitHubURL != "" || c.PlatformURL != "" || c.CoverURL != "" {
		t.Fatalf("empty fields overwritten: %#v", c)
	}
	// An older card whose owner chose other artwork keeps that artwork on upgrade.
	const customCover = "https://raw.githubusercontent.com/example/brand/main/custom.png"
	must(db.Where("name = ?", seekinoMotionMarker).Delete(&model.Option{}).Error)
	must(db.Model(&model.Project{}).Where("slug = ?", "seekino").
		Updates(map[string]any{"cover_url": customCover, "animation_url": ""}).Error)
	must(seedProjects(db))
	if c := load("seekino"); c.CoverURL != customCover || c.AnimationURL != "" {
		t.Fatalf("animation migration changed custom artwork: %#v", c)
	}
	// A database that ran the earlier links upgrade already has independent
	// destinations. Relocating the official logo must only update the URL.
	must(db.Where("name = ?", seekinoLogoMarker).Delete(&model.Option{}).Error)
	must(db.Model(&model.Project{}).Where("slug = ?", "seekino").Updates(map[string]any{
		"cover_url": formerSeekinoLogo, "cover_alt": "My whale", "project_url": "https://example.org/preview", "github_url": "",
	}).Error)
	must(seedProjects(db))
	if c := load("seekino"); c.CoverURL != seekinoCover || c.CoverAlt != "My whale" || c.PlatformURL != "https://example.org/preview" || c.GitHubURL != "" {
		t.Fatalf("moving the logo altered another field: %#v", c)
	}

	must(db.Model(&model.Project{}).Where("slug = ?", "seekino").Updates(map[string]any{"project_url": "", "github_url": "", "cover_url": oldSeekinoCover}).Error)
	must(seedProjects(db))
	if c := load("seekino"); c.PlatformURL != "" || c.GitHubURL != "" || c.CoverURL != oldSeekinoCover {
		t.Fatalf("reopening overwrote owner edits: %#v", c)
	}
}
