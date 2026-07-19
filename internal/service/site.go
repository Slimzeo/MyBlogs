package service

import (
	"sort"
	"strconv"

	"myblog/internal/model"
	"myblog/internal/util"
)

// RecentComments returns the newest comments (clamped 0-10). Mirrors recentComments.
func (s *Service) RecentComments(limit int) []model.Comment {
	if limit < 0 || limit > 10 {
		limit = 10
	}
	var list []model.Comment
	s.db.Order("created desc").Limit(limit).Find(&list)
	return list
}

// RecentContents returns the newest published articles (clamped 0-10). Mirrors recentContents.
func (s *Service) RecentContents(limit int) []model.Content {
	if limit < 0 || limit > 10 {
		limit = 10
	}
	var list []model.Content
	s.db.Where("status = ? AND type = ?", model.TypePublish, model.TypeArticle).
		Order("created desc").Limit(limit).Find(&list)
	return list
}

// GetStatistics returns the admin dashboard counters. Mirrors getStatistics.
func (s *Service) GetStatistics() model.StatisticsBo {
	var st model.StatisticsBo
	s.db.Model(&model.Content{}).
		Where("type = ? AND status = ?", model.TypeArticle, model.TypePublish).
		Count(&st.Articles)
	s.db.Model(&model.Comment{}).Count(&st.Comments)
	s.db.Model(&model.Attach{}).Count(&st.Attachs)
	s.db.Model(&model.Meta{}).Where("type = ?", model.TypeLink).Count(&st.Links)
	return st
}

// GetArchives groups published articles by month. Mirrors getArchives +
// ContentVoMapper.findReturnArchiveBo. Wrapped in singleflight so a burst of
// concurrent /archives requests only runs the aggregation once.
func (s *Service) GetArchives() []model.ArchiveBo {
	key := "archives:" + strconv.FormatUint(s.contentListVersion.Load(), 10)
	if cached, exists := s.cache.Get(key); exists {
		return cached.([]model.ArchiveBo)
	}
	v, _, _ := s.sf.Do(key, func() (interface{}, error) {
		archives := s.buildArchives()
		s.cache.Set(key, archives, 30)
		return archives, nil
	})
	return v.([]model.ArchiveBo)
}

func (s *Service) buildArchives() []model.ArchiveBo {
	// Pull all published articles once, then bucket in Go. This avoids the
	// DB-specific FROM_UNIXTIME used by the original SQL and works on both
	// SQLite and MySQL identically.
	var articles []model.Content
	s.db.Where("type = ? AND status = ?", model.TypeArticle, model.TypePublish).
		Order("created desc").Find(&articles)

	buckets := map[string][]model.Content{}
	var order []string
	for _, a := range articles {
		key := monthKey(a.Created)
		if _, ok := buckets[key]; !ok {
			order = append(order, key)
		}
		buckets[key] = append(buckets[key], a)
	}
	// order is already newest-first because articles were sorted desc.
	out := make([]model.ArchiveBo, 0, len(order))
	for _, k := range order {
		list := buckets[k]
		out = append(out, model.ArchiveBo{
			Date:     k,
			Count:    strconv.Itoa(len(list)),
			Articles: list,
		})
	}
	// Keep months strictly descending in case map iteration perturbed order.
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out
}

func monthKey(unix int) string {
	// Reuse the CN month formatter for display parity with the Java version.
	return util.FormatUnixCN(unix)
}
