package manifest

import (
	"encoding/json"
	"os"
	"time"
)

// Manifest represents the complete documentation manifest for all packages
type Manifest struct {
	Packages        []PackageManifest `json:"packages"`
	WebsiteSections []WebsiteSection  `json:"website_sections,omitempty"`
	GeneratedAt     time.Time         `json:"generated_at"`
}

// WebsiteSection represents a top-level website content section (e.g., overview, concepts)
// These are distinct from package docs and map to separate Astro content collections.
type WebsiteSection struct {
	Name  string            `json:"name"`  // Directory name (e.g., "overview", "concepts")
	Title string            `json:"title"` // Display title (e.g., "Overview", "Concepts")
	Files []SectionManifest `json:"files"` // Individual markdown files in this section
}

// PackageManifest represents documentation manifest for a single package
type PackageManifest struct {
	Name          string            `json:"name"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Category      string            `json:"category"`
	DocsPath      string            `json:"docs_path"`
	Version       string            `json:"version"`
	RepoURL       string            `json:"repo_url,omitempty"`
	ChangelogPath string            `json:"changelog_path,omitempty"`
	Sections      []SectionManifest `json:"sections"`
}

// SectionManifest represents a single documentation section
type SectionManifest struct {
	Name     string    `json:"name"`
	Title    string    `json:"title"`
	Order    int       `json:"order"`
	Path     string    `json:"path"`
	JSONKey  string    `json:"json_key,omitempty"`
	Modified time.Time `json:"modified"`
}

// Save saves the manifest to a JSON file
func (m *Manifest) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}