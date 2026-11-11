package service

import (
	"errors"
	"net/url"
	"regexp"
	"strings"

	"myblog/internal/model"
	"myblog/internal/util"

	"gorm.io/gorm"
)

const (
	projectNameMax    = 120
	projectSummaryMax = 500
	projectAltMax     = 300
	projectURLMax     = 1024
)

var projectSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,99}$`)

// Projects returns every card ordered by the editor's explicit sort value,
// then by id so equal sort values never change places between requests.
func (s *Service) Projects() ([]model.Project, error) {
	var projects []model.Project
	if err := s.db.Preload("Article", publishedProjectArticles).Order("sort ASC, id ASC").Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

func publishedProjectArticles(db *gorm.DB) *gorm.DB {
	return db.Select("cid", "slug", "title").Where("type = ? AND status = ?", model.TypeArticle, model.TypePublish)
}

// PublishedProjectArticles supplies the existing articles available in the card editor.
func (s *Service) PublishedProjectArticles() ([]model.Content, error) {
	var articles []model.Content
	err := publishedProjectArticles(s.db).Order(articleDisplayOrder).Find(&articles).Error
	return articles, err
}

// ProjectBySlug resolves existing bookmarks to their card.
func (s *Service) ProjectBySlug(slug string) (*model.Project, error) {
	var project model.Project
	err := s.db.Where("slug = ?", strings.TrimSpace(slug)).First(&project).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// ProjectByID loads one admin edit form.
func (s *Service) ProjectByID(id int) (*model.Project, error) {
	if id <= 0 {
		return nil, nil
	}
	var project model.Project
	err := s.db.Preload("Article", publishedProjectArticles).First(&project, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// CreateProject validates and inserts a card. Duplicate slugs are rejected
// before the insert; the unique index also protects against a save race.
func (s *Service) CreateProject(project *model.Project) error {
	if err := s.prepareProject(project, nil); err != nil {
		return err
	}
	now := util.CurrentUnixTime()
	project.ID = 0
	project.Created = now
	project.Modified = now
	if err := s.db.Omit("Article").Create(project).Error; err != nil {
		if duplicateKeyError(err) {
			return Tip("该路径已经存在，请重新输入")
		}
		return err
	}
	return nil
}

// UpdateProject saves every editable field without changing the creation time
// or another card's slug.
func (s *Service) UpdateProject(project *model.Project, animationSubmitted bool) error {
	if project == nil || project.ID <= 0 {
		return Tip("项目不存在")
	}
	var existing model.Project
	if err := s.db.First(&existing, project.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Tip("项目不存在")
		}
		return err
	}
	if err := s.prepareProject(project, &existing); err != nil {
		return err
	}
	project.Created = existing.Created
	project.Modified = util.CurrentUnixTime()
	updates := map[string]any{
		"slug":        project.Slug,
		"name":        project.Name,
		"summary":     project.Summary,
		"cover_url":   project.CoverURL,
		"cover_alt":   project.CoverAlt,
		"project_url": project.PlatformURL,
		"github_url":  project.GitHubURL,
		"article_id":  project.ArticleID,
		"sort":        project.Sort,
		"modified":    project.Modified,
	}
	// An editor opened before the animated-cover field existed omits this key.
	if animationSubmitted {
		updates["animation_url"] = project.AnimationURL
	}
	result := s.db.Model(&model.Project{}).Where("id = ?", project.ID).Updates(updates)
	if result.Error != nil {
		if duplicateKeyError(result.Error) {
			return Tip("该路径已经存在，请重新输入")
		}
		return result.Error
	}
	// MySQL may report zero on an unchanged save. Distinguish that case from
	// a project removed by another editor between the read and update.
	if result.RowsAffected == 0 {
		current, err := s.ProjectByID(project.ID)
		if err != nil {
			return err
		}
		if current == nil {
			return Tip("项目不存在")
		}
	}
	return nil
}

// DeleteProject removes one card. An intentionally empty gallery is a valid
// state and is never repopulated by the seed path.
func (s *Service) DeleteProject(id int) error {
	if id <= 0 {
		return Tip("项目不存在")
	}
	result := s.db.Delete(&model.Project{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return Tip("项目不存在")
	}
	return nil
}

func (s *Service) prepareProject(project, previous *model.Project) error {
	if project == nil {
		return Tip("项目不能为空")
	}
	project.Slug = strings.ToLower(strings.TrimSpace(project.Slug))
	project.Name = strings.TrimSpace(project.Name)
	project.Summary = strings.TrimSpace(project.Summary)
	project.CoverURL = strings.TrimSpace(project.CoverURL)
	project.CoverAlt = strings.TrimSpace(project.CoverAlt)
	project.PlatformURL = strings.TrimSpace(project.PlatformURL)
	project.GitHubURL = strings.TrimSpace(project.GitHubURL)

	if project.Name == "" {
		return Tip("项目名称不能为空")
	}
	if len([]rune(project.Name)) > projectNameMax {
		return Tip("项目名称过长")
	}
	if project.Slug == "" {
		return Tip("路径不能为空")
	}
	if !projectSlug.MatchString(project.Slug) {
		return Tip("路径须以字母或数字开头，最多 100 位，可含 - 和 _")
	}
	if len([]rune(project.Summary)) > projectSummaryMax {
		return Tip("项目简介过长")
	}
	if len([]rune(project.CoverAlt)) > projectAltMax {
		return Tip("封面描述过长")
	}

	coverURL, err := normalizeCoverURL(project.CoverURL)
	if err != nil {
		return err
	}
	project.CoverURL = coverURL
	animationURL, err := normalizeCoverURL(strings.TrimSpace(project.AnimationURL))
	if err != nil {
		return Tip("请填写有效的 GitHub 动图地址")
	}
	// A raw comma separates image candidates in an HTML srcset.
	project.AnimationURL = strings.ReplaceAll(animationURL, ",", "%2C")

	platformURL, err := normalizeProjectURL(project.PlatformURL)
	if err != nil {
		return err
	}
	project.PlatformURL = platformURL
	if project.GitHubURL != "" {
		repository, ok := util.GitHubRepositoryURL(project.GitHubURL)
		if !ok || len(repository) > projectURLMax {
			return Tip("请填写 GitHub 仓库地址")
		}
		project.GitHubURL = repository
	}
	if project.ArticleID < 0 {
		return Tip("所选文章无效")
	}
	if project.ArticleID > 0 && (previous == nil || previous.ArticleID != project.ArticleID) {
		var count int64
		err := s.db.Model(&model.Content{}).Where("cid = ? AND type = ? AND status = ?", project.ArticleID, model.TypeArticle, model.TypePublish).Count(&count).Error
		if err != nil {
			return err
		}
		if count == 0 {
			return Tip("请选择已公开发布的文章")
		}
	}

	var count int64
	query := s.db.Model(&model.Project{}).Where("slug = ?", project.Slug)
	if previous != nil {
		query = query.Where("id <> ?", previous.ID)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return Tip("该路径已经存在，请重新输入")
	}
	return nil
}

// normalizeCoverURL accepts direct HTTPS assets from GitHub only. Standard
// github.com blob links are rewritten to raw.githubusercontent.com; GitHub
// user-attachments image links are kept as pasted.
func normalizeCoverURL(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if len(raw) > projectURLMax {
		return "", Tip("封面地址过长")
	}
	parsed, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") ||
		parsed.User != nil || parsed.Host == "" {
		return "", Tip("封面地址必须是有效的 HTTPS GitHub 图片地址")
	}

	host := strings.ToLower(parsed.Host)
	segments, ok := cleanURLSegments(parsed.Path)
	if !ok {
		return "", Tip("封面地址格式不正确")
	}
	switch host {
	case "raw.githubusercontent.com":
		if len(segments) < 4 {
			return "", Tip("封面地址必须指向具体的 GitHub 文件")
		}
		return buildGitHubURL(host, segments), nil
	case "github.com":
		if len(segments) >= 3 && segments[0] == "user-attachments" && segments[1] == "assets" {
			return buildGitHubURL(host, segments), nil
		}
		if len(segments) >= 5 && (segments[2] == "blob" || segments[2] == "raw") {
			rawSegments := append([]string{segments[0], segments[1], segments[3]}, segments[4:]...)
			return buildGitHubURL("raw.githubusercontent.com", rawSegments), nil
		}
	}
	return "", Tip("封面地址仅支持 raw.githubusercontent.com 或 GitHub 仓库图片")
}

func normalizeProjectURL(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if len(raw) > projectURLMax {
		return "", Tip("项目链接过长")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil ||
		(!strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https")) {
		return "", Tip("项目链接必须是有效的 HTTP 或 HTTPS 地址")
	}
	return parsed.String(), nil
}

func cleanURLSegments(path string) ([]string, bool) {
	if path == "" || path == "/" {
		return nil, false
	}
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return nil, false
		}
	}
	return segments, true
}

func buildGitHubURL(host string, segments []string) string {
	return (&url.URL{Scheme: "https", Host: host, Path: "/" + strings.Join(segments, "/")}).String()
}

func duplicateKeyError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "duplicate entry") ||
		strings.Contains(message, "duplicate key")
}
