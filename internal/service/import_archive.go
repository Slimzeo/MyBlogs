package service

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"errors"
	stdhtml "html"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"myblog/internal/model"
	"myblog/internal/util"
)

const (
	maxImportArchiveSize  = 16 << 20
	maxImportExpandedSize = 32 << 20
	maxImportAssetSize    = 4 << 20
	maxImportEntries      = 100
)

var markdownAssetReference = regexp.MustCompile(`(\]\(\s*<?)([^>\s)]+)(>?[^)]*\))`)
var htmlAssetReference = regexp.MustCompile(`(?i)((?:src|href|poster)\s*=\s*)(["'])([^"']+)(["'])`)
var htmlSrcsetReference = regexp.MustCompile(`(?i)(srcset\s*=\s*)(["'])([^"']+)(["'])`)
var htmlCSSAssetReference = regexp.MustCompile(`(?i)(url\(\s*["']?)([^"')]+)(["']?\s*\))`)
var htmlTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

type ImportOptions struct {
	AuthorID    int
	Title       string
	Slug        string
	Tags        string
	Categories  string
	Status      string
	DisplayTime int
}

type importEntry struct {
	name string
	data []byte
}

// ImportArticleArchive imports one Markdown or HTML document and its sibling
// assets as a draft article. Relative asset references are rewritten to the
// generated upload URLs.
func (s *Service) ImportArticleArchive(archiveData []byte, options ImportOptions) (*model.Content, error) {
	if len(archiveData) == 0 || len(archiveData) > maxImportArchiveSize {
		return nil, Tip("压缩包不能超过16MB")
	}
	if options.AuthorID == 0 {
		return nil, Tip("请登录后导入文章")
	}
	if options.Status == "" {
		options.Status = model.TypeDraft
	}
	if !validContentStatus(model.TypeArticle, options.Status) {
		return nil, Tip("文章状态不合法")
	}
	entries, documentPath, contentFormat, err := readImportEntries(archiveData)
	if err != nil {
		return nil, err
	}
	documentEntry := entries[documentPath]
	assetURLs := make(map[string]string, len(entries)-1)
	var storedFiles []string
	var storedAssetPaths []string
	cleanup := func() {
		for _, filePath := range storedFiles {
			_ = os.Remove(filePath)
		}
		if len(storedAssetPaths) > 0 {
			s.db.Where("fkey IN ?", storedAssetPaths).Delete(&model.Attach{})
		}
	}

	for entryPath, entry := range entries {
		if entryPath == documentPath {
			continue
		}
		if contentFormat == model.ContentHTML {
			dataURL, encodeErr := htmlAssetDataURL(entryPath, entry.data)
			if encodeErr != nil {
				return nil, encodeErr
			}
			assetURLs[entryPath] = dataURL
			continue
		}
		fileKey, fileType, filePath, saveErr := s.saveImportedAsset(entryPath, entry.data)
		if saveErr != nil {
			cleanup()
			return nil, saveErr
		}
		if err := s.SaveAttach(filepath.Base(entryPath), fileKey, fileType, options.AuthorID); err != nil {
			_ = os.Remove(filePath)
			cleanup()
			return nil, err
		}
		storedFiles = append(storedFiles, filePath)
		storedAssetPaths = append(storedAssetPaths, fileKey)
		assetURLs[entryPath] = fileKey
	}

	body := string(documentEntry.data)
	if contentFormat == model.ContentHTML {
		body = rewriteHTMLAssets(body, documentPath, assetURLs)
	} else {
		body = rewriteMarkdownAssets(body, documentPath, assetURLs)
	}
	content := &model.Content{
		Title:         importedTitle(options.Title, documentPath, contentFormat, documentEntry.data),
		Slug:          strings.TrimSpace(options.Slug),
		DisplayTime:   options.DisplayTime,
		Content:       body,
		ContentFormat: contentFormat,
		Tags:          strings.TrimSpace(options.Tags),
		Categories:    strings.TrimSpace(options.Categories),
		Status:        options.Status,
		Type:          model.TypeArticle,
		AuthorID:      options.AuthorID,
		AllowComment:  true,
		AllowPing:     true,
		AllowFeed:     true,
	}
	if content.Categories == "" {
		content.Categories = "默认分类"
	}
	if err := s.Publish(content); err != nil {
		cleanup()
		return nil, err
	}
	return content, nil
}

