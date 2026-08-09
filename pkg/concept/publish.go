// Package concept implements the shared concept publishing contract used by
// aggregate and watch.
package concept

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// Manifest is the public-publishing subset of concept-manifest.yml.
type Manifest struct {
	ID            string   `yaml:"id"`
	Title         string   `yaml:"title"`
	Description   string   `yaml:"description"`
	Status        string   `yaml:"status"`
	DocgenPublish string   `yaml:"docgen_publish"`
	DocgenOrder   []string `yaml:"docgen_order"`
}

// LoadManifest parses a concept manifest with the same YAML implementation in
// both aggregate and watch.
func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Published reports whether a concept belongs in the requested build mode.
func (m Manifest) Published(mode string) bool {
	status := m.DocgenPublish
	if status == "" {
		status = "draft"
	}
	return status != "draft" && !(mode == "prod" && status == "dev")
}

// MarkdownFiles returns safe, concept-relative markdown paths. Explicit
// docgen_order is honored; otherwise markdown is discovered recursively.
func MarkdownFiles(conceptDir string, order []string) ([]string, error) {
	if len(order) > 0 {
		files := make([]string, 0, len(order))
		for _, name := range order {
			rel, err := safeRelative(name)
			if err != nil || strings.ToLower(filepath.Ext(rel)) != ".md" {
				return nil, fmt.Errorf("invalid docgen_order path %q", name)
			}
			info, err := os.Stat(filepath.Join(conceptDir, rel))
			if err != nil {
				return nil, fmt.Errorf("docgen_order path %q: %w", name, err)
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("docgen_order path %q is not a regular file", name)
			}
			files = append(files, rel)
		}
		return files, nil
	}

	var files []string
	err := filepath.WalkDir(conceptDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != conceptDir && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			rel, relErr := filepath.Rel(conceptDir, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, rel)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// TransformOptions describes one public concept page.
type TransformOptions struct {
	Package            string
	Category           string
	ConceptID          string
	ConceptTitle       string
	ConceptDescription string
	RelativePath       string
	Order              int
	HasLikeC4Map       bool
}

// TransformMarkdown creates Astro-compatible frontmatter and sanitizes
// notebook-only context from the public body.
func TransformMarkdown(content []byte, opts TransformOptions) ([]byte, error) {
	meta, body, err := splitFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	title := stringField(meta, "title")
	if title == "" {
		if strings.EqualFold(filepath.Base(opts.RelativePath), "overview.md") && opts.ConceptTitle != "" {
			title = opts.ConceptTitle
		} else {
			title = FormatTitle(strings.TrimSuffix(filepath.Base(opts.RelativePath), filepath.Ext(opts.RelativePath)))
		}
	}
	description := stringField(meta, "description")
	if description == "" {
		description = opts.ConceptDescription
	}
	if description == "" {
		description = fmt.Sprintf("Concept documentation for %s.", title)
	}

	body = sanitizePublicBody(body)
	mapMetadata := ""
	if opts.HasLikeC4Map {
		mapMetadata = "concept_map: true\n"
	}
	frontmatter := fmt.Sprintf(`---
title: "%s"
description: "%s"
package: "%s"
category: "%s"
order: %d
concept_title: "%s"
concept_id: "%s"
%s---

`, yamlEscape(title), yamlEscape(description), yamlEscape(opts.Package), yamlEscape(opts.Category), opts.Order, yamlEscape(opts.ConceptTitle), yamlEscape(opts.ConceptID), mapMetadata)
	return []byte(frontmatter + strings.TrimLeft(body, "\n")), nil
}

func splitFrontmatter(content string) (map[string]any, string, error) {
	meta := map[string]any{}
	if !strings.HasPrefix(content, "---\n") {
		return meta, content, nil
	}
	end := strings.Index(content[4:], "\n---")
	if end == -1 {
		return nil, "", fmt.Errorf("unterminated markdown frontmatter")
	}
	if err := yaml.Unmarshal([]byte(content[4:end+4]), &meta); err != nil {
		return nil, "", fmt.Errorf("parse markdown frontmatter: %w", err)
	}
	return meta, content[end+8:], nil
}

func stringField(meta map[string]any, key string) string {
	value, _ := meta[key].(string)
	return value
}

func sanitizePublicBody(body string) string {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	skippingContext := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "## Context" {
			skippingContext = true
			continue
		}
		if skippingContext {
			if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
				skippingContext = false
			} else {
				continue
			}
		}
		out = append(out, line)
	}
	body = strings.Join(out, "\n")
	return localPathPattern.ReplaceAllString(body, "<local-path>")
}

