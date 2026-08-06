package util

import (
	"crypto/rand"
	"encoding/hex"
	"net"
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
	remoteIP := parseRemoteIP(r.RemoteAddr)
	if remoteIP != nil && remoteIP.IsLoopback() {
		if value := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(value) != nil {
			return value
		}
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			for index := len(parts) - 1; index >= 0; index-- {
				value := strings.TrimSpace(parts[index])
				if net.ParseIP(value) != nil {
					return value
				}
			}
		}
	}
	if remoteIP != nil {
		return remoteIP.String()
	}
	return r.RemoteAddr
}

func parseRemoteIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return net.ParseIP(strings.TrimSpace(host))
}
