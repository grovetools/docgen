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
		ConceptDescription: "Child job lifecycle.", RelativePath: "overview.md", Order: 2001,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, `title: "Flow Subjobs"`) || !strings.Contains(text, `description: "Child job lifecycle."`) {
		t.Fatalf("manifest metadata not used:\n%s", text)
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
			RelativePath: rel, Order: 2001 + i,
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
	if err := CopyAssets(source, dest); err != nil {
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
}

func TestCopyAssetsDefinesConceptBucket(t *testing.T) {
	source := t.TempDir()
	dest := t.TempDir()
	fixtures := map[string]string{
		"assets/diagram.svg":     "svg",
		"likec4/model.c4":        "model",
		"likec4/views/system.c4": "view",
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
	if err := CopyAssets(source, dest); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"assets/diagram.svg", "assets/likec4/model.c4", "assets/likec4/views/system.c4", "assets/root.c4"} {
		if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}
