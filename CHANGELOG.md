## v0.6.0 (2026-02-02)

A major workflow overhaul introduces a notebook-first approach to documentation (def60a2), supported by a new three-level publication status system—draft, dev, and production—to manage content lifecycle (d2c7001). This includes new synchronization commands to move content between notebooks and repositories (def60a2), alongside enhanced asset resolution that prioritizes notebook locations while preserving legacy paths (2f34a1f).

The new `watch` command (8d714d9) enables hot-reloading for documentation by monitoring notebook directories and incrementally rebuilding changed packages. Additionally, a new `capture` command (f74d120) allows for recursive crawling of CLI help output to generate styled documentation. Automated schema generation tools (ceb67fb) streamline the creation of configuration references.

Asset management has been expanded with a new logo text generator (308b1e1) and configuration support for generating combined logo-text SVGs (251cd98). The system now also supports a `base_url` configuration for absolute media paths (af32529), ensuring assets render correctly across different hosting environments.

### Features

- Implement notebook-first docgen workflow with sync commands (def60a2)
- Add publication status workflow for documentation (d2c7001)
- Add watch command for hot reloading documentation (8d714d9)
- Add CLI capture command for generating styled command references (f74d120)
- Add automated schema generation and improve doc_sections (ceb67fb)
- Add logos config and logo-with-text SVG generation (251cd98)
- Multi-ecosystem aggregation with website sections (f7ffee8)
- Add reference mode support for schema generation types (a379e14)
- Add doc_sections type for consolidated config reference (98eb2b9)
- Add base_url config for absolute media paths (af32529)
- Support .cx/ location for docgen rules files (24bb94b)
- Respect docgen_order in concept generator (94d7d16)
- Add docgen concept build (8851353)
- Resolve assets from notebook docgen directory (2f34a1f)
- Copy video/asciicasts (91fd370)
- Improve sync to-repo output with detailed status info (cf6c1e6)
- Support notebook location for README templates (bbf87a2)

### Bug Fixes

- Default section status to draft and refactor website sections to be config-driven (0bc165b)
- Support website sections in docgen watch (f81fa8d)
- Section status default to draft + filter packages by sidebar config (1d02ecf)
- Generate docs in nb (3795411)
- Detect notebook mode for centralized notebooks (d0ab896)
- Include dev status sections in prod mode aggregation (286ecf5)
- Don't create empty directories for filtered packages (ff4e222)
- Skip packages with no sections in aggregator (50a1ced)
- Update VERSION_PKG to grovetools/core path (1ad4980)

### Refactoring

- Use config.FindConfigFile for TOML support (8972260)
- Update docgen title to match package name (20205de)

### File Changes

```
 .cx/docs.rules                       |   8 +
 .github/workflows/release.yml        | 113 ++--
 CLAUDE.md                            |  13 +
 LICENSE                              |  21 +
 Makefile                             |  11 +-
 README.md                            |  59 +--
 cmd/aggregate.go                     |  25 +-
 cmd/capture.go                       |  85 +++
 cmd/logo.go                          |  93 ++++
 cmd/migrate_config.go                | 128 +++++
 cmd/root.go                          |   5 +
 cmd/sync.go                          |  22 +
 cmd/sync_from_repo.go                | 148 ++++++
 cmd/sync_to_repo.go                  | 250 +++++++++
 cmd/watch.go                         | 894 ++++++++++++++++++++++++++++++++
 docs/01-overview.md                  |  63 +--
 docs/06-configuration.md             | 155 ++++++
 docs/README.md.tpl                   |   6 -
 docs/docgen.config.yml               |  43 --
 docs/docs.rules                      |   1 -
 docs/prompts/01-overview.md          |  31 --
 docs/prompts/02-examples.md          |  23 -
 docs/prompts/03-configuration.md     |  62 ---
 docs/prompts/04-command-reference.md |  54 --
 go.mod                               |  31 +-
 go.sum                               | 110 +++-
 grove.toml                           |  14 +
 grove.yml                            |  10 -
 pkg/aggregator/aggregator.go         | 964 +++++++++++++++++++++++++++++++++--
 pkg/capture/capture.go               | 465 +++++++++++++++++
 pkg/config/config.go                 | 215 ++++++--
 pkg/docs/docs.json                   |  58 +--
 pkg/generator/concept.go             | 232 +++++++++
 pkg/generator/generator.go           | 645 +++++++++++++++++++++--
 pkg/logo/generator.go                | 382 ++++++++++++++
 pkg/manifest/manifest.go             |  36 +-
 pkg/readme/synchronizer.go           | 221 ++++++--
 pkg/schema/parser.go                 |  42 +-
 pkg/transformer/astro.go             | 165 ++++++
 pkg/watcher/watcher.go               | 157 ++++++
 pkg/writer/astro.go                  |  71 +++
 pkg/writer/writer.go                 |  30 ++
 schema/docgen.config.schema.json     | 610 ++++++++++++++--------
 tools/schema-generator/main.go       |  34 ++
 44 files changed, 5983 insertions(+), 822 deletions(-)
```

