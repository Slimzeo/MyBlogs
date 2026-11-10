package handler

import (
	stdhtml "html"
	"net/url"
	"path"
	"strconv"
	"strings"

	"myblog/internal/model"
	"myblog/internal/util"
)

const journalPageSize = 12

func journalPageURL(page, size int, active string) string {
	if active == "blogs" {
		q := url.Values{}
		if page > 1 {
			q.Set("page", strconv.Itoa(page))
		}
		if size != journalPageSize {
			q.Set("limit", strconv.Itoa(size))
		}
		if len(q) > 0 {
			return "/blogs?" + q.Encode()
		}
		return "/blogs"
	}
	path := "/"
	if page > 1 {
		path = "/page/" + strconv.Itoa(page)
	}
	if size != journalPageSize {
		path += "?limit=" + strconv.Itoa(size)
	}
	return path + "#blogs"
}

// Use the author's excerpt before <!--more--> when available. Keep only a
// short plain-text preview; full journal entries stay on their article pages.
func journalExcerpt(post model.Content, length int) string {
	content := strings.TrimSpace(post.Content)
	if post.ContentFormat != model.ContentHTML {
		if first, rest, ok := strings.Cut(content, "\n"); ok {
			header := strings.TrimLeft(first, "#")
			if header != first && strings.TrimSpace(header) == strings.TrimSpace(post.Title) {
				content = rest
			}
		}
	}
	text := util.ContentIntro(content, post.ContentFormat, length*2)
	if post.ContentFormat != model.ContentHTML {
		text = stdhtml.UnescapeString(text)
	}
	text = strings.Join(strings.Fields(text), " ")
	if post.ContentFormat == model.ContentHTML {
		text = strings.TrimPrefix(text, strings.TrimSpace(post.Title)+" ")
	}
	if text == strings.TrimSpace(post.Title) {
		return ""
	}
	if runes := []rune(text); len(runes) > length {
		return strings.TrimSpace(string(runes[:length])) + "…"
	}
	return text
}

// Preview only article-owned assets served by this blog. A coverless card
// stays text-only; the homepage never hotlinks or proxies an external image.
func journalCover(post model.Content, site string) string {
	if !strings.Contains(post.Content, "![") && !strings.Contains(strings.ToLower(post.Content), "<img") {
		return ""
	}
	raw := util.ContentFirstImage(post.Content, post.ContentFormat)
	if raw == "" || strings.Contains(raw, "\\") {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || strings.Contains(u.Path, "\\") {
		return ""
	}
	if u.IsAbs() || u.Host != "" {
		origin, err := url.Parse(site)
		if err != nil || origin.Host == "" || !strings.EqualFold(u.Host, origin.Host) || (u.Scheme != "https" && u.Scheme != "http") {
			return ""
		}
	}
	clean := path.Clean(u.Path)
	if !strings.HasPrefix(clean, "/upload/") && !strings.HasPrefix(clean, "/user/") && !strings.HasPrefix(clean, "/assets/") {
		return ""
	}
	return (&url.URL{Path: clean, RawQuery: u.RawQuery}).String()
}
