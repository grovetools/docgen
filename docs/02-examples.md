This guide provides practical examples of how to use `grove-docgen` in common development workflows, from initializing a new project to leveraging the full Grove ecosystem for interactive documentation generation.

## Example 1: Basic Project Setup

This example covers the fundamental workflow for a new project: initializing the configuration, customizing a prompt, and generating the documentation.

Let's assume you have a simple Go project and want to generate its initial documentation.

#### 1. Initialize `docgen`

First, run the `docgen init` command in your project root. This command scaffolds the necessary configuration and prompt files.

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

Your `docs/docgen.config.yml` will be pre-filled with sections for an overview, examples, and more, all configured to use the model and rules file you specified.

#### 2. Customize a Prompt

The generated files in `docs/prompts/` are templates that you own and can modify. Let's edit the overview prompt to be more specific to our project.

Open `docs/prompts/01-overview.md` and add details about your project's key features and purpose.

**`docs/prompts/01-overview.md` (modified):**
```markdown
# Documentation Task: Project Overview

You are an expert technical writer. Write a clear, engaging single-page overview for the `my-go-app` tool.

## Task
Based on the provided codebase context, create a complete overview that highlights these specific features:
- A high-performance caching layer.
- A simple configuration file format.
- Integration with Prometheus for metrics.
```

#### 3. Define the Context

Edit the `docs/docs.rules` file to tell `grove-context` which source code files are relevant for generating the documentation.

**`docs/docs.rules`:**
```gitignore
# Include all Go files, but exclude tests
**/*.go
!**/*_test.go

# Include the main README
README.md
```

#### 4. Generate the Documentation

Now, run the `docgen generate` command. It will read your configuration, use `grove-context` to build the context from your rules, and call the LLM for each section defined in your `docgen.config.yml`.

```bash
docgen generate
```

**Expected Output:**

The command will generate Markdown files in the `docs/` directory, one for each section.

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

You now have a complete, AI-generated documentation set based on your source code and custom prompts.

## Example 2: Advanced Configuration for a Complex Project

This example demonstrates advanced features for a larger project, such as using different models, controlling regeneration, and generating structured data.

#### 1. Configure Advanced Settings

Let's modify `docgen.config.yml` for more granular control.

-   **Section-Specific Model**: Use a powerful model for the high-level overview but a faster, cheaper model for the command reference.
-   **Reference Mode**: Use `regeneration_mode: reference` to have the LLM improve existing documentation rather than writing it from scratch.
-   **Structured Output**: Configure `structured_output_file` to generate a JSON file alongside the Markdown, perfect for programmatic use.

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
    model: gemini-2.5-pro # Override with a powerful model for this section
  - name: command-reference
    order: 2
    output: 02-command-reference.md
    prompt: prompts/02-command-reference.md
    title: Command Reference
    json_key: command_reference # Key for the structured JSON output
```

#### 2. Generate a Single Section

If you've only updated the overview prompt, you don't need to regenerate everything. Use the `--section` flag to target a specific part of the documentation.

```bash
docgen generate --section overview
```

This saves time and cost by only re-running the LLM call for the "overview" section.

#### 3. Regenerate the Structured JSON

After generating the Markdown, you can update the structured JSON output without calling the LLM again. The `regen-json` command parses your existing Markdown files and builds the JSON file.

```bash
docgen regen-json
```

This will create or update `pkg/docs/docs.json` based on the content of your generated documentation, providing a machine-readable version of your docs.

## Example 3: Interactive Customization with Grove Flow

For the most complex projects, you can use `grove-docgen`'s integration with `grove-flow` to interactively build your documentation plan with an AI agent. This workflow is ideal when you're not sure about the final structure and want AI assistance in designing the prompts and configuration.

#### 1. Create a Customization Plan

Instead of manually editing files, use the `docgen customize` command. This command creates a `grove-flow` plan, which is a series of steps for an AI agent to follow.

```bash
docgen customize --recipe-type agent
```

This command inspects your `docgen.config.yml` and creates a new plan in the `plans/` directory. The plan contains jobs for an AI agent to:
1.  Review your project's source code.
2.  Discuss the ideal documentation structure with you.
3.  Help you write or refine the prompt files for each section.
4.  Finally, run the `docgen generate` command to produce the documentation.

#### 2. Run the Interactive Plan

Navigate to the newly created plan and start the workflow with `flow run`.

```bash
# The plan name will be printed by the 'customize' command
flow run plans/docgen-customize-agent-my-go-app
```

This command launches an interactive `tmux` session where an AI agent will begin executing the plan. It will ask you questions and work with you to define the documentation.

#### 3. Collaborate with the Agent

Inside the `tmux` session, you can chat with the agent, give it instructions, and review its work. This human-in-the-loop process allows you to guide the AI to create a high-quality, bespoke documentation plan tailored perfectly to your project.

Once you are satisfied, the agent will execute the final generation step, leaving you with a complete set of documentation that you helped design. This workflow combines the power of LLMs with the precision of human oversight.