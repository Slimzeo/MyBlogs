package service

import (
	"strings"

	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"myblog/internal/model"
)

// GetPublishedTopicGroups counts visible articles and keeps only a few linked
// titles for each disclosure. Article bodies are loaded on the reading page.
func (s *Service) GetPublishedTopicGroups(previewSize int, includeEncrypted bool) ([]model.TopicGroup, error) {
	statuses := publicArticleStatuses(includeEncrypted)
	var metas []model.MetaDto
	err := s.db.Table("t_metas AS m").
		Select("m.*, COUNT(*) AS count").
		Joins("JOIN t_relationships AS r ON r.mid = m.mid").
		Joins("JOIN t_contents AS c ON c.cid = r.cid").
		Where("m.type = ? AND c.type = ? AND c.status IN ?", model.TypeCategory, model.TypeArticle, statuses).
		Group("m.mid").
		Order("CASE WHEN m.name = '默认分类' THEN 1 ELSE 0 END, m.sort DESC, count DESC, m.name, m.mid").
		Scan(&metas).Error
	if err != nil || len(metas) == 0 {
		return nil, err
	}
	groups := make([]model.TopicGroup, len(metas))
	index := make(map[int]int, len(metas))
	for i, meta := range metas {
		index[meta.Mid] = i
		groups[i].Meta = meta
	}
	rows, err := s.db.Table("t_contents AS c").
		Select("r.mid, c.cid, COALESCE(c.title, ''), COALESCE(c.slug, ''), c.status").
		Joins("JOIN t_relationships AS r ON r.cid = c.cid").
		Joins("JOIN t_metas AS m ON m.mid = r.mid").
		Where("m.type = ? AND c.type = ? AND c.status IN ?", model.TypeCategory, model.TypeArticle, statuses).
		Order("r.mid, c.display_time DESC, c.created DESC, c.cid DESC").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var mid int
		var post model.Content
		if err := rows.Scan(&mid, &post.Cid, &post.Title, &post.Slug, &post.Status); err != nil {
			return nil, err
		}
		if i, ok := index[mid]; ok && len(groups[i].Articles) < previewSize {
			groups[i].Articles = append(groups[i].Articles, post)
		}
	}
	return groups, rows.Err()
}

// GetMeta returns a category/tag with its article count by type+name.
// Mirrors MetaServiceImpl.getMeta + selectDtoByNameAndType.
func (s *Service) GetMeta(typ, name string) *model.MetaDto {
	if strings.TrimSpace(typ) == "" || strings.TrimSpace(name) == "" {
		return nil
	}
	var dto model.MetaDto
	err := s.db.Table("t_metas a").
		Select("a.*, count(b.cid) as count").
		Joins("left join t_relationships b on a.mid = b.mid").
		Where("a.type = ? AND a.name = ?", typ, name).
		Group("a.mid").
		Scan(&dto).Error
	if err != nil || dto.Mid == 0 {
		return nil
	}
	return &dto
}

func (s *Service) countArticlesByMeta(mid int, includeEncrypted bool) int {
	var count int64
	s.db.Table("t_contents a").
		Joins("left join t_relationships b on a.cid = b.cid").
		Where("b.mid = ? AND a.status IN ? AND a.type = ?",
			mid, publicArticleStatuses(includeEncrypted), model.TypeArticle).
		Count(&count)
	return int(count)
}

// GetMetas lists metas of a type ordered by sort/mid. Mirrors getMetas.
func (s *Service) GetMetas(typ string) []model.Meta {
	if strings.TrimSpace(typ) == "" {
		return nil
	}
	var list []model.Meta
	s.db.Where("type = ?", typ).Order("sort desc, mid desc").Find(&list)
	return list
}

// GetMetaList lists metas of a type with article counts. Mirrors getMetaList/metas.
func (s *Service) GetMetaList(typ, orderBy string, limit int) []model.MetaDto {
	if strings.TrimSpace(typ) == "" {
		return nil
	}
	if strings.TrimSpace(orderBy) == "" {
		orderBy = "count desc, a.mid desc"
	}
	if limit < 1 || limit > model.MaxPosts {
		limit = 10
	}
	var list []model.MetaDto
	s.db.Table("t_metas a").
		Select("a.*, count(b.cid) as count").
		Joins("left join t_relationships b on a.mid = b.mid").
		Where("a.type = ?", typ).
		Group("a.mid").
		Order(orderBy).
		Limit(limit).
		Scan(&list)
	return list
}

