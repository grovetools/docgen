package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDocsRulesFile(t *testing.T) {
	repo := t.TempDir()
	if got, err := ResolveDocsRulesFile(repo); err != nil || got != "" {
		t.Fatalf("configless repo = %q, %v; want empty, nil", got, err)
	}

	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(repo, "docs", ConfigFileName)
	write(configPath, "settings: {}\n")
	if _, err := ResolveDocsRulesFile(repo); err == nil || !strings.Contains(err.Error(), configPath) {
		t.Fatalf("missing rules_file error = %v; want config path", err)
	}

	preset := filepath.Join(repo, ".cx", "doc.rules")
	write(preset, "**/*.go\n")
	write(configPath, "settings:\n  rules_file: doc\n")
	got, err := ResolveDocsRulesFile(repo)
	if err != nil || got != preset {
		t.Fatalf("named preset = %q, %v; want %q", got, err, preset)
	}

	legacy := filepath.Join(repo, "docs", "legacy.rules")
	write(legacy, "**/*.go\n")
	write(configPath, "settings:\n  rules_file: legacy.rules\n")
	got, err = ResolveDocsRulesFile(repo)
	if err != nil || got != legacy {
		t.Fatalf("legacy path = %q, %v; want %q", got, err, legacy)
	}
}
