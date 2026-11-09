package service

import (
	"sort"
	"strconv"
	"time"

	"myblog/internal/model"
)

// PublicSummary contains only statistics for openly published articles.
type PublicSummary struct {
	Categories []model.MetaDto
	Articles   int64
	Comments   int64
	Latest     int
}

type HomeColumn struct {
	Meta    model.MetaDto
	Top     []model.Content
	Latest  []model.Content
	Updated int
}

type WritingDay struct {
	Date  string
	Count int
	Month int
}

type WritingMonth struct {
	Label    string
	Count    int
	Articles []model.Content
}

type HomeContent struct {
	Columns  []HomeColumn
	MostRead []model.Content
	Days     []WritingDay
	Months   []WritingMonth
}

func (s *Service) GetPublicSummary() PublicSummary {
	key := "public-summary:" + strconv.FormatUint(s.contentListVersion.Load(), 10)
	if value, ok := s.cache.Get(key); ok {
		return value.(PublicSummary)
	}
	value, _, _ := s.sf.Do(key, func() (interface{}, error) {
		var summary PublicSummary
		s.db.Model(&model.Content{}).Where("type = ? AND status = ?", model.TypeArticle, model.TypePublish).Count(&summary.Articles)
		s.db.Table("t_comments AS c").Joins("JOIN t_contents AS a ON a.cid = c.cid").Where("a.type = ? AND a.status = ?", model.TypeArticle, model.TypePublish).Count(&summary.Comments)
		var latest model.Content
		s.db.Where("type = ? AND status = ?", model.TypeArticle, model.TypePublish).Order(articleDisplayOrder).Limit(1).Find(&latest)
		summary.Latest = latest.DisplayTime
		s.db.Table("t_metas AS m").Select("m.*, count(distinct a.cid) AS count").Joins("JOIN t_relationships AS r ON r.mid = m.mid").Joins("JOIN t_contents AS a ON a.cid = r.cid").Where("m.type = ? AND a.type = ? AND a.status = ?", model.TypeCategory, model.TypeArticle, model.TypePublish).Group("m.mid").Order("count desc, m.mid desc").Scan(&summary.Categories)
		s.cache.Set(key, summary, 30)
		return summary, nil
	})
	return value.(PublicSummary)
}

func (s *Service) GetHomeContent(includeEncrypted bool, now time.Time) HomeContent {
	key := "home-content:" + strconv.FormatUint(s.contentListVersion.Load(), 10) + ":" + visibilityCacheKey(includeEncrypted) + ":" + now.Format("2006-01")
	if value, ok := s.cache.Get(key); ok {
		return value.(HomeContent)
	}
	value, _, _ := s.sf.Do(key, func() (interface{}, error) {
		var home HomeContent
		var articles []model.Content
		s.db.Where("type = ? AND status = ?", model.TypeArticle, model.TypePublish).Order(articleDisplayOrder).Find(&articles)
		ranked := append([]model.Content(nil), articles...)
		sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].Hits > ranked[j].Hits })
		if len(ranked) > 8 {
			ranked = ranked[:8]
		}
		home.MostRead = ranked
		for _, meta := range s.GetPublicSummary().Categories {
			column := HomeColumn{Meta: meta}
			query := s.db.Table("t_contents AS a").Select("a.*").Joins("JOIN t_relationships AS r ON r.cid = a.cid").Where("r.mid = ? AND a.type = ? AND a.status = ?", meta.Mid, model.TypeArticle, model.TypePublish)
			query.Order("a.hits desc, a.display_time desc, a.cid desc").Limit(3).Scan(&column.Top)
			s.db.Table("t_contents AS a").Select("a.*").Joins("JOIN t_relationships AS r ON r.cid = a.cid").Where("r.mid = ? AND a.type = ? AND a.status IN ?", meta.Mid, model.TypeArticle, publicArticleStatuses(includeEncrypted)).Order("a.display_time desc, a.cid desc").Limit(4).Scan(&column.Latest)
			var latest model.Content
			s.db.Table("t_contents AS a").Select("a.*").Joins("JOIN t_relationships AS r ON r.cid = a.cid").Where("r.mid = ? AND a.type = ? AND a.status = ?", meta.Mid, model.TypeArticle, model.TypePublish).Order("a.display_time desc, a.cid desc").Limit(1).Scan(&latest)
			column.Updated = latest.DisplayTime
			home.Columns = append(home.Columns, column)
		}
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -3, 0)
		counts := map[string]int{}
		for i := 0; i < 4; i++ {
			date := start.AddDate(0, i, 0)
			month := WritingMonth{Label: date.Format("2006年1月")}
			for _, article := range articles {
				stamp := time.Unix(int64(article.DisplayTime), 0).In(now.Location())
				if stamp.Year() == date.Year() && stamp.Month() == date.Month() {
					month.Count++
					counts[stamp.Format("2006-01-02")]++
					if len(month.Articles) < 2 {
						month.Articles = append(month.Articles, article)
					}
				}
			}
			home.Months = append(home.Months, month)
			for day := date; day.Before(date.AddDate(0, 1, 0)); day = day.AddDate(0, 0, 1) {
				key := day.Format("2006-01-02")
				home.Days = append(home.Days, WritingDay{Date: key, Count: counts[key], Month: i})
			}
		}
		s.cache.Set(key, home, 30)
		return home, nil
	})
	return value.(HomeContent)
}