// DeleteMeta moves articles from a removed column to the default in the same
// transaction. Removing a tag only detaches that tag.
func (s *Service) DeleteMeta(mid int) error {
	var affected []model.Content
	err := s.db.Transaction(func(tx txLike) error {
		var meta model.Meta
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&meta, mid).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := tx.Table("t_contents AS c").Select("c.*").
			Joins("JOIN t_relationships r ON r.cid=c.cid").Where("r.mid=?", mid).Scan(&affected).Error; err != nil {
			return err
		}
		for _, post := range affected {
			field, value := "tags", reMeta(meta.Name, post.Tags)
			switch meta.Type {
			case model.TypeCategory:
				field, value = "categories", reMeta(meta.Name, post.Categories)
				if post.Type == model.TypeArticle {
					if meta.Name == model.DefaultColumn {
						return Tip("默认分类里还有文章，请先将它们移到另一个专栏。")
					}
					value = model.DefaultColumn
					if err := s.saveOrUpdateMetaTx(tx, post.Cid, value, model.TypeCategory); err != nil {
						return err
					}
				}
			case model.TypeTag:
			default:
				continue
			}
			if err := tx.Model(&model.Content{}).Where("cid=?", post.Cid).Update(field, value).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("mid=?", mid).Delete(&model.Relationship{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Meta{}, mid).Error
	})
	if err == nil {
		for i := range affected {
			s.invalidateContent(&affected[i])
		}
	}
	return err
}

// SaveOrRenameCategory creates a category, or renames an existing one (by mid)
// propagating the new name to posts. Mirrors saveMeta(type,name,mid).
func (s *Service) SaveOrRenameCategory(typ, name string, mid int) error {
	name = strings.TrimSpace(name)
	if typ != model.TypeCategory && typ != model.TypeTag {
		return Tip("项目类型不合法")
	}
	if name == "" {
		return Tip("名称不能为空")
	}
	if strings.ContainsAny(name, ",，") {
		return Tip("名称不能包含逗号。")
	}
	var existing []model.Meta
	query := s.db.Where("type = ? AND name = ?", typ, name)
	if mid != 0 {
		query = query.Where("mid <> ?", mid)
	}
	query.Find(&existing)
	if len(existing) != 0 {
		return Tip("已经存在该项")
	}
	if mid != 0 {
		var original model.Meta
		if err := s.db.First(&original, mid).Error; err != nil {
			return err
		}
		if original.Type != typ {
			return Tip("项目类型不匹配")
		}
		var affected []model.Content
		err := s.db.Transaction(func(tx txLike) error {
			if err := tx.Model(&model.Meta{}).Where("mid = ?", mid).
				Updates(map[string]any{"name": name, "slug": name}).Error; err != nil {
				return err
			}
			if err := tx.Table("t_contents a").
				Select("a.*").
				Joins("join t_relationships b on a.cid = b.cid").
				Where("b.mid = ?", mid).
				Scan(&affected).Error; err != nil {
				return err
			}
			for _, content := range affected {
				field := "categories"
				value := replaceMeta(original.Name, name, content.Categories)
				if typ == model.TypeTag {
					field = "tags"
					value = replaceMeta(original.Name, name, content.Tags)
				}
				if err := tx.Model(&model.Content{}).Where("cid = ?", content.Cid).
					Update(field, value).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err == nil {
			for index := range affected {
				s.invalidateContent(&affected[index])
			}
		}
		return err
	}
	meta := model.Meta{Name: name, Slug: name, Type: typ}
	return s.db.Create(&meta).Error
}

// SaveMeta inserts a meta as-is (used for link creation). Mirrors saveMeta(MetaVo).
func (s *Service) SaveMeta(m *model.Meta) error {
	if m == nil {
		return nil
	}
	return s.db.Create(m).Error
}

// UpdateMeta selectively updates a meta by mid (used for link edits). Mirrors update(MetaVo).
func (s *Service) UpdateMeta(m *model.Meta) error {
	if m == nil || m.Mid == 0 {
		return nil
	}
	updates := map[string]interface{}{}
	if m.Name != "" {
		updates["name"] = m.Name
	}
	if m.Slug != "" {
		updates["slug"] = m.Slug
	}
	if m.Type != "" {
		updates["type"] = m.Type
	}
	if m.Description != "" {
		updates["description"] = m.Description
	}
	updates["sort"] = m.Sort // sort is meaningful even at 0
	return s.db.Model(&model.Meta{}).Where("mid = ?", m.Mid).Updates(updates).Error
}

// saveMetasTx attaches comma-separated names of a type to a content id,
// creating metas/relationships as needed. Mirrors saveMetas + saveOrUpdate.
func (s *Service) saveMetasTx(tx txLike, cid int, names, typ string) error {
	if cid == 0 {
		return Tip("项目关联id不能为空")
	}
	if strings.TrimSpace(names) == "" || strings.TrimSpace(typ) == "" {
		return nil
	}
	for _, name := range model.MetaNames(names) {
		if err := s.saveOrUpdateMetaTx(tx, cid, name, typ); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) saveOrUpdateMetaTx(tx txLike, cid int, name, typ string) error {
	meta := model.Meta{Slug: name, Name: name, Type: typ}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("type = ? AND name = ?", typ, name).FirstOrCreate(&meta).Error; err != nil {
		return err
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.Relationship{Cid: cid, Mid: meta.Mid}).Error
}

// reMeta removes `name` from a comma-separated list. Mirrors MetaServiceImpl.reMeta.
func reMeta(name, metas string) string {
	var kept []string
	for _, m := range model.MetaNames(metas) {
		if m != name {
			kept = append(kept, m)
		}
	}
	return strings.Join(kept, ",")
}

func replaceMeta(oldName, newName, metas string) string {
	parts := model.MetaNames(metas)
	for index := range parts {
		if parts[index] == oldName {
			parts[index] = newName
		}
	}
	return strings.Join(parts, ",")
}

func normalizeMetaNames(value string) string {
	return strings.Join(model.MetaNames(value), ",")
}
