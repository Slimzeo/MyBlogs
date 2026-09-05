package service_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"myblog/config"
	"myblog/internal/cache"
	"myblog/internal/db"
	"myblog/internal/model"
	"myblog/internal/service"
)

func TestAPITokenLifecycle(t *testing.T) {
	directory := t.TempDir()
	runtimeConfig := &config.Config{
		DBDriver:             "sqlite",
		DBDSN:                filepath.Join(directory, "blog.db"),
		AdminUsername:        "admin",
		AdminEmail:           "admin@example.com",
		AdminInitialPassword: "temporary-test-password",
		UploadDir:            filepath.Join(directory, "upload"),
	}
	database, err := db.Open(runtimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	applicationCache := cache.New()
	t.Cleanup(func() {
		applicationCache.Close()
		sqlDB, _ := database.DB()
		_ = sqlDB.Close()
	})
	services := service.New(database, applicationCache, runtimeConfig)
	user := services.QueryUserByID(1)
	if user == nil {
		t.Fatal("seeded administrator missing")
	}
	record, plaintext, err := services.CreateAPIToken("local-agent", model.ScopeArticleImport, user.Uid, 30)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(plaintext, "mb_v1.") || strings.Contains(record.SecretHash, plaintext) || len(record.SecretHash) != 64 {
		t.Fatalf("unsafe token record/plaintext: %#v / %q", record, plaintext)
	}
	authenticated, err := services.AuthenticateAPIToken(plaintext, model.ScopeArticleImport)
	if err != nil || authenticated.ID != record.ID || authenticated.LastUsed == 0 {
		t.Fatalf("authenticate = %#v, %v", authenticated, err)
	}
	if _, err := services.AuthenticateAPIToken(plaintext, "site:write"); !errors.Is(err, service.ErrAPITokenForbidden) {
		t.Fatalf("wrong scope error = %v", err)
	}
	parts := strings.Split(plaintext, ".")
	if _, err := services.AuthenticateAPIToken(parts[0]+"."+parts[1]+".wrong-secret", model.ScopeArticleImport); !errors.Is(err, service.ErrInvalidAPIToken) {
		t.Fatalf("wrong secret error = %v", err)
	}
	tokens, err := services.APITokens(user.Uid)
	if err != nil || len(tokens) != 1 || tokens[0].SecretHash == "" {
		t.Fatalf("tokens = %#v, %v", tokens, err)
	}
	if err := services.RevokeAPIToken(record.ID, user.Uid); err != nil {
		t.Fatal(err)
	}
	if _, err := services.AuthenticateAPIToken(plaintext, model.ScopeArticleImport); !errors.Is(err, service.ErrInvalidAPIToken) {
		t.Fatalf("revoked token error = %v", err)
	}
}
