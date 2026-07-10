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

// TestWriteProposalBundleClearsStalePrompts verifies a rewrite removes prompt
// files from a PREVIOUS bundle (whose filenames differ) plus a stale
// PROPOSAL.raw.md, so the staged prompts/ matches exactly the new proposal.
func TestWriteProposalBundleClearsStalePrompts(t *testing.T) {
	dir := t.TempDir()

	// Simulate a prior bundle: two prompt files and a leftover raw dump.
	promptsDir := filepath.Join(dir, "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, stale := range []string{"02-channels-guide.md", "08-agent-instructions.md"} {
		if err := os.WriteFile(filepath.Join(promptsDir, stale), []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "PROPOSAL.raw.md"), []byte("raw"), 0o600); err != nil {
		t.Fatal(err)
	}

	b, err := parseProposalResponse(cannedProposalResponse)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := writeProposalBundle(dir, b); err != nil {
		t.Fatalf("write: %v", err)
	}

	entries, err := os.ReadDir(promptsDir)
	if err != nil {
		t.Fatalf("reading prompts dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "01-overview.md" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("stale prompts not cleared; prompts/ = %v, want only 01-overview.md", names)
	}
	if _, err := os.Stat(filepath.Join(dir, "PROPOSAL.raw.md")); !os.IsNotExist(err) {
		t.Errorf("stale PROPOSAL.raw.md not removed (err=%v)", err)
	}
}

// TestAssembleProposeSuffixFresh asserts the --fresh suffix uses the fresh
// instruction, withholds the current prompts/sections/README, and keeps only
// the reduced settings.
func TestAssembleProposeSuffixFresh(t *testing.T) {
	in := proposeInputs{
		repo:       "example",
		configName: "docgen.config.yml",
		configYAML: "enabled: true\ntitle: Example\nsettings:\n  model: claude-haiku-4-5\n",
		fresh:      true,
	}
	suffix := assembleProposeSuffix(in)

	if !strings.Contains(suffix, FreshProposeInstruction[:40]) {
		t.Errorf("fresh suffix must lead with the fresh instruction")
	}
	if !strings.Contains(suffix, "sections withheld") {
		t.Errorf("fresh suffix missing the withheld-sections label:\n%s", suffix)
	}
	if !strings.Contains(suffix, "model: claude-haiku-4-5") {
		t.Errorf("fresh suffix should still carry the settings block")
	}
	// The fresh instruction must NOT carry the evolve-the-current-list framing
	// or a KEPT/ADDED change column.
	if strings.Contains(suffix, "Prefer evolving the current list") {
		t.Errorf("fresh suffix should not tell the model to evolve the current list")
	}
	if strings.Contains(suffix, "KEPT/ADDED/REMOVED/MERGED") {
		t.Errorf("fresh outline must not include the change column")
	}
	// Both instructions must enumerate the valid schema fields (hardening).
	if !strings.Contains(suffix, "path") || !strings.Contains(suffix, "invert_filter") {
		t.Errorf("fresh instruction should enumerate the valid schemas: fields")
	}
}

// TestProposeInstructionHardening asserts the default instruction spells out
// the valid schemas: entry fields (path/title only).
func TestProposeInstructionHardening(t *testing.T) {
	if !strings.Contains(ProposeInstruction, "path") || !strings.Contains(ProposeInstruction, "invert_filter") {
		t.Errorf("ProposeInstruction should name path/title and forbid invert_filter")
	}
}

// TestProposeInstructionsCarryOutputRule asserts BOTH instruction variants tell
// the model that every section needs an explicit output: filename — the rule
// that prevents the --fresh empty-output bug (LLM spent, write onto a directory).
func TestProposeInstructionsCarryOutputRule(t *testing.T) {
	const frag = "output:"
	for name, instr := range map[string]string{
		"ProposeInstruction":      ProposeInstruction,
		"FreshProposeInstruction": FreshProposeInstruction,
	} {
		if !strings.Contains(instr, "EVERY section MUST set an explicit") {
			t.Errorf("%s missing the explicit-output rule", name)
		}
		if !strings.Contains(instr, frag) {
			t.Errorf("%s missing the %q keyword", name, frag)
		}
		if !strings.Contains(instr, "descriptions.json") {
			t.Errorf("%s missing the schema_describe output-file convention", name)
		}
	}
}

// TestProposeInstructionsCarryCaptureAndSchemaRules asserts BOTH instruction
// variants carry the capture-fields rule (binary:, not command:) and the
// schema-pair rules (path is a generated .schema.json, never a .go file;
// schema_describe/schema_table stay separate sections) — the two live --fresh
// defect classes the notify test run exposed.
func TestProposeInstructionsCarryCaptureAndSchemaRules(t *testing.T) {
	frags := []string{
		"`capture` section MUST set `binary:`",
		"There is NO `command:`",
		"NEVER a `.go` source file",
		"never collapse them into one section",
	}
	for name, instr := range map[string]string{
		"ProposeInstruction":      ProposeInstruction,
		"FreshProposeInstruction": FreshProposeInstruction,
	} {
		for _, frag := range frags {
			if !strings.Contains(instr, frag) {
				t.Errorf("%s missing fragment %q", name, frag)
			}
		}
	}
}

// TestWriteProposalBundleWarnsMissingOutputAndPrompt verifies the bundle probe
// surfaces (a) a section with no output: and (b) a prose section whose prompt
// file is not among the bundle's drafted prompts — both joined into one warning.
func TestWriteProposalBundleWarnsMissingOutputAndPrompt(t *testing.T) {
	b := &proposalBundle{
		Rationale: "r",
		Outline:   "o",
		Config: "enabled: true\nsections:\n" +
			"  - name: quick-start\n    type: prose\n    prompt: 03-quick-start.md\n" + // no output, prompt not in bundle
			"  - name: 02-cli\n    type: capture\n    output: 02-cli.md\n",
		// The bundle drafted only 01-overview.md, NOT 03-quick-start.md.
		Prompts: []promptFile{{Name: "01-overview.md", Content: "x"}},
	}
	dir := t.TempDir()
	res, err := writeProposalBundle(dir, b)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(res.ConfigWarning, `section "quick-start" has no output: filename`) {
		t.Errorf("missing empty-output warning, got: %q", res.ConfigWarning)
	}
	if !strings.Contains(res.ConfigWarning, `prose section "quick-start" prompt 03-quick-start.md not in bundle`) {
		t.Errorf("missing missing-prompt warning, got: %q", res.ConfigWarning)
	}
	// The capture section in the fixture has no binary: — the probe must flag it.
	if !strings.Contains(res.ConfigWarning, `capture section "02-cli" has no binary: field`) {
		t.Errorf("missing capture-binary warning, got: %q", res.ConfigWarning)
	}
	if _, err := os.Stat(res.ConfigPath); err != nil {
		t.Errorf("config with warnings should still be written for review: %v", err)
	}
}

// TestStrictConfigWarning covers the KnownFields re-decode: invented keys the
// lenient probe silently drops (the live case: command: on a capture section)
// surface as a warning, while a config using only real fields stays clean.
func TestStrictConfigWarning(t *testing.T) {
	invented := "enabled: true\nsections:\n" +
		"  - name: cli\n    type: capture\n    command: notify --help\n    output: 02-cli.md\n"
	if w := strictConfigWarning(invented); !strings.Contains(w, "not part of the docgen config schema") {
		t.Errorf("expected unknown-field warning for invented command:, got: %q", w)
	}
	valid := "enabled: true\nsections:\n" +
		"  - name: cli\n    type: capture\n    binary: notify\n    format: styled\n    output: 02-cli.md\n" +
		"  - name: overview\n    type: prose\n    prompt: 01-overview.md\n    output: 01-overview.md\n"
	if w := strictConfigWarning(valid); w != "" {
		t.Errorf("expected no warning for a valid config, got: %q", w)
	}
}

// TestValidateSectionOutputs covers the generate-side pre-spend guard: empty
// output errors with the load-bearing fragment and lists all offenders, while a
// fully-populated set passes.
func TestValidateSectionOutputs(t *testing.T) {
	// Happy path: every section has an output.
	ok := []config.SectionConfig{
		{Name: "01-overview", Output: "01-overview.md"},
		{Name: "02-cli", Output: "02-cli.md"},
	}
	if err := validateSectionOutputs(ok); err != nil {
		t.Fatalf("expected no error when all outputs present, got: %v", err)
	}

	// Multiple offenders: the error must carry the classifier fragment and name
	// every offending section.
	bad := []config.SectionConfig{
		{Name: "quick-start", Output: ""},
		{Name: "02-cli", Output: "02-cli.md"},
		{Name: "03-guide", Output: "   "}, // whitespace-only counts as empty
	}
	err := validateSectionOutputs(bad)
	if err == nil {
		t.Fatal("expected an error for sections with empty output")
	}
	if !strings.Contains(err.Error(), "has no output: filename") {
		t.Errorf("error missing the retry-classifier fragment: %v", err)
	}
	for _, name := range []string{"quick-start", "03-guide"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error does not name offender %q: %v", name, err)
		}
	}
	// "1 more" — two offenders means the head plus one in the parenthetical.
	if !strings.Contains(err.Error(), "1 more") {
		t.Errorf("error should summarize the remaining offenders count: %v", err)
	}

	// Capture without binary: fails pre-spend with the config-validation-family
	// fragment grove's retry-classifier marks permanent.
	capture := []config.SectionConfig{
		{Name: "01-overview", Type: "prose", Prompt: "01-overview.md", Output: "01-overview.md"},
		{Name: "02-cli", Type: "capture", Output: "02-cli.md"},
	}
	err = validateSectionOutputs(capture)
	if err == nil {
		t.Fatal("expected an error for a capture section with no binary")
	}
	if !strings.Contains(err.Error(), "section type 'capture' requires 'binary'") {
		t.Errorf("error missing the capture config-validation fragment: %v", err)
	}
	if !strings.Contains(err.Error(), "02-cli") {
		t.Errorf("error does not name the offending capture section: %v", err)
	}

	// A capture section WITH binary passes.
	okCapture := []config.SectionConfig{{Name: "02-cli", Type: "capture", Binary: "notify", Output: "02-cli.md"}}
	if err := validateSectionOutputs(okCapture); err != nil {
		t.Fatalf("expected no error for capture with binary, got: %v", err)
	}
}

