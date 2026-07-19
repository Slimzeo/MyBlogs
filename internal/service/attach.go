package service

import (
	"myblog/internal/model"
	"myblog/internal/util"
)

// GetAttachs paginates uploaded files newest-first. Mirrors AttachServiceImpl.getAttachs.
func (s *Service) GetAttachs(page, limit int) *PageInfo[model.Attach] {
	var total int64
	s.db.Model(&model.Attach{}).Count(&total)
	var list []model.Attach
	s.db.Order("id desc").Offset((page - 1) * limit).Limit(limit).Find(&list)
	return NewPageInfo(list, page, limit, total)
}

// GetAttachByID fetches an attachment. Mirrors selectById.
func (s *Service) GetAttachByID(id int) *model.Attach {
	if id == 0 {
		return nil
	}
	var a model.Attach
	if err := s.db.First(&a, id).Error; err != nil {
		return nil
	}
	return &a
}

// SaveAttach records an uploaded file. Mirrors save.
func (s *Service) SaveAttach(fname, fkey, ftype string, authorID int) error {
	return s.db.Create(&model.Attach{
		Fname:    fname,
		Fkey:     fkey,
		Ftype:    ftype,
		AuthorID: authorID,
		Created:  util.CurrentUnixTime(),
	}).Error
}

// DeleteAttach removes an attachment row. Mirrors deleteById.
func (s *Service) DeleteAttach(id int) {
	if id != 0 {
		s.db.Delete(&model.Attach{}, id)
	}
}
