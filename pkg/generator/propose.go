package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/grovetools/docgen/pkg/config"
	"github.com/grovetools/grove-anthropic/pkg/anthropic"
	"gopkg.in/yaml.v3"
)

// ProposeOptions configures a `docgen propose` "turn 0": one request that rides
// the same cached cx-context prefix the docs fan-out uses and proposes an
// updated docs outline (sections + prompts) as a reviewable bundle.
type ProposeOptions struct {
	// Model is the claude-* model whose cache the proposal warms. It MUST match
	// the model a later `docgen generate` uses, or the cache is not shared.
	Model string
	// CacheTTL is the shared-prefix cache TTL: "5m" or "1h". Empty ⇒ "1h" — the
	// propose default, since human review of the bundle routinely exceeds 5m and
	// a later `generate` must still cache-READ this warmed prefix.
	CacheTTL string
	// OutputDir is where the proposal bundle is written. Required. The live
	// notebook config/prompts are never touched.
	OutputDir string
	// UsageJSONPath, when non-empty, receives the machine-readable usage report
	// (same UsageReport shape as `docgen generate --usage-json`).
	UsageJSONPath string
	// DryRun assembles and saves the request SUFFIX without any API call (no cx
	// build, no upload, no request).
	DryRun bool
}

// promptFile is one named prompt document: a current prompt fed into the
// proposal suffix, or a drafted prompt parsed out of the proposal response.
type promptFile struct {
	Name    string
	Content string
}

// proposeInputs is the material appended AFTER the cached prefix in a propose
// request: the repo's current docgen config, its current prompt files, and its
// README template (if any). This is all cache-SUFFIX (volatile) material — none
// of it is part of the cached prefix, so a human editing prompts/config between
// propose and generate does not change the prefix bytes.
type proposeInputs struct {
	repo       string
	configName string
	configYAML string
	prompts    []promptFile
	readmeName string
	readmeTpl  string
}

// proposalBundle is the parsed proposal, split from the model's delimited
// response into its three deliverables plus the overall rationale.
type proposalBundle struct {
	Rationale string
	Outline   string
	Config    string
	Prompts   []promptFile
}

// proposalWriteResult reports where each bundle file landed plus any config
// validation warning (the proposed config is written regardless — a human
// reviews it — but a parse failure is surfaced).
type proposalWriteResult struct {
	ProposalPath  string
	ConfigPath    string
	PromptPaths   []string
	ConfigWarning string
}

