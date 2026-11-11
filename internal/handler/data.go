package handler

import (
	"myblog/internal/model"
	"myblog/internal/service"
	"myblog/internal/util"
)

// PageData is shared by the migrated html/template pages. Keeping one explicit
// view model makes handler-to-template contracts discoverable.
type PageData struct {
	NativeGuide      bool
	Library          *blogLibrary
	Projects         []model.Project
	Project          *model.Project
	Home             bool
	PublicSummary    service.PublicSummary
	HomeContent      service.HomeContent
	Title            string
	Keywords         string
	Description      string
	IsPost           bool
	Active           string
	Message          string
	CsrfToken        string
	Hits             int
	Type             string
	Keyword          string
	MaxFileSize      int
	AccessKeyEnabled bool
	EncryptedAccess  bool
	AccessExpiresAt  int64

	LoginUser   *model.User
	Article     *model.Content
	ReadingText util.ReadingText
	Contents    *model.Content
	Meta        *model.MetaDto

	Articles      *service.PageInfo[model.Content]
	Comments      *service.PageInfo[model.CommentBo]
	AdminComments *service.PageInfo[model.Comment]
	Attachs       *service.PageInfo[model.Attach]

	RecentArticles []model.Content
	RecentComments []model.Comment
	Logs           []model.Log
	Archives       []model.ArchiveBo
	ArchiveCount   int
	TopicGroups    []model.TopicGroup
	TopicView      string
	Links          []model.Meta
	Categories     []model.MetaDto
	Tags           []model.MetaDto
	Statistics     model.StatisticsBo
	Options        map[string]string
	APITokens      []model.APIToken
}
