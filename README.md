# Grove Docgen

AI-powered documentation generator for Grove ecosystem projects

## Overview

<!-- DOCGEN:INTRODUCTION:START -->

# Introduction to grove-docgen

`grove-docgen` is an LLM-powered, workspace-aware documentation generator designed for the Grove ecosystem. It automates the creation of documentation by analyzing source code and applying technical writing principles. Its purpose is to solve the persistent challenge of keeping documentation comprehensive, accurate, and synchronized with an evolving codebase.

## The "Documentation as Code" Philosophy

Traditional documentation workflows often fail because documentation is treated as an artifact separate from the code it describes. This separation leads to staleness and neglect. `grove-docgen` addresses this by adopting a "Documentation from Code" philosophy, where the entire generation process is defined by configuration and prompt files that live alongside the source code.

## How It Works: An AI-Assisted Workflow

The `docgen generate` command orchestrates a multi-step process to create documentation for a package directly within its directory:

1.  **Configuration Loading:** It reads a `docs/docgen.config.yml` file to understand the project's metadata and the specific sections of documentation to generate (e.g., Introduction, Core Concepts, Best Practices).
2.  **Context Gathering:** It leverages Grove's context-aware tool, `cx`, to scan the codebase. Based on rules defined in a `docs.rules` file, `cx` gathers the most relevant code snippets, file structures, and other artifacts to create a rich, code-aware context.
3.  **LLM-Powered Generation:** For each section, `docgen` combines a user-written prompt with the context gathered by `cx` and sends it to an LLM, such as Google's Gemini. The LLM analyzes the code context and the prompt's instructions to write the documentation section in Markdown.
4.  **Output and Aggregation:** The generated Markdown files are written to the project's `docs` directory. The companion `docgen aggregate` command can then discover all `docgen`-enabled packages in a workspace, collect their documentation, and assemble it into a unified output with a `manifest.json` for consumption by a frontend site.

## Key Differentiators

`grove-docgen` stands apart from traditional documentation tools and generic AI assistants in several ways:

-   **Deep Code Awareness:** By integrating with `cx`, `docgen` provides the LLM with a highly relevant and curated context from the source code. This results in documentation that is technically precise and grounded in the actual implementation, rather than generic explanations.
-   **Structured and Repeatable:** The entire process is driven by version-controlled configuration, eliminating manual steps and ensuring that documentation is consistent and reproducible.
-   **Iterative Refinement:** `grove-docgen` supports a "reference" mode for regeneration. Instead of starting from scratch, it can provide the existing documentation to the LLM as context, instructing it to refine, update, or expand upon the current version. This makes maintaining documentation as the code evolves a manageable process.

<!-- DOCGEN:INTRODUCTION:END -->

## Installation

```bash
grove install grove-docgen
```

## Quick Start

```bash
# Initialize docgen in your project
docgen init library

# Generate documentation
make generate-docs
```

## License

MIT