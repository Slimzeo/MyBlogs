package handler

import (
	"html/template"
	"net/url"
	"strconv"
	"strings"

	"myblog/internal/model"
	"myblog/internal/util"
)

// buildFuncMap returns the template helpers that replace Thymeleaf's
// `commons.*` and `adminCommons.*` utility calls. It closes over the site
// config so `siteOption`/`social` reflect live settings.
func buildFuncMap(sc *SiteConfig) template.FuncMap {
	renderArticle := func(markdown string) template.HTML {
		key := "markdown:" + util.MD5encode(markdown)
		if cached, exists := sc.svc.Cache().GetString(key); exists {
			return template.HTML(cached)
		}
		rendered := util.Article(markdown)
		sc.svc.Cache().Set(key, rendered, 5*60)
		return template.HTML(rendered)
	}
	return template.FuncMap{
		// ---- site config (Commons.site_*) ----
		"siteOption": sc.Option,
		"siteTitle":  sc.Title,
		"siteUrl": func(sub string) string {
			return sc.Option("site_url", "") + sub
		},
		"social": sc.Social,

		// ---- article/meta rendering (Commons.*) ----
		"permalink":      permalink,
		"article":        renderArticle,
		"intro":          util.Intro,
		"showThumbFirst": func(s string) string { return util.FirstImage(s) },
		"showIcon":       showIcon,
		"showCategories": func(s string) template.HTML { return template.HTML(showCategories(s)) },
		"showTags":       func(s string) template.HTML { return template.HTML(showTags(s)) },
		"gravatar":       gravatar,
		"fmtdate":        func(t int) string { return util.FormatUnix(t, "yyyy-MM-dd") },
		"fmtdatef":       func(t int, p string) string { return util.FormatUnix(t, p) },

		// ---- admin helpers (AdminCommons.*) ----
		"randColor": randColor,
		"existCat":  existCat,

		// ---- generic helpers ----
		"substr":    substr,
		"add":       func(a, b int) int { return a + b },
		"sub":       func(a, b int) int { return a - b },
		"eq":        func(a, b interface{}) bool { return a == b },
		"seq":       seq,
		"safeHTML":  func(s string) template.HTML { return template.HTML(s) },
		"urlEncode": url.PathEscape,
		"dict":      dict,
		"default": func(def, val string) string {
			if strings.TrimSpace(val) == "" {
				return def
			}
			return val
		},
	}
}

// dict builds a map from alternating key/value pairs, letting templates pass
// multiple named args to a sub-template (e.g. the pager fragment).
func dict(values ...interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(values)/2)
	for i := 0; i+1 < len(values); i += 2 {
		key, _ := values[i].(string)
		m[key] = values[i+1]
	}
	return m
}

// permalink builds an article URL. Mirrors Commons.permalink.
func permalink(c model.Content) string {
	if strings.TrimSpace(c.Slug) != "" {
		return "/article/" + c.Slug
	}
	return "/article/" + itoa(c.Cid)
}

var icons = []string{"bg-ico-book", "bg-ico-game", "bg-ico-note", "bg-ico-chat", "bg-ico-code", "bg-ico-image", "bg-ico-web", "bg-ico-link", "bg-ico-design", "bg-ico-lock"}

// showIcon mirrors Commons.show_icon.
func showIcon(cid int) string { return icons[cid%len(icons)] }

// showCategories renders category links. Mirrors Commons.show_categories.
func showCategories(categories string) string {
	if strings.TrimSpace(categories) == "" {
		categories = "默认分类"
	}
	var b strings.Builder
	for _, c := range strings.Split(categories, ",") {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		b.WriteString(`<a href="/category/` + url.PathEscape(c) + `">` + template.HTMLEscapeString(c) + `</a>`)
	}
	return b.String()
}

// showTags renders tag links. Mirrors Commons.show_tags.
func showTags(tags string) string {
	if strings.TrimSpace(tags) == "" {
		return ""
	}
	var b strings.Builder
	for _, t := range strings.Split(tags, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		b.WriteString(`<a href="/tag/` + url.PathEscape(t) + `">` + template.HTMLEscapeString(t) + `</a>`)
	}
	return b.String()
}

// gravatar mirrors Commons.gravatar.
func gravatar(email string) string {
	base := "https://secure.gravatar.com/avatar"
	if strings.TrimSpace(email) == "" {
		return base
	}
	return base + "/" + util.MD5encode(strings.ToLower(strings.TrimSpace(email)))
}

// substr mirrors Commons.substr (rune-safe).
func substr(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// existCat mirrors AdminCommons.exist_cat.
func existCat(cat model.Meta, cats string) bool {
	for _, c := range strings.Split(cats, ",") {
		if strings.TrimSpace(c) == cat.Name {
			return true
		}
	}
	return false
}

var colors = []string{"default", "primary", "success", "info", "warning", "danger", "inverse", "purple", "pink"}

// randColor mirrors AdminCommons.rand_color.
func randColor() string { return colors[util.RandInt(0, len(colors)-1)] }

// seq returns [1..n], used for simple range loops in templates.
func seq(n int) []int {
	out := make([]int, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, i)
	}
	return out
}

func itoa(n int) string { return strconv.Itoa(n) }
