package util

import (
	"bytes"
	stdhtml "html"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
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
var nonVisibleHTML = regexp.MustCompile(`(?is)<(?:head|script|style|noscript|template|svg)[^>]*>.*?</(?:head|script|style|noscript|template|svg)\s*>`)

// FirstImage returns the first image URL from rendered markdown (Commons.show_thumb).
func FirstImage(content string) string {
	htmlStr := MdToHTML(content)
	m := firstImg.FindStringSubmatch(htmlStr)
	if len(m) == 2 {
		if strings.HasPrefix(strings.ToLower(m[1]), "data:") {
			return ""
		}
		return m[1]
	}
	return ""
}

// ContentFirstImage returns the first image for either supported article format.
func ContentFirstImage(content, format string) string {
	htmlStr := content
	if format != "html" {
		htmlStr = MdToHTML(content)
	}
	m := firstImg.FindStringSubmatch(htmlStr)
	if len(m) == 2 {
		if strings.HasPrefix(strings.ToLower(m[1]), "data:") {
			return ""
		}
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

// ContentIntro extracts plain-text summary from Markdown or a complete HTML document.
func ContentIntro(value, format string, length int) string {
	if format != "html" {
		return Intro(value, length)
	}
	if pos := strings.Index(value, "<!--more-->"); pos != -1 {
		value = value[:pos]
	}
	value = nonVisibleHTML.ReplaceAllString(value, " ")
	text := strings.Join(strings.Fields(stdhtml.UnescapeString(HTMLToText(value))), " ")
	runes := []rune(text)
	if len(runes) > length {
		return string(runes[:length])
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

// ArticleBlockLines returns the source line for each top-level Markdown block.
// The editor uses these anchors to map source scrolling to rendered blocks.
func ArticleBlockLines(value string) []int {
	if strings.TrimSpace(value) == "" {
		return []int{}
	}
	value = strings.ReplaceAll(value, "<!--more-->", "\r\n")
	source := []byte(value)
	document := md.Parser().Parse(text.NewReader(source))
	lines := make([]int, 0, document.ChildCount())
	for node := document.FirstChild(); node != nil; node = node.NextSibling() {
		start := node.Pos()
		if segments := node.Lines(); segments != nil && segments.Len() > 0 {
			start = segments.At(0).Start
		}
		if start < 0 || start > len(source) {
			continue
		}
		lines = append(lines, 1+bytes.Count(source[:start], []byte{'\n'}))
	}
	return lines
}
