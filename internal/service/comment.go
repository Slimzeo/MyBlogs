package service

import (
	"strconv"
	"strings"

	"myblog/internal/model"
	"myblog/internal/util"
)

// InsertComment validates and saves a comment, bumping the article count.
// Mirrors CommentServiceImpl.insertComment.
func (s *Service) InsertComment(c *model.Comment) error {
	if c == nil {
		return Tip("评论对象为空")
	}
	if strings.TrimSpace(c.Author) == "" {
		c.Author = "热心网友"
	}
	if strings.TrimSpace(c.Mail) != "" && !util.IsEmail(c.Mail) {
		return Tip("请输入正确的邮箱格式")
	}
	if strings.TrimSpace(c.Content) == "" {
		return Tip("评论内容不能为空")
	}
	if n := len([]rune(c.Content)); n < 5 || n > 2000 {
		return Tip("评论字数在5-2000个字符")
	}
	if c.Cid == 0 {
		return Tip("评论文章不能为空")
	}
	content, _ := s.GetContentByID(strconv.Itoa(c.Cid))
	if content == nil {
		return Tip("不存在的文章")
	}
	c.OwnerID = content.AuthorID
	c.Created = util.CurrentUnixTime()
	if c.Type == "" {
		c.Type = "comment"
	}
	if c.Status == "" {
		c.Status = "approved"
	}

	err := s.db.Transaction(func(tx txLike) error {
		if err := tx.Create(c).Error; err != nil {
			return err
		}
		return tx.Model(&model.Content{}).Where("cid = ?", content.Cid).
			Update("comments_num", gormExprAdd("comments_num", 1)).Error
	})
	if err == nil {
		s.invalidateComments(c.Cid)
		s.invalidateContent(content)
	}
	return err
}

// GetComments returns the paginated top-level comments of an article as CommentBo.
// Mirrors CommentServiceImpl.getComments.
func (s *Service) GetComments(cid, page, limit int) *PageInfo[model.CommentBo] {
	if cid == 0 {
		return nil
	}
	key := "comments:" + strconv.Itoa(cid) + ":" +
		strconv.FormatUint(s.commentVersion(cid), 10) + ":" +
		strconv.Itoa(page) + ":" + strconv.Itoa(limit)
	if cached, exists := s.cache.Get(key); exists {
		return cached.(*PageInfo[model.CommentBo])
	}
	value, _, _ := s.sf.Do(key, func() (any, error) {
		var total int64
		if err := s.db.Model(&model.Comment{}).
			Where("cid = ? AND parent = 0", cid).Count(&total).Error; err != nil {
			return NewPageInfo([]model.CommentBo{}, page, limit, 0), err
		}
		var parents []model.Comment
		err := s.db.Where("cid = ? AND parent = 0", cid).Order("coid desc").
			Offset((page - 1) * limit).Limit(limit).Find(&parents).Error
		comments := make([]model.CommentBo, 0, len(parents))
		for _, parent := range parents {
			comments = append(comments, model.CommentBo{Comment: parent})
		}
		result := NewPageInfo(comments, page, limit, total)
		if err == nil {
			s.cache.Set(key, result, 10)
		}
		return result, err
	})
	return value.(*PageInfo[model.CommentBo])
}

// GetCommentsByAuthorNotPaged paginates comments not authored by uid, newest first.
// Mirrors the admin comment list query (authorId != current user).
func (s *Service) GetCommentsExcludingAuthor(uid, page, limit int) *PageInfo[model.Comment] {
	var total int64
	s.db.Model(&model.Comment{}).Where("author_id <> ?", uid).Count(&total)
	var list []model.Comment
	s.db.Where("author_id <> ?", uid).Order("coid desc").
		Offset((page - 1) * limit).Limit(limit).Find(&list)
	return NewPageInfo(list, page, limit, total)
}

// GetCommentByID fetches a single comment. Mirrors getCommentById.
func (s *Service) GetCommentByID(coid int) *model.Comment {
	if coid == 0 {
		return nil
	}
	var c model.Comment
	if err := s.db.First(&c, coid).Error; err != nil {
		return nil
	}
	return &c
}

// UpdateComment updates a comment row selectively (status/content). Mirrors update.
func (s *Service) UpdateComment(c *model.Comment) {
	if c == nil || c.Coid == 0 {
		return
	}
	updates := map[string]interface{}{}
	if c.Status != "" {
		updates["status"] = c.Status
	}
	if c.Content != "" {
		updates["content"] = c.Content
	}
	if len(updates) > 0 {
		var current model.Comment
		_ = s.db.First(&current, c.Coid).Error
		if s.db.Model(&model.Comment{}).Where("coid = ?", c.Coid).Updates(updates).Error == nil {
			s.invalidateComments(current.Cid)
		}
	}
}

// DeleteComment removes a comment and decrements the article count. Mirrors delete.
func (s *Service) DeleteComment(coid, cid int) error {
	if coid == 0 {
		return Tip("主键为空")
	}
	var content model.Content
	_ = s.db.First(&content, cid).Error
	err := s.db.Transaction(func(tx txLike) error {
		if err := tx.Delete(&model.Comment{}, coid).Error; err != nil {
			return err
		}
		return tx.Model(&model.Content{}).Where("cid = ? AND comments_num > 0", cid).
			Update("comments_num", gormExprAdd("comments_num", -1)).Error
	})
	if err == nil {
		s.invalidateComments(cid)
		s.invalidateContent(&content)
	}
	return err
}
