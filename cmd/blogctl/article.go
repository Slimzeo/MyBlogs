package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

const (
	maxArticleArchiveSize  = 16 << 20
	maxArticleExpandedSize = 32 << 20
	maxArticleAssetSize    = 4 << 20
	maxArticleHTMLSize     = 8 << 20
	maxArticleMarkdownSize = 200000
	maxArticleEntries      = 100
)

var (
	frontMatterPattern  = regexp.MustCompile(`(?s)\A---[ \t]*\r?\n(.*?)\r?\n---[ \t]*(?:\r?\n|$)`)
	markdownLinkPattern = regexp.MustCompile(`!?\[[^\]]*\]\(\s*<?([^>\s)]+)>?(?:\s+["'][^"']*["'])?\s*\)`)
	htmlLinkPattern     = regexp.MustCompile(`(?i)(?:src|href|poster)\s*=\s*["']([^"']+)["']`)
	slugPattern         = regexp.MustCompile(`^[A-Za-z0-9_-]{5,100}$`)
)

type articleMetadata struct {
	Title       string `json:"title,omitempty"`
	Slug        string `json:"slug,omitempty"`
	Tags        string `json:"tags,omitempty"`
	Categories  string `json:"categories,omitempty"`
	DisplayTime string `json:"displayTime,omitempty"`
	Status      string `json:"status"`
}

type articleOverrides struct {
	Title       string
	Slug        string
	Tags        string
	Categories  string
	DisplayTime string
}

type articleUpload struct {
	Filename string
	Data     []byte
	Metadata articleMetadata
	Assets   []string
	Format   string
}

type articleFrontMatter struct {
	Title       string `yaml:"title"`
	Slug        string `yaml:"slug"`
	Tags        any    `yaml:"tags"`
	Categories  any    `yaml:"categories"`
	DisplayTime string `yaml:"display_time"`
	Status      string `yaml:"status"`
}

func prepareArticle(sourcePath string, overrides articleOverrides) (articleUpload, error) {
	var upload articleUpload
	absolutePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return upload, err
	}
	info, err := os.Stat(absolutePath)
	if err != nil {
		return upload, fmt.Errorf("读取文章文件: %w", err)
	}
	if !info.Mode().IsRegular() {
		return upload, errors.New("文章路径必须是普通文件")
	}
	if info.Size() <= 0 || info.Size() > maxArticleArchiveSize {
		return upload, errors.New("文章文件必须大于0且不能超过16MB")
	}
	data, err := os.ReadFile(absolutePath)
	if err != nil {
		return upload, fmt.Errorf("读取文章文件: %w", err)
	}
	extension := strings.ToLower(filepath.Ext(absolutePath))
	switch extension {
	case ".md", ".markdown":
		if len(data) > maxArticleMarkdownSize {
			return upload, errors.New("Markdown 正文不能超过200KB")
		}
		body, metadata, err := parseMarkdownArticle(data)
		if err != nil {
			return upload, err
		}
		if len(body) == 0 || strings.TrimSpace(string(body)) == "" {
			return upload, errors.New("Markdown 正文不能为空")
		}
		applyArticleOverrides(&metadata, overrides)
		if err := validateArticleMetadata(&metadata); err != nil {
			return upload, err
		}
		archive, assets, filename, err := packageMarkdownArticle(absolutePath, body)
		if err != nil {
			return upload, err
		}
		upload = articleUpload{Filename: filename + ".zip", Data: archive, Metadata: metadata, Assets: assets, Format: "markdown"}
	case ".html", ".htm":
		if len(data) > maxArticleHTMLSize {
			return upload, errors.New("HTML 文件不能超过8MB")
		}
		metadata := articleMetadata{Status: "draft"}
		applyArticleOverrides(&metadata, overrides)
		if err := validateArticleMetadata(&metadata); err != nil {
			return upload, err
		}
		upload = articleUpload{Filename: filepath.Base(absolutePath), Data: data, Metadata: metadata, Format: "html"}
	case ".zip":
		assets, format, err := validateArticleZIP(data)
		if err != nil {
			return upload, err
		}
		metadata := articleMetadata{Status: "draft"}
		applyArticleOverrides(&metadata, overrides)
		if err := validateArticleMetadata(&metadata); err != nil {
			return upload, err
		}
		upload = articleUpload{Filename: filepath.Base(absolutePath), Data: data, Metadata: metadata, Assets: assets, Format: format}
	default:
		return upload, errors.New("只支持 .md、.markdown、.html、.htm 或 .zip 文件")
	}
	return upload, nil
}

