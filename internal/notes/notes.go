package notes

import (
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"myblog/internal/util"
)

var ErrNotFound = errors.New("note not found")

type Node struct {
	Name     string
	Title    string
	Path     string
	IsFolder bool
	Children []Node
	Modified time.Time
}

type Document struct {
	Title    string
	Path     string
	Modified time.Time
	HTML     template.HTML
}

type Store struct {
	root string
}

func NewStore(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("notes root is empty")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve notes root: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create notes root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve notes root links: %w", err)
	}
	return &Store{root: filepath.Clean(root)}, nil
}

func (store *Store) Tree() ([]Node, error) {
	return store.readDirectory("")
}

func (store *Store) Folder(path string) ([]Node, error) {
	absolute, relative, err := store.safePath(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, ErrNotFound
	}
	return store.readDirectory(relative)
}

func (store *Store) Document(path string) (*Document, error) {
	path = strings.TrimSuffix(strings.TrimSpace(strings.Trim(path, "/")), ".md")
	if path == "" {
		return nil, ErrNotFound
	}
	path += ".md"
	absolute, relative, err := store.safePath(path)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(absolute), ".md") == false {
		return nil, ErrNotFound
	}
	info, err := os.Stat(absolute)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if info.IsDir() {
		return nil, ErrNotFound
	}
	content, err := os.ReadFile(absolute)
	if err != nil {
		return nil, err
	}
	title := noteTitle(relative)
	return &Document{
		Title:    title,
		Path:     relative,
		Modified: info.ModTime(),
		HTML:     template.HTML(util.Article(string(content))),
	}, nil
}

func (store *Store) readDirectory(relative string) ([]Node, error) {
	absolute := store.root
	if relative != "" {
		absolute = filepath.Join(store.root, filepath.FromSlash(relative))
	}
	entries, err := os.ReadDir(absolute)
	if err != nil {
		return nil, err
	}
	nodes := make([]Node, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") ||
			(!entry.IsDir() && strings.EqualFold(entry.Name(), "README.md")) {
			continue
		}
		if !entry.IsDir() && !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			continue
		}
		entryRelative := filepath.ToSlash(filepath.Join(relative, entry.Name()))
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		node := Node{
			Name:     strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			Title:    noteTitle(entryRelative),
			Path:     strings.TrimSuffix(entryRelative, filepath.Ext(entryRelative)),
			IsFolder: entry.IsDir(),
			Modified: info.ModTime(),
		}
		if entry.IsDir() {
			node.Children, err = store.readDirectory(entryRelative)
			if err != nil {
				return nil, err
			}
		}
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IsFolder != nodes[j].IsFolder {
			return nodes[i].IsFolder
		}
		return strings.ToLower(nodes[i].Title) < strings.ToLower(nodes[j].Title)
	})
	return nodes, nil
}

func (store *Store) safePath(path string) (string, string, error) {
	path = strings.TrimSpace(strings.Trim(path, "/"))
	if path == "" || strings.ContainsRune(path, '\x00') {
		return store.root, "", nil
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", ErrNotFound
	}
	relative := filepath.ToSlash(clean)
	for _, part := range strings.Split(relative, "/") {
		if part == "" || part == "." || part == ".." || strings.HasPrefix(part, ".") {
			return "", "", ErrNotFound
		}
	}
	absolute := filepath.Join(store.root, filepath.FromSlash(relative))
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", ErrNotFound
		}
		return "", "", err
	}
	relativeRoot, err := filepath.Rel(store.root, resolved)
	if err != nil || relativeRoot == ".." || strings.HasPrefix(relativeRoot, ".."+string(filepath.Separator)) {
		return "", "", ErrNotFound
	}
	return resolved, filepath.ToSlash(relativeRoot), nil
}

func noteTitle(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	return strings.TrimSpace(base)
}
