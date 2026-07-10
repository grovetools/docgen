package generator

// Machine-splittable delimiters for the propose response. The model is asked to
// emit exactly these header lines (each on its own line, no surrounding text) so
// parseProposalResponse can split the response deterministically instead of
// guessing at prose boundaries. They are deliberately unlikely to collide with
// real markdown or YAML content.
const (
	proposeDelimRationale = "===GROVE-PROPOSAL-RATIONALE==="
	proposeDelimOutline   = "===GROVE-PROPOSAL-OUTLINE==="
	proposeDelimConfig    = "===GROVE-PROPOSAL-CONFIG==="
	// proposeDelimPromptPrefix begins a per-prompt block; the section name
	// follows after "name=", e.g. "===GROVE-PROPOSAL-PROMPT name=01-overview===".
	proposeDelimPromptPrefix = "===GROVE-PROPOSAL-PROMPT name="
	proposeDelimPromptSuffix = "==="
	proposeDelimEnd          = "===GROVE-PROPOSAL-END==="
)

// ProposeInstruction is the standing instruction block that leads the propose
// request SUFFIX (everything AFTER the cached cx-context prefix). It is a
// package-level constant, factored out of the suffix assembler, so a future
// multi-turn "docs chat" mode can reuse the exact same turn-0 instruction
// verbatim while riding the same cached prefix. The current config, prompts, and
// README template are appended after this block by assembleProposeSuffix.
//
// The instruction never restates the repo source — that is the cached prefix the
// request rides on. It asks only for a proposed docs OUTLINE and draft prompts,
// emitted in the delimited format above so the bundle can be split and written
// without an LLM in the loop.
const ProposeInstruction = `You are proposing an updated documentation OUTLINE for this repository.

The repository's source code and context are provided ABOVE this message as a
cached prefix. Below, after this instruction, you are given the repository's
CURRENT docgen configuration, its CURRENT per-section prompt files, and (if
present) its README template. These describe how the docs are generated today.

Your job: propose how the documentation should be organized and prompted GOING
FORWARD, given what the code actually does now. This is a review artifact — a
human will read, edit, and later feed the approved outline back into doc
generation. Do not write the documentation itself; propose the structure and the
prompts that would generate it.

Produce three things:

1. An updated SECTION LIST. For every section give: order, name (a stable slug),
   title, and type. Valid types are: prose (LLM-written narrative from a prompt),
   capture (CLI --help capture), schema_to_md / schema_table / schema_describe /
   schema_examples (config-schema docs), tui_keymaps / tui_describe (TUI docs).
   For each section note whether it is KEPT, ADDED, REMOVED, or MERGED versus the
   current config, with a one-line reason. Prefer evolving the current list over
   replacing it; only add/remove/merge where the code justifies it.

2. A full draft PROMPT for every PROSE section (only prose sections need a
   prompt). Write each as a content OUTLINE that instructs the doc writer what to
   cover, in what order, with what emphasis — the same house style as the current
   prompt files shown below (a titled brief, an ordered list of the subsections
   to produce, and notes on tone and what to include or omit). Do not write the
   final prose; write the instructions that would produce it.

3. A short overall RATIONALE (a few sentences) explaining the shape of the
   proposed outline and the most significant changes.

Return your answer in EXACTLY this delimited format, with each delimiter on its
own line and nothing else on that line. Emit the sections in this order:

` + proposeDelimRationale + `
<the overall rationale prose>
` + proposeDelimOutline + `
| Order | Name | Title | Type | Change | Reason |
| ----- | ---- | ----- | ---- | ------ | ------ |
<one row per proposed section; Change is KEPT/ADDED/REMOVED/MERGED>
` + proposeDelimConfig + `
` + "```yaml" + `
<a COMPLETE, valid docgen.config.yml: preserve the current settings block
verbatim, and replace the sections list with your proposed sections. Every prose
section's prompt: field must point at the corresponding drafted prompt filename
below (e.g. prompt: 01-overview.md).>
` + "```" + `
` + proposeDelimPromptPrefix + `01-overview` + proposeDelimPromptSuffix + `
` + "```markdown" + `
<the full draft prompt for the section named 01-overview>
` + "```" + `
<repeat a ` + `===GROVE-PROPOSAL-PROMPT name=<slug>===` + ` block for every prose section>
` + proposeDelimEnd + `

Rules:
- Emit the delimiters literally and exactly; do not add commentary outside the
  blocks.
- The name in each PROMPT block header must match a prose section's name and its
  config prompt: filename stem.
- Provide a prompt block for EVERY prose section and for no non-prose section.
` + proposeSchemaFieldsRule + proposeOutputFieldRule + proposeCaptureFieldsRule

// proposeSchemaFieldsRule is appended to both instruction variants. Models have
// invented unsupported keys (filter/invert_filter) on schemas: entries; the
// docgen config's SchemaInput has ONLY path and title, so spelling that out
// keeps the proposed config valid.
const proposeSchemaFieldsRule = `- Each entry under a section's ` + "`schemas:`" + ` list supports ONLY two fields:
  ` + "`path`" + ` (the schema file) and ` + "`title`" + ` (the H2 heading). Do not add any
  other keys (there is no filter, invert_filter, or similar field).
- Every ` + "`schemas:`" + ` ` + "`path`" + ` must point at an EXISTING generated ` + "`.schema.json`" + `
  artifact in the repository — NEVER a ` + "`.go`" + ` source file. If the repository has
  no generated schema artifact, do not propose schema_* sections.
- ` + "`schema_describe`" + ` (producer: writes the descriptions JSON named by its
  ` + "`output:`" + `) and ` + "`schema_table`" + ` (consumer: references that exact file via
  ` + "`descriptions:`" + ` and renders the .md page) are SEPARATE sections working as a
  pair — never collapse them into one section.
`