func htmlAssetDataURL(name string, data []byte) (string, error) {
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") ||
		!allowedImportFile(strings.ToLower(filepath.Ext(name)), model.TypeImage) {
		return "", Tip("HTML ZIP 只支持图片资源，CSS 和 JavaScript 请内联到 HTML")
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// ImportHTMLDocument imports a standalone HTML file. Files with relative asset
// references should be uploaded as a ZIP through ImportArticleArchive instead.
func (s *Service) ImportHTMLDocument(data []byte, filename string, options ImportOptions) (*model.Content, error) {
	if len(data) == 0 || len(data) > model.MaxHTMLSize {
		return nil, Tip("HTML 文件不能超过8MB")
	}
	if options.AuthorID == 0 {
		return nil, Tip("请登录后导入文章")
	}
	if options.Status == "" {
		options.Status = model.TypeDraft
	}
	if !validContentStatus(model.TypeArticle, options.Status) {
		return nil, Tip("文章状态不合法")
	}
	content := &model.Content{
		Title:         importedTitle(options.Title, filename, model.ContentHTML, data),
		Slug:          strings.TrimSpace(options.Slug),
		DisplayTime:   options.DisplayTime,
		Content:       string(data),
		ContentFormat: model.ContentHTML,
		Tags:          strings.TrimSpace(options.Tags),
		Categories:    strings.TrimSpace(options.Categories),
		Status:        options.Status,
		Type:          model.TypeArticle,
		AuthorID:      options.AuthorID,
		AllowComment:  true,
		AllowPing:     true,
		AllowFeed:     true,
	}
	if content.Categories == "" {
		content.Categories = "默认分类"
	}
	if err := s.Publish(content); err != nil {
		return nil, err
	}
	return content, nil
}

func readImportEntries(archiveData []byte) (map[string]importEntry, string, string, error) {
	reader, err := zip.NewReader(bytes.NewReader(archiveData), int64(len(archiveData)))
	if err != nil {
		return nil, "", "", Tip("压缩包格式无法读取")
	}
	if len(reader.File) > maxImportEntries {
		return nil, "", "", Tip("压缩包文件数量不能超过100个")
	}
	entries := make(map[string]importEntry, len(reader.File))
	var documentPath string
	var contentFormat string
	var expandedSize uint64
	for _, file := range reader.File {
		normalized, normalizeErr := normalizeImportPath(file.Name)
		if normalizeErr != nil {
			return nil, "", "", Tip("压缩包包含不安全的文件路径")
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return nil, "", "", Tip("压缩包不能包含符号链接")
		}
		if file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/") {
			continue
		}
		expandedSize += file.UncompressedSize64
		if expandedSize > maxImportExpandedSize {
			return nil, "", "", Tip("压缩包解压内容不能超过32MB")
		}
		entryFormat := importContentFormat(filepath.Ext(normalized))
		if entryFormat == model.ContentMarkdown && file.UncompressedSize64 > uint64(model.MaxTextCount) {
			return nil, "", "", Tip("Markdown 文件不能超过200KB")
		}
		if entryFormat == model.ContentHTML && file.UncompressedSize64 > uint64(model.MaxHTMLSize) {
			return nil, "", "", Tip("HTML 文件不能超过8MB")
		}
		if entryFormat == "" && file.UncompressedSize64 > uint64(maxImportAssetSize) {
			return nil, "", "", Tip("单个图片或附件不能超过4MB")
		}
		source, openErr := file.Open()
		if openErr != nil {
			return nil, "", "", openErr
		}
		data, readErr := io.ReadAll(io.LimitReader(source, maxImportExpandedSize+1))
		_ = source.Close()
		if readErr != nil {
			return nil, "", "", readErr
		}
		if uint64(len(data)) != file.UncompressedSize64 {
			return nil, "", "", Tip("压缩包文件读取不完整")
		}
		if entryFormat != "" {
			if documentPath != "" {
				return nil, "", "", Tip("一个压缩包只能包含一个 Markdown 或 HTML 文件")
			}
			documentPath = normalized
			contentFormat = entryFormat
		} else if !allowedImportFile(filepath.Ext(normalized), detectImportFileType(data)) {
			return nil, "", "", Tip("压缩包内存在不支持的附件格式")
		}
		if _, exists := entries[normalized]; exists {
			return nil, "", "", Tip("压缩包内存在重复文件路径")
		}
		entries[normalized] = importEntry{name: normalized, data: data}
	}
	if documentPath == "" {
		return nil, "", "", Tip("压缩包内没有 Markdown 或 HTML 文件")
	}
	return entries, documentPath, contentFormat, nil
}

func importContentFormat(extension string) string {
	switch strings.ToLower(extension) {
	case ".md":
		return model.ContentMarkdown
	case ".html", ".htm":
		return model.ContentHTML
	default:
		return ""
	}
}

func (s *Service) saveImportedAsset(name string, data []byte) (string, string, string, error) {
	extension := strings.ToLower(filepath.Ext(name))
	fileType := detectImportFileType(data)
	if !allowedImportFile(extension, fileType) {
		return "", "", "", Tip("压缩包内存在不支持的附件格式")
	}
	relativeDirectory := filepath.Join(time.Now().Format("2006"), time.Now().Format("01"))
	directory := filepath.Join(s.cfg.UploadDir, relativeDirectory)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", "", "", err
	}
	fileName := util.UU32() + extension
	filePath := filepath.Join(directory, fileName)
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return "", "", "", err
	}
	return "/upload/" + filepath.ToSlash(filepath.Join(relativeDirectory, fileName)), fileType, filePath, nil
}

