package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"myblog/config"
	"myblog/internal/model"

	"github.com/glebarez/sqlite" // pure-Go sqlite driver (no cgo)
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Open connects to the database, tunes the connection pool for high
// concurrency, runs auto-migration and seeds first-run data.
func Open(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch cfg.DBDriver {
	case "mysql":
		dialector = mysql.Open(cfg.DBDSN)
	default:
		databasePath := cfg.DBDSN
		if separator := index(databasePath, '?'); separator >= 0 {
			databasePath = databasePath[:separator]
		}
		if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite directory: %w", err)
		}
		dialector = sqlite.Open(cfg.DBDSN)
	}

	gdb, err := gorm.Open(dialector, &gorm.Config{
		Logger:                 gormlogger.Default.LogMode(gormlogger.Error),
		SkipDefaultTransaction: true, // faster; we open explicit tx where needed
		PrepareStmt:            true, // cache prepared statements for throughput
	})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	maxOpenConns := cfg.DBMaxOpenConns
	maxIdleConns := cfg.DBMaxIdleConns
	if cfg.DBDriver == "sqlite" {
		maxOpenConns = min(maxOpenConns, 20)
		maxIdleConns = min(maxIdleConns, 10)
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.DBConnMaxLifetime)

	if err := autoMigrate(gdb); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := seed(gdb, cfg); err != nil {
		return nil, fmt.Errorf("seed: %w", err)
	}
	return gdb, nil
}

func index(value string, target byte) int {
	for position := range len(value) {
		if value[position] == target {
			return position
		}
	}
	return -1
}

func autoMigrate(gdb *gorm.DB) error {
	return gdb.AutoMigrate(
		&model.Content{},
		&model.Comment{},
		&model.Meta{},
		&model.User{},
		&model.Option{},
		&model.Relationship{},
		&model.Attach{},
		&model.Log{},
	)
}

// seed inserts a default admin user, site options and a welcome article on the
// first run so the app is immediately usable. Mirrors the tale.sql fixture data.
func seed(gdb *gorm.DB, cfg *config.Config) error {
	var userCount int64
	if err := gdb.Model(&model.User{}).Count(&userCount).Error; err != nil {
		return err
	}
	if userCount == 0 {
		username := strings.TrimSpace(cfg.AdminUsername)
		email := strings.TrimSpace(cfg.AdminEmail)
		password := cfg.AdminInitialPassword
		if username == "" || email == "" || password == "" {
			return fmt.Errorf("fresh database requires BLOG_ADMIN_USERNAME, BLOG_ADMIN_EMAIL and BLOG_ADMIN_INITIAL_PASSWORD")
		}
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		admin := model.User{
			Username:   username,
			Password:   string(passwordHash),
			Email:      email,
			ScreenName: username,
			Created:    int(time.Now().Unix()),
			GroupName:  "visitor",
		}
		if err := gdb.Create(&admin).Error; err != nil {
			return err
		}
		log.Printf("[seed] created administrator account from deployment configuration")
	}

	var optCount int64
	if err := gdb.Model(&model.Option{}).Count(&optCount).Error; err != nil {
		return err
	}
	if optCount == 0 {
		opts := []model.Option{
			{Name: "site_title", Value: "HypN0s-Cloud"},
			{Name: "site_keywords", Value: "Blog"},
			{Name: "site_description", Value: "Go + Gin + GORM 搭建的高并发博客系统"},
			{Name: "site_theme", Value: "default"},
			{Name: "site_url", Value: ""},
			{Name: "theme_slogan", Value: "山水有相逢"},
			{Name: "theme_home_banner", Value: "/user/img/forest.jpg"},
			{Name: "theme_post_banner", Value: "/user/img/forest.jpg"},
			{Name: "theme_page_banner", Value: "/user/img/forest.jpg"},
			{Name: "theme_font", Value: "wenkai"},
			{Name: "theme_music_url", Value: ""},
			{Name: "theme_music_title", Value: "留一首歌给今天"},
			{Name: "social_github", Value: "https://github.com/"},
			{Name: "social_weibo", Value: ""},
			{Name: "social_zhihu", Value: ""},
			{Name: "social_twitter", Value: ""},
			{Name: "allow_install", Value: ""},
		}
		if err := gdb.Create(&opts).Error; err != nil {
			return err
		}
	}
	if err := migrateThemeBanners(gdb); err != nil {
		return err
	}
	if err := migrateSiteTitle(gdb); err != nil {
		return err
	}

	var contentCount int64
	if err := gdb.Model(&model.Content{}).Count(&contentCount).Error; err != nil {
		return err
	}
	if contentCount == 0 {
		now := int(time.Now().Unix())
		welcome := model.Content{
			Title:        "欢迎使用 Go My-Blog",
			Slug:         "welcome",
			Created:      now,
			Modified:     now,
			Content:      welcomeMarkdown,
			AuthorID:     1,
			Type:         model.TypeArticle,
			Status:       model.TypePublish,
			Tags:         "Go,Blog",
			Categories:   "默认分类",
			AllowComment: true,
			AllowPing:    true,
			AllowFeed:    true,
		}
		about := model.Content{
			Title:        "关于",
			Slug:         "about",
			Created:      now,
			Modified:     now,
			Content:      aboutMarkdown,
			AuthorID:     1,
			Type:         model.TypePage,
			Status:       model.TypePublish,
			AllowComment: true,
			AllowPing:    true,
			AllowFeed:    true,
		}
		if err := gdb.Create(&welcome).Error; err != nil {
			return err
		}
		if err := gdb.Create(&about).Error; err != nil {
			return err
		}
		// seed the category/tag metas + relationships for the welcome post
		seedMetas(gdb, welcome.Cid, "Go,Blog", model.TypeTag)
		seedMetas(gdb, welcome.Cid, "默认分类", model.TypeCategory)
	}
	if err := migrateAboutPage(gdb); err != nil {
		return err
	}

	return nil
}

