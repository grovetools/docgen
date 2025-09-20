# Introduction to grove-docgen

`grove-docgen` is an LLM-powered, workspace-aware documentation generator designed specifically for the Grove ecosystem. It automates the creation of high-quality, narrative documentation by analyzing source code and applying expert technical writing principles. Its purpose is to solve the persistent problem of keeping documentation comprehensive, accurate, and synchronized with an evolving codebase.

## The Challenge of Traditional Documentation

In software development, documentation is crucial but often neglected. Manual documentation writing is time-consuming and frequently falls out of sync with the code it describes. Traditional tools can help format documentation or generate basic API references, but they fall short in creating the conceptual, "how-to," and "why" content that developers need most. This leads to a documentation gap where the code is the only source of truth, increasing the barrier to entry for new contributors and slowing down development.

## The grove-docgen Approach

`grove-docgen` addresses this challenge by treating documentation as code. The entire generation process is defined by configuration files and prompts that live alongside the source code in a project's repository. This approach has several key benefits:

-   **Version Controlled:** Documentation definitions are versioned with the source code, ensuring that changes to the code can be accompanied by corresponding updates to the documentation prompts.
-   **Repeatable:** The generation process is deterministic and can be run on any machine or in a CI/CD pipeline, guaranteeing consistent output.
-   **Integrated:** Documentation becomes an integral part of the development workflow, not an afterthought.

The core philosophy is to leverage the analytical power of Large Language Models (LLMs) to bridge the gap between code and human understanding, producing documentation that is both technically accurate and easy to read.

### Isolated Generation Environments

A unique and critical aspect of `grove-docgen` is its use of isolated environments. Before generating documentation, it performs a local `git clone` of the current repository into a temporary directory. All subsequent steps—context gathering, LLM prompting, and file generation—happen within this clean, isolated clone.

This approach ensures that the documentation is generated based purely on the committed state of the repository. It prevents local, uncommitted changes, untracked files, or environment-specific configurations from influencing the output, leading to highly reproducible and accurate results.

## How It Works

The `docgen generate` command orchestrates a multi-step process to create documentation for a package:

1.  **Configuration Loading:** It reads a `docs/docgen.config.yml` file to understand the project's title, description, and the specific sections of documentation to generate (e.g., Introduction, Core Concepts, Best Practices).
2.  **Isolation:** It creates a temporary, clean clone of the repository.
3.  **Context Gathering:** Within the clone, it leverages Grove's context-aware tool, `cx`, to scan the codebase. Based on rules defined in a `docs.rules` file, `cx` gathers the most relevant code snippets, file structures, and other artifacts.
4.  **LLM Prompting:** For each section defined in the configuration, `docgen` combines a user-written prompt with the context gathered by `cx` and sends it to an LLM (e.g., Google's Gemini).
5.  **Content Generation:** The LLM analyzes the code and the prompt's instructions to write the documentation section in Markdown format.
6.  **Output:** The generated Markdown files are written back to the original project's `docs` directory.

The `docgen aggregate` command complements this by discovering all `docgen`-enabled packages within a Grove workspace, collecting their generated documentation, and assembling it into a unified output directory with a `manifest.json` file, ready to be consumed by a frontend documentation site.