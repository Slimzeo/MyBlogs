package db

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"myblog/config"
	"myblog/internal/model"
)

func TestMigrateArticleColumnsPreservesTagsAndOriginalChoices(t *testing.T) {
	cfg := &config.Config{DBDriver: "sqlite", DBDSN: filepath.Join(t.TempDir(), "legacy.db"), DBMaxOpenConns: 1, DBMaxIdleConns: 1, AdminUsername: "test", AdminEmail: "test@example.com", AdminInitialPassword: "column-test-password"}
	db, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	create := func(value any) {
		t.Helper()
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	legacy := model.Content{Type: model.TypeArticle, Status: model.TypeDraft, Title: "Legacy", Categories: "默认分类，读书,研究", Tags: "研究,Go"}
	create(&legacy)
	for _, item := range []struct{ kind, name string }{{model.TypeCategory, "研究"}, {model.TypeCategory, "读书"}, {model.TypeTag, "研究"}, {model.TypeTag, "Go"}} {
		m := model.Meta{Name: item.name, Slug: item.name, Type: item.kind}
		if err := db.Where("type=? AND name=?", item.kind, item.name).FirstOrCreate(&m).Error; err != nil {
			t.Fatal(err)
		}
		create(&model.Relationship{Cid: legacy.Cid, Mid: m.Mid})
	}
	blank := model.Content{Type: model.TypeArticle, Status: model.TypePrivate, Title: "Unassigned"}
	create(&blank)
	linked := model.Content{Type: model.TypeArticle, Status: model.TypeEncrypted, Title: "Relationship only"}
	create(&linked)
	var research model.Meta
	if err := db.Where("type=? AND name=?", model.TypeCategory, "研究").First(&research).Error; err != nil {
		t.Fatal(err)
	}
	create(&model.Relationship{Cid: linked.Cid, Mid: research.Mid})
	for i := 0; i < 2; i++ {
		if err := migrateArticleColumns(db); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		id     int
		column string
	}{{legacy.Cid, "读书"}, {blank.Cid, model.DefaultColumn}, {linked.Cid, "研究"}} {
		var got model.Content
		if err := db.First(&got, tc.id).Error; err != nil || got.Categories != tc.column {
			t.Fatalf("cid %d got %q, %v", tc.id, got.Categories, err)
		}
		var columns []string
		if err := db.Table("t_metas m").Joins("JOIN t_relationships r ON r.mid=m.mid").Where("r.cid=? AND m.type=?", tc.id, model.TypeCategory).Pluck("m.name", &columns).Error; err != nil || !reflect.DeepEqual(columns, []string{tc.column}) {
			t.Fatalf("cid %d linked columns %v, %v", tc.id, columns, err)
		}
	}
	var tags []string
	if err := db.Table("t_metas m").Joins("JOIN t_relationships r ON r.mid=m.mid").Where("r.cid=? AND m.type=?", legacy.Cid, model.TypeTag).Order("m.name").Pluck("m.name", &tags).Error; err != nil || !reflect.DeepEqual(tags, []string{"Go", "研究"}) {
		t.Fatalf("legacy tags %v, %v", tags, err)
	}
	var audit []model.Log
	if err := db.Where("action=?", "归并文章专栏").Find(&audit).Error; err != nil || len(audit) != 1 || !strings.Contains(audit[0].Data, "默认分类, 读书, 研究") {
		t.Fatalf("expected one original assignment record, got %v, %v", audit, err)
	}
}
