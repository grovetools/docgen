package generator

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/grovetools/docgen/pkg/config"
)

// newTestLogger returns a logrus logger that discards output, so Propose's
// progress lines do not clutter test output.
func newTestLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return l
}

// cannedProposalResponse is a well-formed model response in the delimited
// propose format, used to exercise parsing + bundle writing without any API call.
const cannedProposalResponse = "" +
	proposeDelimRationale + "\n" +
	"We split the monolithic overview into an overview plus a dedicated CLI\n" +
	"reference, and dropped the stale schema page.\n" +
	proposeDelimOutline + "\n" +
	"| Order | Name | Title | Type | Change | Reason |\n" +
	"| ----- | ---- | ----- | ---- | ------ | ------ |\n" +
	"| 1 | 01-overview | Overview | prose | KEPT | still the entry point |\n" +
	"| 2 | 02-cli | CLI Reference | capture | ADDED | commands were undocumented |\n" +
	proposeDelimConfig + "\n" +
	"```yaml\n" +
	"enabled: true\n" +
	"title: Example\n" +
	"settings:\n" +
	"  model: claude-haiku-4-5\n" +
	"  rules_file: doc\n" +
	"sections:\n" +
	"  - name: 01-overview\n" +
	"    title: Overview\n" +
	"    order: 1\n" +
	"    type: prose\n" +
	"    prompt: 01-overview.md\n" +
	"    output: 01-overview.md\n" +
	"  - name: 02-cli\n" +
	"    title: CLI Reference\n" +
	"    order: 2\n" +
	"    type: capture\n" +
	"    binary: example\n" +
	"    output: 02-cli.md\n" +
	"```\n" +
	proposeDelimPromptPrefix + "01-overview" + proposeDelimPromptSuffix + "\n" +
	"```markdown\n" +
	"# Overview prompt\n\n" +
	"Write an overview that covers:\n" +
	"1. What the tool does\n" +
	"2. Who it is for\n" +
	"```\n" +
	proposeDelimEnd + "\n"

// TestParseProposalResponse verifies the delimited response splits into the
// three deliverables plus the rationale, and that fences are stripped.
func TestParseProposalResponse(t *testing.T) {
	b, err := parseProposalResponse(cannedProposalResponse)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !strings.Contains(b.Rationale, "split the monolithic overview") {
		t.Errorf("rationale not captured: %q", b.Rationale)
	}
	if !strings.Contains(b.Outline, "| 2 | 02-cli |") {
		t.Errorf("outline not captured: %q", b.Outline)
	}
	// Config must have its ```yaml fence stripped and remain valid.
	if strings.Contains(b.Config, "```") {
		t.Errorf("config fence not stripped: %q", b.Config)
	}
	var cfg config.DocgenConfig
	if err := yaml.Unmarshal([]byte(b.Config), &cfg); err != nil {
		t.Fatalf("proposed config not valid yaml: %v", err)
	}
	if len(cfg.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(cfg.Sections))
	}
	if len(b.Prompts) != 1 || b.Prompts[0].Name != "01-overview.md" {
		t.Fatalf("expected one prompt named 01-overview.md, got %+v", b.Prompts)
	}
	if strings.Contains(b.Prompts[0].Content, "```") {
		t.Errorf("prompt fence not stripped: %q", b.Prompts[0].Content)
	}
	if !strings.Contains(b.Prompts[0].Content, "Write an overview") {
		t.Errorf("prompt body not captured: %q", b.Prompts[0].Content)
	}
}

// TestParseProposalResponseErrors covers the two hard-failure cases: no
// delimiters at all, and a response missing the required config block.
func TestParseProposalResponseErrors(t *testing.T) {
	if _, err := parseProposalResponse("just some prose, no delimiters"); err == nil {
		t.Error("expected an error for a response with no delimiters")
	}
	noConfig := proposeDelimRationale + "\nwhy\n" + proposeDelimOutline + "\n| a |\n" + proposeDelimEnd + "\n"
	if _, err := parseProposalResponse(noConfig); err == nil {
		t.Error("expected an error for a response missing the config block")
	}
}

