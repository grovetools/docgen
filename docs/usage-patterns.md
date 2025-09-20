# Usage Patterns

This document outlines common workflows and patterns for using `grove-docgen` to create, manage, and maintain high-quality documentation for your projects.

## 1. Initial Documentation Creation

This pattern covers the first-time setup for a new project. The goal is to quickly bootstrap the documentation structure and generate a first draft.

**Workflow:**

1.  **Initialize Configuration:** In your package's root directory, run the `init` command. This creates a `docs/` directory with a configuration file and a set of starter prompts.

    ```bash
    docgen init --type library
    ```
    This will create:
    - `docs/docgen.config.yml`
    - `docs/prompts/introduction.md`
    - `docs/prompts/core-concepts.md`
    - `docs/prompts/usage-patterns.md`
    - `docs/prompts/best-practices.md`

2.  **Customize Configuration:** Edit `docs/docgen.config.yml` to match your project's details. At a minimum, update the `title`, `description`, and `category`.

    ```yaml
    # docs/docgen.config.yml
    enabled: true
    title: "Grove Tend" # Changed from "My Library"
    description: "A Go library for creating powerful, scenario-based end-to-end testing frameworks." # Changed
    category: "Core Libraries" # Changed
    settings:
      model: "gemini-1.5-pro-latest" # Consider a more capable model for initial generation
    # ... sections remain the same initially
    ```

3.  **Review and Refine Prompts:** Read through the generated prompts in `docs/prompts/`. Add specific details, requirements, or context about your library to guide the LLM more effectively. For example, in `core-concepts.prompt.md`, list the exact concepts you want to be documented.

4.  **Generate Initial Docs:** Run the `generate` command from your package's root directory. This will read your configuration, build context from your source code, and generate the initial markdown files.

    ```bash
    docgen generate
    ```

After this process, you will have a full set of generated documentation in your `docs/` directory, ready for review and version control.

## 2. Iterative Documentation Development

Once the initial docs are created, you'll want to refine them. This workflow focuses on making targeted changes efficiently.

**Workflow:**

1.  **Selective Generation:** To speed up iteration, generate only the section you are working on using the `--section` (or `-s`) flag. This avoids waiting for all sections to be regenerated.

    ```bash
    # Regenerate only the 'core-concepts' section
    docgen generate --section core-concepts
    ```

2.  **Tune Generation Parameters:** If the output is too verbose or not creative enough, adjust the generation parameters in `docgen.config.yml`. You can set these globally or on a per-section basis.

    ```yaml
    # docs/docgen.config.yml
    settings:
      model: "gemini-1.5-flash-latest"
      temperature: 0.5 # Make output more focused globally
    
    sections:
      - name: "introduction"
        # ...
      - name: "api-reference"
        title: "API Reference"
        order: 5
        prompt: "prompts/api.md"
        output: "api.md"
        # Override for a specific section needing more detail
        model: "gemini-1.5-pro-latest"
        max_output_tokens: 8192
        temperature: 0.2 # Be very deterministic for API docs
    ```

3.  **Use Reference Mode for Refinements:** After you've manually edited a generated file, you can use `reference` mode to ask the LLM to improve it rather than starting from scratch. This preserves your manual changes while incorporating new context.

    ```yaml
    # docs/docgen.config.yml
    settings:
      model: "gemini-1.5-pro-latest"
      regeneration_mode: "reference" # Changed from "scratch"
    ```
    Now, when you run `docgen generate`, the existing markdown file will be included in the prompt as a reference for the LLM to edit and improve.

## 3. Maintaining Documentation

To keep documentation synchronized with code, integrate `docgen` into your development and CI/CD process.

**Workflow:**

1.  **Version Control:** Always commit the generated markdown files (`*.md`) and the `docgen` configuration (`docs/docgen.config.yml`, `docs/prompts/`) to your Git repository. This ensures documentation is versioned alongside the code it describes.

2.  **CI/CD Integration:** Add a step to your CI pipeline (e.g., GitHub Actions) that runs `docgen generate`. You can configure this to run on every commit to a feature branch or before merging to `main`. This ensures documentation is always up-to-date.

3.  **Regular Updates:** After significant code changes, run `docgen generate` to capture the updates. Using `regeneration_mode: "reference"` can help maintain consistency and style across updates.

## 4. Multi-Package Documentation

For a Grove workspace with multiple packages, `docgen aggregate` collects all documentation into a single, structured output suitable for a documentation website.

**Workflow:**

1.  **Ensure Packages are Generated:** First, make sure you have run `docgen generate` within each individual package that you want to include in the aggregated output. The `aggregate` command collects existing documentation; it does not generate it.

2.  **Run Aggregation:** From the root of your workspace (or any subdirectory), run the `aggregate` command.

    ```bash
    # This will find all packages and output to a 'dist' directory
    docgen aggregate --output-dir ./dist
    ```

3.  **Review the Output:** The output directory will contain:
    - A `manifest.json` file listing all packages, categories, and sections.
    - A subdirectory for each package containing its generated markdown files.

    This structure is designed to be consumed directly by frontends like the Grove developer portal, which uses the manifest to build its navigation.

## 5. Advanced Workflows

`docgen` provides several advanced features for fine-grained control and integration.

-   **Custom System Prompts:** To enforce a consistent tone, style, or set of rules across all generated sections, define a custom system prompt.

    ```yaml
    # docs/docgen.config.yml
    settings:
      # Use a custom prompt instead of the default guidelines
      system_prompt: "prompts/custom-style-guide.md" 
    ```

-   **Structured JSON Output:** For programmatic use (e.g., feeding documentation into another tool or an MCP server), you can generate a structured JSON representation of the documentation.

    ```yaml
    # docs/docgen.config.yml
    settings:
      # ...
      # This will parse the generated markdown and create a JSON file
      structured_output_file: "pkg/docs/tend-docs.json"
    
    sections:
      - name: "introduction"
        # ...
        json_key: "introduction" # Maps this section to a key in the JSON
      - name: "core-concepts"
        # ...
        json_key: "core_concepts"
    ```

## 6. Prompt Engineering

The quality of your documentation is directly tied to the quality of your prompts.

-   **Be Specific:** Your prompts should be explicit about what you want. Instead of "Document the concepts," use a list: "Document the following core concepts: Scenario, Step, Context, and Harness."

-   **Define the Output Structure:** Clearly state the desired format. The default prompts are a good example of this.

    *Example from `grove-tend/docs/prompts/core-concepts.prompt.md`:*
    ```markdown
    # Core Concepts Section Prompt
    
    ## Core Concepts to Document
    1. **Scenario**: The fundamental unit of a test.
    2. **Step**: A single action within a Scenario.
    # ...
    
    ## Output Format
    Please provide the output as a well-structured Markdown document with the following format:
    
    # Core Concepts
    
    For each concept, create a section with:
    - A clear heading (## Concept Name)
    - A description paragraph explaining the concept
    - A code example in a fenced code block
    ```

-   **Control Context with a Rules File:** The most powerful feature for prompt engineering is the `rules_file`. This file tells the underlying `cx` tool exactly which source code files to include as context for the LLM. This is critical for generating accurate, code-aware documentation.

    ```yaml
    # docs/docgen.config.yml
    settings:
      # ...
      # Point to a file that defines context for documentation generation
      rules_file: "docs.rules"
    ```

    *Example `docs.rules` file:*
    ```
    # Include all Go files in the pkg directory
    pkg/**/*.go
    
    # Also include the main README file
    README.md
    
    # Exclude test files from the context
    !**/*_test.go
    ```