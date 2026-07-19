package util

import (
	"crypto/md5"
	"encoding/hex"
	"regexp"
	"strings"
)

// MD5encode mirrors TaleUtils.MD5encode: lowercase hex md5 of the UTF-8 bytes.
func MD5encode(source string) string {
	if strings.TrimSpace(source) == "" {
		return ""
	}
	sum := md5.Sum([]byte(source))
	return hex.EncodeToString(sum[:])
}

var (
	validEmail = regexp.MustCompile(`(?i)^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,6}$`)
	slugRegex  = regexp.MustCompile(`(?i)^[A-Za-z0-9_-]{5,100}$`)
	urlRegex   = regexp.MustCompile(`^(https?://(w{3}\.)?)?\w+\.\w+(\.[a-zA-Z]+)*(:\d{1,5})?(/\w*)*(\??(.+=.*)?(&.+=.*)?)?$`)
	numberRe   = regexp.MustCompile(`^\d+$`)
)

// IsEmail mirrors TaleUtils.isEmail.
func IsEmail(s string) bool { return validEmail.MatchString(s) }

// IsURL mirrors PatternKit.isURL.
func IsURL(s string) bool { return urlRegex.MatchString(s) }

// IsNumber reports whether s is a non-empty run of digits (Tools.isNumber).
func IsNumber(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && numberRe.MatchString(s)
}

// IsPath mirrors TaleUtils.isPath: no slash/space/dot and 5-100 slug chars.
func IsPath(slug string) bool {
	if strings.TrimSpace(slug) == "" {
		return false
	}
	if strings.ContainsAny(slug, "/ .") {
		return false
	}
	return slugRegex.MatchString(slug)
}

// CleanXSS mirrors TaleUtils.cleanXSS: escape a handful of dangerous tokens.
func CleanXSS(value string) string {
	r := strings.NewReplacer(
		"<", "&lt;",
		">", "&gt;",
		"(", "&#40;",
		")", "&#41;",
		"'", "&#39;",
	)
	value = r.Replace(value)
	value = regexp.MustCompile(`eval\((.*)\)`).ReplaceAllString(value, "")
	value = regexp.MustCompile(`[\"\'][\s]*javascript:(.*)[\"\']`).ReplaceAllString(value, `""`)
	value = strings.ReplaceAll(value, "script", "")
	return value
}

// HTMLToText strips tags, mirroring TaleUtils.htmlToText.
var tagStrip = regexp.MustCompile(`(?s)<[^>]*>(\s*<[^>]*>)*`)

func HTMLToText(html string) string {
	if strings.TrimSpace(html) == "" {
		return ""
	}
	return tagStrip.ReplaceAllString(html, " ")
}
