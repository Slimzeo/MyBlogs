package service

import (
	"myblog/internal/model"
	"myblog/internal/util"
)

// InsertLog records an admin operation. Mirrors LogServiceImpl.insertLog.
func (s *Service) InsertLog(action, data, ip string, authorID int) {
	s.db.Create(&model.Log{
		Action:   action,
		Data:     data,
		IP:       ip,
		AuthorID: authorID,
		Created:  util.CurrentUnixTime(),
	})
}

// GetLogs paginates operation logs newest-first. Mirrors getLogs.
func (s *Service) GetLogs(page, limit int) []model.Log {
	if page <= 0 {
		page = 1
	}
	if limit < 1 || limit > model.MaxPosts {
		limit = 10
	}
	var logs []model.Log
	s.db.Order("id desc").Offset((page - 1) * limit).Limit(limit).Find(&logs)
	return logs
}
