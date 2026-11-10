package util

import (
	"golang.org/x/net/html"
	"html/template"
	"strings"
)

// ReadingText is split from already sanitized Markdown. Moving an authored
// heading preserves its inline formatting and fragment ID in the page header.
type ReadingText struct {
	Title         string
	Heading, Body template.HTML
}

func SplitReadingHeading(rendered template.HTML, articleTitle string) ReadingText {
	result := ReadingText{Title: articleTitle, Body: rendered}
	source := strings.TrimLeft(string(rendered), " \t\r\n")
	tokens := html.NewTokenizer(strings.NewReader(source))
	if tokens.Next() != html.StartTagToken {
		return result
	}
	end := len(tokens.Raw())
	name, _ := tokens.TagName()
	if string(name) != "h1" {
		return result
	}
	var text strings.Builder
	for {
		token := tokens.Next()
		end += len(tokens.Raw())
		switch token {
		case html.ErrorToken:
			return result
		case html.TextToken:
			text.Write(tokens.Text())
		case html.EndTagToken:
			tag, _ := tokens.TagName()
			if string(tag) != "h1" {
				continue
			}
			title := strings.Join(strings.Fields(text.String()), " ")
			if title == "" || (articleTitle != "" && title != strings.Join(strings.Fields(articleTitle), " ")) {
				return result
			}
			result.Title = title
			result.Heading = template.HTML(source[:end])
			result.Body = template.HTML(source[end:])
			return result
		}
	}
}
