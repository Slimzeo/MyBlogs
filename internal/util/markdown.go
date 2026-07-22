package util

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// md is configured once. GFM + unsafe HTML passthrough (the original commonmark
// renderer allowed raw HTML); we sanitize the output separately for safety.
var md = goldmark.New(
	goldmark.WithParser(parser.NewParser(
		parser.WithBlockParsers(parser.DefaultBlockParsers()[1:]...),
		parser.WithInlineParsers(parser.DefaultInlineParsers()...),
		parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
	)),
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithUnsafe(), html.WithHardWraps()),
)

// sanitizer scrubs dangerous markup while keeping the tags a blog post needs
// (images, code, tables, links). This is stricter than the Java original,
// which did no server-side sanitization at all.
var sanitizer = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("class").Globally()
	p.AllowAttrs("id").Globally()
	p.AllowAttrs("frameborder", "border", "marginwidth", "marginheight", "width", "height", "src", "allowfullscreen").OnElements("iframe")
	p.AllowElements("iframe")
	return p
}()

// MdToHTML converts markdown to sanitized HTML, mirroring TaleUtils.mdToHtml.
func MdToHTML(markdown string) string {
	if strings.TrimSpace(markdown) == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return ""
	}
	return sanitizer.Sanitize(buf.String())
}

var firstImg = regexp.MustCompile(`(?i)<img[^>]*?src\s*=\s*['"]?([^'">\s]+)`)

// FirstImage returns the first image URL from rendered markdown (Commons.show_thumb).
func FirstImage(content string) string {
	htmlStr := MdToHTML(content)
	m := firstImg.FindStringSubmatch(htmlStr)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

// Intro extracts an article summary, mirroring Commons.intro: content before
// <!--more--> if present, otherwise the first `length` runes of plain text.
func Intro(value string, length int) string {
	if pos := strings.Index(value, "<!--more-->"); pos != -1 {
		return HTMLToText(MdToHTML(value[:pos]))
	}
	text := HTMLToText(MdToHTML(value))
	r := []rune(text)
	if len(r) > length {
		return string(r[:length])
	}
	return text
}

// Article renders an article body to HTML, mirroring Commons.article.
func Article(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "<!--more-->", "\r\n")
	return MdToHTML(value)
}
