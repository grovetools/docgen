# `grove-docgen` CLI Reference

This document provides a comprehensive reference for the `grove-docgen` command-line interface. `docgen` is a tool for generating and managing documentation for projects within the Grove ecosystem using Large Language Models (LLMs).

## Global Options

These options can be used with any `docgen` command.

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

| Flag     | Description                                                  | Default   |
| :------- | :----------------------------------------------------------- | :-------- |
| `--type` | The type of project to initialize templates for. Currently, only `library` is supported. | `library` |

### Examples

**Initialize a standard library project:**

This is the most common use case. It will create the configuration and prompts suitable for a Go library.

```bash
# Run from the root of your package
docgen init
```

**Expected Output:**

```
✓ Created configuration file: docs/docgen.config.yml
✓ Created prompt file: docs/prompts/best-practices.md
✓ Created prompt file: docs/prompts/core-concepts.md
✓ Created prompt file: docs/prompts/introduction.md
✓ Created prompt file: docs/prompts/usage-patterns.md
✅ Docgen initialized successfully.
   Next steps: 1. Edit docs/docgen.config.yml to match your project.
               2. Review and customize the prompts in docs/prompts/.
               3. Run 'docgen generate' to create your documentation.
```

---

## `docgen generate`

Generates documentation for the package in the current working directory.

### Syntax

```bash
docgen generate [flags]
```

### Description

This is the core command of `docgen`. It reads the `docs/docgen.config.yml` file, builds a context from your source code, calls an LLM for each configured section, and writes the generated markdown files to the `docs/` directory.

To ensure that generated documentation doesn't pollute the context for subsequent runs (creating a feedback loop), `generate` performs its work in an isolated environment.

### The Generation Process

1.  **Isolation**: A temporary directory is created, and the current package is cloned into it using a local `git clone`.
2.  **Context Building**: The `cx generate` command is run within the isolated clone. This command analyzes your source code (based on rules in `.grove/rules` or your `docs.rules` file) to create a comprehensive context.
3.  **LLM Call**: For each section defined in `docgen.config.yml`, `docgen` combines the system prompt, the section-specific prompt, and the code context, then sends it to the configured LLM via `gemapi`.
4.  **Output Writing**: The LLM's response is saved as a markdown file in the `docs/` directory of the isolated clone.
5.  **Copy Back**: The newly generated markdown files are copied from the temporary directory back into your project's actual `docs/` directory.
6.  **JSON Generation (Optional)**: If `structured_output_file` is set in the config, `docgen` parses the generated markdown files into a structured JSON file.

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

**Generate the 'core-concepts' and 'best-practices' sections using the alias:**

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

It also creates a `manifest.json` file, which serves as an index for all the collected documentation. This manifest is designed to be consumed by a frontend application, such as `grove-website`, to build a unified documentation site.

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

**Aggregate documentation into a custom directory for a web frontend:**

```bash
docgen aggregate --output-dir ./website/public/docs
```

### Manifest Structure

The generated `manifest.json` has the following structure:

```json
{
  "generatedAt": "2023-10-27T10:00:00Z",
  "packages": [
    {
      "name": "grove-tend",
      "title": "Grove Tend",
      "category": "Core Libraries",
      "docsPath": "./grove-tend",
      "version": "v1.2.3",
      "repoURL": "https://github.com/user/grove-tend",
      "sections": [
        {
          "title": "Introduction",
          "path": "./grove-tend/introduction.md"
        },
        {
          "title": "Core Concepts",
          "path": "./grove-tend/core-concepts.md"
        }
      ]
    }
    // ... more packages
  ]
}
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

**Expected Output:**

```
docgen version main-a1b2c3d built on 2023-10-27T10:00:00Z
```

**Display version information as JSON:**

```bash
docgen version --json
```

**Expected Output:**

```json
{
  "Version": "main-a1b2c3d",
  "Commit": "a1b2c3d",
  "Branch": "main",
  "BuildDate": "2023-10-27T10:00:00Z"
}
```