package concept

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTransformMarkdownPreservesPublicMetadataAndSanitizes(t *testing.T) {
	input := []byte(`---
title: "Auth Pipeline"
description: "How auth flows."
private: true
---

# Auth

See /Users/alice/Notebooks/secret.md.

## Context

Preset: load with cx rules load /Users/alice/private.rules

## Details

Public detail.
`)
	got, err := TransformMarkdown(input, TransformOptions{
		Package: "core", Category: "Core", ConceptID: "auth", ConceptTitle: "Authentication",
		ConceptDescription: "Manifest fallback", RelativePath: "deep/overview.md", Order: 2001,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{
		`title: "Auth Pipeline"`, `description: "How auth flows."`,
		`concept_title: "Authentication"`, "## Details", "Public detail.", "<local-path>",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{"private: true", "## Context", "/Users/", "private.rules"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("output contains %q:\n%s", forbidden, text)
		}
	}
}

func TestTransformMarkdownFallsBackToManifestMetadata(t *testing.T) {
	got, err := TransformMarkdown([]byte("Body\n"), TransformOptions{
		Package: "flow", Category: "Agents", ConceptID: "subjobs", ConceptTitle: "Flow Subjobs",
		ConceptDescription: "Child job lifecycle.", RelativePath: "overview.md", Order: 2001, HasLikeC4Map: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, `title: "Flow Subjobs"`) || !strings.Contains(text, `description: "Child job lifecycle."`) {
		t.Fatalf("manifest metadata not used:\n%s", text)
	}
	if !strings.Contains(text, "concept_map: true") {
		t.Fatalf("map metadata not emitted:\n%s", text)
	}
}

func TestMarkdownFilesPreservesNestedPathsAndRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"overview.md", "guides/setup.md", "reference/setup.md"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := MarkdownFiles(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"guides/setup.md", "overview.md", "reference/setup.md"}
	if strings.Join(files, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", files, want)
	}
	if _, err := MarkdownFiles(dir, []string{"../secret.md"}); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}

func TestPilotFixturePublicationContract(t *testing.T) {
	source := filepath.Join("testdata", "pilot")
	manifest, err := LoadManifest(filepath.Join(source, "concept-manifest.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !manifest.Published("prod") {
		t.Fatal("pilot fixture should publish in prod")
	}
	files, err := MarkdownFiles(source, manifest.DocgenOrder)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(files, "|"); got != "overview.md|guides/details.md" {
		t.Fatalf("unexpected ordered files: %s", got)
	}

	dest := t.TempDir()
	for i, rel := range files {
		input, err := os.ReadFile(filepath.Join(source, rel))
		if err != nil {
			t.Fatal(err)
		}
		output, err := TransformMarkdown(input, TransformOptions{
			Package: "pilot", Category: "Test", ConceptID: manifest.ID,
			ConceptTitle: manifest.Title, ConceptDescription: manifest.Description,
			RelativePath: rel, Order: 2001 + i, HasLikeC4Map: HasLikeC4Map(source),
		})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, output, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := CopyAssets(source, dest, "prod"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"overview.md", "guides/details.md", "assets/likec4/model.c4"} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Errorf("pilot output missing %s: %v", rel, err)
		}
	}
	published, err := os.ReadFile(filepath.Join(dest, "overview.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(published), "## Context") || strings.Contains(string(published), "/Users/") {
		t.Fatalf("pilot leaked notebook context:\n%s", published)
	}
	if !strings.Contains(string(published), "concept_map: true") {
		t.Fatalf("pilot map metadata missing:\n%s", published)
	}
}

func TestCopyAssetsDefinesConceptBucket(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	fixtures := map[string]string{
		"assets/diagram.svg":     "svg",
		"likec4/model.c4":        "model",
		"likec4/views/system.c4": "view",
		"src/index.c4":           "map scaffold",
		"likec4.config.json":     `{"name":"fixture"}`,
		"root.c4":                "root",
	}
	for name, content := range fixtures {
		path := filepath.Join(source, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := CopyAssets(source, dest, "prod"); err != nil {
		t.Fatal(err)
	}
	if !HasLikeC4Map(source) {
		t.Fatal("fixture should be detected as a LikeC4 map")
	}
	for _, name := range []string{"assets/diagram.svg", "assets/likec4/model.c4", "assets/likec4/views/system.c4", "assets/likec4/src/index.c4", "assets/likec4/likec4.config.json", "assets/root.c4", "assets/likec4/root.c4"} {
		if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestCopyAssetsVendorsPublishedIncludeClosure(t *testing.T) {
	notebook := t.TempDir()
	root := filepath.Join(notebook, "workspaces", "core", "concepts", "architecture")
	dependency := filepath.Join(notebook, "workspaces", "ui", "concepts", "panels")
	transitive := filepath.Join(notebook, "workspaces", "core", "concepts", "identity")
	writeFixture := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFixture(filepath.Join(root, "src", "model.c4"), "model {}\n")
	writeFixture(filepath.Join(root, "likec4.config.json"), `{"name":"architecture","include":{"paths":["../../../ui/concepts/panels/likec4"]}}`)
	writeFixture(filepath.Join(dependency, "concept-manifest.yml"), "id: panels\ndocgen_publish: production\n")
	writeFixture(filepath.Join(dependency, "likec4", "panels.c4"), "model panels {}\n  link file:///Users/alice/Notebooks/private.md 'local source'\n")
	writeFixture(filepath.Join(dependency, "likec4.config.json"), `{"include":{"paths":["../../../core/concepts/identity/likec4"]}}`)
	writeFixture(filepath.Join(transitive, "concept-manifest.yml"), "id: identity\ndocgen_publish: production\n")
	writeFixture(filepath.Join(transitive, "likec4", "identity.c4"), "model identity {}\n")

	dest := t.TempDir()
	if err := CopyAssets(root, dest, "prod"); err != nil {
		t.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(dest, "assets", "likec4", "likec4.config.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(config)
	for _, want := range []string{"_includes/ui/panels/likec4", "_includes/core/identity/likec4"} {
		if !strings.Contains(got, want) {
			t.Errorf("rewritten config missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, notebook) || strings.Contains(got, "../../../") {
		t.Fatalf("rewritten config leaked notebook path:\n%s", got)
	}
	for _, rel := range []string{
		"assets/likec4/_includes/ui/panels/likec4/panels.c4",
		"assets/likec4/_includes/core/identity/likec4/identity.c4",
	} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Errorf("missing vendored source %s: %v", rel, err)
		}
	}
	panelSource, err := os.ReadFile(filepath.Join(dest, "assets/likec4/_includes/ui/panels/likec4/panels.c4"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(panelSource), "/Users/") || !strings.Contains(string(panelSource), "model panels") {
		t.Fatalf("vendored source was not safely sanitized:\n%s", panelSource)
	}
}

func TestCopyAssetsRejectsUnpublishedInclude(t *testing.T) {
	notebook := t.TempDir()
	root := filepath.Join(notebook, "workspaces", "core", "concepts", "architecture")
	dependency := filepath.Join(notebook, "workspaces", "ui", "concepts", "private-map")
	for name, content := range map[string]string{
		filepath.Join(root, "src", "model.c4"):                "model {}\n",
		filepath.Join(root, "likec4.config.json"):             `{"include":{"paths":["../../../ui/concepts/private-map/likec4"]}}`,
		filepath.Join(dependency, "concept-manifest.yml"):     "id: private-map\ndocgen_publish: draft\n",
		filepath.Join(dependency, "likec4", "private.likec4"): "model private {}\n",
	} {
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := CopyAssets(root, t.TempDir(), "prod")
	if err == nil || !strings.Contains(err.Error(), "unpublished concept ui:private-map") {
		t.Fatalf("expected actionable unpublished dependency error, got %v", err)
	}
}
