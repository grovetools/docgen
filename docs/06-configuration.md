## Docgen Settings

This section configures the core metadata, generation settings, and structure of the documentation for the project.

| Property | Description |
| :--- | :--- |
| `enabled` | (boolean, required, default: true) <br> A master switch to enable or disable documentation generation for this specific package. If set to `false`, the `docgen` tool will skip this package during bulk operations like `aggregate` or `watch`. |
| `title` | (string, required) <br> The display title for the package's documentation. This is used in the sidebar, site navigation, and aggregated documentation views. |
| `description` | (string, required) <br> A brief summary of what the package does. This description may be used in index pages, cards, or SEO metadata for the generated website. |
| `category` | (string, required) <br> Defines the organizational category for the documentation (e.g., 'Core Libraries', 'Tools', 'Templates'). This helps group related packages together in the generated website sidebar. |
| `settings` | (object, optional) <br> Contains global configuration settings for the documentation generation process, such as the default LLM model to use, output directories, and context rules. |
| `sections` | (array, required) <br> A list of individual documentation sections to be generated. Each item in this array corresponds to a specific topic or file (e.g., "Overview", "Usage", "API Reference"). |
| `readme` | (object, optional) <br> Configuration for automatically synchronizing the project's root `README.md` file using generated documentation content. |
| `sidebar` | (object, optional) <br> Configuration for the website sidebar display, including category ordering and icons. |
| `logos` | (array, optional) <br> A list of additional logo files (absolute paths or paths starting with `~/`) to copy to the output directory during aggregation. This is useful for including assets that aren't generated but are referenced by the documentation. |

```toml
title = "Grove Core"
description = "Core libraries for the Grove ecosystem"
category = "Libraries"
enabled = true
logos = ["~/assets/grove-logo-dark.svg"]

[settings]
model = "claude-3-opus-20240229"
```

### Settings Configuration

Global settings that control the generation process and environment.

| Property | Description |
| :--- | :--- |
| `model` | (string, optional) <br> The default LLM model to use for generation across all sections (e.g., `gemini-2.0-flash`, `claude-3-opus`). Can be overridden per section. |
| `output_mode` | (string, optional) <br> Determines how the output is structured. `package` (default) treats the directory as a standard package. `sections` treats it as a collection of website content sections (e.g., overview, concepts), where subdirectories represent distinct content collections. |
| `ecosystems` | (array, optional) <br> A list of ecosystem names to include when aggregating documentation. If not specified, defaults to the current ecosystem. |
| `regeneration_mode` | (string, optional) <br> Controls how documentation is updated. `scratch` generates fresh content. `reference` (default) provides the existing file as context to the LLM to preserve manual edits. |
| `rules_file` | (string, optional) <br> Path to a custom rules file (e.g., `.cx/rules`) used by `cx generate` to build the context for the LLM. |
| `structured_output_file` | (string, optional) <br> If defined, parses the generated Markdown into a structured JSON file at this path. |
| `system_prompt` | (string, optional) <br> Path to a custom system prompt file or `default` to use the built-in technical writer persona. |
| `output_dir` | (string, optional) <br> The directory where generated documentation files will be saved. Defaults to `docs`. |
| `temperature` | (number, optional) <br> Global control for randomness in generation (0.0-1.0). Higher values produce more creative output, while lower values are more deterministic. |
| `top_p` | (number, optional) <br> Global nucleus sampling parameter (0.0-1.0). Controls the diversity of the generated text. |
| `top_k` | (integer, optional) <br> Global top-k sampling parameter. Limits the vocabulary used during generation. |
| `max_output_tokens` | (integer, optional) <br> Global limit on the length of generated content. |

```toml
[settings]
model = "gemini-2.0-flash"
output_dir = "docs"
regeneration_mode = "reference"
temperature = 0.2
```

### Sections Configuration

Each item in the `sections` array defines a specific piece of documentation to be generated or captured.

