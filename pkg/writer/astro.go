package writer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// AstroWriter writes content in Astro's expected format.
// It handles:
// - Writing docs to src/content/docs/{pkg}/
// - Writing assets to public/docs/{pkg}/
// - Rewriting relative paths to absolute paths
// - Injecting/managing frontmatter
type AstroWriter struct {
	websiteDir string // e.g., "./grove-website"
}

// NewAstro creates a new AstroWriter for the given website directory
func NewAstro(websiteDir string) *AstroWriter {
	return &AstroWriter{websiteDir: websiteDir}
}

// WebsiteDir returns the target website directory
func (w *AstroWriter) WebsiteDir() string {
	return w.websiteDir
}

// WriteDoc writes a documentation file to src/content/docs/{pkg}/{filename}
func (w *AstroWriter) WriteDoc(pkg, filename string, content []byte, meta DocMetadata) error {
	path := filepath.Join(w.websiteDir, "src/content/docs", pkg, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(path, content, 0644)
}

// WriteAsset writes an asset file to public/docs/{pkg}/{assetType}/{filename}
func (w *AstroWriter) WriteAsset(pkg, assetType, filename string, data []byte) error {
	path := filepath.Join(w.websiteDir, "public/docs", pkg, assetType, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// WriteManifest writes the manifest file to docgen-output/manifest.json
func (w *AstroWriter) WriteManifest(manifest []byte) error {
	path := filepath.Join(w.websiteDir, "docgen-output/manifest.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return os.WriteFile(path, manifest, 0644)
}

// TransformContent applies Astro-specific transformations to markdown content.
// This includes:
// - Rewriting relative image paths to absolute paths
// - Rewriting asciinema paths
// - Rewriting video paths
// - Injecting/updating frontmatter
func (w *AstroWriter) TransformContent(content []byte, pkg string, meta DocMetadata) ([]byte, error) {
	s := string(content)
	baseURL := fmt.Sprintf("/docs/%s", pkg)

	// 1. Rewrite image paths: ./images/ -> /docs/{pkg}/images/
	imageRegex := regexp.MustCompile(`!\[([^\]]*)\]\(\./images/([^)]+)\)`)
	s = imageRegex.ReplaceAllString(s, fmt.Sprintf("![$1](%s/images/$2)", baseURL))

	// 2. Rewrite HTML img tags: <img src="./images/..." -> <img src="/docs/{pkg}/images/..."
	htmlImgRegex := regexp.MustCompile(`<img\s+([^>]*\s)?src="\./images/([^"]+)"([^>]*)>`)
	s = htmlImgRegex.ReplaceAllString(s, fmt.Sprintf(`<img $1src="%s/images/$2"$3>`, baseURL))

	// 3. Rewrite asciinema paths in code blocks
	asciiRegex := regexp.MustCompile(`("src":\s*")(\./asciicasts/)([^"]+)(")`)
	s = asciiRegex.ReplaceAllString(s, fmt.Sprintf("${1}%s/asciicasts/$3$4", baseURL))

	// 4. Rewrite video paths: ./videos/ -> /docs/{pkg}/videos/
	videoRegex := regexp.MustCompile(`!\[([^\]]*)\]\(\./videos/([^)]+)\)`)
	s = videoRegex.ReplaceAllString(s, fmt.Sprintf("![$1](%s/videos/$2)", baseURL))

	// 5. Inject/update frontmatter
	s = w.ensureFrontmatter(s, pkg, meta)

	return []byte(s), nil
}

// ensureFrontmatter adds or replaces frontmatter in the content
func (w *AstroWriter) ensureFrontmatter(content, pkg string, meta DocMetadata) string {
	// Build new frontmatter
	frontmatter := fmt.Sprintf(`---
title: "%s"
description: "%s"
package: "%s"
version: "%s"
category: "%s"
order: %d
---

`, escapeYAMLString(meta.Title), escapeYAMLString(meta.Description), escapeYAMLString(meta.Package), meta.Version, meta.Category, meta.Order)

	// Strip existing frontmatter if present
	if strings.HasPrefix(content, "---\n") {
		if end := strings.Index(content[4:], "\n---"); end != -1 {
			// Skip past the closing --- and any following newlines
			afterFrontmatter := content[end+8:]
			content = strings.TrimLeft(afterFrontmatter, "\n")
		}
	}

	return frontmatter + content
}

// escapeYAMLString escapes a string for use in YAML frontmatter
func escapeYAMLString(s string) string {
	// Escape double quotes and backslashes
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