func parseMarkdownArticle(data []byte) ([]byte, articleMetadata, error) {
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	metadata := articleMetadata{Status: "draft"}
	matches := frontMatterPattern.FindSubmatchIndex(data)
	if len(matches) == 0 {
		return data, metadata, nil
	}
	var frontMatter articleFrontMatter
	if err := yaml.Unmarshal(data[matches[2]:matches[3]], &frontMatter); err != nil {
		return nil, metadata, fmt.Errorf("解析 YAML front matter: %w", err)
	}
	tags, err := yamlList(frontMatter.Tags, "tags")
	if err != nil {
		return nil, metadata, err
	}
	categories, err := yamlList(frontMatter.Categories, "categories")
	if err != nil {
		return nil, metadata, err
	}
	metadata = articleMetadata{
		Title:       frontMatter.Title,
		Slug:        frontMatter.Slug,
		Tags:        tags,
		Categories:  categories,
		DisplayTime: frontMatter.DisplayTime,
		Status:      frontMatter.Status,
	}
	return bytes.TrimLeft(data[matches[1]:], "\r\n"), metadata, nil
}

func yamlList(value any, field string) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return normalizeCommaList(typed), nil
	case []string:
		return normalizeCommaList(strings.Join(typed, ",")), nil
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return "", fmt.Errorf("front matter 的 %s 只能是字符串或字符串列表", field)
			}
			values = append(values, text)
		}
		return normalizeCommaList(strings.Join(values, ",")), nil
	default:
		return "", fmt.Errorf("front matter 的 %s 只能是字符串或字符串列表", field)
	}
}

func applyArticleOverrides(metadata *articleMetadata, overrides articleOverrides) {
	if value := strings.TrimSpace(overrides.Title); value != "" {
		metadata.Title = value
	}
	if value := strings.TrimSpace(overrides.Slug); value != "" {
		metadata.Slug = value
	}
	if value := strings.TrimSpace(overrides.Tags); value != "" {
		metadata.Tags = value
	}
	if value := strings.TrimSpace(overrides.Categories); value != "" {
		metadata.Categories = value
	}
	if value := strings.TrimSpace(overrides.DisplayTime); value != "" {
		metadata.DisplayTime = value
	}
}

func validateArticleMetadata(metadata *articleMetadata) error {
	metadata.Title = strings.TrimSpace(metadata.Title)
	if len([]rune(metadata.Title)) > 200 {
		return errors.New("文章标题不能超过200个字符")
	}
	metadata.Slug = strings.TrimSpace(metadata.Slug)
	if metadata.Slug != "" && !slugPattern.MatchString(metadata.Slug) {
		return errors.New("slug 只能包含字母、数字、下划线和连字符，长度为5-100")
	}
	metadata.Tags = normalizeCommaList(metadata.Tags)
	metadata.Categories = normalizeCommaList(metadata.Categories)
	status := strings.ToLower(strings.TrimSpace(metadata.Status))
	if status == "" {
		status = "draft"
	}
	if status != "draft" {
		return errors.New("CLI v1 只允许导入草稿；请将 front matter 的 status 设为 draft")
	}
	metadata.Status = status
	displayTime, err := normalizeDisplayTime(metadata.DisplayTime)
	if err != nil {
		return err
	}
	metadata.DisplayTime = displayTime
	return nil
}

func normalizeCommaList(value string) string {
	seen := make(map[string]struct{})
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
	}
	return strings.Join(result, ",")
}

func normalizeDisplayTime(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05"} {
		if _, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return value, nil
		}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", errors.New("display_time 应为 YYYY-MM-DDTHH:MM 或 RFC3339")
	}
	location, locationErr := time.LoadLocation("Asia/Shanghai")
	if locationErr != nil {
		location = time.FixedZone("CST", 8*60*60)
	}
	return parsed.In(location).Format("2006-01-02T15:04:05"), nil
}

