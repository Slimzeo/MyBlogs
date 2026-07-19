package service

import (
	"strconv"
	"strings"

	"myblog/internal/model"
	"myblog/internal/util"
)

// Publish creates a new article/page. Mirrors ContentServiceImpl.publish.
func (s *Service) Publish(c *model.Content) error {
	if c == nil {
		return Tip("文章对象为空")
	}
	if !validContentStatus(c.Type, c.Status) {
		return Tip("文章状态不合法")
	}
	if strings.TrimSpace(c.Title) == "" {
		return Tip("文章标题不能为空")
	}
	if strings.TrimSpace(c.Content) == "" {
		return Tip("文章内容不能为空")
	}
	if len([]rune(c.Title)) > model.MaxTitleCount {
		return Tip("文章标题过长")
	}
	if len([]rune(c.Content)) > model.MaxTextCount {
		return Tip("文章内容过长")
	}
	if c.AuthorID == 0 {
		return Tip("请登录后发布文章")
	}
	if strings.TrimSpace(c.Slug) != "" {
		if len(c.Slug) < 5 {
			return Tip("路径太短了")
		}
		if !util.IsPath(c.Slug) {
			return Tip("您输入的路径不合法")
		}
		var count int64
		s.db.Model(&model.Content{}).Where("type = ? AND slug = ?", c.Type, c.Slug).Count(&count)
		if count > 0 {
			return Tip("该路径已经存在，请重新输入")
		}
	} else {
		c.Slug = ""
	}

	now := util.CurrentUnixTime()
	c.Created = now
	c.Modified = now
	c.Hits = 0
	c.CommentsNum = 0

	tags := c.Tags
	categories := c.Categories

	// One transaction: insert content + attach its metas/relationships.
	err := s.db.Transaction(func(tx txLike) error {
		create := tx
		if c.Slug == "" {
			create = create.Omit("Slug")
		}
		if err := create.Create(c).Error; err != nil {
			return err
		}
		if err := s.saveMetasTx(tx, c.Cid, tags, model.TypeTag); err != nil {
			return err
		}
		return s.saveMetasTx(tx, c.Cid, categories, model.TypeCategory)
	})
	if err == nil {
		s.invalidateContent(c)
	}
	return err
}

// GetContents paginates published articles. Mirrors getContents(p, limit).
func (s *Service) GetContents(page, limit int) *PageInfo[model.Content] {
	key := "contents:" + strconv.FormatUint(s.contentListVersion.Load(), 10) +
		":" + strconv.Itoa(page) + ":" + strconv.Itoa(limit)
	if cached, exists := s.cache.Get(key); exists {
		return cached.(*PageInfo[model.Content])
	}
	value, _, _ := s.sf.Do(key, func() (any, error) {
		var total int64
		query := s.db.Model(&model.Content{}).
			Where("type = ? AND status = ?", model.TypeArticle, model.TypePublish)
		if err := query.Count(&total).Error; err != nil {
			return NewPageInfo([]model.Content{}, page, limit, 0), err
		}
		var data []model.Content
		err := s.db.Where("type = ? AND status = ?", model.TypeArticle, model.TypePublish).
			Order("created desc").Offset((page - 1) * limit).Limit(limit).Find(&data).Error
		result := NewPageInfo(data, page, limit, total)
		if err == nil {
			s.cache.Set(key, result, 10)
		}
		return result, err
	})
	return value.(*PageInfo[model.Content])
}

// GetContentByID fetches an article by numeric id or slug and increments hits
// for the numeric-id path. Mirrors getContents(String id).
func (s *Service) GetContentByID(id string) (*model.Content, error) {
	if strings.TrimSpace(id) == "" {
		return nil, nil
	}
	key := "content:" + id
	if cached, exists := s.cache.Get(key); exists {
		content := cached.(model.Content)
		return &content, nil
	}
	value, err, _ := s.sf.Do(key, func() (any, error) {
		var content model.Content
		if util.IsNumber(id) {
			cid, _ := strconv.Atoi(id)
			if err := s.db.First(&content, cid).Error; err != nil {
				return nil, nil
			}
		} else {
			var list []model.Content
			if err := s.db.Where("slug = ?", id).Limit(2).Find(&list).Error; err != nil {
				return nil, err
			}
			if len(list) == 0 {
				return nil, nil
			}
			if len(list) > 1 {
				return nil, Tip("query content by id and return is not one")
			}
			content = list[0]
		}
		s.cache.Set("content:"+strconv.Itoa(content.Cid), content, 30)
		if content.Slug != "" {
			s.cache.Set("content:"+content.Slug, content, 30)
		}
		return content, nil
	})
	if err != nil || value == nil {
		return nil, err
	}
	content := value.(model.Content)
	return &content, nil
}

