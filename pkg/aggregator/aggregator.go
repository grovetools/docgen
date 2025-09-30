package aggregator

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mattsolo1/grove-docgen/pkg/config"
	"github.com/mattsolo1/grove-docgen/pkg/manifest"
	"github.com/mattsolo1/grove-meta/pkg/workspace"
	"github.com/sirupsen/logrus"
)

type Aggregator struct {
	logger *logrus.Logger
}

func New(logger *logrus.Logger) *Aggregator {
	return &Aggregator{logger: logger}
}

func (a *Aggregator) Aggregate(outputDir string) error {
	rootDir, err := workspace.FindRoot("")
	if err != nil {
		return fmt.Errorf("could not find workspace root: %w", err)
	}

	workspaces, err := workspace.Discover(rootDir)
	if err != nil {
		return fmt.Errorf("could not discover workspaces: %w", err)
	}

	m := &manifest.Manifest{
		Packages: []manifest.PackageManifest{},
	}

	// Removed generator - aggregator should only collect, not generate

	for _, wsPath := range workspaces {
		wsName := filepath.Base(wsPath)
		cfg, err := config.Load(wsPath)
		if err != nil {
			if os.IsNotExist(err) {
				a.logger.Debugf("Skipping %s: no docgen.config.yml found", wsName)
			} else {
				a.logger.Warnf("Skipping %s: could not load config: %v", wsName, err)
			}
			continue
		}

		if !cfg.Enabled {
			a.logger.Infof("Skipping %s: documentation is disabled in config", wsName)
			continue
		}


		// Get version and repo URL
		version := a.getPackageVersion(wsPath)
		repoURL := a.getRepoURL(wsPath)

		// Add to manifest
		pkgManifest := manifest.PackageManifest{
			Name:        wsName,
			Title:       cfg.Title,
			Description: cfg.Description,
			Category:    cfg.Category,
			DocsPath:    fmt.Sprintf("./%s", wsName),
			Version:     version,
			RepoURL:     repoURL,
		}

		// Copy generated files and build section manifest
		// Copy only the markdown output files specified in the config, not everything in docs/
		distDest := filepath.Join(outputDir, wsName)
		if err := os.MkdirAll(distDest, 0755); err != nil {
			a.logger.WithError(err).Errorf("Failed to create output directory for %s", wsName)
			continue
		}
		
		// Copy output files or use prompt files as placeholders
		for _, section := range cfg.Sections {
			srcFile := filepath.Join(wsPath, "docs", section.Output)
			destFile := filepath.Join(distDest, section.Output)
			
			// Check if the actual documentation file exists
			if _, err := os.Stat(srcFile); os.IsNotExist(err) {
				// Try to use the prompt file as a placeholder
				promptFile := filepath.Join(wsPath, "docs", "prompts", section.Prompt)
				if _, promptErr := os.Stat(promptFile); promptErr == nil {
					a.logger.Infof("Using prompt file as placeholder for %s/%s", wsName, section.Output)
					
					// Read the prompt file
					promptData, err := os.ReadFile(promptFile)
					if err != nil {
						a.logger.WithError(err).Errorf("Failed to read prompt %s", promptFile)
						continue
					}
					
					// Add a header to indicate this is a placeholder
					placeholder := fmt.Sprintf("# %s\n\n*Note: This is a placeholder generated from the prompt file. Full documentation is pending.*\n\n---\n\n%s", section.Title, string(promptData))
					
					if err := os.WriteFile(destFile, []byte(placeholder), 0644); err != nil {
						a.logger.WithError(err).Errorf("Failed to write placeholder %s", destFile)
						continue
					}
				} else {
					a.logger.Warnf("No documentation or prompt found for %s/%s", wsName, section.Output)
					continue
				}
			} else {
				// Copy the actual documentation file
				a.logger.Infof("Copying documentation for %s/%s", wsName, section.Output)
				
				srcData, err := os.ReadFile(srcFile)
				if err != nil {
					a.logger.WithError(err).Errorf("Failed to read %s", srcFile)
					continue
				}
				
				if err := os.WriteFile(destFile, srcData, 0644); err != nil {
					a.logger.WithError(err).Errorf("Failed to write %s", destFile)
					continue
				}
			}
		}

		// Copy images directory if it exists
		imagesSrcPath := filepath.Join(wsPath, "docs", "images")
		if _, err := os.Stat(imagesSrcPath); err == nil {
			imagesDestPath := filepath.Join(distDest, "images")
			a.logger.Infof("Copying images for %s from %s to %s", wsName, imagesSrcPath, imagesDestPath)
			if err := copyDir(imagesSrcPath, imagesDestPath); err != nil {
				a.logger.WithError(err).Errorf("Failed to copy images directory for %s", wsName)
				// Log error but continue
			}
		}

		sort.Slice(cfg.Sections, func(i, j int) bool {
			return cfg.Sections[i].Order < cfg.Sections[j].Order
		})

		for _, sec := range cfg.Sections {
			pkgManifest.Sections = append(pkgManifest.Sections, manifest.SectionManifest{
				Title: sec.Title,
				Path:  fmt.Sprintf("./%s/%s", wsName, sec.Output),
			})
		}

		// Check for and copy CHANGELOG.md if it exists
		changelogSrc := filepath.Join(wsPath, "CHANGELOG.md")
		if _, err := os.Stat(changelogSrc); err == nil {
			changelogDest := filepath.Join(distDest, "CHANGELOG.md")
			
			// Copy the CHANGELOG.md file
			changelogData, err := os.ReadFile(changelogSrc)
			if err != nil {
				a.logger.WithError(err).Errorf("Failed to read CHANGELOG.md for %s", wsName)
			} else {
				if err := os.WriteFile(changelogDest, changelogData, 0644); err != nil {
					a.logger.WithError(err).Errorf("Failed to write CHANGELOG.md for %s", wsName)
				} else {
					// Update the manifest with the changelog path
					pkgManifest.ChangelogPath = fmt.Sprintf("./%s/CHANGELOG.md", wsName)
					a.logger.Infof("Copied CHANGELOG.md for %s", wsName)
				}
			}
		} else {
			a.logger.Debugf("No CHANGELOG.md found for %s", wsName)
		}

		m.Packages = append(m.Packages, pkgManifest)
	}

	m.GeneratedAt = time.Now()

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Save the manifest
	manifestPath := filepath.Join(outputDir, "manifest.json")
	return m.Save(manifestPath)
}

// getPackageVersion attempts to get the version from git tags or grove.yml
func (a *Aggregator) getPackageVersion(wsPath string) string {
	// Try to get version from git tags
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = wsPath
	output, err := cmd.Output()
	if err == nil {
		version := strings.TrimSpace(string(output))
		if version != "" {
			return version
		}
	}

	// Fall back to checking grove.yml for version info
	groveYmlPath := filepath.Join(wsPath, "grove.yml")
	data, err := os.ReadFile(groveYmlPath)
	if err == nil {
		// Simple search for version field in YAML
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "version:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// Default to "latest" if no version found
	return "latest"
}

// getRepoURL attempts to get the repository URL from git remote
func (a *Aggregator) getRepoURL(wsPath string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = wsPath
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	url := strings.TrimSpace(string(output))
	// Convert SSH URLs to HTTPS URLs for consistency
	if strings.HasPrefix(url, "git@github.com:") {
		url = strings.Replace(url, "git@github.com:", "https://github.com/", 1)
	}
	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")
	
	return url
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}