func packageMarkdownArticle(sourcePath string, body []byte) ([]byte, []string, string, error) {
	baseDirectory := filepath.Dir(sourcePath)
	documentName := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath)) + ".md"
	if strings.HasPrefix(documentName, ".") {
		return nil, nil, "", errors.New("文章文件名不能以 . 开头")
	}
	references := markdownAssetReferences(body)
	if len(references)+1 > maxArticleEntries {
		return nil, nil, "", errors.New("文章与引用资源总数不能超过100个")
	}
	canonicalBase, err := filepath.EvalSymlinks(baseDirectory)
	if err != nil {
		return nil, nil, "", fmt.Errorf("解析文章目录: %w", err)
	}
	type assetFile struct {
		name string
		data []byte
	}
	assets := make([]assetFile, 0, len(references))
	expandedSize := len(body)
	for _, reference := range references {
		archiveName, localPath, err := localAssetPath(canonicalBase, reference)
		if err != nil {
			return nil, nil, "", err
		}
		data, err := os.ReadFile(localPath)
		if err != nil {
			return nil, nil, "", fmt.Errorf("读取引用资源 %s: %w", reference, err)
		}
		if len(data) == 0 || len(data) > maxArticleAssetSize {
			return nil, nil, "", fmt.Errorf("引用资源 %s 必须大于0且不能超过4MB", reference)
		}
		if !allowedArticleAsset(archiveName, data) {
			return nil, nil, "", fmt.Errorf("引用资源 %s 的类型不受支持", reference)
		}
		expandedSize += len(data)
		if expandedSize > maxArticleExpandedSize {
			return nil, nil, "", errors.New("文章与引用资源总大小不能超过32MB")
		}
		assets = append(assets, assetFile{name: archiveName, data: data})
	}

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	if err := writeZIPFile(writer, documentName, body); err != nil {
		return nil, nil, "", err
	}
	assetNames := make([]string, 0, len(assets))
	for _, asset := range assets {
		if err := writeZIPFile(writer, asset.name, asset.data); err != nil {
			return nil, nil, "", err
		}
		assetNames = append(assetNames, asset.name)
	}
	if err := writer.Close(); err != nil {
		return nil, nil, "", err
	}
	if buffer.Len() > maxArticleArchiveSize {
		return nil, nil, "", errors.New("打包后的文章不能超过16MB")
	}
	slices.Sort(assetNames)
	return buffer.Bytes(), assetNames, strings.TrimSuffix(documentName, ".md"), nil
}

func markdownAssetReferences(body []byte) []string {
	seen := make(map[string]struct{})
	var references []string
	collect := func(matches [][][]byte) {
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			reference := stdhtml.UnescapeString(strings.TrimSpace(string(match[1])))
			if separator := strings.IndexAny(reference, "?#"); separator >= 0 {
				reference = reference[:separator]
			}
			if !isLocalArticleAsset(reference) {
				continue
			}
			normalized := path.Clean(reference)
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			references = append(references, normalized)
		}
	}
	collect(markdownLinkPattern.FindAllSubmatch(body, -1))
	collect(htmlLinkPattern.FindAllSubmatch(body, -1))
	slices.Sort(references)
	return references
}

func isLocalArticleAsset(reference string) bool {
	if reference == "" || strings.HasPrefix(reference, "/") || strings.HasPrefix(reference, "#") || strings.HasPrefix(reference, "//") {
		return false
	}
	lower := strings.ToLower(reference)
	for _, prefix := range []string{"http:", "https:", "data:", "mailto:", "tel:"} {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	extension := strings.ToLower(path.Ext(reference))
	return extension != ".md" && extension != ".markdown" && extension != ""
}

func localAssetPath(canonicalBase, reference string) (string, string, error) {
	archiveName := path.Clean(strings.TrimPrefix(reference, "./"))
	if archiveName == "." || archiveName == ".." || strings.HasPrefix(archiveName, "../") || hasHiddenPathPart(archiveName) {
		return "", "", fmt.Errorf("引用资源路径不安全: %s", reference)
	}
	localPath := filepath.Join(canonicalBase, filepath.FromSlash(archiveName))
	canonicalPath, err := filepath.EvalSymlinks(localPath)
	if err != nil {
		return "", "", fmt.Errorf("引用资源不存在: %s", reference)
	}
	relative, err := filepath.Rel(canonicalBase, canonicalPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("引用资源不能位于文章目录之外: %s", reference)
	}
	info, err := os.Stat(canonicalPath)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("引用资源不是普通文件: %s", reference)
	}
	return archiveName, canonicalPath, nil
}

