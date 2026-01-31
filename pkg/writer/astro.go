package writer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/grovetools/docgen/pkg/transformer"
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
// This delegates to the central transformer package for consistency.
func (w *AstroWriter) TransformContent(content []byte, pkg string, meta DocMetadata) ([]byte, error) {
	trans := transformer.NewAstroTransformer()
	opts := transformer.TransformOptions{
		PackageName: pkg,
		Title:       meta.Title,
		Description: meta.Description,
		Version:     meta.Version,
		Category:    meta.Category,
		Order:       meta.Order,
	}
	return trans.TransformStandardDoc(content, opts), nil
}
