package watcher

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// RecursiveWatcher wraps fsnotify with recursive directory support.
// fsnotify is NOT recursive on Linux/POSIX, so we must explicitly
// watch all subdirectories and dynamically add watchers for new directories.
type RecursiveWatcher struct {
	*fsnotify.Watcher
	pathToWorkspace map[string]string
	mu              sync.RWMutex
}

// New creates a new RecursiveWatcher
func New() (*RecursiveWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &RecursiveWatcher{
		Watcher:         w,
		pathToWorkspace: make(map[string]string),
	}, nil
}

// AddRecursive adds a directory and all its subdirectories to the watcher.
// The workspacePath is associated with all paths under root for later lookup.
func (w *RecursiveWatcher) AddRecursive(root, workspacePath string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip inaccessible directories
		}
		if d.IsDir() {
			// Skip hidden directories (e.g., .git)
			if strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			if err := w.Add(path); err != nil {
				return nil // Skip, don't fail entirely
			}
			w.mu.Lock()
			w.pathToWorkspace[path] = workspacePath
			w.mu.Unlock()
		}
		return nil
	})
}

// HandleNewDirectory checks if an event is a new directory and adds it to the watcher.
// Returns true if a new directory was added.
func (w *RecursiveWatcher) HandleNewDirectory(event fsnotify.Event, workspacePath string) bool {
	if !event.Has(fsnotify.Create) {
		return false
	}
	info, err := os.Stat(event.Name)
	if err != nil || !info.IsDir() {
		return false
	}

	// Skip hidden directories
	if strings.HasPrefix(filepath.Base(event.Name), ".") {
		return false
	}

	// New directory created - add it and all subdirectories to the watcher
	w.AddRecursive(event.Name, workspacePath)
	return true
}

// FindWorkspace returns the workspace path for a given file path.
// It walks up the directory tree to find the watched parent.
func (w *RecursiveWatcher) FindWorkspace(path string) string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Check if the path itself is a watched directory
	if ws, ok := w.pathToWorkspace[path]; ok {
		return ws
	}

	// Walk up the path to find the watched parent
	dir := filepath.Dir(path)
	for dir != "/" && dir != "." {
		if ws, ok := w.pathToWorkspace[dir]; ok {
			return ws
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

// IsRelevantFile checks if a file is relevant for documentation (markdown, images, etc.)
func IsRelevantFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	relevantExtensions := map[string]bool{
		".md":   true,
		".mdx":  true,
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
		".svg":  true,
		".webp": true,
		".mp4":  true,
		".webm": true,
		".mov":  true,
		".cast": true, // asciinema cast files
	}
	return relevantExtensions[ext]
}

// IsMarkdownFile checks if a file is a markdown file
func IsMarkdownFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".mdx"
}

// IsAssetFile checks if a file is an asset file (image, video, cast)
func IsAssetFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	assetExtensions := map[string]bool{
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
		".svg":  true,
		".webp": true,
		".mp4":  true,
		".webm": true,
		".mov":  true,
		".cast": true,
	}
	return assetExtensions[ext]
}

// GetAssetType returns the asset type directory based on file extension
func GetAssetType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp":
		return "images"
	case ".mp4", ".webm", ".mov":
		return "videos"
	case ".cast":
		return "asciicasts"
	default:
		return ""
	}
}
