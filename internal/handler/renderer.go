package handler

import (
	"html/template"
	"io"
	"path/filepath"
)

// Renderer parses the migrated Go templates once at startup. html/template
// values are safe for concurrent execution after parsing.
type Renderer struct {
	templates *template.Template
}

func NewRenderer(root string, siteConfig *SiteConfig) (*Renderer, error) {
	patterns := []string{
		filepath.Join(root, "theme", "*.html"),
		filepath.Join(root, "admin", "*.html"),
		filepath.Join(root, "comm", "*.html"),
	}
	templates := template.New("blog").Funcs(buildFuncMap(siteConfig))
	for _, pattern := range patterns {
		var err error
		templates, err = templates.ParseGlob(pattern)
		if err != nil {
			return nil, err
		}
	}
	return &Renderer{templates: templates}, nil
}

func (renderer *Renderer) Render(writer io.Writer, name string, data any, _ ...string) error {
	return renderer.templates.ExecuteTemplate(writer, name, data)
}
