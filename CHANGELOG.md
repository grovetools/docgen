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