// Propose runs the docs-outline "turn 0" for packageDir: it resolves config +
// rules exactly like generate, builds the same cx context, warms a shared prefix
// byte-identical to the docs fan-out's, sends ONE request whose suffix carries
// the current config/prompts/README and the propose instruction, and writes the
// parsed proposal bundle to opts.OutputDir. It never overwrites live files.
func (g *Generator) Propose(packageDir string, opts ProposeOptions) error {
	if strings.TrimSpace(opts.OutputDir) == "" {
		return fmt.Errorf("propose requires an --output-dir for the proposal bundle")
	}

	cfg, configPath, err := config.LoadWithNotebook(packageDir)
	if err != nil {
		return fmt.Errorf("failed to load docgen config: %w", err)
	}

	// Claude models only — the entire point is that the proposal warms a cache a
	// later `generate` cache-reads. A non-claude model cannot share that prefix.
	model := opts.Model
	if model == "" {
		model = cfg.Settings.Model
	}
	if model == "" {
		return fmt.Errorf("propose requires a claude model; the point is the shared cache — pass --model claude-* or set settings.model")
	}
	if !anthropic.IsAnthropicModel(model) {
		return fmt.Errorf("propose requires a claude model; the point is the shared cache (got %q)", model)
	}

	ttl := opts.CacheTTL
	if ttl == "" {
		ttl = anthropic.CacheTTL1h
	}

	// Assemble the request SUFFIX from the repo's current docs material.
	inputs, err := gatherProposeInputs(packageDir, cfg, configPath)
	if err != nil {
		return err
	}
	suffix := assembleProposeSuffix(inputs)

	// Dry run: save the suffix and stop before any cx build or API spend. The
	// cached prefix (cx context) is NOT part of the suffix, so a dry run needs
	// no context build at all.
	if opts.DryRun {
		if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil { //nolint:gosec // internal doc tool
			return fmt.Errorf("creating output dir: %w", err)
		}
		suffixPath := filepath.Join(opts.OutputDir, "PROPOSE_SUFFIX.md")
		if err := os.WriteFile(suffixPath, []byte(suffix), 0o644); err != nil { //nolint:gosec // internal doc tool
			return fmt.Errorf("writing dry-run suffix: %w", err)
		}
		g.logger.Infof("Dry run: assembled propose suffix (%d bytes, %d current prompt(s)) to %s — no API call",
			len(suffix), len(inputs.prompts), suffixPath)
		return nil
	}

	// Resolve rules + build the cx context exactly as generate does, so the
	// prefix bytes are identical to the docs fan-out's.
	rulesPath, err := config.ResolveDocsRulesFile(packageDir)
	if err != nil {
		return fmt.Errorf("failed to resolve docs rules: %w", err)
	}
	g.logger.Info("Building context with 'cx generate'...")
	if err := g.BuildContext(packageDir, rulesPath); err != nil {
		return fmt.Errorf("failed to build context: %w", err)
	}

	ctxFiles := anthropic.WorkDirContextFiles(packageDir)
	if len(ctxFiles) == 0 {
		return fmt.Errorf("cx produced no context in %s; cannot warm a shared prefix for the proposal", packageDir)
	}
	// Same pre-spend window guard the fan-out uses — an over-window prefix 400s.
	if err := checkDocsWindow(model, ctxFiles); err != nil {
		return err
	}

	prefix, err := newDocsSharedPrefix(ctxFiles, model, ttl)
	if err != nil {
		return fmt.Errorf("failed to set up shared prefix for propose: %w", err)
	}
	defer func() { _ = prefix.Close() }()

	g.logger.Infof("Propose via cache fan-out: model=%s ttl=%s prefix_docs=%d", prefix.Model(), ttl, len(ctxFiles))
	ulog.Info("Propose cache fan-out").
		Field("model", prefix.Model()).
		Field("ttl", ttl).
		Field("prefix_docs", len(ctxFiles)).
		Emit()

	if opts.UsageJSONPath != "" {
		defer g.writeUsageReport(opts.UsageJSONPath, model)
	}

	g.currentSection = "propose"
	text, usage, reqErr := prefix.Request(context.Background(), suffix)
	g.logFanoutUsage(usage)
	if reqErr != nil {
		return fmt.Errorf("propose request failed: %w", reqErr)
	}

	bundle, err := parseProposalResponse(text)
	if err != nil {
		// Never lose a paid-for turn: save the raw response for manual recovery.
		_ = os.MkdirAll(opts.OutputDir, 0o755) //nolint:gosec,errcheck // best-effort recovery
		rawPath := filepath.Join(opts.OutputDir, "PROPOSAL.raw.md")
		_ = os.WriteFile(rawPath, []byte(text), 0o644) //nolint:gosec,errcheck // best-effort recovery
		return fmt.Errorf("failed to parse proposal response (raw saved to %s): %w", rawPath, err)
	}

	written, err := writeProposalBundle(opts.OutputDir, bundle)
	if err != nil {
		return err
	}
	if written.ConfigWarning != "" {
		g.logger.Warnf("proposed config did not validate (written anyway for review): %s", written.ConfigWarning)
		ulog.Warn("Proposed config invalid").Field("error", written.ConfigWarning).Emit()
	}
	g.logger.Infof("Wrote proposal bundle to %s (%s, %s, %d prompt(s))",
		opts.OutputDir, filepath.Base(written.ProposalPath), filepath.Base(written.ConfigPath), len(written.PromptPaths))
	ulog.Success("Proposal bundle written").
		Field("dir", opts.OutputDir).
		Field("prompts", len(written.PromptPaths)).
		Emit()
	return nil
}

