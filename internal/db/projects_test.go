package db

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"myblog/config"
	"myblog/internal/model"

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
	for _, project := range seeded {
		if project.CoverURL == "" || strings.Contains(project.Notes, project.CoverURL) {
			t.Fatalf("seeded project %s duplicates its editable cover in notes, or has no cover", project.Slug)
		}
	}
	var marker model.Option
	if err := first.Where("name = ?", projectSeedMarker).First(&marker).Error; err != nil {
		t.Fatal("project seed marker was not written")
	}

	if err := first.Model(&model.Project{}).Where("slug = ?", "seekino").
		Updates(map[string]any{"name": "Seekino Edited"}).Error; err != nil {
		t.Fatal(err)
	}
	closeDatabase(first)

	second := openDatabase()
	t.Cleanup(func() { closeDatabase(second) })
	var edited model.Project
	if err := second.Where("slug = ?", "seekino").First(&edited).Error; err != nil {
		t.Fatal(err)
	}
	if edited.Name != "Seekino Edited" {
		t.Fatalf("reopening the database overwrote the edit: %q", edited.Name)
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
