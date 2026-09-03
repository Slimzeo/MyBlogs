package util

import (
	"os"
	"testing"
)

func TestExtractHTMLThemeColor(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "body variable background",
			source: `<style>:root{--paper:#f2ede4}html{background:#fff}body{background:var(--paper)}</style>`,
			want:   "#e9e4d6",
		},
		{
			name:   "inline body wins",
			source: `<style>body{background:#ffffff!important}</style><body style="background-color:#223344!important"></body>`,
			want:   "#2c3c51",
		},
		{
			name:   "transparent body uses html",
			source: `<style>html{background:#182230}body{background:transparent}</style>`,
			want:   "#222b3d",
		},
		{
			name:   "theme color fallback",
			source: `<meta name="theme-color" content="#496052">`,
			want:   "#546b5f",
		},
		{
			name:   "gradient is ignored",
			source: `<style>body{background:linear-gradient(#fff,#000)}</style>`,
			want:   "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ExtractHTMLThemeColor(test.source); got != test.want {
				t.Fatalf("ExtractHTMLThemeColor() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestExtractHTMLThemeColorFromFile(t *testing.T) {
	file := os.Getenv("MYBLOG_HTML_THEME_FIXTURE")
	if file == "" {
		t.Skip("MYBLOG_HTML_THEME_FIXTURE is not set")
	}
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := ExtractHTMLThemeColor(string(source)); got == "" {
		t.Fatal("real HTML fixture did not yield a theme color")
	} else {
		t.Logf("theme color: %s", got)
	}
}