// TestReduceConfigForFresh verifies the sections list is dropped while the
// settings survive, and the result is still valid YAML.
func TestReduceConfigForFresh(t *testing.T) {
	full := []byte("enabled: true\ntitle: Example\nsettings:\n  model: claude-haiku-4-5\n  rules_file: doc\nsections:\n  - name: 01-overview\n    title: Overview\n    order: 1\n    type: prose\n    output: 01-overview.md\n")
	reduced, err := reduceConfigForFresh(full)
	if err != nil {
		t.Fatalf("reduce: %v", err)
	}
	var cfg config.DocgenConfig
	if err := yaml.Unmarshal([]byte(reduced), &cfg); err != nil {
		t.Fatalf("reduced config not valid yaml: %v", err)
	}
	if len(cfg.Sections) != 0 {
		t.Errorf("reduced config should have no sections, got %d", len(cfg.Sections))
	}
	if cfg.Settings.Model != "claude-haiku-4-5" {
		t.Errorf("reduced config dropped settings.model: %q", cfg.Settings.Model)
	}
	if strings.Contains(reduced, "01-overview") {
		t.Errorf("reduced config still references a section: %s", reduced)
	}
}

// TestProposeTranscriptRoundTrip verifies transcript.json writes and loads back
// with its model and turns intact, and that toMessageTurns preserves order.
func TestProposeTranscriptRoundTrip(t *testing.T) {
	dir := t.TempDir()
	orig := &proposeTranscript{
		Model: "claude-haiku-4-5",
		Turns: []proposeTranscriptTurn{
			{Role: "user", Content: "turn-0 suffix"},
			{Role: "assistant", Content: "turn-0 proposal"},
		},
	}
	if err := writeProposeTranscript(dir, orig); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	loaded, err := loadProposeTranscript(filepath.Join(dir, "transcript.json"))
	if err != nil {
		t.Fatalf("load transcript: %v", err)
	}
	if loaded.Model != orig.Model || len(loaded.Turns) != 2 {
		t.Fatalf("round-trip mismatch: %+v", loaded)
	}
	turns := toMessageTurns(loaded.Turns)
	if len(turns) != 2 || turns[0].Role != "user" || turns[1].Content != "turn-0 proposal" {
		t.Fatalf("toMessageTurns mismatch: %+v", turns)
	}
	if toMessageTurns(nil) != nil {
		t.Errorf("toMessageTurns(nil) should be nil (turn 0)")
	}
}

