package util

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/aymerick/douceur/css"
	"github.com/aymerick/douceur/parser"
	"golang.org/x/net/html"
)

const HTMLThemeColorVersion = 1

type themeColor struct {
	r, g, b float64
	a       float64
}

type cssChoice struct {
	value     string
	important bool
	rank      int
	order     int
}

type themeCSS struct {
	background map[string]cssChoice
	variables  map[string]map[string]cssChoice
	order      int
}

var (
	hexColorPattern = regexp.MustCompile(`(?i)^#([0-9a-f]{3,8})$`)
	rgbColorPattern = regexp.MustCompile(`(?i)^rgba?\((.*)\)$`)
	varColorPattern = regexp.MustCompile(`(?is)^var\(\s*(--[a-z0-9_-]+)\s*(?:,\s*(.+))?\)$`)
)

// ExtractHTMLThemeColor finds the document canvas color and returns a nearby
// host-page color. It intentionally ignores images and gradients so importing
// an article stays deterministic and bounded.
func ExtractHTMLThemeColor(source string) string {
	styles, inline, metaColor := collectHTMLThemeSources(source)
	theme := themeCSS{
		background: map[string]cssChoice{},
		variables: map[string]map[string]cssChoice{
			"root": {},
			"html": {},
			"body": {},
		},
	}
	for _, style := range styles {
		theme.addStylesheet(style)
	}
	for _, target := range []string{"html", "body"} {
		if inline[target] != "" {
			theme.addDeclarations(target, inline[target], 100)
		}
	}

	htmlVars := mergeThemeVariables(theme.variables["root"], theme.variables["html"])
	htmlColor, htmlOK := parseThemeChoice(theme.background["html"], htmlVars, nil)
	bodyVars := mergeThemeVariables(theme.variables["root"], theme.variables["html"], theme.variables["body"])
	var backdrop *themeColor
	if htmlOK {
		backdrop = &htmlColor
	}
	if bodyColor, ok := parseThemeChoice(theme.background["body"], bodyVars, backdrop); ok {
		return adjacentThemeColor(bodyColor)
	}
	if htmlOK {
		return adjacentThemeColor(htmlColor)
	}
	if color, ok := parseThemeColor(metaColor, nil, nil, 0); ok {
		return adjacentThemeColor(color)
	}
	return ""
}

func collectHTMLThemeSources(source string) ([]string, map[string]string, string) {
	tokenizer := html.NewTokenizer(strings.NewReader(source))
	inline := map[string]string{}
	var styles []string
	var styleText strings.Builder
	insideStyle := false
	metaColor := ""
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return styles, inline, metaColor
		case html.StartTagToken, html.SelfClosingTagToken:
			nameBytes, hasAttributes := tokenizer.TagName()
			name := strings.ToLower(string(nameBytes))
			attributes := map[string]string{}
			for hasAttributes {
				key, value, more := tokenizer.TagAttr()
				attributes[strings.ToLower(string(key))] = string(value)
				hasAttributes = more
			}
			switch name {
			case "style":
				insideStyle = true
				styleText.Reset()
			case "html", "body":
				inline[name] = attributes["style"]
			case "meta":
				if metaColor == "" && strings.EqualFold(attributes["name"], "theme-color") && attributes["media"] == "" {
					metaColor = attributes["content"]
				}
			}
		case html.TextToken:
			if insideStyle {
				styleText.Write(tokenizer.Text())
			}
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			if insideStyle && strings.EqualFold(string(name), "style") {
				styles = append(styles, styleText.String())
				insideStyle = false
			}
		}
	}
}

func (theme *themeCSS) addStylesheet(source string) {
	stylesheet, err := parser.Parse(source)
	if err != nil {
		return
	}
	for _, rule := range stylesheet.Rules {
		if rule.Kind != css.QualifiedRule {
			continue
		}
		for _, selector := range rule.Selectors {
			target := simpleThemeTarget(selector)
			if target != "" {
				theme.applyDeclarations(target, rule.Declarations, 10)
			}
		}
	}
}

func (theme *themeCSS) addDeclarations(target, source string, rank int) {
	declarations, err := parser.ParseDeclarations(source + ";")
	if err == nil {
		theme.applyDeclarations(target, declarations, rank)
	}
}

func (theme *themeCSS) applyDeclarations(target string, declarations []*css.Declaration, rank int) {
	for _, declaration := range declarations {
		theme.order++
		property := strings.ToLower(strings.TrimSpace(declaration.Property))
		choice := cssChoice{
			value:     strings.TrimSpace(declaration.Value),
			important: declaration.Important,
			rank:      rank,
			order:     theme.order,
		}
		if property == "background" || property == "background-color" {
			theme.background[target] = chooseThemeCSS(theme.background[target], choice)
		}
		if strings.HasPrefix(property, "--") {
			theme.variables[target][property] = chooseThemeCSS(theme.variables[target][property], choice)
		}
	}
}

func simpleThemeTarget(selector string) string {
	selector = strings.ToLower(strings.TrimSpace(selector))
	switch selector {
	case ":root":
		return "root"
	case "html":
		return "html"
	case "body":
		return "body"
	default:
		return ""
	}
}

func chooseThemeCSS(current, candidate cssChoice) cssChoice {
	if current.value == "" ||
		(candidate.important && !current.important) ||
		(candidate.important == current.important && candidate.rank > current.rank) ||
		(candidate.important == current.important && candidate.rank == current.rank && candidate.order >= current.order) {
		return candidate
	}
	return current
}

func mergeThemeVariables(groups ...map[string]cssChoice) map[string]string {
	result := map[string]string{}
	for _, group := range groups {
		for name, choice := range group {
			result[name] = choice.value
		}
	}
	return result
}

