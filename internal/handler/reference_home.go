package handler

import (
	"bytes"
	"html"
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

func (server *Server) renderReferencePage(context *gin.Context, data PageData, partial string) bool {
	filename := "index.html"
	if !data.Home {
		filename = "journal.html"
	}
	page, err := os.ReadFile(filepath.Join(server.ReferenceRoot(), "blog", filename))
	if err != nil {
		return false
	}
	// Both shells render the very same navigation, including enabled modules and access controls.
	for _, slot := range []struct{ marker, partial string }{
		{"<!--BLOG_TOPBAR-->", "topbar"}, {"<!--BLOG_DIALOGS-->", "topbar-dialogs"}, {"<!--BLOG_FEED-->", partial},
	} {
		marker := []byte(slot.marker)
		if bytes.Count(page, marker) != 1 {
			return false
		}
		var fragment bytes.Buffer
		if err := server.renderer.Render(&fragment, slot.partial, data); err != nil {
			return false
		}
		page = bytes.Replace(page, marker, fragment.Bytes(), 1)
	}
	title := "HYPNOS · Blog"
	if data.Title != "" {
		title = data.Title + " · HYPNOS"
	}
	page = bytes.Replace(page, []byte("<!--BLOG_TITLE-->"), []byte(html.EscapeString(title)), 1)
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