func detectImportFileType(data []byte) string {
	if strings.HasPrefix(http.DetectContentType(data), "image/") {
		return model.TypeImage
	}
	return model.TypeFile
}

func allowedImportFile(extension, fileType string) bool {
	imageExtensions := map[string]struct{}{
		".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {}, ".webp": {}, ".bmp": {},
	}
	fileExtensions := map[string]struct{}{
		".txt": {}, ".pdf": {}, ".zip": {}, ".doc": {}, ".docx": {},
		".xls": {}, ".xlsx": {}, ".ppt": {}, ".pptx": {},
	}
	if fileType == model.TypeImage {
		_, ok := imageExtensions[extension]
		return ok
	}
	_, ok := fileExtensions[extension]
	return ok
}

func normalizeImportPath(name string) (string, error) {
	if strings.ContainsRune(name, '\x00') || strings.Contains(name, "\\") {
		return "", errors.New("invalid archive path")
	}
	name = strings.TrimPrefix(name, "./")
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return "", errors.New("invalid archive path")
	}
	for _, part := range strings.Split(clean, "/") {
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") {
			return "", errors.New("invalid archive path")
		}
	}
	return clean, nil
}

func rewriteMarkdownAssets(markdown, markdownPath string, assetURLs map[string]string) string {
	baseDirectory := path.Dir(markdownPath)
	return markdownAssetReference.ReplaceAllStringFunc(markdown, func(reference string) string {
		matches := markdownAssetReference.FindStringSubmatch(reference)
		if len(matches) != 4 {
			return reference
		}
		assetPath := resolveImportAssetPath(baseDirectory, matches[2])
		if url, ok := assetURLs[assetPath]; ok {
			return matches[1] + url + matches[3]
		}
		return reference
	})
}

func rewriteHTMLAssets(document, documentPath string, assetURLs map[string]string) string {
	baseDirectory := path.Dir(documentPath)
	rewrite := func(reference string, pattern *regexp.Regexp) string {
		matches := pattern.FindStringSubmatch(reference)
		if len(matches) != 5 || matches[2] != matches[4] {
			return reference
		}
		if rewritten := rewriteImportReference(baseDirectory, matches[3], assetURLs); rewritten != "" {
			return matches[1] + matches[2] + rewritten + matches[4]
		}
		return reference
	}
	document = htmlAssetReference.ReplaceAllStringFunc(document, func(reference string) string {
		return rewrite(reference, htmlAssetReference)
	})
	document = htmlSrcsetReference.ReplaceAllStringFunc(document, func(reference string) string {
		matches := htmlSrcsetReference.FindStringSubmatch(reference)
		if len(matches) != 5 || matches[2] != matches[4] || strings.Contains(matches[3], "data:") {
			return reference
		}
		candidates := strings.Split(matches[3], ",")
		for index, candidate := range candidates {
			parts := strings.Fields(candidate)
			if len(parts) == 0 {
				continue
			}
			if rewritten := rewriteImportReference(baseDirectory, parts[0], assetURLs); rewritten != "" {
				parts[0] = rewritten
				candidates[index] = strings.Join(parts, " ")
			}
		}
		return matches[1] + matches[2] + strings.Join(candidates, ", ") + matches[4]
	})
	return htmlCSSAssetReference.ReplaceAllStringFunc(document, func(reference string) string {
		matches := htmlCSSAssetReference.FindStringSubmatch(reference)
		if len(matches) != 4 {
			return reference
		}
		if rewritten := rewriteImportReference(baseDirectory, matches[2], assetURLs); rewritten != "" {
			return matches[1] + rewritten + matches[3]
		}
		return reference
	})
}

func rewriteImportReference(baseDirectory, reference string, assetURLs map[string]string) string {
	suffix := ""
	if separator := strings.IndexAny(reference, "?#"); separator >= 0 {
		suffix = reference[separator:]
		reference = reference[:separator]
	}
	assetPath := resolveImportAssetPath(baseDirectory, reference)
	if url, ok := assetURLs[assetPath]; ok {
		return url + suffix
	}
	return ""
}

func resolveImportAssetPath(baseDirectory, reference string) string {
	if strings.Contains(reference, "://") || strings.HasPrefix(reference, "/") ||
		strings.HasPrefix(reference, "#") || strings.HasPrefix(reference, "data:") {
		return ""
	}
	return path.Clean(path.Join(baseDirectory, reference))
}

func importTitle(documentPath, contentFormat string, data []byte) string {
	if contentFormat == model.ContentHTML {
		if matches := htmlTitle.FindSubmatch(data); len(matches) == 2 {
			title := strings.TrimSpace(stdhtml.UnescapeString(util.HTMLToText(string(matches[1]))))
			if title != "" {
				return title
			}
		}
	}
	base := filepath.Base(documentPath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func importedTitle(override, documentPath, contentFormat string, data []byte) string {
	if title := strings.TrimSpace(override); title != "" {
		return title
	}
	return importTitle(documentPath, contentFormat, data)
}