func writeZIPFile(writer *zip.Writer, name string, data []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o644)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = entry.Write(data)
	return err
}

func validateArticleZIP(data []byte) ([]string, string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, "", errors.New("ZIP 文件无法读取")
	}
	if len(reader.File) > maxArticleEntries {
		return nil, "", errors.New("ZIP 文件数量不能超过100个")
	}
	var document string
	var format string
	var assets []string
	var expanded uint64
	seen := make(map[string]struct{})
	type zipAsset struct {
		name string
		data []byte
	}
	var candidates []zipAsset
	for _, file := range reader.File {
		name := strings.TrimPrefix(file.Name, "./")
		clean := path.Clean(name)
		if strings.ContainsRune(name, '\x00') || strings.Contains(name, "\\") || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || hasHiddenPathPart(clean) {
			return nil, "", fmt.Errorf("ZIP 包含不安全路径: %s", file.Name)
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return nil, "", errors.New("ZIP 不能包含符号链接")
		}
		if file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/") {
			continue
		}
		if _, exists := seen[clean]; exists {
			return nil, "", fmt.Errorf("ZIP 包含重复路径: %s", clean)
		}
		seen[clean] = struct{}{}
		expanded += file.UncompressedSize64
		if expanded > maxArticleExpandedSize {
			return nil, "", errors.New("ZIP 解压内容不能超过32MB")
		}
		extension := strings.ToLower(path.Ext(clean))
		if extension == ".md" || extension == ".html" || extension == ".htm" {
			if document != "" {
				return nil, "", errors.New("ZIP 只能包含一个 Markdown 或 HTML 主文件")
			}
			document = clean
			if extension == ".md" {
				format = "markdown"
				if file.UncompressedSize64 > maxArticleMarkdownSize {
					return nil, "", errors.New("ZIP 中的 Markdown 不能超过200KB")
				}
			} else {
				format = "html"
				if file.UncompressedSize64 > maxArticleHTMLSize {
					return nil, "", errors.New("ZIP 中的 HTML 不能超过8MB")
				}
			}
			continue
		}
		if file.UncompressedSize64 == 0 || file.UncompressedSize64 > maxArticleAssetSize {
			return nil, "", fmt.Errorf("ZIP 资源 %s 必须大于0且不能超过4MB", clean)
		}
		prefix, err := readZIPPrefix(file)
		if err != nil {
			return nil, "", fmt.Errorf("读取 ZIP 资源 %s: %w", clean, err)
		}
		candidates = append(candidates, zipAsset{name: clean, data: prefix})
	}
	if document == "" {
		return nil, "", errors.New("ZIP 中没有 Markdown 或 HTML 主文件")
	}
	for _, asset := range candidates {
		if !allowedArticleAsset(asset.name, asset.data) {
			return nil, "", fmt.Errorf("ZIP 资源 %s 的类型不受支持", asset.name)
		}
		if format == "html" && !strings.HasPrefix(http.DetectContentType(asset.data), "image/") {
			return nil, "", errors.New("HTML ZIP 只支持图片资源；CSS 和 JavaScript 请内联")
		}
		assets = append(assets, asset.name)
	}
	slices.Sort(assets)
	return assets, format, nil
}

func readZIPPrefix(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(io.LimitReader(reader, 512))
}

func allowedArticleAsset(name string, data []byte) bool {
	extension := strings.ToLower(path.Ext(name))
	if strings.HasPrefix(http.DetectContentType(data), "image/") {
		return slices.Contains([]string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp"}, extension)
	}
	return slices.Contains([]string{".txt", ".pdf", ".zip", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx"}, extension)
}

func hasHiddenPathPart(name string) bool {
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}