// proposeOutputFieldRule is appended to both instruction variants. A --fresh
// proposal once emitted sections with NO output: field; applying that config and
// running `docgen generate` spent an LLM call per section and THEN failed the
// write ("open .../docs: is a directory") because an empty output: joined onto
// the output dir resolves to the dir itself. Spelling out the requirement keeps
// every proposed section writable.
const proposeOutputFieldRule = `- EVERY section MUST set an explicit ` + "`output:`" + ` filename (the file written under
  the docs output dir), e.g. ` + "`output: 01-overview.md`" + `. A section with no output:
  cannot be written and wastes its generation.
- Rendered doc sections use ` + "`.md`" + `; a ` + "`schema_describe`" + ` section's output is a data
  file conventionally named ` + "`<repo>.descriptions.json`" + `, and any ` + "`schema_table`" + `
  section that consumes it via ` + "`descriptions:`" + ` must reference that exact filename.
`

// proposeCaptureFieldsRule is appended to both instruction variants. A live
// --fresh run invented a `command:` field on capture sections (with no example
// config in the suffix, the model guessed at the shape); docgen's capture
// section takes `binary:` — a CLI binary name whose --help tree is crawled —
// and there is no command/args field.
const proposeCaptureFieldsRule = `- A ` + "`capture`" + ` section MUST set ` + "`binary:`" + ` — the name of an installed CLI binary
  whose --help output is crawled (e.g. ` + "`binary: notify`" + `). There is NO ` + "`command:`" + `,
  ` + "`args:`" + `, or similar field; one capture section covers the whole binary. Optional
  ` + "`format:`" + ` is styled (default) or plain.
`

// FreshProposeInstruction is the green-field variant of ProposeInstruction used
// by `docgen propose --fresh`. It proposes a documentation outline derived
// from the repository's code context ALONE — the request SUFFIX carries only the
// current settings block (sections withheld, no current prompts, no README
// template), so the model is not anchored to the existing outline. The delimited
// output format and delimiters are IDENTICAL to ProposeInstruction (the parser
// is shared); only the framing and the outline table's columns differ (no
// KEPT/ADDED/REMOVED/MERGED change column, since there is no current list to
// diff against).
const FreshProposeInstruction = `You are proposing a documentation OUTLINE for this repository FROM SCRATCH.

The repository's source code and context are provided ABOVE this message as a
cached prefix. Below, after this instruction, you are given only the
repository's CURRENT docgen settings block (its section list has been withheld
on purpose). Design the documentation outline from the code alone — do not try
to reconstruct or evolve any prior outline; propose the structure the code
actually warrants today.

This is a review artifact — a human will read, edit, and later feed the approved
outline back into doc generation. Do not write the documentation itself; propose
the structure and the prompts that would generate it.

Produce three things:

1. A SECTION LIST. For every section give: order, name (a stable slug), title,
   and type. Valid types are: prose (LLM-written narrative from a prompt),
   capture (CLI --help capture), schema_to_md / schema_table / schema_describe /
   schema_examples (config-schema docs), tui_keymaps / tui_describe (TUI docs).
   Give a one-line reason for each section.

2. A full draft PROMPT for every PROSE section (only prose sections need a
   prompt). Write each as a content OUTLINE that instructs the doc writer what to
   cover, in what order, with what emphasis (a titled brief, an ordered list of
   the subsections to produce, and notes on tone and what to include or omit). Do
   not write the final prose; write the instructions that would produce it.

3. A short overall RATIONALE (a few sentences) explaining the shape of the
   proposed outline.

Return your answer in EXACTLY this delimited format, with each delimiter on its
own line and nothing else on that line. Emit the sections in this order:

` + proposeDelimRationale + `
<the overall rationale prose>
` + proposeDelimOutline + `
| Order | Name | Title | Type | Reason |
| ----- | ---- | ----- | ---- | ------ |
<one row per proposed section>
` + proposeDelimConfig + `
` + "```yaml" + `
<a COMPLETE, valid docgen.config.yml: preserve the current settings block
verbatim, and provide your proposed sections list. Every prose section's prompt:
field must point at the corresponding drafted prompt filename below (e.g.
prompt: 01-overview.md).>
` + "```" + `
` + proposeDelimPromptPrefix + `01-overview` + proposeDelimPromptSuffix + `
` + "```markdown" + `
<the full draft prompt for the section named 01-overview>
` + "```" + `
<repeat a ` + `===GROVE-PROPOSAL-PROMPT name=<slug>===` + ` block for every prose section>
` + proposeDelimEnd + `

Rules:
- Emit the delimiters literally and exactly; do not add commentary outside the
  blocks.
- The name in each PROMPT block header must match a prose section's name and its
  config prompt: filename stem.
- Provide a prompt block for EVERY prose section and for no non-prose section.
` + proposeSchemaFieldsRule + proposeOutputFieldRule + proposeCaptureFieldsRule
