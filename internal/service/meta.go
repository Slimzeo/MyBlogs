package service

import (
	"strings"

	"myblog/internal/model"
)

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

func (s *Service) countArticlesByMeta(mid int) int {
	var count int64
	s.db.Table("t_contents a").
		Joins("left join t_relationships b on a.cid = b.cid").
		Where("b.mid = ? AND a.status = ? AND a.type = ?", mid, model.TypePublish, model.TypeArticle).
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

// DeleteMeta removes a meta and rewrites the tags/categories of its posts.
// Mirrors MetaServiceImpl.delete.
func (s *Service) DeleteMeta(mid int) error {
	var meta model.Meta
	if err := s.db.First(&meta, mid).Error; err != nil {
		return nil // not found: nothing to do
	}
	typ, name := meta.Type, meta.Name

	var affected []model.Content
	err := s.db.Transaction(func(tx txLike) error {
		if err := tx.Delete(&model.Meta{}, mid).Error; err != nil {
			return err
		}
		var rels []model.Relationship
		tx.Where("mid = ?", mid).Find(&rels)
		for _, r := range rels {
			var content model.Content
			if err := tx.First(&content, r.Cid).Error; err != nil {
				continue
			}
			affected = append(affected, content)
			updates := map[string]interface{}{}
			if typ == model.TypeCategory {
				updates["categories"] = reMeta(name, content.Categories)
			}
			if typ == model.TypeTag {
				updates["tags"] = reMeta(name, content.Tags)
			}
			if len(updates) > 0 {
				tx.Model(&model.Content{}).Where("cid = ?", r.Cid).Updates(updates)
			}
		}
		return tx.Where("mid = ?", mid).Delete(&model.Relationship{}).Error
	})
	if err == nil {
		for index := range affected {
			s.invalidateContent(&affected[index])
		}
	}
	return err
}

// SaveOrRenameCategory creates a category, or renames an existing one (by mid)
// propagating the new name to posts. Mirrors saveMeta(type,name,mid).
func (s *Service) SaveOrRenameCategory(typ, name string, mid int) error {
	if strings.TrimSpace(typ) == "" || strings.TrimSpace(name) == "" {
		return nil
	}
	var existing []model.Meta
	s.db.Where("type = ? AND name = ?", typ, name).Find(&existing)
	if len(existing) != 0 {
		return Tip("已经存在该项")
	}
	if mid != 0 {
		var original model.Meta
		if err := s.db.First(&original, mid).Error; err != nil {
			return err
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
				categories := replaceMeta(original.Name, name, content.Categories)
				if err := tx.Model(&model.Content{}).Where("cid = ?", content.Cid).
					Update("categories", categories).Error; err != nil {
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
	meta := model.Meta{Name: name, Type: typ}
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
	for _, name := range strings.Split(names, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if err := s.saveOrUpdateMetaTx(tx, cid, name, typ); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) saveOrUpdateMetaTx(tx txLike, cid int, name, typ string) error {
	meta := model.Meta{Slug: name, Name: name, Type: typ}
	if err := tx.Where("type = ? AND name = ?", typ, name).FirstOrCreate(&meta).Error; err != nil {
		return err
	}
	mid := meta.Mid
	if mid != 0 {
		var count int64
		tx.Model(&model.Relationship{}).Where("cid = ? AND mid = ?", cid, mid).Count(&count)
		if count == 0 {
			return tx.Create(&model.Relationship{Cid: cid, Mid: mid}).Error
		}
	}
	return nil
}

// reMeta removes `name` from a comma-separated list. Mirrors MetaServiceImpl.reMeta.
func reMeta(name, metas string) string {
	parts := strings.Split(metas, ",")
	var kept []string
	for _, m := range parts {
		if m != name && m != "" {
			kept = append(kept, m)
		}
	}
	return strings.Join(kept, ",")
}

func replaceMeta(oldName, newName, metas string) string {
	parts := strings.Split(metas, ",")
	for index := range parts {
		if strings.TrimSpace(parts[index]) == oldName {
			parts[index] = newName
		}
	}
	return strings.Join(parts, ",")
}
