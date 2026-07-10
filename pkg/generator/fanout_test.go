package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grovetools/docgen/pkg/config"
	"github.com/sirupsen/logrus"
)

func newTestGenerator() *Generator {
	l := logrus.New()
	l.SetOutput(os.Stderr)
	l.SetLevel(logrus.WarnLevel)
	return New(l)
}

func TestCleanLLMResponse(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"  hello  ", "hello"},
		{"```markdown\n# Title\nbody\n```", "# Title\nbody"},
		{"```md\n# Title\nbody\n```", "# Title\nbody"},
		{"```\n# Title\nbody\n```", "# Title\nbody"},
		{"# no fence\nbody", "# no fence\nbody"},
	}
	for _, c := range cases {
		if got := cleanLLMResponse(c.in); got != c.want {
			t.Errorf("cleanLLMResponse(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSetupFanout_NonClaudeIsNoop(t *testing.T) {
	g := newTestGenerator()
	cfg := &config.DocgenConfig{Settings: config.SettingsConfig{Model: "gemini-3-pro-preview"}}
	teardown, err := g.setupFanout(t.TempDir(), cfg, GenerateOptions{})
	if err != nil {
		t.Fatalf("setupFanout: %v", err)
	}
	defer teardown()

	if g.prefix != nil {
		t.Error("expected no fan-out prefix for a non-Claude model")
	}
	if g.forceModel != "" {
		t.Errorf("expected no forced model, got %q", g.forceModel)
	}
}

func TestSetupFanout_ClaudeWithNoContextIsNoop(t *testing.T) {
	g := newTestGenerator()
	cfg := &config.DocgenConfig{}
	// Bare dir: WorkDirContextFiles finds no cx context, so fan-out must fall
	// back rather than warm an empty prefix.
	teardown, err := g.setupFanout(t.TempDir(), cfg, GenerateOptions{Model: "claude-haiku-4-5"})
	if err != nil {
		t.Fatalf("setupFanout: %v", err)
	}
	defer teardown()

	if g.prefix != nil {
		t.Error("expected no fan-out prefix when cx produced no context")
	}
}

func TestSetupFanout_ClaudeWithContextEnables(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-dummy") // NewClient needs a key; no network is used

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# build"), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	g := newTestGenerator()
	cfg := &config.DocgenConfig{}
	teardown, err := g.setupFanout(dir, cfg, GenerateOptions{Model: "claude-haiku-4-5", CacheTTL: "1h"})
	if err != nil {
		t.Fatalf("setupFanout: %v", err)
	}

	if g.prefix == nil {
		t.Fatal("expected an active fan-out prefix")
	}
	if g.forceModel != "claude-haiku-4-5" {
		t.Errorf("forceModel = %q, want claude-haiku-4-5", g.forceModel)
	}
	if got := g.prefix.Model(); got != "claude-haiku-4-5-20251001" {
		t.Errorf("prefix model = %q, want resolved id", got)
	}

	teardown()
	if g.prefix != nil || g.forceModel != "" {
		t.Error("teardown should clear fan-out state")
	}
}

// TestSetupFanout_OverWindowContextFails covers the window precheck: a context
// whose byte size cannot fit the model window must fail the run up front
// (permanent, before any spend) instead of fanning out into a wave of API 400s.
func TestSetupFanout_OverWindowContextFails(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-dummy")

	dir := t.TempDir()
	// docsWindowTokens * docsBytesPerToken is the byte ceiling; exceed it by a
	// whole token so integer division lands strictly past the window.
	big := make([]byte, docsWindowTokens*docsBytesPerToken+docsBytesPerToken)
	for i := range big {
		big[i] = 'a'
	}
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), big, 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	g := newTestGenerator()
	teardown, err := g.setupFanout(dir, &config.DocgenConfig{}, GenerateOptions{Model: "claude-haiku-4-5"})
	if err == nil {
		teardown()
		t.Fatal("expected an over-window error, got nil")
	}
	if !strings.Contains(err.Error(), "docs context too large") {
		t.Errorf("error should carry the permanent-classification marker, got: %v", err)
	}
	if g.prefix != nil {
		t.Error("no prefix must be set up for an over-window context")
	}
}

// TestFailedSectionsError covers the run-level error composition: the first
// failed section's cause must ride along when recorded.
func TestFailedSectionsError(t *testing.T) {
	g := newTestGenerator()
	g.recordSectionFailure("overview", fmt.Errorf("anthropic API error (HTTP 400 invalid_request_error): prompt is too long"))
	g.recordSectionFailure("examples", fmt.Errorf("prompt is too long"))

	err := g.failedSectionsError([]string{"overview", "examples"})
	msg := err.Error()
	if !strings.Contains(msg, "2 section(s) failed: overview, examples") {
		t.Errorf("missing section list: %v", msg)
	}
	if !strings.Contains(msg, "first error: anthropic API error") {
		t.Errorf("missing first cause: %v", msg)
	}

	// Without a recorded cause the error stays cause-less but well-formed.
	g2 := newTestGenerator()
	if got := g2.failedSectionsError([]string{"x"}).Error(); strings.Contains(got, "first error") {
		t.Errorf("unexpected cause segment: %v", got)
	}
}

// TestParseTUIRegistry accepts both `grove keys dump` shapes: the current
// {"tui": [...], "tmux": [...]} object and the legacy bare array.
func TestParseTUIRegistry(t *testing.T) {
	entryJSON := `{"Name":"cx-view","Package":"cx","Description":"d","Sections":[]}`

	got, err := parseTUIRegistry([]byte(`{"tui":[` + entryJSON + `],"tmux":[{"Name":"leader"}]}`))
	if err != nil || len(got) != 1 || got[0].Name != "cx-view" {
		t.Fatalf("object form: got %v, err %v", got, err)
	}

	got, err = parseTUIRegistry([]byte(`[` + entryJSON + `]`))
	if err != nil || len(got) != 1 || got[0].Name != "cx-view" {
		t.Fatalf("legacy array form: got %v, err %v", got, err)
	}

	if _, err = parseTUIRegistry([]byte(`"nonsense"`)); err == nil {
		t.Fatal("expected an error for a non-registry payload")
	}
}
