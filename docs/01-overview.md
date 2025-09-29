# Grove Docgen

`grove-docgen` is an LLM-powered, workspace-aware documentation generator. It automates the creation of technical documentation by combining a project's source code with user-defined prompts, providing a structured and repeatable workflow for keeping documentation in sync with development.

By defining documentation as a series of configurable sections, `grove-docgen` allows you to maintain full control over the structure, tone, and content of the output.

<!-- placeholder for animated gif -->

### Key Features

*   **Section-Based Architecture**: Define your documentation structure in a `docgen.config.yml` file. Each section is configured with its own prompt, output file, and LLM settings, allowing for granular control over the final output.
*   **Customizable Prompts**: Use the `docgen init` command to scaffold a set of starter prompts. You have full ownership of these prompts, enabling you to engineer them to match your project's specific needs and desired documentation style.
*   **Context-Aware Generation**: Leverages `grove-context` to automatically build a comprehensive understanding of your codebase based on a `.grove/rules` file. This ensures the LLM has the necessary context to generate accurate and relevant documentation.
*   **Interactive Customization**: The `docgen customize` command integrates with `grove-flow` to create a multi-step plan for generating your documentation. This interactive workflow uses AI agents to help you refine the structure and content before final generation.
*   **Multi-Model Support**: Configure different LLM models for different tasks. You can set a global default model or override it for specific sections, allowing you to use the best tool for each job (e.g., a powerful model for overviews and a faster model for simple sections).
*   **Workspace Aggregation**: The `docgen aggregate` command discovers all `docgen`-enabled packages within a Grove workspace, generates their documentation, and aggregates the results into a single, unified output directory with a `manifest.json`.

## Ecosystem Integration

`grove-docgen` is a key component of the Grove ecosystem, designed to work seamlessly with other developer tools.

*   **`grove-context` (`cx`)**: Serves as the foundational context provider. Before generating any documentation, `docgen` uses `cx` to gather relevant source code and files, ensuring the LLM has a deep and accurate understanding of the project.
*   **`grove-flow`**: The `docgen customize` command uses `grove-flow` to create and manage an interactive, plan-based workflow. This turns documentation generation into a guided, agent-assisted process.
*   **`grove-gemini` and `grove-openai`**: The underlying `grove llm request` facade is used to execute the calls to the configured LLM providers, handling the API interactions required to generate the documentation content.

By integrating these tools, `grove-docgen` provides a powerful, end-to-end solution for creating and maintaining high-quality technical documentation.

## Installation

Install via the Grove meta-CLI:
```bash
grove install docgen
```

Verify installation:
```bash
docgen version
```

Requires the `grove` meta-CLI. See the [Grove Installation Guide](https://github.com/mattsolo1/grove-meta/blob/main/docs/02-installation.md) if you don't have it installed.
