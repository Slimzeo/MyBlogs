package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// ReferenceRoot locates the optional, locally generated home page.
func (server *Server) ReferenceRoot() string {
	root := os.Getenv("BLOG_REFERENCE_DIR")
	if root == "" {
		root = "refs_blog/payathon-2026"
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	return root
}

func (server *Server) renderReferenceHome(context *gin.Context, data PageData) bool {
	page, err := os.ReadFile(filepath.Join(server.ReferenceRoot(), "blog", "index.html"))
	if err != nil {
		return false
	}
	data.PublicSummary = server.service.GetPublicSummary()
	modules := make(map[string]bool, 4)
	for _, name := range []string{"notes", "archives", "links", "about"} {
		modules[name] = server.siteConfig.ModuleEnabled(name)
	}
	payload, err := json.Marshal(struct {
		Data        map[string]any  `json:"data"`
		Title       string          `json:"title"`
		Description string          `json:"description"`
		Modules     map[string]bool `json:"modules"`
	}{Data: map[string]any{
		"PublicSummary": data.PublicSummary,
		"HomeContent":   data.HomeContent,
		"Articles":      data.Articles,
		"Tags":          data.Tags,
		"Links":         data.Links,
		"TopicGroups":   data.TopicGroups,
	}, Title: server.siteConfig.Title(), Description: server.siteConfig.Option("site_description", ""), Modules: modules})
	if err != nil {
		return false
	}
	marker := []byte("<!--BLOG_HOME_DATA-->")
	if bytes.Count(page, marker) != 1 {
		return false
	}
	block := append([]byte(`<script id="blog-home-data" type="application/json">`), payload...)
	block = append(block, []byte(`</script>`)...)
	page = bytes.Replace(page, marker, block, 1)
	context.Header("Cache-Control", "private, no-store")
	context.Header("Vary", "Cookie")
	context.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' blob:; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; worker-src 'self' blob:; media-src 'self' blob:; object-src 'none'; base-uri 'self'; form-action 'self'")
	context.Data(http.StatusOK, "text/html; charset=utf-8", page)
	return true
}

// ReferenceAsset serves regular files only; directories never produce listings.
func ReferenceAsset(root string) gin.HandlerFunc {
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	fs := http.FileServer(http.Dir(root))
	return func(context *gin.Context) {
		name := filepath.Clean("/" + context.Param("filepath"))
		filename := filepath.Join(root, name)
		resolved, err := filepath.EvalSymlinks(filename)
		if err != nil {
			context.Status(http.StatusNotFound)
			return
		}
		relative, err := filepath.Rel(root, resolved)
		if err != nil || relative == ".." || (len(relative) > 3 && relative[:3] == "../") {
			context.Status(http.StatusNotFound)
			return
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			context.Status(http.StatusNotFound)
			return
		}
		request := context.Request.Clone(context.Request.Context())
		request.URL.Path = name
		fs.ServeHTTP(context.Writer, request)
	}
}