// gatherProposeInputs collects the cache-SUFFIX material for a propose request:
// the current docgen config (read from the exact path LoadWithNotebook
// resolved), every current prompt file next to it, and the README template if
// present. Prompts live in <configDir>/prompts (true in both notebook and repo
// mode, since configDir is the docgen dir or repo docs dir respectively).
func gatherProposeInputs(packageDir string, cfg *config.DocgenConfig, configPath string) (proposeInputs, error) {
	in := proposeInputs{
		repo:       filepath.Base(filepath.Clean(packageDir)),
		configName: config.ConfigFileName,
	}

	data, err := os.ReadFile(configPath) //nolint:gosec // path from trusted config discovery
	if err != nil {
		return in, fmt.Errorf("reading current docgen config %s: %w", configPath, err)
	}
	in.configYAML = string(data)

	promptsDir := filepath.Join(filepath.Dir(configPath), "prompts")
	if entries, rerr := os.ReadDir(promptsDir); rerr == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			content, cerr := os.ReadFile(filepath.Join(promptsDir, e.Name())) //nolint:gosec // trusted notebook path
			if cerr != nil {
				continue
			}
			in.prompts = append(in.prompts, promptFile{Name: e.Name(), Content: string(content)})
		}
		sort.Slice(in.prompts, func(i, j int) bool { return in.prompts[i].Name < in.prompts[j].Name })
	}

	// README template: honor cfg.Readme.Template (repo-relative) first, then the
	// conventional README.md.tpl beside the config / in the repo docs dir.
	var readmeCandidates []string
	if cfg.Readme != nil && strings.TrimSpace(cfg.Readme.Template) != "" {
		readmeCandidates = append(readmeCandidates, filepath.Join(packageDir, cfg.Readme.Template))
	}
	readmeCandidates = append(readmeCandidates,
		filepath.Join(filepath.Dir(configPath), "README.md.tpl"),
		filepath.Join(packageDir, "docs", "README.md.tpl"),
	)
	for _, c := range readmeCandidates {
		if content, rerr := os.ReadFile(c); rerr == nil { //nolint:gosec // trusted repo/notebook path
			in.readmeName = filepath.Base(c)
			in.readmeTpl = string(content)
			break
		}
	}

	return in, nil
}

// assembleProposeSuffix builds the request SUFFIX: the standing propose
// instruction followed by the repo's current config, prompts, and README
// template, each in a labeled fenced block. This is a pure function of its
// inputs (no I/O) so it is unit-testable and so a future multi-turn chat mode
// can reuse it verbatim as its first user turn.
func assembleProposeSuffix(in proposeInputs) string {
	var b strings.Builder
	b.WriteString(ProposeInstruction)
	b.WriteString("\n\n---\n\n")
	b.WriteString(fmt.Sprintf("# Current documentation setup for `%s`\n\n", in.repo))

	b.WriteString(fmt.Sprintf("## Current %s\n\n", in.configName))
	b.WriteString("```yaml\n")
	b.WriteString(strings.TrimRight(in.configYAML, "\n"))
	b.WriteString("\n```\n\n")

	if len(in.prompts) == 0 {
		b.WriteString("## Current prompt files\n\n(none)\n\n")
	} else {
		b.WriteString("## Current prompt files\n\n")
		for _, p := range in.prompts {
			b.WriteString(fmt.Sprintf("### Prompt: `%s`\n\n", p.Name))
			b.WriteString("```markdown\n")
			b.WriteString(strings.TrimRight(p.Content, "\n"))
			b.WriteString("\n```\n\n")
		}
	}

	if in.readmeTpl != "" {
		b.WriteString(fmt.Sprintf("## Current README template (`%s`)\n\n", in.readmeName))
		b.WriteString("```\n")
		b.WriteString(strings.TrimRight(in.readmeTpl, "\n"))
		b.WriteString("\n```\n")
	}

	return b.String()
}

// parseProposalResponse splits the model's delimited response into the proposal
// bundle. It is tolerant of a missing trailing END delimiter (the last block is
// still flushed) and strips a wrapping code fence from the config and prompt
// blocks. It errors only when the response carries no recognizable structure.
func parseProposalResponse(text string) (*proposalBundle, error) {
	b := &proposalBundle{}

	const (
		stNone = iota
		stRationale
		stOutline
		stConfig
		stPrompt
		stEnd
	)
	state := stNone
	var buf []string
	var promptName string

	flush := func() {
		body := strings.Join(buf, "\n")
		switch state {
		case stRationale:
			b.Rationale = strings.TrimSpace(body)
		case stOutline:
			b.Outline = strings.TrimSpace(body)
		case stConfig:
			b.Config = stripFence(body)
		case stPrompt:
			if promptName != "" {
				b.Prompts = append(b.Prompts, promptFile{Name: promptName, Content: stripFence(body)})
			}
		}
		buf = nil
	}

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == proposeDelimRationale:
			flush()
			state = stRationale
		case trimmed == proposeDelimOutline:
			flush()
			state = stOutline
		case trimmed == proposeDelimConfig:
			flush()
			state = stConfig
		case trimmed == proposeDelimEnd:
			flush()
			state = stEnd
		case strings.HasPrefix(trimmed, proposeDelimPromptPrefix) && strings.HasSuffix(trimmed, proposeDelimPromptSuffix):
			flush()
			raw := strings.TrimSuffix(strings.TrimPrefix(trimmed, proposeDelimPromptPrefix), proposeDelimPromptSuffix)
			promptName = sanitizePromptName(raw)
			state = stPrompt
		default:
			if state != stNone && state != stEnd {
				buf = append(buf, line)
			}
		}
	}
	flush()

	if b.Rationale == "" && b.Outline == "" && b.Config == "" && len(b.Prompts) == 0 {
		return nil, fmt.Errorf("no proposal delimiters found in response (expected %s ... %s)", proposeDelimRationale, proposeDelimEnd)
	}
	if b.Config == "" {
		return nil, fmt.Errorf("proposal response missing the %s block", proposeDelimConfig)
	}
	return b, nil
}

