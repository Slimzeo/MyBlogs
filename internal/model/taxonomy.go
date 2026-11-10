package model

import (
	"errors"
	"strings"
)

const DefaultColumn = "默认分类"

// ArticleColumn accepts one distinct column and supplies the existing fallback
// for older clients which omit the field. Tag lists use MetaNames directly.
func ArticleColumn(value string) (string, error) {
	names := MetaNames(value)
	switch len(names) {
	case 0:
		return DefaultColumn, nil
	case 1:
		return names[0], nil
	default:
		return "", errors.New("一篇文章只能属于一个专栏。请选择一个专栏，其他关键词可以填入标签。")
	}
}

// MetaNames preserves input order and accepts both comma styles.
func MetaNames(value string) []string {
	value = strings.ReplaceAll(value, "，", ",")
	seen := make(map[string]struct{})
	names := make([]string, 0)
	for _, candidate := range strings.Split(value, ",") {
		name := strings.TrimSpace(candidate)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}
