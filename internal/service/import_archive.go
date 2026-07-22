package service

import (
	"archive/zip"
	"bytes"
	"errors"
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
	maxImportEntries      = 100
)

var markdownAssetReference = regexp.MustCompile(`(\]\(\s*<?)([^>\s)]+)(>?[^)]*\))`)

type ImportOptions struct {
	AuthorID   int
	Tags       string
	Categories string
	Status     string
}

type importEntry struct {
	name string
	data []byte
}

// ImportMarkdownArchive imports one Markdown file and its sibling assets as a
// draft article. Asset references are rewritten to the generated upload URLs.
func (s *Service) ImportMarkdownArchive(archiveData []byte, options ImportOptions) (*model.Content, error) {
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
	entries, markdownPath, err := readImportEntries(archiveData)
	if err != nil {
		return nil, err
	}
	markdownEntry := entries[markdownPath]
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
		if entryPath == markdownPath {
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

	content := &model.Content{
		Title:        importTitle(markdownPath),
		Content:      rewriteMarkdownAssets(string(markdownEntry.data), markdownPath, assetURLs),
		Tags:         strings.TrimSpace(options.Tags),
		Categories:   strings.TrimSpace(options.Categories),
		Status:       options.Status,
		Type:         model.TypeArticle,
		AuthorID:     options.AuthorID,
		AllowComment: true,
		AllowPing:    true,
		AllowFeed:    true,
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

func readImportEntries(archiveData []byte) (map[string]importEntry, string, error) {
	reader, err := zip.NewReader(bytes.NewReader(archiveData), int64(len(archiveData)))
	if err != nil {
		return nil, "", Tip("压缩包格式无法读取")
	}
	if len(reader.File) > maxImportEntries {
		return nil, "", Tip("压缩包文件数量不能超过100个")
	}
	entries := make(map[string]importEntry, len(reader.File))
	var markdownPath string
	var expandedSize uint64
	for _, file := range reader.File {
		normalized, normalizeErr := normalizeImportPath(file.Name)
		if normalizeErr != nil {
			return nil, "", Tip("压缩包包含不安全的文件路径")
		}
		if file.FileInfo().Mode()&os.ModeSymlink != 0 {
			return nil, "", Tip("压缩包不能包含符号链接")
		}
		if file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/") {
			continue
		}
		expandedSize += file.UncompressedSize64
		if expandedSize > maxImportExpandedSize {
			return nil, "", Tip("压缩包解压内容不能超过32MB")
		}
		if file.UncompressedSize64 > uint64(model.MaxTextCount) && strings.EqualFold(filepath.Ext(normalized), ".md") {
			return nil, "", Tip("Markdown 文件不能超过200KB")
		}
		if file.UncompressedSize64 > uint64(model.MaxFileSize) {
			return nil, "", Tip("单个图片或附件不能超过1MB")
		}
		source, openErr := file.Open()
		if openErr != nil {
			return nil, "", openErr
		}
		data, readErr := io.ReadAll(io.LimitReader(source, maxImportExpandedSize+1))
		_ = source.Close()
		if readErr != nil {
			return nil, "", readErr
		}
		if uint64(len(data)) != file.UncompressedSize64 {
			return nil, "", Tip("压缩包文件读取不完整")
		}
		if strings.EqualFold(filepath.Ext(normalized), ".md") {
			if markdownPath != "" {
				return nil, "", Tip("一个压缩包只能包含一个 Markdown 文件")
			}
			markdownPath = normalized
		} else if !allowedImportFile(filepath.Ext(normalized), detectImportFileType(data)) {
			return nil, "", Tip("压缩包内存在不支持的附件格式")
		}
		if _, exists := entries[normalized]; exists {
			return nil, "", Tip("压缩包内存在重复文件路径")
		}
		entries[normalized] = importEntry{name: normalized, data: data}
	}
	if markdownPath == "" {
		return nil, "", Tip("压缩包内没有 Markdown 文件")
	}
	return entries, markdownPath, nil
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

func resolveImportAssetPath(baseDirectory, reference string) string {
	if strings.Contains(reference, "://") || strings.HasPrefix(reference, "/") ||
		strings.HasPrefix(reference, "#") || strings.HasPrefix(reference, "data:") {
		return ""
	}
	return path.Clean(path.Join(baseDirectory, reference))
}

func importTitle(markdownPath string) string {
	base := filepath.Base(markdownPath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