## v1.1.1-nightly.5b5626e (2025-10-03)

## v0.1.0 (2025-10-01)

This release introduces a major new feature for synchronizing `README.md` files from documentation source and templates. The new `sync-readme` command (2f2950e) automates the process of keeping project READMEs up-to-date by injecting content from generated documentation. This workflow supports stripping header lines to avoid duplication (9495569), automatic table of contents generation (ff6af83), and path rewriting for both Markdown and HTML image tags to ensure they render correctly on sites like GitHub (0a69818, 57a8822). A fix was also included to prevent broken images from appearing in the generated TOC (e4cc9e5).

A new `agg_strip_lines` configuration option has been added, allowing for the removal of a specified number of lines from a section's content during the documentation aggregation process (a7643d2). The `regen-json` command has been improved; its parser now dynamically adapts to the section names defined in `docgen.config.yml` instead of relying on hardcoded values, fixing a major bug where it would generate empty files (6deaf54). The documentation content and structure has also been extensively refactored for clarity and consistency across the ecosystem (700f504, 816adf8, c045a4b).

### Features

- Add `sync-readme` command to generate README.md from templates and documentation source (2f2950e)
- Add `strip_lines` option for README synchronization to prevent duplicate headings (9495569)
- Add `agg_strip_lines` config option for per-section content stripping during aggregation (a7643d2)
- Add image support for copying `docs/images` during aggregation and rewriting paths (0a69818)
- Update the default system prompt for documentation generation (cc98a41)
- Update documentation prompts for clarity and focus (137a876)

### Bug Fixes

- Make `regen-json` parser dynamically adapt to section names in `docgen.config.yml` (6deaf54)
- Handle HTML `<img>` tags in `sync-readme` path rewriting (57a8822)
- Remove broken image descriptions from table of contents generation (e4cc9e5)
- Update CI workflow to use `branches: [ none ]` for disabling workflows (5d5087c)
- Clean up formatting in the `README.md.tpl` template (44da94d)

### Refactoring

- Standardize `docgen.config.yml` key order and default settings (390168a)

### Documentation

- Update docgen configuration and add table of contents support to README template (ff6af83)
- Simplify documentation structure to four essential sections (700f504)
- Rename "Introduction" sections to "Overview" for consistency (816adf8)
- Simplify installation instructions to point to the main Grove guide (9be70a2)
- Update docgen config and overview prompt (f79adc1)

### Chore

- Standardize documentation filenames to `DD-name.md` convention (c045a4b)
- Temporarily disable CI workflow (0760764)

### CI

- Remove redundant tests from release workflow (6fc0cbc)

### File Changes