| Property | Description |
| :--- | :--- |
| `name` | (string, required) <br> A unique identifier for this section. Used for referencing in `readme.source_section`. |
| `title` | (string, required) <br> The human-readable display title. Often used as the top-level heading (# Title) in the output. |
| `output` | (string, required) <br> The filename for the generated Markdown output (e.g., `01-overview.md`). |
| `order` | (integer, required) <br> A numeric value for sorting sections in the aggregated documentation. |
| `prompt` | (string, optional) <br> Path to the prompt file relative to `docs/` (e.g., `prompts/01-overview.md`). Required for standard generation. |
| `model` | (string, optional) <br> Override the global model setting for this section. If not specified, uses the global model setting. |
| `status` | (string, optional) <br> Publication status: `draft` (notebook only), `dev` (dev website), or `production`. Default is usually `draft`. |
| `type` | (string, optional) <br> The type of generation: `schema_to_md` (JSON schema to Markdown), `doc_sections` (aggregation of other docs), `capture` (CLI help output), or standard generation (default). |
| `output_dir` | (string, optional) <br> Override output directory for this specific section. Useful in `sections` output mode to direct files to specific content collections. |
| `agg_strip_lines` | (integer, optional) <br> Number of lines to strip from the top of this section's content during aggregation. Useful for removing headers that might not be appropriate in aggregated context. |
| `json_key` | (string, optional) <br> Optional key to use when parsing this section into the structured JSON output file. |
| `schemas` | (array, optional) <br> For `schema_to_md` type: A list of objects containing `path` (to schema file) and `title` to aggregate multiple schemas into one document. |
| `doc_sources` | (array, optional) <br> For `doc_sections` type: A list of source configurations (`package`, `doc`, `title`, `description`) to pull content from other generated docs. |
| `binary` | (string, optional) <br> For `capture` type: The name of the binary to execute (e.g., `grove`). |
| `format` | (string, optional) <br> For `capture` type: Output format, either `styled` (HTML terminal block) or `plain` (Markdown code block). |
| `depth` | (integer, optional, default: 5) <br> For `capture` type: Maximum recursion depth for subcommand crawling. |
| `subcommand_order` | (array, optional) <br> For `capture` type: Priority order for sorting subcommands. |
| `temperature` | (number, optional) <br> Temperature override for this section. |
| `top_p` | (number, optional) <br> Top-p override for this section. |
| `top_k` | (integer, optional) <br> Top-k override for this section. |
| `max_output_tokens` | (integer, optional) <br> Maximum output tokens override for this section. |
| `source` | (string, optional) <br> **Deprecated**: Use `schemas` list instead. Source file for `schema_to_md` type. |

```toml
[[sections]]
name = "overview"
title = "Overview"
output = "01-overview.md"
prompt = "prompts/01-overview.md"
order = 1
status = "production"

[[sections]]
name = "cli-reference"
title = "CLI Reference"
output = "05-commands.md"
type = "capture"
binary = "grove"
order = 5
```

### Readme Synchronization

The `readme` object configures how the root `README.md` is generated from a template and a specific documentation section.

| Property | Description |
| :--- | :--- |
| `template` | (string, required, default: docs/README.md.tpl) <br> Path to the README template file (relative to package root). |
| `output` | (string, required, default: README.md) <br> Path to the output README file (relative to package root). |
| `source_section` | (string, required, default: introduction) <br> The `name` of the documentation section to inject into the template. |
| `strip_lines` | (integer, optional, default: 0) <br> Number of lines to strip from the top of the source documentation file before injection. Useful for removing headings, blank lines, or metadata. |
| `generate_toc` | (boolean, optional, default: false) <br> Whether to automatically generate a table of contents from documentation sections. Requires `<!-- DOCGEN:TOC:START -->` and `<!-- DOCGEN:TOC:END -->` markers in the template. |
| `base_url` | (string, optional) <br> Base URL for converting root-relative paths in the documentation to absolute URLs in the README. |
| `logo` | (object, optional) <br> Configuration for generating a combined logo+text header image. |

```toml
[readme]
template = "docs/README.md.tpl"
output = "README.md"
source_section = "overview"
generate_toc = true
```

#### Logo Configuration

| Property | Description |
| :--- | :--- |
| `input` | (string, required) <br> Path to input logo SVG (relative to base_url root or absolute). |
| `output` | (string, required) <br> Path for output logo-with-text SVG. |
| `text` | (string, required) <br> Text to display below logo. |
| `font` | (string, required) <br> Path to TTF/OTF font file. |
| `color` | (string, optional) <br> Text color (hex). |
| `spacing` | (number, optional, default: 35) <br> Spacing between logo and text. |
| `text_scale` | (number, optional, default: 1.1) <br> Text width as proportion of logo. |
| `width` | (number, optional, default: 200) <br> Output SVG width in pixels. |

### Sidebar Configuration

Configuration for the website sidebar display.

| Property | Description |
| :--- | :--- |
| `category_order` | (array, optional) <br> Defines the display order of categories in the sidebar. |
| `categories` | (object, optional) <br> Map of category names to configuration (icon, flat mode, package order). |
| `packages` | (object, optional) <br> Map of package names to configuration (icon, color, status). |
| `package_category_override` | (object, optional) <br> Overrides the default category for specific packages. |

```toml
[sidebar]
category_order = ["Guides", "Libraries", "Tools"]

[sidebar.categories.Tools]
icon = "ph:terminal"
flat = true
```