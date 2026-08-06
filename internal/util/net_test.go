package util

import (
	"net/http"
	"testing"
)

func TestClientIPPrefersRealIP(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Real-IP", "203.0.113.10")
	request.Header.Set("X-Forwarded-For", "198.51.100.22, 203.0.113.10")

	if got := ClientIP(request); got != "203.0.113.10" {
		t.Fatalf("ClientIP = %q, want X-Real-IP", got)
	}
}

func TestClientIPFallsBackToForwardedAndRemote(t *testing.T) {
	request, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.22, 203.0.113.10")

	if got := ClientIP(request); got != "203.0.113.10" {
		t.Fatalf("ClientIP = %q, want nearest forwarded proxy value", got)
	}

	request.Header.Del("X-Forwarded-For")
	if got := ClientIP(request); got != "127.0.0.1" {
		t.Fatalf("ClientIP = %q, want remote host", got)
	}
}
