package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPrepareMarkdownArticlePackagesFrontMatterAndAssets(t *testing.T) {
	directory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(directory, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "images", "pixel.png"), []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "paper.pdf"), []byte("%PDF-1.4\nexample"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(directory, "agent-notes.markdown")
	markdown := `---
title: Agent 阅读笔记
slug: agent-reading-notes
tags: [Agent, Paper, Agent]
categories: 研究, 学习
display_time: "2026-09-05T21:30"
status: draft
---
# 正文

![像素](images/pixel.png?raw=1)
[论文](paper.pdf#page=2)
[下一篇](next.md)
`
	if err := os.WriteFile(source, []byte(markdown), 0o644); err != nil {
		t.Fatal(err)
	}
	upload, err := prepareArticle(source, articleOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if upload.Filename != "agent-notes.zip" || upload.Format != "markdown" {
		t.Fatalf("upload identity = %q/%q", upload.Filename, upload.Format)
	}
	if upload.Metadata.Title != "Agent 阅读笔记" || upload.Metadata.Tags != "Agent,Paper" ||
		upload.Metadata.Categories != "研究,学习" || upload.Metadata.DisplayTime != "2026-09-05T21:30" {
		t.Fatalf("metadata = %#v", upload.Metadata)
	}
	if !reflect.DeepEqual(upload.Assets, []string{"images/pixel.png", "paper.pdf"}) {
		t.Fatalf("assets = %#v", upload.Assets)
	}
	reader, err := zip.NewReader(bytes.NewReader(upload.Data), int64(len(upload.Data)))
	if err != nil {
		t.Fatal(err)
	}
	entries := make(map[string]string)
	for _, file := range reader.File {
		opened, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		data, readErr := io.ReadAll(opened)
		_ = opened.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		entries[file.Name] = string(data)
	}
	if _, exists := entries["agent-notes.md"]; !exists {
		t.Fatalf("ZIP entries = %#v", entries)
	}
	if strings.Contains(entries["agent-notes.md"], "title: Agent") || !strings.Contains(entries["agent-notes.md"], "# 正文") {
		t.Fatalf("front matter was not stripped: %q", entries["agent-notes.md"])
	}
	if first, second := articleIdempotencyKey(upload), articleIdempotencyKey(upload); first != second || len(first) != 64 {
		t.Fatalf("idempotency keys = %q/%q", first, second)
	}
}

func TestPrepareMarkdownArticleRejectsPublishAndEscapingAsset(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "post.md")
	if err := os.WriteFile(source, []byte("---\nstatus: publish\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareArticle(source, articleOverrides{}); err == nil || !strings.Contains(err.Error(), "只允许导入草稿") {
		t.Fatalf("publish error = %v", err)
	}

	child := filepath.Join(directory, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "secret.pdf"), []byte("%PDF-1.4"), 0o644); err != nil {
		t.Fatal(err)
	}
	escaping := filepath.Join(child, "post.md")
	if err := os.WriteFile(escaping, []byte("[secret](../secret.pdf)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareArticle(escaping, articleOverrides{}); err == nil || !strings.Contains(err.Error(), "路径不安全") {
		t.Fatalf("escaping asset error = %v", err)
	}
}

func TestConfigRoundTripAndServerSafety(t *testing.T) {
	path := filepath.Join(t.TempDir(), "myblogs", "credentials.json")
	t.Setenv("BLOGCTL_CONFIG", path)
	t.Setenv("BLOGCTL_SERVER", "")
	t.Setenv("BLOGCTL_TOKEN", "")
	if err := saveConfig(cliConfig{Server: "http://127.0.0.1:8081/", Token: "mb_v1.example.secret"}); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Server != "http://127.0.0.1:8081" || config.Token != "mb_v1.example.secret" {
		t.Fatalf("config = %#v", config)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config permissions = %o", info.Mode().Perm())
	}
	for _, unsafe := range []string{"http://example.com", "https://example.com/admin", "https://user:pass@example.com"} {
		if _, err := normalizeServer(unsafe); err == nil {
			t.Fatalf("unsafe server accepted: %s", unsafe)
		}
	}
}