```
 .github/workflows/ci.yml                           |   4 +-
 .github/workflows/release.yml                      |   3 +-
 CHANGELOG.md                                       |   2 +-
 Makefile                                           |   2 +
 README.md                                          |  94 +++----
 cmd/recipe.go                                      |   7 +
 cmd/root.go                                        |   1 +
 cmd/sync_readme.go                                 |  66 +++++
 docs/01-overview.md                                |  46 ++++
 docs/02-examples.md                                | 183 +++++++++++++
 docs/03-configuration.md                           | 151 +++++++++++
 docs/04-command-reference.md                       | 195 +++++++++++++
 docs/README.md.tpl                                 |   6 +
 docs/cli-reference.md                              | 271 -------------------
 docs/configuration.md                              | 270 ------------------
 docs/docgen.config.yml                             |  66 +++--
 docs/docs.rules                                    |  33 +--
 docs/getting-started.md                            | 195 -------------
 docs/introduction.md                               |  24 --
 docs/prompts/01-overview.md                        |  31 +++
 docs/prompts/02-examples.md                        |  23 ++
 docs/prompts/03-configuration.md                   |  62 +++++
 .../{cli-reference.md => 04-command-reference.md}  |   0
 docs/prompts/configuration.md                      |  50 ----
 docs/prompts/getting-started.md                    |  38 ---
 docs/prompts/introduction.md                       |  21 --
 docs/prompts/usage-patterns.md                     |  52 ----
 docs/usage-patterns.md                             | 200 --------------
 internal/scaffold/scaffold.go                      |  21 +-
 .../scaffold/templates/library/docgen.config.yml   |   7 +
 .../scaffold/templates/library/docs/README.md.tpl  |  26 ++
 pkg/aggregator/aggregator.go                       |  36 ++-
 pkg/config/config.go                               |  11 +
 pkg/docs/docs.json                                 | 118 ++++++++
 pkg/generator/system_prompt.go                     |  95 ++++---
 pkg/parser/parser.go                               | 173 +++++-------
 pkg/readme/synchronizer.go                         | 301 +++++++++++++++++++++
 pkg/recipes/builtin.go                             |   5 +-
 .../builtin/add-readme-template/01-implement.md    | 211 +++++++++++++++
 schema/docgen.config.schema.json                   |  45 +++
 40 files changed, 1745 insertions(+), 1400 deletions(-)
```

## v1.0.0 (2025-09-26)

This release introduces `grove-docgen`, an LLM-powered, workspace-aware documentation generator for the Grove ecosystem. The tool includes new commands to streamline the documentation process, such as `docgen init` for scaffolding new projects with configurable templates (d929db2, 68664db), `docgen regen-json` for updating structured output without calling LLMs (502b05a), and `docgen generate` for creating documentation for a single package (1c648e6).

A key feature is the new `docgen customize` command, which provides deep integration with `grove-flow` for creating interactive documentation plans (a55d718). This is powered by a dynamic recipe provider that serves recipes directly from the `docgen` binary (7a7fd40), and supports multiple recipe types like `agent` and `prompts` for different customization workflows (652e699). The system has also been refactored to use the `grove llm` facade, making it provider-agnostic (bc3925e).

Documentation generation is now highly configurable. Users can set global and per-section generation parameters like temperature and max tokens, and even override the model for specific sections (2c0191a). The `aggregate` command has been enhanced to automatically include `CHANGELOG.md` files (fe2ed10) and use prompt files as placeholders when generated documentation is missing, improving its utility during development (2040f24).

### Features

- Implement `grove-docgen` LLM-powered documentation generator (1c648e6)
- Add scaffolding, system prompts, and improved documentation workflow (d929db2)
- Add generation parameters and selective section generation (2c0191a)
- Add self-documentation configuration for grove-docgen (4e8fffb)
- Add `generate-docs` Makefile recipe (28c51ce)
- Add `regen-json` command for regenerating structured JSON output (502b05a)
- Add standardized documentation output to scaffold template (52da8f2)
- Implement `docgen-customize` feature and local generation (a55d718)
- Add recipe provider for `grove-flow` integration (7a7fd40)
- Add changelog aggregation support (fe2ed10)
- Add support for multiple recipe types in `customize` command (652e699)
- Enhance documentation generation with structured logging (b11d4a2)
- Update release workflow to use CHANGELOG.md (04c2e7f)
- Auto-create rules file when using `--rules-file` flag in init (68664db)
- Use prompt files as placeholders in aggregate when docs are missing (2040f24)
- Update project documentation (0828453)