// TestWriteProposalBundle feeds the parsed canned response through the writer
// and asserts all three outputs land, the config is valid, and the prompt file
// is created with content.
func TestWriteProposalBundle(t *testing.T) {
	b, err := parseProposalResponse(cannedProposalResponse)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	dir := t.TempDir()
	res, err := writeProposalBundle(dir, b)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if res.ConfigWarning != "" {
		t.Errorf("unexpected config warning: %s", res.ConfigWarning)
	}

	proposal, err := os.ReadFile(filepath.Join(dir, "PROPOSAL.md"))
	if err != nil {
		t.Fatalf("reading PROPOSAL.md: %v", err)
	}
	if !strings.Contains(string(proposal), "## Rationale") || !strings.Contains(string(proposal), "## Proposed Outline") {
		t.Errorf("PROPOSAL.md missing expected sections:\n%s", proposal)
	}
	if !strings.Contains(string(proposal), "split the monolithic overview") {
		t.Errorf("PROPOSAL.md missing rationale text")
	}

	cfgData, err := os.ReadFile(filepath.Join(dir, "proposed.docgen.config.yml"))
	if err != nil {
		t.Fatalf("reading proposed config: %v", err)
	}
	var cfg config.DocgenConfig
	if err := yaml.Unmarshal(cfgData, &cfg); err != nil {
		t.Fatalf("written config not valid: %v", err)
	}
	if len(cfg.Sections) != 2 {
		t.Fatalf("expected 2 sections in written config, got %d", len(cfg.Sections))
	}

	if len(res.PromptPaths) != 1 {
		t.Fatalf("expected 1 written prompt, got %d", len(res.PromptPaths))
	}
	promptData, err := os.ReadFile(res.PromptPaths[0])
	if err != nil {
		t.Fatalf("reading written prompt: %v", err)
	}
	if !strings.Contains(string(promptData), "Write an overview") {
		t.Errorf("written prompt missing content:\n%s", promptData)
	}
	if !strings.HasSuffix(res.PromptPaths[0], filepath.Join("prompts", "01-overview.md")) {
		t.Errorf("prompt written to unexpected path: %s", res.PromptPaths[0])
	}
}

// TestWriteProposalBundleInvalidConfig verifies an unparseable config is still
// written (for human review) but surfaces a warning rather than failing.
func TestWriteProposalBundleInvalidConfig(t *testing.T) {
	b := &proposalBundle{
		Rationale: "r",
		Outline:   "o",
		Config:    "this: : : not: valid: yaml",
	}
	dir := t.TempDir()
	res, err := writeProposalBundle(dir, b)
	if err != nil {
		t.Fatalf("write should not fail on invalid config: %v", err)
	}
	if res.ConfigWarning == "" {
		t.Error("expected a config validation warning for invalid yaml")
	}
	if _, err := os.Stat(res.ConfigPath); err != nil {
		t.Errorf("invalid config should still be written for review: %v", err)
	}
}

// TestAssembleProposeSuffix asserts the suffix carries the standing instruction
// plus the current config, prompts, and README template — the material a review
// turn must see — all as cache-suffix (never prefix) content.
func TestAssembleProposeSuffix(t *testing.T) {
	in := proposeInputs{
		repo:       "example",
		configName: "docgen.config.yml",
		configYAML: "enabled: true\nsettings:\n  model: claude-haiku-4-5\n",
		prompts: []promptFile{
			{Name: "01-overview.md", Content: "# Overview\nWrite about the tool."},
			{Name: "02-usage.md", Content: "# Usage\nWrite about usage."},
		},
		readmeName: "README.md.tpl",
		readmeTpl:  "# {{ .Title }}\n{{ .Body }}",
	}
	suffix := assembleProposeSuffix(in)

	for _, want := range []string{
		ProposeInstruction[:40],     // the standing instruction leads
		"Current docgen.config.yml", // config included
		"model: claude-haiku-4-5",   // config body included
		"Prompt: `01-overview.md`",  // each prompt named
		"Write about usage.",        // prompt body included
		"Current README template",   // readme included
		"{{ .Title }}",              // readme body included
		proposeDelimRationale,       // the required output format is specified
	} {
		if !strings.Contains(suffix, want) {
			t.Errorf("suffix missing %q", want)
		}
	}
}

