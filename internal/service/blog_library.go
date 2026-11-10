package service

import (
	"myblog/internal/model"

	"gorm.io/gorm"
)

func (s *Service) blogScope(category, tag string, includeEncrypted bool) *gorm.DB {
	q := s.db.Model(&model.Content{}).Where("type = ? AND status IN ?", model.TypeArticle, publicArticleStatuses(includeEncrypted))
	for _, filter := range []struct{ kind, name string }{{model.TypeCategory, category}, {model.TypeTag, tag}} {
		if filter.name != "" {
			ids := s.db.Table("t_relationships AS r").Select("r.cid").
				Joins("JOIN t_metas AS m ON m.mid = r.mid").
				Where("m.type = ? AND m.name = ?", filter.kind, filter.name)
			q = q.Where("cid IN (?)", ids)
		}
	}
	return q
}

// Tag counts describe the selected column (or all columns) before applying
// the tag. Changing tags thus never silently discards the column selection.
func (s *Service) BlogTags(category string, includeEncrypted bool) ([]model.MetaDto, error) {
	ids := s.blogScope(category, "", includeEncrypted).Select("cid")
	var tags []model.MetaDto
	err := s.db.Table("t_metas AS m").Select("m.mid, m.name, COUNT(*) AS count").
		Joins("JOIN t_relationships AS r ON r.mid = m.mid").
		Where("m.type = ? AND r.cid IN (?)", model.TypeTag, ids).
		Group("m.mid, m.name").Order("count DESC, m.name, m.mid").Scan(&tags).Error
	return tags, err
}

func (s *Service) BlogArticles(category, tag string, page, size int, includeEncrypted bool) (*PageInfo[model.Content], error) {
	var total int64
	scope := s.blogScope(category, tag, includeEncrypted)
	if err := scope.Count(&total).Error; err != nil {
		return nil, err
	}
	var posts []model.Content
	if err := scope.Select("cid", "title", "slug", "display_time", "created", "status", "categories").
		Order(articleDisplayOrder).Limit(size).Offset((page - 1) * size).Find(&posts).Error; err != nil {
		return nil, err
	}
	return NewPageInfo(posts, page, size, total), nil
}