// TestLoadProposeTranscriptRejectsEmpty covers the missing-model / empty-turns
// guard.
func TestLoadProposeTranscriptRejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.json")
	if err := os.WriteFile(path, []byte(`{"model":"","turns":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProposeTranscript(path); err == nil {
		t.Error("expected an error for an empty transcript")
	}
}

// TestFollowupTaskPrompt asserts the feedback is wrapped in a re-emit
// instruction rather than passed bare.
func TestFollowupTaskPrompt(t *testing.T) {
	p := followupTaskPrompt("  merge the CLI pages  ")
	if !strings.Contains(p, "merge the CLI pages") {
		t.Errorf("feedback not carried: %q", p)
	}
	if !strings.Contains(p, "COMPLETE") || !strings.Contains(p, "delimited format") {
		t.Errorf("followup prompt should instruct a complete re-emit: %q", p)
	}
}

// writeProposeTestRepo writes a minimal docgen repo (config + one prompt) and
// returns its root, for the dry-run/guard tests below.
func writeProposeTestRepo(t *testing.T) string {
	t.Helper()
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
	return repo
}

// TestProposeFreshDryRunSuffix verifies --fresh --dry-run records a suffix that
// uses the fresh instruction and withholds the current prompt/section.
func TestProposeFreshDryRunSuffix(t *testing.T) {
	repo := writeProposeTestRepo(t)
	out := filepath.Join(t.TempDir(), "proposal")
	g := New(newTestLogger())
	if err := g.Propose(repo, ProposeOptions{
		Model:     "claude-haiku-4-5",
		OutputDir: out,
		DryRun:    true,
		Fresh:     true,
	}); err != nil {
		t.Fatalf("fresh dry-run failed: %v", err)
	}
	suffix, err := os.ReadFile(filepath.Join(out, "PROPOSE_SUFFIX.md"))
	if err != nil {
		t.Fatalf("dry run did not record the suffix: %v", err)
	}
	if !strings.Contains(string(suffix), "sections withheld") {
		t.Errorf("fresh suffix missing withheld-sections framing")
	}
	if strings.Contains(string(suffix), "Write the overview.") {
		t.Errorf("fresh suffix must NOT include current prompt bodies")
	}
	if strings.Contains(string(suffix), "01-overview\n    title") {
		t.Errorf("fresh suffix must NOT include the current section list")
	}
}

// TestProposeFreshFollowupMutuallyExclusive asserts the two modes cannot be
// combined.
func TestProposeFreshFollowupMutuallyExclusive(t *testing.T) {
	repo := writeProposeTestRepo(t)
	g := New(newTestLogger())
	err := g.Propose(repo, ProposeOptions{
		Model:     "claude-haiku-4-5",
		OutputDir: filepath.Join(t.TempDir(), "p"),
		Fresh:     true,
		Followup:  "do it differently",
	})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected a mutual-exclusion error, got %v", err)
	}
}

// TestProposeFollowupRequiresTranscript asserts --followup without a transcript
// path errors before any network use.
func TestProposeFollowupRequiresTranscript(t *testing.T) {
	repo := writeProposeTestRepo(t)
	g := New(newTestLogger())
	err := g.Propose(repo, ProposeOptions{
		Model:     "claude-haiku-4-5",
		OutputDir: filepath.Join(t.TempDir(), "p"),
		Followup:  "revise",
	})
	if err == nil || !strings.Contains(err.Error(), "requires --transcript") {
		t.Fatalf("expected a missing-transcript error, got %v", err)
	}
}

// TestProposeFollowupModelMismatch asserts a followup against a transcript from
// a different model is rejected (cache + coherence depend on the model).
func TestProposeFollowupModelMismatch(t *testing.T) {
	repo := writeProposeTestRepo(t)
	tdir := t.TempDir()
	tpath := filepath.Join(tdir, "transcript.json")
	if err := os.WriteFile(tpath, []byte(`{"model":"claude-sonnet-4-5","turns":[{"role":"user","content":"x"},{"role":"assistant","content":"y"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	g := New(newTestLogger())
	err := g.Propose(repo, ProposeOptions{
		Model:          "claude-haiku-4-5",
		OutputDir:      filepath.Join(t.TempDir(), "p"),
		Followup:       "revise",
		TranscriptPath: tpath,
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected a model-mismatch error, got %v", err)
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