// TestAssembleProposeSuffixNoPrompts covers the empty-prompts and no-readme case.
func TestAssembleProposeSuffixNoPrompts(t *testing.T) {
	suffix := assembleProposeSuffix(proposeInputs{
		repo:       "bare",
		configName: "docgen.config.yml",
		configYAML: "enabled: true\n",
	})
	if !strings.Contains(suffix, "## Current prompt files\n\n(none)") {
		t.Errorf("expected an explicit (none) for a repo with no prompts:\n%s", suffix)
	}
	if strings.Contains(suffix, "Current README template") {
		t.Errorf("README section should be omitted when there is no template")
	}
}

// TestProposeDryRunNoNetwork verifies --dry-run assembles + saves the suffix and
// makes no API call. It runs Propose against a temp repo with a claude model; a
// dry run must not touch the network (no cx build, no request), so it completes
// with only local file I/O.
func TestProposeDryRunNoNetwork(t *testing.T) {
	repo := t.TempDir()
	docsDir := filepath.Join(repo, "docs")
	if err := os.MkdirAll(filepath.Join(docsDir, "prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgYAML := "enabled: true\ntitle: Example\nsettings:\n  model: claude-haiku-4-5\n  rules_file: doc\nsections:\n  - name: 01-overview\n    title: Overview\n    order: 1\n    type: prose\n    prompt: 01-overview.md\n    output: 01-overview.md\n"
	if err := os.WriteFile(filepath.Join(docsDir, config.ConfigFileName), []byte(cfgYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "prompts", "01-overview.md"), []byte("# Overview\nWrite the overview."), 0o600); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "proposal")
	g := New(newTestLogger())
	err := g.Propose(repo, ProposeOptions{
		Model:     "claude-haiku-4-5",
		OutputDir: out,
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("dry-run propose failed: %v", err)
	}
	suffix, err := os.ReadFile(filepath.Join(out, "PROPOSE_SUFFIX.md"))
	if err != nil {
		t.Fatalf("dry run did not write the suffix: %v", err)
	}
	if !strings.Contains(string(suffix), "model: claude-haiku-4-5") {
		t.Errorf("dry-run suffix missing current config")
	}
	if !strings.Contains(string(suffix), "Write the overview.") {
		t.Errorf("dry-run suffix missing current prompt")
	}
	// A dry run must not have written any bundle files (that needs an API call).
	if _, err := os.Stat(filepath.Join(out, "PROPOSAL.md")); err == nil {
		t.Error("dry run must not write a PROPOSAL.md (no API call)")
	}
}

// TestProposeRejectsNonClaudeModel asserts the claude-only guard fires before
// any network use, with a clear message.
func TestProposeRejectsNonClaudeModel(t *testing.T) {
	repo := t.TempDir()
	docsDir := filepath.Join(repo, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgYAML := "enabled: true\nsettings:\n  model: gemini-3-pro-preview\n  rules_file: doc\nsections: []\n"
	if err := os.WriteFile(filepath.Join(docsDir, config.ConfigFileName), []byte(cfgYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	g := New(newTestLogger())
	err := g.Propose(repo, ProposeOptions{
		Model:     "gemini-3-pro-preview",
		OutputDir: filepath.Join(t.TempDir(), "p"),
	})
	if err == nil || !strings.Contains(err.Error(), "requires a claude model") {
		t.Fatalf("expected a claude-only error, got %v", err)
	}
}
