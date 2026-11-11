package db

import (
	"strings"
	"time"

	"myblog/internal/model"
	"myblog/internal/util"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	projectSeedMarker  = "project_cards_seeded"
	projectLinksMarker = "project_card_links_v2"
	seekinoLogoMarker  = "project_card_seekino_logo_path"
	seekinoPlatform    = "https://seekino.hypn0s.cloud/search"
	seekinoRepo        = "https://github.com/owariband/Seekino"
	catendarRepo       = "https://github.com/Slimzeo/catendar"
	seekinoCover       = "https://raw.githubusercontent.com/owariband/Seekino/main/assets/seekino-logo.svg"
	formerSeekinoLogo  = "https://raw.githubusercontent.com/owariband/Seekino/main/assets/mascot/seekino-logo.svg"
	catendarCover      = "https://raw.githubusercontent.com/Slimzeo/catendar/main/build/appicon.png"
	seekinoAlt         = "Seekino 的青绿色鲸鲨标志"
	catendarAlt        = "catendar 的紫色折角猫图标"
	oldSeekinoCover    = "https://raw.githubusercontent.com/owariband/Seekino/main/assets/screenshots/search-results.jpg"
	oldCatendarCover   = "https://raw.githubusercontent.com/Slimzeo/catendar/main/docs/daily-board.png"
	oldSeekinoAlt      = "Seekino 的片段检索结果，右侧可播放带起止时间的原片"
	oldCatendarAlt     = "catendar 的每日看板，任务按待办、进行中和已完成分为三列"
)

// Persist each marker in the same transaction as its edits so a restart
// retries a failed upgrade and never refills cards the author deleted.
func projectMigration(db *gorm.DB, name, description string, apply func(*gorm.DB) error) error {
	var initialized int64
	if err := db.Model(&model.Option{}).Where("name = ?", name).Count(&initialized).Error; err != nil || initialized > 0 {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		claim := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.Option{
			Name: name, Value: "1", Description: description,
		})
		if claim.Error != nil || claim.RowsAffected == 0 {
			return claim.Error
		}
		return apply(tx)
	})
}

func seedProjects(db *gorm.DB) error {
	err := projectMigration(db, projectSeedMarker, "项目卡片默认内容已初始化", func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.Project{}).Count(&count).Error; err != nil || count != 0 {
			return err
		}
		now := int(time.Now().Unix())
		projects := []model.Project{
			{Slug: "seekino", Name: "Seekino", Summary: "用一句话或一张截图，在视频里找到记得的那一幕。",
				CoverURL: seekinoCover, CoverAlt: seekinoAlt, PlatformURL: seekinoPlatform, GitHubURL: seekinoRepo,
				Sort: 10, Created: now, Modified: now},
			{Slug: "catendar", Name: "catendar", Summary: "今天的任务放进看板，有时间的约定留给月历。",
				CoverURL: catendarCover, CoverAlt: catendarAlt, GitHubURL: catendarRepo,
				Sort: 20, Created: now, Modified: now},
		}
		return tx.Create(&projects).Error
	})
	if err != nil {
		return err
	}
	if err := projectMigration(db, projectLinksMarker, "项目卡片链接与标志已更新", migrateProjectLinks); err != nil {
		return err
	}
	// Seekino moved its official asset when publishing the smaller source tree.
	// Upgrade the former default without overwriting an author's replacement.
	return projectMigration(db, seekinoLogoMarker, "Seekino 官方标志路径已更新", func(tx *gorm.DB) error {
		return tx.Model(&model.Project{}).Where("slug = ? AND cover_url = ?", "seekino", formerSeekinoLogo).
			Update("cover_url", seekinoCover).Error
	})
}

func migrateProjectLinks(tx *gorm.DB) error {
	var cards []model.Project
	if err := tx.Find(&cards).Error; err != nil {
		return err
	}
	for _, card := range cards {
		edits := make(map[string]any)
		// Only move repository homepages. Custom destinations such as a GitHub
		// release or a deep link with a fragment retain their original behavior.
		repo, isRepo := util.GitHubRepositoryURL(card.PlatformURL)
		if isRepo && card.GitHubURL == "" && !strings.ContainsAny(card.PlatformURL, "?#") {
			edits["github_url"] = repo
			platform := ""
			if card.Slug == "seekino" && strings.EqualFold(repo, seekinoRepo) {
				platform = seekinoPlatform
			}
			edits["project_url"] = platform
		}
		switch {
		case card.Slug == "seekino" && card.CoverURL == oldSeekinoCover:
			edits["cover_url"] = seekinoCover
			if card.CoverAlt == "" || card.CoverAlt == oldSeekinoAlt {
				edits["cover_alt"] = seekinoAlt
			}
		case card.Slug == "catendar" && card.CoverURL == oldCatendarCover:
			edits["cover_url"] = catendarCover
			if card.CoverAlt == "" || card.CoverAlt == oldCatendarAlt {
				edits["cover_alt"] = catendarAlt
			}
		}
		if len(edits) > 0 {
			if err := tx.Model(&model.Project{}).Where("id = ?", card.ID).Updates(edits).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