// GetArticlesByMeta paginates published articles under a category/tag mid.
// Mirrors getArticles(mid, page, limit) + ContentVoMapper.findByCatalog.
func (s *Service) GetArticlesByMeta(mid, page, limit int) *PageInfo[model.Content] {
	total := s.countArticlesByMeta(mid)
	var list []model.Content
	s.db.Table("t_contents a").
		Select("a.*").
		Joins("left join t_relationships b on a.cid = b.cid").
		Where("b.mid = ? AND a.status = ? AND a.type = ?", mid, model.TypePublish, model.TypeArticle).
		Order("a.created desc").
		Offset((page - 1) * limit).Limit(limit).
		Scan(&list)
	return NewPageInfo(list, page, limit, int64(total))
}

// SearchArticles paginates published articles whose title matches keyword.
// Mirrors getArticles(keyword, page, limit).
func (s *Service) SearchArticles(keyword string, page, limit int) *PageInfo[model.Content] {
	like := "%" + keyword + "%"
	var total int64
	s.db.Model(&model.Content{}).
		Where("type = ? AND status = ? AND title LIKE ?", model.TypeArticle, model.TypePublish, like).
		Count(&total)
	var list []model.Content
	s.db.Where("type = ? AND status = ? AND title LIKE ?", model.TypeArticle, model.TypePublish, like).
		Order("created desc").Offset((page - 1) * limit).Limit(limit).Find(&list)
	return NewPageInfo(list, page, limit, total)
}

// ArticlesByTypePaged paginates content of a given type (used by admin lists).
// Mirrors getArticlesWithpage with a type criterion.
func (s *Service) ArticlesByTypePaged(typ string, page, limit int) *PageInfo[model.Content] {
	var total int64
	s.db.Model(&model.Content{}).Where("type = ?", typ).Count(&total)
	var list []model.Content
	s.db.Where("type = ?", typ).Order("created desc").
		Offset((page - 1) * limit).Limit(limit).Find(&list)
	return NewPageInfo(list, page, limit, total)
}

// DeleteByCid removes a content row and its meta relationships.
// Mirrors deleteByCid.
func (s *Service) DeleteByCid(cid int) error {
	c, _ := s.GetContentByID(strconv.Itoa(cid))
	if c == nil {
		return nil
	}
	err := s.db.Transaction(func(tx txLike) error {
		if err := tx.Delete(&model.Content{}, cid).Error; err != nil {
			return err
		}
		return tx.Where("cid = ?", cid).Delete(&model.Relationship{}).Error
	})
	if err == nil {
		s.invalidateContent(c)
	}
	return err
}

// UpdateArticle edits an existing article/page and rebuilds its metas.
// Mirrors updateArticle.
func (s *Service) UpdateArticle(c *model.Content) error {
	if c == nil || c.Cid == 0 {
		return Tip("文章对象不能为空")
	}
	if !validContentStatus(c.Type, c.Status) {
		return Tip("文章状态不合法")
	}
	if strings.TrimSpace(c.Title) == "" {
		return Tip("文章标题不能为空")
	}
	if strings.TrimSpace(c.Content) == "" {
		return Tip("文章内容不能为空")
	}
	if len([]rune(c.Title)) > 200 {
		return Tip("文章标题过长")
	}
	if len([]rune(c.Content)) > 65000 {
		return Tip("文章内容过长")
	}
	if c.AuthorID == 0 {
		return Tip("请登录后发布文章")
	}
	if strings.TrimSpace(c.Slug) == "" {
		c.Slug = ""
	}
	c.Modified = util.CurrentUnixTime()
	cid := c.Cid
	tags := c.Tags
	categories := c.Categories

	var original model.Content
	_ = s.db.First(&original, cid).Error
	err := s.db.Transaction(func(tx txLike) error {
		slug := any(c.Slug)
		if c.Slug == "" {
			slug = nil
		}
		updates := map[string]interface{}{
			"title":         c.Title,
			"slug":          slug,
			"modified":      c.Modified,
			"author_id":     c.AuthorID,
			"type":          c.Type,
			"status":        c.Status,
			"tags":          c.Tags,
			"categories":    c.Categories,
			"allow_comment": c.AllowComment,
			"allow_ping":    c.AllowPing,
			"allow_feed":    c.AllowFeed,
			"content":       c.Content,
		}
		if err := tx.Model(&model.Content{}).Where("cid = ?", cid).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where("cid = ?", cid).Delete(&model.Relationship{}).Error; err != nil {
			return err
		}
		if err := s.saveMetasTx(tx, cid, tags, model.TypeTag); err != nil {
			return err
		}
		return s.saveMetasTx(tx, cid, categories, model.TypeCategory)
	})
	if err == nil {
		s.invalidateContent(&original)
		s.invalidateContent(c)
	}
	return err
}

func validContentStatus(contentType, status string) bool {
	switch status {
	case model.TypePublish, model.TypeDraft, model.TypePrivate:
		return status != model.TypePrivate || contentType == model.TypeArticle
	default:
		return false
	}
}
