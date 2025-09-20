# grove-docgen

LLM-powered, workspace-aware documentation generator for Grove ecosystem projects.

## Features

- Automatically discovers packages in a Grove workspace
- Uses LLMs to generate documentation from code context
- Aggregates documentation into a structured, frontend-agnostic format
- Configurable per-package documentation generation

## Installation

```bash
grove install docgen
```

## Usage

### Generate documentation for a single package

```bash
# In a package directory with docs/docgen.config.yml
docgen generate
```

### Aggregate documentation for entire workspace

```bash
# From anywhere in the Grove workspace
docgen aggregate --output-dir=./dist
```

## Configuration

Create a `docs/docgen.config.yml` in your package:

```yaml
enabled: true
title: "My Package"
description: "A package that does amazing things"
category: "Tools"
sections:
  - name: "overview"
    title: "Overview"
    order: 1
    prompt: "prompts/overview.md"
    output: "overview.md"
  - name: "api"
    title: "API Reference" 
    order: 2
    prompt: "prompts/api.md"
    output: "api.md"
```

## Development

### Building

```bash
make build
```

### Testing

```bash
make test
```

## Contributing

This is a private repository. Please ensure all contributions follow the Grove ecosystem conventions.