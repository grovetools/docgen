# Getting Started with grove-docgen

This guide provides a step-by-step walkthrough to get you from installation to generating your first set of documentation using `grove-docgen`.

## 1. Prerequisites

Before you begin, ensure your development environment meets the following requirements:

-   **Grove Ecosystem**: You must have the Grove development environment set up and your project must be part of a Grove workspace.
-   **Required Tools**: `grove-docgen` relies on other Grove tools. Make sure `gemapi` (for LLM interaction) and `cx` (for context building) are installed and available in your system's `PATH`.
-   **Git Repository**: Your project must be a Git repository. `grove-docgen` creates an isolated local clone to build context without affecting your working directory.

## 2. Installation

You can install `grove-docgen` using the standard Grove installer or by building it from the source code.

### Using grove install (Recommended)

This is the simplest method. Run the following command from anywhere within your Grove workspace:

```bash
grove install docgen
```

### Building from Source

If you need to build from a specific branch or want to contribute to development, you can clone the repository and use the `Makefile`:

```bash
# Clone the repository (adjust URL if needed)
git clone git@github.com:mattsolo1/grove-docgen.git
cd grove-docgen

# Build the binary
make build
```
This will create the `docgen` binary in the `bin/` directory. You can then move this binary to a location in your `PATH`.

### Verification

After installation, verify that `docgen` is working correctly by checking its version:

```bash
docgen version
```

You should see output detailing the version, commit, and build date.

## 3. Quick Start: Scaffolding and First Generation

The easiest way to start is by using the `docgen init` command within your package's directory. This command sets up the necessary configuration and prompt templates.

### Step 1: Initialize Docgen

Navigate to the root of the package you want to document and run `init`. Currently, `library` is the supported project type.

```bash
cd /path/to/your/go-library
docgen init --type library
```

### Step 2: Understand the Generated Structure

The `init` command creates a `docs/` directory with the following structure:

```
your-library/
├── docs/
│   ├── docgen.config.yml      # Main configuration file
│   └── prompts/
│       ├── introduction.md    # Prompt for the introduction section
│       ├── core-concepts.md   # Prompt for core concepts
│       ├── usage-patterns.md  # Prompt for usage patterns
│       └── best-practices.md  # Prompt for best practices
└── ... (your other source files)
```

-   **`docgen.config.yml`**: This is where you define what documentation to generate. You'll edit the `title`, `description`, and `category` to match your project.
-   **`prompts/`**: This directory contains Markdown files that serve as instructions for the LLM for each section of your documentation.

### Step 3: Run Your First Generation

With the default configuration in place, you can now generate the documentation.

```bash
docgen generate
```

This command performs several actions:
1.  Creates a temporary, isolated clone of your repository.
2.  Runs `cx generate` within the clone to build a comprehensive context from your source code.
3.  Calls the LLM via `gemapi` for each section defined in `docgen.config.yml`, using the corresponding prompt.
4.  Writes the generated Markdown content to the output files specified in your configuration.
5.  Copies the final Markdown files back into your project's `docs/` directory.

### Step 4: Review the Output

Once the command completes, you will find the generated documentation inside the `docs/` directory:

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

## 4. Your First Custom Documentation

The initial output provides a solid baseline. Now, let's customize it to better reflect your project.

### Modifying Prompts

The quality of your documentation depends heavily on the quality of your prompts. Edit the files in `docs/prompts/` to give the LLM better instructions.

For example, to improve the "Core Concepts" section, you might edit `docs/prompts/core-concepts.md` to list the specific concepts you want documented:

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

Open `docs/docgen.config.yml` and tailor it to your project.

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

By following these steps, you can quickly integrate `grove-docgen` into your workflow, moving from a fully automated baseline to finely tuned, high-quality documentation.