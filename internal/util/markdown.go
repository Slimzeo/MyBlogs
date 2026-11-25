package util

import (
	"bytes"
	stdhtml "html"
	"regexp"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

// Highlight code in article bodies and editor previews. Excerpts and thumbnail
// detection use the regular Markdown renderer.
var md = newMarkdown()
var articleMarkdown = newMarkdown(highlighting.NewHighlighting(
	highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
	highlighting.WithCodeBlockOptions(func(context highlighting.CodeBlockContext) []chromahtml.Option {
		language, _ := context.Language()
		return []chromahtml.Option{chromahtml.WithPreWrapper(codePreWrapper(language))}
	}),
))

func newMarkdown(extras ...goldmark.Extender) goldmark.Markdown {
	extensions := append([]goldmark.Extender{extension.GFM, mathMarkdown{}}, extras...)
	return goldmark.New(
		goldmark.WithParser(parser.NewParser(
			parser.WithBlockParsers(parser.DefaultBlockParsers()[1:]...),
			parser.WithInlineParsers(parser.DefaultInlineParsers()...),
			parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
		)),
		goldmark.WithExtensions(extensions...),
		goldmark.WithRendererOptions(html.WithUnsafe(), html.WithHardWraps()),
	)
}

// Keep the fence's language class on highlighted code, as on plain code.
type codePreWrapper string

func (language codePreWrapper) Start(code bool, attrs string) string {
	tag := "<pre" + attrs + ">"
	if code {
		tag += "<code class=\"language-" + stdhtml.EscapeString(string(language)) + "\">"
	}
	return tag
}

func (codePreWrapper) End(code bool) string {
	if code {
		return "</code></pre>"
	}
	return "</pre>"
}

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
	return renderMarkdown(md, markdown)
}

func renderMarkdown(engine goldmark.Markdown, markdown string) string {
	if strings.TrimSpace(markdown) == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := engine.Convert([]byte(markdown), &buf); err != nil {
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
	value = strings.ReplaceAll(value, "<!--more-->", "\r\n")
	return renderMarkdown(articleMarkdown, value)
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