### Bug Fixes

- Add version/repo helpers and make LLM model configurable (d35b385)
- Ensure output directory exists before saving manifest (dc7a9fa)
- Simplify default rules file to just contain '*' (ce11a2e)
- Add rules_file to frontmatter and use correct path in recipes (0bae005)

### Refactoring

- Migrate `grove-docgen` to use `grove llm` facade (bc3925e)
- Replace `cli.GetLogger` with `logging.NewLogger` (8cc8205)

### File Changes

```
 .github/workflows/ci.yml                           |  10 +-
 .github/workflows/release.yml                      |  10 +-
 Makefile                                           |  37 +--
 README.md                                          |  50 ++-
 cmd/aggregate.go                                   |  22 ++
 cmd/customize.go                                   | 248 +++++++++++++++
 cmd/generate.go                                    |  46 +++
 cmd/init.go                                        |  46 +++
 cmd/loggers.go                                     |  16 +
 cmd/recipe.go                                      |  58 ++++
 cmd/regen_json.go                                  |  44 +++
 cmd/root.go                                        |   6 +
 docs/cli-reference.md                              | 271 ++++++++++++++++
 docs/configuration.md                              | 270 ++++++++++++++++
 docs/docgen.config.yml                             |  51 +++
 docs/docs.rules                                    |  32 ++
 docs/getting-started.md                            | 195 ++++++++++++
 docs/introduction.md                               |  24 ++
 docs/prompts/cli-reference.md                      |  54 ++++
 docs/prompts/configuration.md                      |  50 +++
 docs/prompts/getting-started.md                    |  38 +++
 docs/prompts/introduction.md                       |  21 ++
 docs/prompts/usage-patterns.md                     |  52 ++++
 docs/usage-patterns.md                             | 200 ++++++++++++
 go.mod                                             |  18 +-
 go.sum                                             |  44 +++
 internal/scaffold/scaffold.go                      | 181 +++++++++++
 .../scaffold/templates/library/docgen.config.yml   |  44 +++
 .../templates/library/prompts/best-practices.md    |  20 ++
 .../templates/library/prompts/core-concepts.md     |  19 ++
 .../templates/library/prompts/introduction.md      |  17 +
 .../templates/library/prompts/usage-patterns.md    |  19 ++
 pkg/aggregator/aggregator.go                       | 260 ++++++++++++++++
 pkg/config/config.go                               | 111 +++++++
 pkg/generator/generator.go                         | 344 +++++++++++++++++++++
 pkg/generator/system_prompt.go                     |  51 +++
 pkg/manifest/manifest.go                           |  45 +++
 pkg/parser/parser.go                               | 281 +++++++++++++++++
 pkg/recipes/builtin.go                             |   9 +
 .../docgen-customize-agent/01-customize-docs.md    |  44 +++
 .../docgen-customize-agent/02-generate-docs.md     |  45 +++
 .../docgen-customize-prompts/01-customize-docs.md  |  43 +++
 .../docgen-customize-prompts/02-generate-docs.md   |  14 +
 pkg/recipes/types.go                               |  10 +
 schema/docgen.config.schema.json                   | 209 +++++++++++++
 tests/e2e/main.go                                  |  24 --
 tests/e2e/scenarios_basic.go                       |  49 ---
 tests/e2e/test_utils.go                            |  45 ---
 48 files changed, 3629 insertions(+), 168 deletions(-)
```

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial implementation of grove-docgen
- Basic command structure
- E2E test framework
