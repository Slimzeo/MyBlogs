package web

import (
	"myblog/internal/model"
	"myblog/internal/service"
)

// PageData is shared by the migrated html/template pages. Keeping one explicit
// view model makes handler-to-template contracts discoverable.
type PageData struct {
	Title       string
	Keywords    string
	Description string
	IsPost      bool
	Active      string
	Message     string
	CsrfToken   string
	Hits        int
	Type        string
	Keyword     string
	MaxFileSize int

	LoginUser *model.User
	Article   *model.Content
	Contents  *model.Content
	Meta      *model.MetaDto

	Articles      *service.PageInfo[model.Content]
	Comments      *service.PageInfo[model.CommentBo]
	AdminComments *service.PageInfo[model.Comment]
	Attachs       *service.PageInfo[model.Attach]

	RecentArticles []model.Content
	RecentComments []model.Comment
	Logs           []model.Log
	Archives       []model.ArchiveBo
	Links          []model.Meta
	Categories     []model.MetaDto
	Tags           []model.MetaDto
	Statistics     model.StatisticsBo
	Options        map[string]string
}