var localPathPattern = regexp.MustCompile(`(?:file://)?(?:/Users/|/home/|/private/var/)[^\s\x60\"'<>\])}]+|[A-Za-z]:\\Users\\[^\s\x60\"'<>\])}]+`)

func yamlEscape(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return value
}

// FormatTitle converts a markdown filename stem to a display title.
func FormatTitle(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' })
	acronyms := map[string]string{"cli": "CLI", "tui": "TUI", "api": "API", "ui": "UI", "id": "ID", "llm": "LLM"}
	for i, part := range parts {
		if acronym, ok := acronyms[strings.ToLower(part)]; ok {
			parts[i] = acronym
		} else {
			parts[i] = cases.Title(language.English).String(strings.ToLower(part))
		}
	}
	return strings.Join(parts, " ")
}

// HasLikeC4Map reports whether a concept contains source files that LikeC4 can
// compile. Both the older likec4/ convention and the concept-map src/ scaffold
// are supported.
func HasLikeC4Map(conceptDir string) bool {
	for _, dir := range []string{"likec4", "src"} {
		found := false
		_ = filepath.WalkDir(filepath.Join(conceptDir, dir), func(_ string, entry fs.DirEntry, err error) error {
			if err != nil {
				return filepath.SkipDir
			}
			if !entry.IsDir() && (strings.EqualFold(filepath.Ext(entry.Name()), ".c4") || strings.EqualFold(filepath.Ext(entry.Name()), ".likec4")) {
				found = true
				return fs.SkipAll
			}
			return nil
		})
		if found {
			return true
		}
	}
	for _, pattern := range []string{"*.c4", "*.likec4"} {
		matches, _ := filepath.Glob(filepath.Join(conceptDir, pattern))
		if len(matches) > 0 {
			return true
		}
	}
	return false
}

// CopyAssets materializes the public concept asset bucket. Source assets/ is
// copied as-is. LikeC4 sources are normalized below assets/likec4/ from either
// the older likec4/ convention or the concept-map src/ scaffold. Root *.c4
// files are copied to assets/. Symlinks are ignored.
func CopyAssets(conceptDir, conceptDestDir string) error {
	assetDest := filepath.Join(conceptDestDir, "assets")
	likec4Dest := filepath.Join(assetDest, "likec4")
	for _, mapping := range []struct{ source, dest string }{
		{filepath.Join(conceptDir, "assets"), assetDest},
		{filepath.Join(conceptDir, "likec4"), likec4Dest},
		{filepath.Join(conceptDir, "src"), filepath.Join(likec4Dest, "src")},
	} {
		if info, err := os.Stat(mapping.source); err == nil && info.IsDir() {
			if err := copyTree(mapping.source, mapping.dest); err != nil {
				return err
			}
		}
	}
	var rootSources []string
	for _, pattern := range []string{"*.c4", "*.likec4"} {
		matches, err := filepath.Glob(filepath.Join(conceptDir, pattern))
		if err != nil {
			return err
		}
		rootSources = append(rootSources, matches...)
	}
	for _, source := range rootSources {
		name := filepath.Base(source)
		if err := copyRegularFile(source, filepath.Join(assetDest, name)); err != nil {
			return err
		}
		if err := copyRegularFile(source, filepath.Join(likec4Dest, name)); err != nil {
			return err
		}
	}
	configSource := filepath.Join(conceptDir, "likec4.config.json")
	if info, statErr := os.Stat(configSource); statErr == nil && info.Mode().IsRegular() {
		if err := copyRegularFile(configSource, filepath.Join(likec4Dest, "likec4.config.json")); err != nil {
			return err
		}
	}
	return nil
}

func copyTree(source, dest string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyRegularFile(path, target)
	})
}

func copyRegularFile(source, dest string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("asset %s is not a regular file", source)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close() //nolint:errcheck
	_, err = io.Copy(out, in)
	return err
}

func safeRelative(name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", fmt.Errorf("path must be relative")
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes concept")
	}
	return clean, nil
}
