package generator

import (
	"os"
	"path/filepath"
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
	teardown := g.setupFanout(t.TempDir(), cfg, GenerateOptions{})
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
	teardown := g.setupFanout(t.TempDir(), cfg, GenerateOptions{Model: "claude-haiku-4-5"})
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
	teardown := g.setupFanout(dir, cfg, GenerateOptions{Model: "claude-haiku-4-5", CacheTTL: "1h"})

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
