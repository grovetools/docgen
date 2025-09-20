package aggregator

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mattsolo1/grove-docgen/pkg/config"
	"github.com/mattsolo1/grove-docgen/pkg/generator"
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

	gen := generator.New(a.logger)

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

		// Generate docs for this package
		if err := gen.Generate(wsPath); err != nil {
			a.logger.WithError(err).Errorf("Failed to generate docs for %s, skipping aggregation for this package.", wsName)
			continue
		}

		// Add to manifest
		pkgManifest := manifest.PackageManifest{
			Name:     wsName,
			Title:    cfg.Title,
			Category: cfg.Category,
			DocsPath: fmt.Sprintf("./%s", wsName),
			// Version and RepoURL will be filled in a later step
		}

		// Copy generated files and build section manifest
		distSrc := filepath.Join(wsPath, "docs", "dist")
		distDest := filepath.Join(outputDir, wsName)
		if err := copyDir(distSrc, distDest); err != nil {
			a.logger.WithError(err).Errorf("Failed to copy generated docs for %s", wsName)
			continue
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

		m.Packages = append(m.Packages, pkgManifest)
	}

	m.GeneratedAt = time.Now()

	// Save the manifest
	manifestPath := filepath.Join(outputDir, "manifest.json")
	return m.Save(manifestPath)
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