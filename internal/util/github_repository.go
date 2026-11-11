package util

import (
	"net/url"
	"regexp"
	"strings"
)

var repositoryPath = regexp.MustCompile(`^/[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9_.-]+$`)

// GitHubRepositoryURL returns the HTTPS homepage of a GitHub repository.
func GitHubRepositoryURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || !(u.Scheme == "https" || u.Scheme == "http") ||
		!(strings.EqualFold(u.Host, "github.com") || strings.EqualFold(u.Host, "www.github.com")) {
		return "", false
	}
	path := strings.TrimSuffix(strings.TrimSuffix(u.EscapedPath(), "/"), ".git")
	if !repositoryPath.MatchString(path) || strings.HasSuffix(path, "/.") || strings.HasSuffix(path, "/..") {
		return "", false
	}
	return "https://github.com" + path, true
}
