package service

import "testing"

func TestNormalizeCoverURLAcceptsOnlyGitHubImageSources(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: ""},
		{
			name:  "raw remains raw",
			input: "https://raw.githubusercontent.com/owariband/Seekino/main/assets/screenshots/search-results.jpg",
			want:  "https://raw.githubusercontent.com/owariband/Seekino/main/assets/screenshots/search-results.jpg",
		},
		{
			name:  "blob becomes raw",
			input: "https://github.com/owariband/Seekino/blob/main/assets/screenshots/search-results.jpg",
			want:  "https://raw.githubusercontent.com/owariband/Seekino/main/assets/screenshots/search-results.jpg",
		},
		{
			name:  "raw github path becomes raw",
			input: "https://github.com/Slimzeo/catendar/raw/main/docs/daily-board.png",
			want:  "https://raw.githubusercontent.com/Slimzeo/catendar/main/docs/daily-board.png",
		},
		{
			name:  "user attachments allowed",
			input: "https://github.com/user-attachments/assets/1234-abcd",
			want:  "https://github.com/user-attachments/assets/1234-abcd",
		},
		{name: "non github host", input: "https://cdn.example.com/cover.png", wantErr: true},
		{name: "http rejected", input: "http://raw.githubusercontent.com/o/r/main/a.png", wantErr: true},
		{name: "userinfo rejected", input: "https://user@raw.githubusercontent.com/o/r/main/a.png", wantErr: true},
		{name: "javascript rejected", input: "javascript:alert(1)", wantErr: true},
		{name: "data uri rejected", input: "data:image/png;base64,AAAA", wantErr: true},
		{name: "github repo page rejected", input: "https://github.com/owariband/Seekino", wantErr: true},
		{name: "blob without file rejected", input: "https://github.com/owariband/Seekino/blob/main", wantErr: true},
		{name: "raw without file rejected", input: "https://raw.githubusercontent.com/owariband/Seekino/main", wantErr: true},
		{name: "traversal rejected", input: "https://raw.githubusercontent.com/../../etc/passwd", wantErr: true},
		{name: "css expression rejected", input: "url(javascript:alert(1))", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeCoverURL(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("normalizeCoverURL(%q) = %q, want error", test.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeCoverURL(%q): %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("normalizeCoverURL(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestNormalizeProjectURLRequiresHTTPOrHTTPS(t *testing.T) {
	for _, valid := range []string{
		"https://github.com/owariband/Seekino",
		"http://example.com/project",
		"https://example.com/project?tab=readme",
	} {
		if got, err := normalizeProjectURL(valid); err != nil || got == "" {
			t.Fatalf("normalizeProjectURL(%q) = %q, %v", valid, got, err)
		}
	}
	for _, invalid := range []string{
		"javascript:alert(1)",
		"ftp://example.com/project",
		"https://user:pass@example.com/project",
		"example.com/project",
		"",
	} {
		if invalid == "" {
			continue
		}
		if got, err := normalizeProjectURL(invalid); err == nil {
			t.Fatalf("normalizeProjectURL(%q) = %q, want error", invalid, got)
		}
	}
}
