# Command Reference

This document provides a comprehensive reference for all `docgen` commands, organized by function.

## Core Commands

These commands form the primary workflow for initializing, generating, and managing documentation.

### docgen init

Initializes a new documentation generation configuration for the current project.

-   **Usage**: `docgen init [flags]`
-   **Description**: Creates a `docs/` directory with a default `docgen.config.yml` file and a set of starter prompt files in `docs/prompts/`. This command sets up the necessary structure to begin generating documentation. It will not overwrite existing files.
-   **Flags**:

| Flag | Description | Default |
| :--- | :--- | :--- |
| `--type` | The type of project to initialize. Currently, only `library` is supported. | `library` |
| `--model` | The default LLM model to use for generation (e.g., `gemini-1.5-flash-latest`). | (none) |
| `--regeneration-mode` | The regeneration mode: `scratch` or `reference`. | (none) |
| `--rules-file` | The name of the rules file for context generation (e.g., `docs.rules`). | (none) |
| `--structured-output-file` | The path for the structured JSON output file. | (none) |
| `--system-prompt` | The system prompt to use: `default` or a path to a custom file. | (none) |
| `--output-dir` | The output directory for generated documentation. | (none) |

-   **Examples**:
    ```bash
    # Initialize with default settings
    docgen init

    # Initialize and specify a default model and rules file
    docgen init --model gemini-2.5-pro --rules-file docs.rules
    ```

---

### docgen generate

Generates documentation for the current package based on its configuration.

-   **Usage**: `docgen generate [flags]`
-   **Description**: Reads the `docs/docgen.config.yml` file, builds the necessary context using `grove-context`, calls an LLM for each configured section, and writes the generated Markdown files to the specified output directory.
-   **Flags**:

| Flag | Shorthand | Description |
| :--- | :--- | :--- |
| `--section` | `-s` | Generate only the specified sections by name. Can be used multiple times. |

-   **Examples**:
    ```bash
    # Generate all documentation sections defined in the config
    docgen generate

    # Generate only the 'overview' section
    docgen generate --section overview

    # Generate the 'overview' and 'examples' sections
    docgen generate -s overview -s examples
    ```

---

### docgen aggregate

Discovers all `docgen`-enabled packages in a workspace and aggregates their documentation into a single output directory.

-   **Usage**: `docgen aggregate [flags]`
-   **Description**: This command is designed for monorepos. It scans the workspace for packages with an enabled `docgen.config.yml`, copies their final documentation into a unified directory, and creates a `manifest.json` file that describes all the collected documentation, which is useful for static site generators.
-   **Flags**:

| Flag | Shorthand | Description | Default |
| :--- | :--- | :--- | :--- |
| `--output-dir` | `-o` | The directory to save the aggregated documentation. | `dist` |

-   **Examples**:
    ```bash
    # Aggregate documentation from all workspace packages into the ./dist directory
    docgen aggregate

    # Specify a different output directory
    docgen aggregate --output-dir ./public/docs
    ```

---

### docgen sync-readme

Generates the `README.md` from a template and a source documentation file.

-   **Usage**: `docgen sync-readme [flags]`
-   **Description**: Synchronizes the project's `README.md` based on the `readme` configuration in `docs/docgen.config.yml`. It reads a template file, injects a specified documentation section into it, and writes the result to the output `README.md` file.
-   **Flags**:

| Flag | Description |
| :--- | :--- |
| `--generate-source` | Generate the source documentation section before syncing the README. |

-   **Examples**:
    ```bash
    # Sync the README using the existing documentation
    docgen sync-readme

    # Regenerate the overview section first, then sync the README
    docgen sync-readme --generate-source
    ```

---

## Advanced Commands

These commands provide more specialized functionality for customization and maintenance.

### docgen customize

Creates a `grove-flow` plan for interactively customizing and generating documentation.

-   **Usage**: `docgen customize [flags]`
-   **Description**: This command bridges `docgen` with `grove-flow` to create a guided, interactive workflow for documentation generation. It reads your `docgen.config.yml`, creates a new `grove-flow` plan, and passes your configuration to the plan as variables. This allows you to use AI agents or structured prompts to refine your documentation before final generation.
-   **Prerequisites**: The `flow` command must be installed and available in your `PATH`.
-   **Flags**:

| Flag | Shorthand | Description | Default |
| :--- | :--- | :--- | :--- |
| `--recipe-type` | `-r` | The recipe to use: `agent` for an interactive AI agent, or `prompts` for a structured prompt-based flow. | `agent` |

-   **Examples**:
    ```bash
    # Create a customization plan using the default 'agent' recipe
    docgen customize

    # Create a plan using the 'prompts' recipe
    docgen customize --recipe-type prompts

    # After creating the plan, run it with grove-flow
    flow plan run
    ```

---

### docgen regen-json

Regenerates the structured JSON output from existing Markdown files.

-   **Usage**: `docgen regen-json`
-   **Description**: If you have configured a `structured_output_file` in your `docgen.config.yml`, this command will re-parse your existing generated Markdown files and update the JSON output. It does not call any LLMs or modify the Markdown files, making it a fast way to update the structured data if you've made manual edits or if the parsing logic has changed.
-   **Arguments**: None
-   **Flags**: None
-   **Examples**:
    ```bash
    # Regenerate the JSON output based on the current state of the markdown files
    docgen regen-json
    ```

---

### docgen recipe

Manages and displays documentation recipes for use with `grove-flow`.

-   **Usage**: `docgen recipe [subcommand]`
-   **Description**: This is a parent command for working with `docgen` recipes.
-   **Subcommands**:
    -   **`print`**: Prints all available `docgen` recipes in a JSON format that is consumable by `grove-flow`. This is used internally by the `docgen customize` command.
        -   **Usage**: `docgen recipe print`
        -   **Example**:
            ```bash
            # Print available recipes to stdout
            docgen recipe print
            ```

---

## Utility Commands

### docgen version

Prints the version information for the `docgen` binary.

-   **Usage**: `docgen version [flags]`
-   **Description**: Displays the version, commit hash, and build date of the installed `docgen` command.
-   **Flags**:

| Flag | Description |
| :--- | :--- |
| `--json` | Output the version information in JSON format. |

-   **Examples**:
    ```bash
    # Display version information in a human-readable format
    docgen version

    # Get version information as JSON for scripting
    docgen version --json
    ```