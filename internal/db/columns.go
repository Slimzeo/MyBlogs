package db

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"myblog/internal/model"
)

// Older editors stored a CSV plus category relationships. Repair only posts
// with missing, multiple or inconsistent assignments. Never rewrite tag edges.
func migrateArticleColumns(database *gorm.DB) error {
	const relations = "SELECT 1 FROM t_relationships r JOIN t_metas m ON m.mid = r.mid WHERE r.cid = t_contents.cid AND m.type = ? AND "
	return database.Transaction(func(tx *gorm.DB) error {
		var lastID int
		for {
			var batch []model.Content
			query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("cid", "categories").
				Where("type = ? AND cid > ?", model.TypeArticle, lastID).
				Where("categories IS NULL OR TRIM(categories) = '' OR LENGTH(categories) <> LENGTH(TRIM(categories)) OR categories LIKE ? OR categories LIKE ? OR NOT EXISTS ("+relations+"m.name = t_contents.categories) OR EXISTS ("+relations+"m.name <> t_contents.categories)", "%,%", "%，%", model.TypeCategory, model.TypeCategory).
				Order("cid").Limit(100)
			if err := query.Find(&batch).Error; err != nil {
				return err
			}
			if len(batch) == 0 {
				return nil
			}
			for _, post := range batch {
				if err := repairArticleColumn(tx, post); err != nil {
					return err
				}
			}
			lastID = batch[len(batch)-1].Cid
		}
	})
}

func repairArticleColumn(tx *gorm.DB, post model.Content) error {
	var linked []string
	if err := tx.Table("t_metas m").Joins("JOIN t_relationships r ON r.mid=m.mid").
		Where("r.cid=? AND m.type=?", post.Cid, model.TypeCategory).Order("m.mid").Pluck("m.name", &linked).Error; err != nil {
		return err
	}
	// Preserve the author's order; relationships recover old rows with blank CSV.
	names := model.MetaNames(post.Categories + "," + strings.Join(linked, ","))
	name := model.DefaultColumn
	for _, candidate := range names {
		if candidate != model.DefaultColumn {
			name = candidate
			break
		}
	}
	column := model.Meta{Type: model.TypeCategory, Name: name, Slug: name}
	if err := tx.Where("type=? AND name=?", model.TypeCategory, name).FirstOrCreate(&column).Error; err != nil {
		return err
	}
	categoryIDs := tx.Model(&model.Meta{}).Select("mid").Where("type=?", model.TypeCategory)
	if err := tx.Where("cid=? AND mid<>? AND mid IN (?)", post.Cid, column.Mid, categoryIDs).Delete(&model.Relationship{}).Error; err != nil {
		return err
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.Relationship{Cid: post.Cid, Mid: column.Mid}).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.Content{}).Where("cid=?", post.Cid).Update("categories", name).Error; err != nil {
		return err
	}
	if len(names) > 1 {
		entry := model.Log{Action: "归并文章专栏", Data: fmt.Sprintf("#%d [%s] → %s", post.Cid, strings.Join(names, ", "), name), Created: int(time.Now().Unix())}
		return tx.Create(&entry).Error
	}
	return nil
}