// writeProposalBundle writes the parsed bundle to dir as PROPOSAL.md (rationale
// + outline), proposed.docgen.config.yml (validated, but written regardless for
// human review), and prompts/<name>.md for each drafted prose prompt. It never
// touches the live notebook files — the caller passes a staging/output dir.
func writeProposalBundle(dir string, b *proposalBundle) (proposalWriteResult, error) {
	var res proposalWriteResult
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // internal doc tool
		return res, fmt.Errorf("creating proposal dir: %w", err)
	}

	var pb strings.Builder
	pb.WriteString("# Docs Outline Proposal\n\n")
	pb.WriteString("## Rationale\n\n")
	if b.Rationale != "" {
		pb.WriteString(b.Rationale)
	} else {
		pb.WriteString("(none provided)")
	}
	pb.WriteString("\n\n## Proposed Outline\n\n")
	if b.Outline != "" {
		pb.WriteString(b.Outline)
	} else {
		pb.WriteString("(none provided)")
	}
	pb.WriteString("\n")
	res.ProposalPath = filepath.Join(dir, "PROPOSAL.md")
	if err := os.WriteFile(res.ProposalPath, []byte(pb.String()), 0o644); err != nil { //nolint:gosec // internal doc tool
		return res, fmt.Errorf("writing PROPOSAL.md: %w", err)
	}

	res.ConfigPath = filepath.Join(dir, "proposed.docgen.config.yml")
	if err := os.WriteFile(res.ConfigPath, []byte(ensureTrailingNewline(b.Config)), 0o644); err != nil { //nolint:gosec // internal doc tool
		return res, fmt.Errorf("writing proposed config: %w", err)
	}
	var probe config.DocgenConfig
	if err := yaml.Unmarshal([]byte(b.Config), &probe); err != nil {
		res.ConfigWarning = fmt.Sprintf("not valid YAML: %v", err)
	} else if len(probe.Sections) == 0 {
		res.ConfigWarning = "proposed config parsed but has no sections"
	}

	if len(b.Prompts) > 0 {
		promptsDir := filepath.Join(dir, "prompts")
		if err := os.MkdirAll(promptsDir, 0o755); err != nil { //nolint:gosec // internal doc tool
			return res, fmt.Errorf("creating prompts dir: %w", err)
		}
		for _, p := range b.Prompts {
			if p.Name == "" {
				continue
			}
			path := filepath.Join(promptsDir, p.Name)
			if err := os.WriteFile(path, []byte(ensureTrailingNewline(p.Content)), 0o644); err != nil { //nolint:gosec // internal doc tool
				return res, fmt.Errorf("writing prompt %s: %w", p.Name, err)
			}
			res.PromptPaths = append(res.PromptPaths, path)
		}
	}

	return res, nil
}

// stripFence removes surrounding blank lines and a single wrapping code fence
// (```lang ... ```) from a block body, leaving the inner content.
func stripFence(s string) string {
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) >= 2 &&
		strings.HasPrefix(strings.TrimSpace(lines[0]), "```") &&
		strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[1 : len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// sanitizePromptName reduces a proposed prompt name to a safe .md basename (no
// path separators, always a .md suffix), or "" when nothing usable remains.
func sanitizePromptName(name string) string {
	name = strings.TrimSpace(name)
	name = filepath.Base(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return ""
	}
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	return name
}

// ensureTrailingNewline appends a newline if s is non-empty and lacks one, so
// written bundle files are well-formed text.
func ensureTrailingNewline(s string) string {
	if s == "" || strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}