func migrateThemeBanners(gdb *gorm.DB) error {
	return gdb.Model(&model.Option{}).
		Where(
			"name IN ? AND (value = ? OR value = '')",
			[]string{"theme_home_banner", "theme_post_banner", "theme_page_banner"},
			"/user/img/blog-banner.jpg",
		).
		Update("value", "/user/img/forest.jpg").Error
}

func migrateSiteTitle(gdb *gorm.DB) error {
	return gdb.Model(&model.Option{}).
		Where("name = ? AND (value = ? OR value = '')", "site_title", "My Blog").
		Update("value", "HypN0s-Cloud").Error
}

func migrateAboutPage(gdb *gorm.DB) error {
	var about model.Content
	err := gdb.Where("slug = ? AND type = ?", "about", model.TypePage).First(&about).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	if strings.TrimSpace(about.Content) != strings.TrimSpace(legacyAboutMarkdown) {
		return nil
	}
	return gdb.Model(&model.Content{}).
		Where("cid = ?", about.Cid).
		Updates(map[string]interface{}{
			"content":  aboutMarkdown,
			"modified": int(time.Now().Unix()),
		}).Error
}

func seedMetas(gdb *gorm.DB, cid int, names, typ string) {
	for _, name := range splitComma(names) {
		meta := model.Meta{Name: name, Slug: name, Type: typ}
		gdb.Where("type = ? AND name = ?", typ, name).FirstOrCreate(&meta)
		if meta.Mid != 0 {
			gdb.Create(&model.Relationship{Cid: cid, Mid: meta.Mid})
		}
	}
}

const welcomeMarkdown = `## 欢迎 👋

这是一个从 **Java (Spring Boot + MyBatis + Thymeleaf)** 完整迁移到 **Go (Gin + GORM + html/template)** 的博客系统。

### 特性

+ 高并发：Goroutine 天然并发模型 + 连接池调优 + 分片内存缓存
+ 零依赖启动：默认使用纯 Go 的 SQLite，无需外部数据库
+ 可切换 MySQL：设置 ` + "`BLOG_DB_DRIVER=mysql`" + ` 即可复用原 tale 数据库

`

const legacyAboutMarkdown = "## 关于本站\n\n这是由 Java Spring Boot 博客迁移而来的 Go 版本，使用 Gin + GORM 构建，支持高并发访问。"

const aboutMarkdown = `Hi, 这里是Hypnos

一个低能量,低精力的鼠鼠

目前在某宇宙厂实习

喜欢音乐,少女乐队番拥护者

在吉他,钢琴层面是横着的凶

目前在艰难向AI Infra和LLM相关知识蠕动`

func splitComma(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
