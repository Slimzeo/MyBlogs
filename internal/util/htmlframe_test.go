package util

import (
	"strings"
	"testing"
)

func TestPrepareHTMLForFrame(t *testing.T) {
	source := `<!doctype html><html><head><title>article</title></head><body><main>content</main></body></html>`
	prepared := PrepareHTMLForFrame(source)
	for _, marker := range []string{
		`name="viewport"`,
		`myblog:html-size`,
		`myblog:html-ready`,
		`myblog:html-viewport`,
		`root?root.scrollWidth:0`,
		`ResizeObserver`,
		`MutationObserver`,
		`readyImageRatio=.75`,
		`pageLoaded=document.readyState==="complete"`,
		`image.loading==="lazy"&&!nearViewport`,
		`return relevant?completed/relevant:1`,
		`document.querySelectorAll("[data-reveal]")`,
		`classList.add("is-visible")`,
		`lastHeight=0`,
		`<main>content</main>`,
	} {
		if !strings.Contains(prepared, marker) {
			t.Fatalf("prepared HTML missing %q", marker)
		}
	}
	if HTMLFrameDocumentVersion != "3" || !strings.Contains(prepared, `version:protocolVersion`) {
		t.Fatal("prepared HTML is missing frame protocol version 3")
	}
	if strings.Count(prepared, `name="viewport"`) != 1 {
		t.Fatalf("viewport count = %d, want 1", strings.Count(prepared, `name="viewport"`))
	}
}

func TestPrepareHTMLForFrameKeepsExistingViewport(t *testing.T) {
	source := `<html><head><meta name="viewport" content="width=device-width"></head><body>content</body></html>`
	prepared := PrepareHTMLForFrame(source)
	if strings.Count(prepared, `name="viewport"`) != 1 {
		t.Fatalf("viewport count = %d, want 1", strings.Count(prepared, `name="viewport"`))
	}
}
