# Examples 

This guide provides practical examples of using `grove-docgen` in common development workflows, from initializing a new project to using `grove-flow` for interactive documentation generation.

## Example 1: Basic Project Setup

This example covers the fundamental workflow for a new project: initializing the configuration, customizing a prompt, and generating the documentation.

Assume you have a Go project and want to generate its initial documentation.

#### 1. Initialize `docgen`

Run the `docgen init` command in the project root. This command scaffolds the necessary configuration and prompt files.

```bash
docgen init --model gemini-2.5-pro --rules-file docs.rules
```

This creates the following structure:

```
my-go-app/
├── docs/
│   ├── docgen.config.yml  # Main configuration
│   ├── docs.rules         # Context rules for 'grove-context'
│   └── prompts/
│       ├── 01-overview.md
│       ├── 02-examples.md
│       └── ... (other prompt templates)
└── ... (your source code)
```

The `docs/docgen.config.yml` file is pre-filled with sections for an overview, examples, and more, all configured to use the model and rules file specified.

#### 2. Customize a Prompt

The generated files in `docs/prompts/` are templates that can be modified. Edit the overview prompt to be more specific to the project.

Open `docs/prompts/01-overview.md` and add details about the project's features and purpose.

**`docs/prompts/01-overview.md` (modified):**
```markdown
# Documentation Task: Project Overview

You are a technical writer. Write a single-page overview for the `my-go-app` tool.

## Task
Based on the provided codebase context, create an overview that describes these specific features:
- A caching layer.
- A configuration file format.
- Integration with Prometheus for metrics.
```

#### 3. Define the Context

Edit the `docs/docs.rules` file to specify which source code files are relevant for generating the documentation. `grove-context` will use these rules.

**`docs/docs.rules`:**
```gitignore
# Include all Go files, but exclude tests
**/*.go
!**/*_test.go

# Include the main README
README.md
```

#### 4. Generate the Documentation

Run the `docgen generate` command. It reads the configuration, uses `grove-context` to build the context from the rules, and calls the LLM for each section defined in `docgen.config.yml`.

```bash
docgen generate
```

**Expected Output:**

The command generates Markdown files in the `docs/` directory, one for each section.

```
my-go-app/
└── docs/
    ├── 01-overview.md         # Generated documentation
    ├── 02-examples.md         # Generated documentation
    ├── ...
    ├── docgen.config.yml
    ├── docs.rules
    └── prompts/
        └── ...
```

This process produces a documentation set based on the source code and custom prompts.

## Example 2: Advanced Configuration for a Complex Project

This example demonstrates features for a larger project, such as using different models, controlling regeneration, and generating structured data.

#### 1. Configure Advanced Settings

Modify `docgen.config.yml` for more granular control.

-   **Section-Specific Model**: Use a different model for the overview than for the command reference.
-   **Reference Mode**: Use `regeneration_mode: reference` to have the LLM improve existing documentation rather than writing it from scratch.
-   **Structured Output**: Configure `structured_output_file` to generate a JSON file alongside the Markdown for programmatic use.

**`docs/docgen.config.yml` (snippet):**
```yaml
# yaml-language-server: $schema=...
title: "My Advanced Tool"
description: "..."
enabled: true
settings:
  model: gemini-2.0-flash  # Default to a fast model
  regeneration_mode: reference
  rules_file: docs.rules
  structured_output_file: pkg/docs/docs.json
sections:
  - name: overview
    order: 1
    output: 01-overview.md
    prompt: prompts/01-overview.md
    title: Overview
    model: gemini-2.5-pro # Override with a different model for this section
  - name: command-reference
    order: 2
    output: 02-command-reference.md
    prompt: prompts/02-command-reference.md
    title: Command Reference
    json_key: command_reference # Key for the structured JSON output
```

#### 2. Generate a Single Section

To regenerate only one part of the documentation, use the `--section` flag.

```bash
docgen generate --section overview
```

This re-runs the LLM call only for the "overview" section.

#### 3. Regenerate the Structured JSON

After generating the Markdown, the structured JSON output can be updated without calling the LLM again. The `regen-json` command parses existing Markdown files and builds the JSON file.

```bash
docgen regen-json
```

This creates or updates `pkg/docs/docs.json` based on the content of the generated documentation, providing a machine-readable version of the docs.

## Example 3: Interactive Customization with Grove Flow

For projects where the documentation structure is not yet defined, `grove-docgen` integrates with `grove-flow` to interactively build the documentation plan with an AI agent.

#### 1. Create a Customization Plan

The `docgen customize` command creates a `grove-flow` plan, which is a series of steps for an AI agent to follow.

```bash
docgen customize --recipe-type agent
```

This command inspects `docgen.config.yml` and creates a new plan in the `plans/` directory. The plan contains jobs for an AI agent to:
1.  Review the project's source code.
2.  Discuss the documentation structure.
3.  Refine the prompt files for each section.
4.  Run the `docgen generate` command to produce the documentation.

#### 2. Run the Interactive Plan

Start the workflow with `flow run`.

```bash
# The plan name will be printed by the 'customize' command
flow run plans/docgen-customize-agent-my-go-app
```

This command launches an interactive `tmux` session where an AI agent begins executing the plan.

#### 3. Collaborate with the Agent

Inside the `tmux` session, you can provide instructions to the agent and review its work. This human-in-the-loop process allows for guiding the AI to create a documentation plan tailored to the project. Once the process is complete, the agent executes the final generation step.
