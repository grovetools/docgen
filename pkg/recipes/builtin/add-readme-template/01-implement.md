---
id: add-readme-template
title: "Add README Template to Repository"
status: pending
type: interactive_agent
template: agent{{ if .Vars.model }}
model: "{{ .Vars.model }}"{{ end }}
---

# Add README Template to Existing Repository

You are tasked with adding the docgen README template feature to an existing repository that already has docgen initialized.

## Overview

The docgen README template feature allows READMEs to be automatically generated from documentation templates, maintaining a single source of truth while allowing customization. The generated README combines a template with content from the generated documentation.

## Prerequisites

Before starting, verify:
1. The repository has `docs/docgen.config.yml` (docgen is already initialized)
2. The repository has generated documentation files in `docs/` (e.g., `01-overview.md`)
3. You have the latest grove-docgen installed

## Implementation Steps

### Step 1: Analyze Existing Configuration

First, read the current `docs/docgen.config.yml` to understand:
- The project title and description
- What sections are defined
- Which section would be best for the README (usually "overview")

### Step 2: Preserve Existing README Content

Read the current `README.md` to identify any custom content that should be preserved in the template, such as:
- Badges (CI status, version, etc.)
- Installation instructions specific to the project
- Usage examples
- Contributing guidelines
- License information

### Step 3: Create the README Template

Create `docs/README.md.tpl` with:

```markdown
# {{ .Title }}

{{ .Description }}

## Overview

<!-- DOCGEN:OVERVIEW:START -->
<!-- This content will be automatically replaced by generated documentation -->
<!-- DOCGEN:OVERVIEW:END -->

## Installation

```bash
grove install {{ .PackageName }}
```

## Usage

[Add any preserved usage examples here]

## Contributing

Contributions welcome! Please read our [Contributing Guide](CONTRIBUTING.md) for details.

## License

MIT
```

**Important customizations to make:**

1. If the main section is not "overview", adjust the markers accordingly (e.g., `DOCGEN:GETTING-STARTED:START` for a "getting-started" section)
2. Preserve any important badges at the top of the file
3. Keep project-specific installation or usage instructions
4. Maintain any existing contributing or license sections

### Step 4: Update Configuration

Add the `readme` configuration to `docs/docgen.config.yml` after the `category` field:

```yaml
readme:
  template: docs/README.md.tpl
  output: README.md
  source_section: overview  # or the appropriate section name
```

Make sure the `source_section` matches an actual section name from the `sections` list in the config.

### Step 5: Generate and Test

Run the following commands to test the implementation:

```bash
# Generate the specific section if not already done
docgen generate --section overview

# Sync the README
docgen sync-readme
```

Verify that:
1. The README.md is generated successfully
2. The documentation content is properly injected
3. Custom template sections are preserved
4. The markers remain in the output for future updates

### Step 6: Update Makefile (if present)

If the repository has a Makefile with a `generate-docs` target, update it to include sync-readme:

```makefile
generate-docs: build
	@echo "Generating documentation..."
	@docgen generate
	@echo "Synchronizing README.md..."
	@docgen sync-readme
```

## Common Adjustments by Project Type

### For CLI Tools
Include command examples and help output in the template:

```markdown
## Quick Start

```bash
# Install
grove install {{ .PackageName }}

# Get help
{{ .PackageName }} --help

# Common commands
{{ .PackageName }} init
{{ .PackageName }} generate
```
```

### For Libraries
Include import statements and basic usage:

```markdown
## Quick Start

```go
import "github.com/mattsolo1/{{ .PackageName }}"

func main() {
    // Basic usage example
}
```
```

### For Services/APIs
Include API endpoints or service configuration:

```markdown
## Quick Start

Start the service:
```bash
{{ .PackageName }} serve --port 8080
```

API endpoints:
- `GET /health` - Health check
- `POST /api/v1/...` - Main API
```

## Troubleshooting

### Section not found error
If you get `source_section 'overview' not found`, check that:
1. The section name in `readme.source_section` matches a section in the `sections` list
2. The section has been generated (`docs/01-overview.md` or similar exists)

### Template not found error
Ensure the template path is correct. It should be `docs/README.md.tpl` relative to the repository root.

### Documentation not generated error
Run `docgen generate --section <section-name>` before running `sync-readme`.

## Final Checklist

Before committing your changes:

- [ ] `docs/README.md.tpl` exists and contains appropriate content
- [ ] `docs/docgen.config.yml` has the `readme` section configured
- [ ] Running `docgen sync-readme` generates README.md successfully
- [ ] The generated README contains both template content and injected documentation
- [ ] Important existing README content has been preserved in the template
- [ ] Makefile updated if present
- [ ] Test that `make generate-docs` works if Makefile was updated

## Important Notes

- The marker format is `<!-- DOCGEN:{SECTION_NAME}:START -->` where SECTION_NAME is the uppercase version of your section name
- The `{{ .PackageName }}` placeholder uses the directory name of the project
- You can have multiple sections with different markers, but only one is automated via config
- Content outside the markers is preserved during updates

Remember: The goal is to maintain consistency across the Grove ecosystem while preserving each project's unique characteristics.