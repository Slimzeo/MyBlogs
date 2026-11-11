package util

import (
	"reflect"
	"strings"
	"testing"
)

func TestArticlePreservesTeXBeforeMarkdown(t *testing.T) {
	input := "中文 $x_i * y_j$ 和 " + `\(\text{a&b} < c\)` + "。\n\n" +
		"$$\n" + `\begin{aligned}x &= a \\` + "\n- b\n\n" + `\end{aligned}` + "\n$$\n\n" +
		`同一行 \[\frac{a}{b}\]。` + "\n\n" +
		"> " + `\[` + "\n> " + `x_1\leq x_2` + "\n> " + `\]` + "\n\n" +
		"| 变量 | 值 |\n| --- | --- |\n| $x$ | $2$ |"
	html := Article(input)
	for _, want := range []string{
		`<span class="math-inline">$x_i * y_j$</span>`,
		`<span class="math-inline">\(\text{a&amp;b} &lt; c\)</span>`,
		`<div class="math-display">$$` + "\n" + `\begin{aligned}x &amp;= a \\` + "\n- b\n\n" + `\end{aligned}` + "\n$$</div>",
		`<span class="math-display">\[\frac{a}{b}\]</span>`,
		`<div class="math-display">\[` + "\n" + `x_1\leq x_2` + "\n" + `\]</div>`,
		`<td><span class="math-inline">$2$</span></td>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in %s", want, html)
		}
	}
}

func TestArticleKeepsCodeCurrencyAndUnclosedMathLiteral(t *testing.T) {
	input := "Costs $5 and $10. Use " + "`$x$`" + " or " + `\$2` + ".\n\n" +
		"```python\nprint('$x_i$')\n```\n\n" + "Unclosed $x\n\n$$\n\n# Next section\n\n" +
		`写作 $a+\$5$ 和 \(a\\b\)。`
	html := Article(input)
	if strings.Count(html, `class="math-inline"`) != 2 || strings.Contains(html, `class="math-display"`) {
		t.Fatalf("unexpected formulas: %s", html)
	}
	for _, want := range []string{"$5 and $10", "<code>$x$</code>", "$2.", `class="language-python"`, "$x_i$", "Unclosed $x", "<h1>Next section</h1>", `$a+\$5$`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in %s", want, html)
		}
	}
}

func TestMathBlocksPreserveEditorScrollAnchors(t *testing.T) {
	source := "先看 $x$。\n\n$$\na+b\n$$\n\n然后看 `$x$`。"
	if got, want := ArticleBlockLines(source), []int{1, 3, 7}; !reflect.DeepEqual(got, want) {
		t.Errorf("block lines = %v, want %v", got, want)
	}
}