func parseThemeChoice(choice cssChoice, variables map[string]string, backdrop *themeColor) (themeColor, bool) {
	if choice.value == "" {
		return themeColor{}, false
	}
	return parseThemeColor(choice.value, variables, backdrop, 0)
}

func parseThemeColor(value string, variables map[string]string, backdrop *themeColor, depth int) (themeColor, bool) {
	if depth > 8 {
		return themeColor{}, false
	}
	value = strings.TrimSpace(value)
	if match := varColorPattern.FindStringSubmatch(value); len(match) == 3 {
		if resolved := variables[strings.ToLower(match[1])]; resolved != "" {
			return parseThemeColor(resolved, variables, backdrop, depth+1)
		}
		if match[2] != "" {
			return parseThemeColor(match[2], variables, backdrop, depth+1)
		}
		return themeColor{}, false
	}
	color, ok := parseLiteralThemeColor(value)
	if !ok || color.a <= 0 {
		return themeColor{}, false
	}
	if color.a < 1 {
		base := themeColor{r: 255, g: 255, b: 255, a: 1}
		if backdrop != nil {
			base = *backdrop
		}
		color.r = color.r*color.a + base.r*(1-color.a)
		color.g = color.g*color.a + base.g*(1-color.a)
		color.b = color.b*color.a + base.b*(1-color.a)
		color.a = 1
	}
	return color, true
}

func parseLiteralThemeColor(value string) (themeColor, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if match := hexColorPattern.FindStringSubmatch(value); len(match) == 2 {
		return parseHexThemeColor(match[1])
	}
	if match := rgbColorPattern.FindStringSubmatch(value); len(match) == 2 {
		return parseRGBThemeColor(match[1])
	}
	named := map[string]themeColor{
		"black":       {0, 0, 0, 1},
		"white":       {255, 255, 255, 1},
		"gray":        {128, 128, 128, 1},
		"grey":        {128, 128, 128, 1},
		"whitesmoke":  {245, 245, 245, 1},
		"ivory":       {255, 255, 240, 1},
		"transparent": {0, 0, 0, 0},
	}
	color, ok := named[value]
	return color, ok
}

func parseHexThemeColor(value string) (themeColor, bool) {
	if len(value) == 3 || len(value) == 4 {
		expanded := make([]byte, 0, len(value)*2)
		for index := range len(value) {
			expanded = append(expanded, value[index], value[index])
		}
		value = string(expanded)
	}
	if len(value) != 6 && len(value) != 8 {
		return themeColor{}, false
	}
	parsed, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return themeColor{}, false
	}
	if len(value) == 6 {
		return themeColor{float64(parsed >> 16), float64((parsed >> 8) & 0xff), float64(parsed & 0xff), 1}, true
	}
	return themeColor{float64(parsed >> 24), float64((parsed >> 16) & 0xff), float64((parsed >> 8) & 0xff), float64(parsed&0xff) / 255}, true
}

func parseRGBThemeColor(value string) (themeColor, bool) {
	parts := strings.Split(value, ",")
	if len(parts) != 3 && len(parts) != 4 {
		return themeColor{}, false
	}
	channels := [4]float64{0, 0, 0, 1}
	for index, part := range parts {
		part = strings.TrimSpace(part)
		percent := strings.HasSuffix(part, "%")
		part = strings.TrimSuffix(part, "%")
		parsed, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return themeColor{}, false
		}
		if index < 3 {
			if percent {
				parsed = parsed * 2.55
			}
			channels[index] = math.Max(0, math.Min(255, parsed))
		} else {
			if percent {
				parsed /= 100
			}
			channels[index] = math.Max(0, math.Min(1, parsed))
		}
	}
	return themeColor{channels[0], channels[1], channels[2], channels[3]}, true
}

func adjacentThemeColor(color themeColor) string {
	hue, saturation, lightness := rgbToHSL(color.r/255, color.g/255, color.b/255)
	hue = math.Mod(hue+4, 360)
	saturation *= .88
	if lightness >= .55 {
		lightness = math.Max(.08, lightness-.045)
	} else {
		lightness = math.Min(.92, lightness+.045)
	}
	r, g, b := hslToRGB(hue, saturation, lightness)
	return fmt.Sprintf("#%02x%02x%02x", int(math.Round(r*255)), int(math.Round(g*255)), int(math.Round(b*255)))
}

func rgbToHSL(r, g, b float64) (float64, float64, float64) {
	maximum := math.Max(r, math.Max(g, b))
	minimum := math.Min(r, math.Min(g, b))
	lightness := (maximum + minimum) / 2
	if maximum == minimum {
		return 0, 0, lightness
	}
	delta := maximum - minimum
	saturation := delta / (1 - math.Abs(2*lightness-1))
	var hue float64
	switch maximum {
	case r:
		hue = 60 * math.Mod((g-b)/delta, 6)
	case g:
		hue = 60 * ((b-r)/delta + 2)
	default:
		hue = 60 * ((r-g)/delta + 4)
	}
	if hue < 0 {
		hue += 360
	}
	return hue, saturation, lightness
}

func hslToRGB(hue, saturation, lightness float64) (float64, float64, float64) {
	chroma := (1 - math.Abs(2*lightness-1)) * saturation
	x := chroma * (1 - math.Abs(math.Mod(hue/60, 2)-1))
	var r, g, b float64
	switch {
	case hue < 60:
		r, g = chroma, x
	case hue < 120:
		r, g = x, chroma
	case hue < 180:
		g, b = chroma, x
	case hue < 240:
		g, b = x, chroma
	case hue < 300:
		r, b = x, chroma
	default:
		r, b = chroma, x
	}
	match := lightness - chroma/2
	return r + match, g + match, b + match
}
