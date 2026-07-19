package util

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

// Token returns a URL-safe random token, used for CSRF tokens (replaces UUID.UU64).
func Token() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		// extremely unlikely; fall back to time-seeded pseudo-random
		return MD5encode(RandomNumber(16))
	}
	return hex.EncodeToString(b)
}

// UU32 returns a 32-char lowercase hex random id, used for upload file names.
func UU32() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return MD5encode(RandomNumber(16))
	}
	return hex.EncodeToString(b)
}

// ClientIP extracts the caller IP, mirroring IPKit.getIpAddrByRequest.
func ClientIP(r *http.Request) string {
	for _, h := range []string{"x-forwarded-for", "Proxy-Client-IP", "WL-Proxy-Client-IP"} {
		v := r.Header.Get(h)
		if v != "" && !strings.EqualFold(v, "unknown") {
			// x-forwarded-for may be a list; take the first.
			if i := strings.IndexByte(v, ','); i >= 0 {
				return strings.TrimSpace(v[:i])
			}
			return v
		}
	}
	// RemoteAddr is host:port.
	if i := strings.LastIndexByte(r.RemoteAddr, ':'); i >= 0 {
		return r.RemoteAddr[:i]
	}
	return r.RemoteAddr
}
