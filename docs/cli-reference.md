# `grove-docgen` CLI Reference

This document provides a comprehensive reference for the `grove-docgen` command-line interface. `docgen` is a tool for generating and managing documentation for projects within the Grove ecosystem using Large Language Models (LLMs).

## Global Options

These options are provided by the Grove CLI framework and can be used with any `docgen` command.

| Flag      | Description                               | Default |
| :-------- | :---------------------------------------- | :------ |
| `--help`  | Show help for any command.                | `false` |
| `--verbose`| Enable verbose logging for debugging.     | `false` |

---

## `docgen init`

Initializes a new documentation configuration for the current package by creating a `docs` directory with a configuration file and starter prompts.

### Syntax

```bash
docgen init [flags]
```

### Description

The `init` command scaffolds the necessary files to start using `docgen`. It creates a `docs/docgen.config.yml` file and a set of default prompt templates in `docs/prompts/`. This provides a starting point that you can customize for your project's specific needs. The command will not overwrite any existing files.

### Options

| Flag                       | Description                                                              | Default   |
| :------------------------- | :----------------------------------------------------------------------- | :-------- |
| `--type`                   | The type of project to initialize templates for. Currently, only `library` is supported. | `library` |
| `--model`                  | LLM model to use for generation (e.g., `gemini-1.5-flash-latest`).       | `""`      |
| `--regeneration-mode`      | Regeneration mode: `scratch` or `reference`.                             | `""`      |
| `--rules-file`             | Rules file for context generation (e.g., `docs.rules`).                  | `""`      |
| `--structured-output-file` | Path for structured JSON output (e.g., `pkg/docs/docs.json`).            | `""`      |
| `--system-prompt`          | System prompt: `default` or path to a custom prompt file.                | `""`      |
| `--output-dir`             | Output directory for generated documentation (e.g., `generated-docs`).   | `""`      |

### Examples

**Initialize a standard library project:**
This is the most common use case. It will create the configuration and prompts suitable for a Go library.
```bash
# Run from the root of your package
docgen init
```

**Initialize with a specific model and rules file:**
```bash
docgen init --model gemini-1.5-pro-latest --rules-file custom.rules
```

---

## `docgen generate`

Generates documentation for the package in the current working directory.

### Syntax

```bash
docgen generate [flags]
```

### Description

This is the core command of `docgen`. It reads the `docs/docgen.config.yml` file, builds a context from your source code, calls an LLM for each configured section, and writes the generated markdown files to the specified output directory (defaulting to `docs/`).

### The Generation Process

1.  **Load Configuration**: Reads `docs/docgen.config.yml` to determine which sections to generate and what settings to use.
2.  **Context Building**: Runs the `cx generate` command in the current directory. This command analyzes your source code (based on rules in your specified `rules_file`) to create a comprehensive context.
3.  **LLM Call**: For each section, `docgen` combines the system prompt, the section-specific prompt, and the code context, then sends it to the configured LLM via the `grove llm request` command.
4.  **Output Writing**: The LLM's response is saved as a markdown file in the configured output directory.
5.  **JSON Generation (Optional)**: If `structured_output_file` is set in the config, `docgen` parses the newly generated markdown files into a structured JSON file.

### Options

| Flag             | Alias | Description                                                  | Default |
| :--------------- | :---- | :----------------------------------------------------------- | :------ |
| `--section <name>` | `-s`  | Generate only the specified section(s). This flag can be used multiple times. The `<name>` corresponds to the `name` field in the config file. | `(all)` |

### Examples

**Generate all documentation sections:**
```bash
# Run from the root of your package
docgen generate
```

**Generate only the introduction:**
```bash
docgen generate --section introduction
```

**Generate the 'core-concepts' and 'best-practices' sections:**
```bash
docgen generate -s core-concepts -s best-practices
```

---

## `docgen aggregate`

Discovers all packages in the workspace, collects their generated documentation, and aggregates it into a single output directory.

### Syntax

```bash
docgen aggregate [flags]
```

### Description

The `aggregate` command is designed for monorepo setups. It scans your entire Grove workspace for packages that have `docgen` enabled. For each enabled package, it copies the *pre-generated* documentation files into a structured output directory.

Crucially, **this command does not generate documentation**; it only collects what has already been created by `docgen generate`.

It also creates a `manifest.json` file, which serves as an index for all the collected documentation. This manifest is designed to be consumed by a frontend application to build a unified documentation site.

### Options

| Flag           | Alias | Description                               | Default |
| :------------- | :---- | :---------------------------------------- | :------ |
| `--output-dir` | `-o`  | Directory to save the aggregated documentation. | `dist`  |

### Examples

**Aggregate all workspace documentation into the default `dist` directory:**
```bash
# Run from anywhere in the Grove workspace
docgen aggregate
```

**Aggregate documentation into a custom directory:**
```bash
docgen aggregate --output-dir ./website/public/docs
```

---

## `docgen customize`

Creates a Grove Flow plan for interactively customizing and generating documentation.

### Syntax

```bash
docgen customize [flags]
```

### Description

This command integrates `docgen` with `grove-flow` to provide an interactive, guided process for customizing your documentation. It reads your `docgen.config.yml` and generates a `grove-flow` plan based on a chosen recipe. This allows for more sophisticated, multi-step documentation workflows, such as using AI agents to refine prompts before generation.

**Prerequisites:**
- Run `docgen init` first to create a `docgen.config.yml` file.
- The `flow` command must be available in your `PATH`.

### Options

| Flag            | Alias | Description                                     | Default |
| :-------------- | :---- | :---------------------------------------------- | :------ |
| `--recipe-type` | `-r`  | Recipe to use: `agent` or `prompts`.            | `agent` |

### Examples

**Create a customization plan using the default 'agent' recipe:**
```bash
docgen customize
```

**Create a plan using the 'prompts' recipe:**
```bash
docgen customize --recipe-type prompts
```

After running the command, a new plan will be created in the `plans/` directory. You can then start the interactive process by running `flow run`.

---

## `docgen regen-json`

Regenerates the structured JSON output file from existing markdown files.

### Syntax

```bash
docgen regen-json
```

### Description

This is a utility command for quickly updating the structured JSON output without running a full `generate` command. It reads the `docs/docgen.config.yml` file, parses the existing markdown documentation files, and overwrites the JSON file specified in `structured_output_file`.

This is useful after manually editing generated markdown files or when the JSON parsing logic in `docgen` has been updated. This command **does not** call any LLMs or modify your markdown files.

### Options

This command has no specific options.

### Example

**Regenerate the JSON output for the current package:**
```bash
# Run from the root of your package
docgen regen-json
```

---

## `docgen recipe`

Manages and displays documentation recipes for `grove-flow` integration.

### Syntax

```bash
docgen recipe [subcommand]
```

### Description

This command group is primarily for internal use by `grove-flow`.

#### `docgen recipe print`

Prints all available documentation recipes in a JSON format suitable for consumption by `grove-flow`. Most users will not need to run this command directly.

### Syntax

```bash
docgen recipe print
```

---

## `docgen version`

Prints the version information for the `docgen` binary.

### Syntax

```bash
docgen version [flags]
```

### Description

Displays the build and version details for the `docgen` executable, including the Git commit, branch, and build date. This is useful for bug reporting and ensuring you are using the correct version.

### Options

| Flag   | Description                               | Default |
| :----- | :---------------------------------------- | :------ |
| `--json` | Output version information in JSON format. | `false` |

### Examples

**Display standard version information:**
```bash
docgen version
```

**Display version information as JSON:**
```bash
docgen version --json
```