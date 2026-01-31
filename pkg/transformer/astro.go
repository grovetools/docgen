// Package transformer provides content transformation for different output formats.
// It centralizes the logic for rewriting asset paths and handling frontmatter,
// ensuring consistent behavior between docgen aggregate and docgen watch commands.
package transformer

import (
	"fmt"
	"regexp"
	"strings"
)

// TransformOptions holds metadata for content transformation
type TransformOptions struct {
	// For standard package documentation
	PackageName string
	Title       string
	Description string
	Version     string
	Category    string
	Order       int

	// For website sections (overview, concepts)
	SectionName string
}

// AstroTransformer handles content transformations for Astro
type AstroTransformer struct{}

// NewAstroTransformer creates a new Astro transformer
func NewAstroTransformer() *AstroTransformer {
	return &AstroTransformer{}
}

// TransformStandardDoc applies transformations for standard package documentation:
// - Rewrites relative asset paths to absolute /docs/{pkg}/... paths
// - Replaces any existing frontmatter with a new one
func (t *AstroTransformer) TransformStandardDoc(content []byte, opts TransformOptions) []byte {
	s := string(content)
	baseURL := fmt.Sprintf("/docs/%s", opts.PackageName)

	s = t.rewritePaths(s, baseURL)
	s = t.ensureFrontmatter(s, opts)

	return []byte(s)
}

// TransformWebsiteSection applies transformations for website sections (overview, concepts):
// - Rewrites relative asset paths to absolute /docs/{section}/... paths
// - Augments existing frontmatter (preserves manual fields) with category and package
func (t *AstroTransformer) TransformWebsiteSection(content []byte, opts TransformOptions) []byte {
	s := string(content)
	// For sections like "overview", the base URL is /docs/overview
	baseURL := fmt.Sprintf("/docs/%s", opts.SectionName)

	s = t.rewritePaths(s, baseURL)
	s = t.augmentFrontmatter(s, opts)

	return []byte(s)
}

// rewritePaths rewrites all relative asset paths to absolute website paths
func (t *AstroTransformer) rewritePaths(content, baseURL string) string {
	// 1. Rewrite markdown image syntax: ![alt](./images/file.ext)
	imageRegex := regexp.MustCompile(`!\[([^\]]*)\]\(\./images/([^)]+)\)`)
	content = imageRegex.ReplaceAllString(content, fmt.Sprintf("![$1](%s/images/$2)", baseURL))

	// 2. Rewrite HTML img tags: <img src="./images/file.ext" ...>
	htmlImgRegex := regexp.MustCompile(`<img\s+([^>]*\s)?src="\./images/([^"]+)"([^>]*)>`)
	content = htmlImgRegex.ReplaceAllString(content, fmt.Sprintf(`<img $1src="%s/images/$2"$3>`, baseURL))

	// 3. Rewrite asciinema blocks' src paths: "src": "./asciicasts/file.cast"
	asciiRegex := regexp.MustCompile(`("src":\s*")(\./asciicasts/)([^"]+)(")`)
	content = asciiRegex.ReplaceAllString(content, fmt.Sprintf("${1}%s/asciicasts/$3$4", baseURL))

	// 4. Rewrite video paths (markdown image syntax): ![alt](./videos/file.mp4)
	videoRegex := regexp.MustCompile(`!\[([^\]]*)\]\(\./videos/([^)]+)\)`)
	content = videoRegex.ReplaceAllString(content, fmt.Sprintf("![$1](%s/videos/$2)", baseURL))

	return content
}

// ensureFrontmatter replaces any existing frontmatter with a new one for package docs
func (t *AstroTransformer) ensureFrontmatter(content string, opts TransformOptions) string {
	frontmatter := fmt.Sprintf(`---
title: "%s"
description: "%s"
package: "%s"
version: "%s"
category: "%s"
order: %d
---

`, escapeYAMLString(opts.Title), escapeYAMLString(opts.Description), escapeYAMLString(opts.PackageName), opts.Version, opts.Category, opts.Order)

	// Remove existing frontmatter if present
	if strings.HasPrefix(content, "---\n") {
		if end := strings.Index(content[4:], "\n---"); end != -1 {
			content = strings.TrimLeft(content[end+8:], "\n")
		}
	}

	return frontmatter + content
}

// augmentFrontmatter merges additional fields into existing frontmatter for website sections.
// If no frontmatter exists, it creates new frontmatter.
// Existing fields are preserved; only category and package are added if missing.
func (t *AstroTransformer) augmentFrontmatter(content string, opts TransformOptions) string {
	// Map section names to sidebar category names
	category := opts.Category
	if category == "" {
		categoryMap := map[string]string{
			"overview": "Overview",
			"concepts": "Concepts",
		}
		category = categoryMap[opts.SectionName]
		if category == "" {
			category = opts.SectionName
		}
	}

	if !strings.HasPrefix(content, "---\n") {
		// No frontmatter, create new
		newFrontmatter := fmt.Sprintf("---\ncategory: \"%s\"\npackage: \"Grove Ecosystem\"\n---\n\n", category)
		return newFrontmatter + content
	}

	// Find end of frontmatter
	endIdx := strings.Index(content[4:], "\n---")
	if endIdx == -1 {
		return content // Malformed frontmatter, skip
	}

	existingFrontmatter := content[4 : endIdx+4]
	restOfContent := content[endIdx+8:] // Skip past "\n---"

	// Check which fields already exist
	hasCategory := strings.Contains(existingFrontmatter, "category:")
	hasPackage := strings.Contains(existingFrontmatter, "package:")

	// Build new fields to add
	var newFields []string
	if !hasCategory {
		newFields = append(newFields, fmt.Sprintf("category: \"%s\"", category))
	}
	if !hasPackage {
		newFields = append(newFields, "package: \"Grove Ecosystem\"")
	}

	// If no new fields needed, return as-is
	if len(newFields) == 0 {
		return content
	}

	// Append new fields to existing frontmatter
	return "---\n" + existingFrontmatter + "\n" + strings.Join(newFields, "\n") + "\n---" + restOfContent
}

// escapeYAMLString escapes special characters for YAML string values
func escapeYAMLString(s string) string {
	// Escape double quotes and backslashes
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
