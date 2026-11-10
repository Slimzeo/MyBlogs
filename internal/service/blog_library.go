package service

import (
	"myblog/internal/model"

	"gorm.io/gorm"
)

// BlogColumn is a category overview. Only the latest article's metadata is
// needed for the directory; long article bodies stay out of this query.
type BlogColumn struct {
	Name, Description, LatestTitle, LatestStatus string
	Count, LatestDate                            int
}

func (s *Service) BlogColumns(includeEncrypted bool) ([]BlogColumn, error) {
	statuses := publicArticleStatuses(includeEncrypted)
	sizes := s.db.Table("t_relationships AS r").Select("r.mid, COUNT(*) AS entries").
		Joins("JOIN t_contents AS c ON c.cid = r.cid").
		Where("c.type = ? AND c.status IN ?", model.TypeArticle, statuses).Group("r.mid")
	latest := s.db.Table("t_relationships AS r").Select("c.cid").
		Joins("JOIN t_contents AS c ON c.cid = r.cid").
		Where("r.mid = m.mid AND c.type = ? AND c.status IN ?", model.TypeArticle, statuses).
		Order("c.display_time DESC, c.created DESC, c.cid DESC").Limit(1)
	var cols []BlogColumn
	err := s.db.Table("t_metas AS m").
		Select("m.name, m.description, volume.entries AS count, recent.title AS latest_title, recent.status AS latest_status, recent.display_time AS latest_date").
		Joins("JOIN (?) AS volume ON volume.mid = m.mid", sizes).
		Joins("JOIN t_contents AS recent ON recent.cid = (?)", latest).
		Where("m.type = ?", model.TypeCategory).
		Order("CASE WHEN m.name = '默认分类' THEN 1 ELSE 0 END, m.sort DESC, volume.entries DESC, m.name, m.mid").
		Scan(&cols).Error
	return cols, err
}

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
	if err := scope.Order(articleDisplayOrder).Limit(size).Offset((page - 1) * size).Find(&posts).Error; err != nil {
		return nil, err
	}
	return NewPageInfo(posts, page, size, total), nil
}
