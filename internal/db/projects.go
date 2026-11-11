package db

import (
	"time"

	"myblog/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const projectSeedMarker = "project_cards_seeded"

const (
	seekinoCover  = "https://raw.githubusercontent.com/owariband/Seekino/main/assets/screenshots/search-results.jpg"
	catendarCover = "https://raw.githubusercontent.com/Slimzeo/catendar/main/docs/daily-board.png"
)

const seekinoNotes = `## 找回记得的那一幕

想重看动画里的一场演出，手上可能只有一张截图。找课上的一段讲解，也常常只记得老师提过的几个词。把视频加入 Seekino 的素材库并解析一次，之后就能用图片或文字反复搜索。

## 从结果回到原片

每条结果都标出来自哪部视频、从哪里开始、到哪里结束。可以先在结果旁预览，再跳回整集看前后文。观看课程时也可以围绕当前片段继续提问。`

const catendarNotes = `## 打开就是今天的看板

把今天要做的事写成卡片，拖到「进行中」或「已完成」就能更新进度。同一列里，越靠上的卡片优先级越高。昨天没做完的任务会带到今天，已经完成的留在原来的日期。

## 给有时间的事留个位置

会议和面试可以填起止时间，切到月历就能看见安排。任务存在本机。需要整理时，可以手动运行 AI Sync，从最近的 Codex 对话里汇总进展，或从邮箱提取日程。`

// seedProjects inserts the two original cards exactly once. The marker option
// makes deletion intentional: an empty project table after seeding is never
// refilled. Existing databases without the marker are recognized as already
// populated when they contain project rows, so their data is left untouched.
func seedProjects(gdb *gorm.DB) error {
	var initialized int64
	if err := gdb.Model(&model.Option{}).Where("name = ?", projectSeedMarker).Count(&initialized).Error; err != nil || initialized > 0 {
		return err
	}
	// Claim initialization before reading the project table so concurrent
	// startup cannot seed two copies. The claim and the rows commit together.
	return gdb.Transaction(func(tx *gorm.DB) error {
		claim := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.Option{
			Name: projectSeedMarker, Value: "1", Description: "项目卡片默认内容已初始化",
		})
		if claim.Error != nil || claim.RowsAffected == 0 {
			return claim.Error
		}
		var count int64
		if err := tx.Model(&model.Project{}).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			now := int(time.Now().Unix())
			projects := []model.Project{
				{
					Slug:       "seekino",
					Name:       "Seekino",
					Summary:    "用一句话或一张截图，在视频里找到记得的那一幕。",
					Notes:      seekinoNotes,
					CoverURL:   seekinoCover,
					CoverAlt:   "Seekino 的片段检索结果，右侧可播放带起止时间的原片",
					ProjectURL: "https://github.com/owariband/Seekino",
					Sort:       10,
					Created:    now,
					Modified:   now,
				},
				{
					Slug:       "catendar",
					Name:       "catendar",
					Summary:    "今天的任务放进看板，有时间的约定留给月历。",
					Notes:      catendarNotes,
					CoverURL:   catendarCover,
					CoverAlt:   "catendar 的每日看板，任务按待办、进行中和已完成分为三列",
					ProjectURL: "https://github.com/Slimzeo/catendar",
					Sort:       20,
					Created:    now,
					Modified:   now,
				},
			}
			if err := tx.Create(&projects).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
