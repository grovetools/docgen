# Getting Started with grove-docgen

This guide provides a step-by-step walkthrough to get you from installation to generating your first set of documentation using `grove-docgen`.

## 1. Prerequisites

Before you begin, ensure your development environment meets the following requirements:

-   **Grove Ecosystem**: You must have the Grove development environment set up, and your project must be part of a Grove workspace.
-   **Required Tools**: `grove-docgen` relies on other Grove tools. Make sure `grove` and `cx` (for context building) are installed and available in your system's `PATH`.
-   **Git Repository**: Your project should be a Git repository. This is used for gathering metadata like versioning and repository URLs during the aggregation step.

## 2. Installation

The recommended way to install `grove-docgen` is by using the `grove` meta-CLI, which manages all tools within the Grove ecosystem.

### Prerequisites

You must have the `grove` meta-CLI installed. If you don't, please follow the **[Grove Ecosystem Installation Guide](https://github.com/mattsolo1/grove-meta/blob/main/docs/02-installation.md)** first. This guide also covers essential setup like configuring your `PATH`.

### Install Command

Once the `grove` CLI is set up, you can install `grove-docgen` with a single command:

```bash
grove install docgen
```

### Verifying the Installation

To confirm that the tool was installed correctly, you can run its `version` command:

```bash
docgen version
```

### Building from Source

For contributors, the recommended way to work with the source code is to clone the entire Grove ecosystem monorepo. Please refer to the **[Building from Source](https://github.com/mattsolo1/grove-meta/blob/main/docs/02-installation.md#3-building-from-source-for-contributors)** section in the main installation guide for details.

## 3. Quick Start: Your First Generation

The easiest way to start is by using the `docgen init` command within your package's directory. This command sets up the necessary configuration and prompt templates.

### Step 1: Initialize Docgen

Navigate to the root of the package you want to document and run `init`.

```bash
cd /path/to/your/go-library
docgen init
```
This command scaffolds a default configuration suitable for a Go library.

### Step 2: Understand the Generated Structure

The `init` command creates a `docs/` directory with the following structure:

```
your-library/
├── docs/
│   ├── docgen.config.yml      # Main configuration file
│   ├── docs.rules             # Rules for context gathering
│   └── prompts/
│       ├── introduction.md    # Prompt for the introduction section
│       ├── core-concepts.md   # Prompt for core concepts
│       ├── usage-patterns.md  # Prompt for usage patterns
│       └── best-practices.md  # Prompt for best practices
└── ... (your other source files)
```

-   **`docgen.config.yml`**: This is where you define what documentation to generate. You'll edit the `title`, `description`, and `category` to match your project.
-   **`docs.rules`**: This file tells `cx` which source files to include when building the context for the LLM.
-   **`prompts/`**: This directory contains Markdown files that serve as instructions for the LLM for each section of your documentation.

### Step 3: Run Your First Generation

With the default configuration in place, you can now generate the documentation.

```bash
docgen generate
```

This command performs the following actions directly in your project directory:
1.  Reads your `docs/docgen.config.yml`.
2.  Runs `cx generate` to build a comprehensive context from your source code, based on the patterns in `docs/docs.rules`.
3.  For each section in your config, it reads the corresponding prompt from `docs/prompts/`.
4.  Calls the configured LLM via `grove llm request`, sending the source code context and the prompt.
5.  Writes the generated Markdown content to the output files specified in your configuration (e.g., `docs/introduction.md`).

### Step 4: Review the Output

Once the command completes, you will find the generated documentation inside the `docs/` directory, alongside your configuration files:

```
your-library/
├── docs/
│   ├── docgen.config.yml
│   ├── introduction.md      # <-- Generated output
│   ├── core-concepts.md     # <-- Generated output
│   ├── usage-patterns.md    # <-- Generated output
│   ├── best-practices.md    # <-- Generated output
│   └── prompts/
│       └── ...
└── ...
```
Open these files to review the initial, AI-generated documentation for your project.

## 4. Customizing Your Documentation

The initial output provides a solid baseline. Now, let's customize it to better reflect your project.

### Modifying Prompts

The quality of your documentation depends heavily on the quality of your prompts. Edit the files in `docs/prompts/` to give the LLM better instructions. For example, to improve the "Core Concepts" section, you might edit `docs/prompts/core-concepts.md` to list the specific concepts you want documented:

```markdown
# Core Concepts Section Prompt

You are an expert Go developer...

## Core Concepts to Document
1.  **Scenario**: The fundamental unit of a test.
2.  **Step**: A single action within a Scenario.
3.  **Context**: The state container for sharing data between steps.
4.  **Harness**: The test runner that orchestrates scenarios.
```

### Adjusting Configuration

Open `docs/docgen.config.yml` and tailor it to your project. You can change the title, description, use a more capable model, or tune generation parameters.

```yaml
# docs/docgen.config.yml
enabled: true
title: "My Awesome Go Library" # <-- Change this
description: "A library for doing incredible things with data streams." # <-- Change this
category: "Utilities" # <-- Change this
settings:
  model: "gemini-1.5-pro-latest" # Use a more capable model for higher quality
  # Add generation parameters for more control
  temperature: 0.5 # A balance between creativity and determinism
sections:
  # ...
```

### Selective Generation

When iterating on a single section, you don't need to regenerate everything. Use the `--section` (or `-s`) flag to target specific parts. This is much faster.

```bash
# Regenerate only the core-concepts section after updating its prompt
docgen generate --section core-concepts

# Regenerate both introduction and core-concepts
docgen generate -s introduction -s core-concepts
```

## 5. Next Steps

### Aggregating Workspace Documentation

Once you have generated documentation for one or more packages, you can aggregate it into a single, unified documentation site. From the root of your workspace, run:

```bash
docgen aggregate --output-dir ./dist
```
This command finds all packages with enabled `docgen` configs, copies their generated documentation into the output directory, and creates a `manifest.json` file that a frontend can use to build a navigation structure.

### Advanced Customization with Grove Flow

For a more interactive and guided documentation process, `grove-docgen` integrates with `grove-flow`. The `customize` command creates a multi-step plan to help you refine prompts and generate documentation.

```bash
# From your package directory
docgen customize

# Then, run the plan
flow run
```
This will launch an interactive process to help you build out a high-quality documentation suite.

## 6. Troubleshooting

-   **`docgen.config.yml not found`**: Make sure you are in the root directory of your package and that you have run `docgen init` to create the configuration file.
-   **`command not found: cx` or `command not found: grove`**: Ensure the Grove ecosystem is set up correctly and that the Grove binaries are in your system's `PATH`.
-   **Poor Quality Output**: The LLM's output is only as good as its input. To improve results, refine your prompts in `docs/prompts/` to be more specific. You can also switch to a more advanced model (like `gemini-1.5-pro-latest`) in your `docgen.config.yml`.