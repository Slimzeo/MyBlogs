package util

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Keep TeX opaque until KaTeX receives it. Code spans and fenced code already
// have their own Goldmark nodes, so examples there remain literal.
type mathMarkdown struct{}

func (m mathMarkdown) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(parser.WithInlineParsers(util.Prioritized(m, 150)),
		parser.WithBlockParsers(util.Prioritized(displayFence{}, 750)))
	md.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(m, 100)))
}

var (
	mathSpanKind  = ast.NewNodeKind("MathSpan")
	mathFenceKind = ast.NewNodeKind("MathFence")
)

type mathSpan struct {
	ast.BaseInline
	source  []byte
	display bool
}

func (*mathSpan) Kind() ast.NodeKind           { return mathSpanKind }
func (n *mathSpan) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

func (mathMarkdown) Trigger() []byte { return []byte{'$', '\\'} }

func (mathMarkdown) Parse(_ ast.Node, reader text.Reader, _ parser.Context) ast.Node {
	line, _ := reader.PeekLine()
	var close string
	switch {
	case bytes.HasPrefix(line, []byte("$$")):
		close = "$$"
	case bytes.HasPrefix(line, []byte(`\(`)):
		close = `\)`
	case bytes.HasPrefix(line, []byte(`\[`)):
		close = `\]`
	case line[0] == '$' && len(line) > 1 && !util.IsSpace(line[1]):
		close = "$"
	default:
		return nil
	}
	var source bytes.Buffer
	width := len(close)
	source.Write(line[:width])
	reader.Advance(width)
	for {
		line, _ = reader.PeekLine()
		for i := 0; i < len(line); i++ {
			if close == "$" && line[i] == '`' {
				return nil // A prose price must not match a dollar inside a code span.
			}
			if bytes.HasPrefix(line[i:], []byte(close)) {
				// A closing dollar must touch its formula and cannot introduce a price.
				if close == "$" && (i == 0 || util.IsSpace(line[i-1]) || (i+1 < len(line) && util.IsNumeric(line[i+1]))) {
					return nil
				}
				source.Write(line[:i+width])
				reader.Advance(i + width)
				return &mathSpan{source: source.Bytes(), display: close == "$$" || close == `\]`}
			}
			if line[i] == '\\' {
				i++ // TeX escapes the next character, including a delimiter.
			}
		}
		if len(line) == 0 || close == "$" {
			return nil
		}
		source.Write(line)
		reader.AdvanceLine()
	}
}

type displayFence struct{}
type mathFence struct {
	ast.BaseBlock
	close string
}

func (*mathFence) Kind() ast.NodeKind                            { return mathFenceKind }
func (*mathFence) IsRaw() bool                                   { return true }
func (n *mathFence) Dump(src []byte, level int)                  { ast.DumpHelper(n, src, level, nil, nil) }
func (displayFence) Trigger() []byte                             { return []byte{'$', '\\'} }
func (displayFence) CanInterruptParagraph() bool                 { return true }
func (displayFence) CanAcceptIndentedLine() bool                 { return false }
func (displayFence) Close(ast.Node, text.Reader, parser.Context) {}

// Check for a closing line before consuming a block. An unfinished delimiter
// in the live editor must still allow the following paragraphs to render.
var displayClosers = map[string]*regexp.Regexp{
	"$$": regexp.MustCompile(`(?m)^[ \t>]*\$\$[ \t]*\r?$`),
	`\]`: regexp.MustCompile(`(?m)^[ \t>]*\\\][ \t]*\r?$`),
}

func (displayFence) Open(_ ast.Node, reader text.Reader, _ parser.Context) (ast.Node, parser.State) {
	line, seg := reader.PeekLine()
	close := string(bytes.TrimSpace(line))
	switch close {
	case `\[`:
		close = `\]`
	case "$$":
	default:
		return nil, parser.NoChildren
	}
	if !displayClosers[close].Match(reader.Source()[seg.Stop:]) {
		return nil, parser.NoChildren
	}
	node := &mathFence{close: close}
	node.Lines().Append(seg)
	return node, parser.NoChildren
}

func (displayFence) Continue(n ast.Node, reader text.Reader, _ parser.Context) parser.State {
	line, segment := reader.PeekLine()
	n.Lines().Append(segment)
	reader.AdvanceToEOL()
	if string(bytes.TrimSpace(line)) == n.(*mathFence).close {
		return parser.Close
	}
	return parser.Continue | parser.NoChildren
}

func (mathMarkdown) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(mathSpanKind, renderMathSource)
	reg.Register(mathFenceKind, renderMathSource)
}

func renderMathSource(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	var tex []byte
	tag, class := "span", "math-inline"
	switch node := n.(type) {
	case *mathSpan:
		tex = node.source
		if node.display {
			class = "math-display"
		}
	case *mathFence:
		tag, class = "div", "math-display"
		tex = bytes.TrimSpace(node.Lines().Value(source))
	}
	w.WriteString("<" + tag + " class=\"" + class + "\">")
	w.Write(util.EscapeHTML(tex))
	w.WriteString("</" + tag + ">")
	return ast.WalkSkipChildren, nil
}
