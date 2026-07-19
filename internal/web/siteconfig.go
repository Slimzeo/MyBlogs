package web

import (
	"sync"
	"sync/atomic"

	"myblog/internal/service"
)

// SiteConfig is a thread-safe, cached view of the t_options table. It replaces
// the Java WebConst.initConfig static map + Commons.site_option accessor.
// Reads are lock-free via an atomic snapshot pointer; writes swap the snapshot.
type SiteConfig struct {
	svc  *service.Service
	snap atomic.Value // map[string]string
	mu   sync.Mutex   // serializes refreshes
}

// NewSiteConfig loads options once and returns the provider.
func NewSiteConfig(svc *service.Service) *SiteConfig {
	sc := &SiteConfig{svc: svc}
	sc.Refresh()
	return sc
}

// Refresh reloads the options snapshot from the database.
func (sc *SiteConfig) Refresh() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.snap.Store(sc.svc.OptionsMap())
}

func (sc *SiteConfig) all() map[string]string {
	v := sc.snap.Load()
	if v == nil {
		return map[string]string{}
	}
	return v.(map[string]string)
}

// Option returns a config value or the default when blank/missing.
// Mirrors Commons.site_option(key, default).
func (sc *SiteConfig) Option(key, def string) string {
	if key == "" {
		return ""
	}
	if v, ok := sc.all()[key]; ok && v != "" {
		return v
	}
	return def
}

// Title returns the site title with a sensible fallback.
func (sc *SiteConfig) Title() string { return sc.Option("site_title", "My Blog") }

// Social returns the configured social links map. Mirrors Commons.social.
func (sc *SiteConfig) Social() map[string]string {
	m := sc.all()
	return map[string]string{
		"weibo":   m["social_weibo"],
		"zhihu":   m["social_zhihu"],
		"github":  m["social_github"],
		"twitter": m["social_twitter"],
	}
}
