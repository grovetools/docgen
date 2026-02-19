# CLI Reference

Complete command reference for `docgen`.

## docgen

<div class="terminal">
LLM-powered, workspace-aware documentation generator.

Usage:
  docgen [command]

Available Commands:
  aggregate       Generate and aggregate documentation from all workspace packages
  capture         Capture help output for all commands in a binary
  completion      Generate the autocompletion script for the specified shell
  customize       Create a customized documentation plan using grove-flow
  generate        Generate documentation for the current package
  help            Help about any command
  init            Initialize docgen configuration and prompts for a package
  logo            Logo asset generation commands
  migrate-config  Migrate docgen config from docs/ to notebook workspace
  migrate-prompts Migrate prompts from docs/prompts to notebook workspace
  recipe          Manage and display documentation recipes
  regen-json      Regenerate the structured JSON output from existing markdown files
  schema          Manage and process JSON schemas
  sync            Synchronize documentation between notebook and repository
  sync-readme     Generate the README.md from a template and a source documentation file
  version         Print the version information for this binary
  watch           Watch documentation sources and hot-reload on changes

Flags:
  -c, --config string   Path to grove.yml config file
  -h, --help            help for docgen
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging

Use "docgen [command] --help" for more information about a command.
</div>

### docgen aggregate

<div class="terminal">
Discovers all packages in the workspace, generates documentation for each enabled package, and aggregates the results into an output directory with a manifest.json file.

The --mode flag controls which documentation status levels are included:
  dev: Includes draft, dev, and production sections (for dev website)
  prod: Only includes production sections (for production website)

Mode can also be set via the DOCGEN_MODE environment variable.

The --transform flag applies output-specific transformations to the documentation:
  astro: Rewrites asset paths and adds Astro-compatible frontmatter for the Grove website

Usage:
  docgen aggregate [flags]

Flags:
  -h, --help                help for aggregate
  -m, --mode string         Aggregation mode: 'dev' (all statuses) or 'prod' (production only) (default "dev")
  -o, --output-dir string   Directory to save the aggregated documentation (default "dist")
      --transform string    Apply transformations to output (e.g., 'astro' for website builds)

Global Flags:
  -c, --config string   Path to grove.yml config file
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging
</div>

### docgen capture

<div class="terminal">
Recursively executes a binary with --help to capture and compile a complete command reference.

This is useful for generating documentation for CLI tools that use Cobra or similar frameworks.
It parses the "COMMANDS" section of the help output to discover subcommands.

Output formats:
  markdown  Plain text in markdown code blocks (default)
  html      Styled HTML with terminal colors preserved

Examples:
  docgen capture nb --output docs/commands.md
  docgen capture grove -o commands.html --format html
  docgen capture grove -o commands.md --depth 3

Usage:
  docgen capture &lt;binary&gt; [flags]

Flags:
  -d, --depth int       Maximum recursion depth (default 5)
  -f, --format string   Output format: markdown, html (default "markdown")
  -h, --help            help for capture
  -o, --output string   Output file (default: commands.md or commands.html)

Global Flags:
  -c, --config string   Path to grove.yml config file
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging
</div>

### docgen customize

<div class="terminal">
Creates a Grove Flow plan for interactively customizing and generating documentation.

This command:
1. Reads your docgen.config.yml configuration
2. Creates a Grove Flow plan with the selected recipe type
3. Passes your configuration to the flow plan via recipe variables

Available recipe types:
- agent: Uses AI agents for interactive customization (default)
- prompts: Uses structured prompts for customization

The resulting flow plan will have jobs specific to the chosen recipe type.

Prerequisites:
- Run 'docgen init' first to create docgen.config.yml
- Ensure 'flow' command is available in your PATH

Examples:
  docgen customize                        # Create plan with default agent recipe
  docgen customize --recipe-type agent    # Create plan with agent recipe
  docgen customize --recipe-type prompts  # Create plan with prompts recipe
  flow run                                # Run the plan after creation

Usage:
  docgen customize [subcommand] [flags]

Flags:
  -h, --help                 help for customize
  -r, --recipe-type string   Recipe type to use: 'agent' or 'prompts' (default "agent")

Global Flags:
  -c, --config string   Path to grove.yml config file
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging
</div>

### docgen generate

<div class="terminal">
Reads the docs/docgen.config.yml in the current directory, builds context, calls an LLM for each section, and writes the output to docs/.

Examples:
  docgen generate                          # Generate all sections
  docgen generate --section introduction   # Generate only introduction
  docgen generate -s intro -s core         # Generate multiple specific sections

Usage:
  docgen generate [flags]

Flags:
  -h, --help              help for generate
  -s, --section strings   Generate only specified sections (by name)

Global Flags:
  -c, --config string   Path to grove.yml config file
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging
</div>

### docgen init

<div class="terminal">
Creates a default docs/docgen.config.yml and a set of starter prompt files in docs/prompts/ based on the specified project type.

This command provides a starting point for your documentation generation. It copies templates into your project, which you fully own and can modify as needed.

It will not overwrite existing files.

Examples:
  docgen init                                    # Initialize with defaults
  docgen init --model gemini-2.0-flash-latest    # Use a specific model
  docgen init --rules-file custom.rules          # Use a custom rules file
  docgen init --output-dir generated-docs        # Output to a different directory

Usage:
  docgen init [flags]

Flags:
  -h, --help                            help for init
      --model string                    LLM model to use for generation
      --output-dir string               Output directory for generated documentation
      --regeneration-mode string        Regeneration mode: scratch or reference
      --rules-file string               Rules file for context generation
      --structured-output-file string   Path for structured JSON output
      --system-prompt string            System prompt: 'default' or path to custom prompt file
      --type string                     Type of project to initialize (e.g., library) (default "library")

Global Flags:
  -c, --config string   Path to grove.yml config file
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging
</div>

### docgen logo

<div class="terminal">
Commands for generating logo assets, including combined logo+text SVGs.

Usage:
  docgen logo [command]

Available Commands:
  generate    Generate a combined logo+text SVG with text converted to paths

Flags:
  -h, --help   help for logo

Global Flags:
  -c, --config string   Path to grove.yml config file
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging

Use "docgen logo [command] --help" for more information about a command.
</div>

#### docgen logo generate

<div class="terminal">
Creates a new SVG that combines an existing logo with text below it.
The text is converted to SVG paths so it renders correctly without requiring
the font to be installed on the viewer's system.

This is useful for generating README-friendly logos that include the product name,
since GitHub READMEs cannot rely on custom fonts being available.

Example:
  docgen logo generate logo-dark.svg --text "grove flow" --font /path/to/FiraCode.ttf --color "#589ac7" -o logo-with-text-dark.svg

Usage:
  docgen logo generate &lt;input-svg&gt; [flags]

Flags:
      --color string       Text color (hex, empty for auto-detect from source SVG)
      --font string        Path to TTF/OTF font file (required)
  -h, --help               help for generate
  -o, --output string      Output path (defaults to input-with-text.svg)
      --size float         Font size in pixels (default 48)
      --spacing float      Spacing between logo and text in pixels (default 20)
      --text string        Text to display below the logo (required)
      --text-scale float   Text width as proportion of logo width (e.g., 1.0 = same width) (default 0.8)
      --width float        Output SVG width in pixels (default 200)

Global Flags:
  -c, --config string   Path to grove.yml config file
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging
</div>

### docgen recipe

<div class="terminal">
Commands for working with documentation generation recipes

Usage:
  docgen recipe [command]

Available Commands:
  print       Print available recipes in JSON format

Flags:
  -h, --help   help for recipe

Global Flags:
  -c, --config string   Path to grove.yml config file
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging

Use "docgen recipe [command] --help" for more information about a command.
</div>

#### docgen recipe print

<div class="terminal">
Print all available documentation recipes in a format suitable for grove-flow integration

Usage:
  docgen recipe print [flags]

Flags:
  -h, --help   help for print

Global Flags:
  -c, --config string   Path to grove.yml config file
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging
</div>

### docgen schema

<div class="terminal">
Provides tools for generating, enriching, and documenting JSON schemas.

Usage:
  docgen schema [command]

Available Commands:
  enrich      Enrich a JSON schema with AI-generated descriptions
  generate    Generate JSON schemas from Go source code

Flags:
  -h, --help   help for schema

Global Flags:
  -c, --config string   Path to grove.yml config file
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging

Use "docgen schema [command] --help" for more information about a command.
</div>

#### docgen schema enrich

<div class="terminal">
Analyzes a JSON schema file, identifies properties lacking descriptions, and uses an LLM with project context to generate and insert those descriptions.

The enriched schema is printed to stdout unless the --in-place flag is used.

Usage:
  docgen schema enrich &lt;path/to/schema.json&gt; [flags]

Flags:
  -h, --help       help for enrich
      --in-place   Modify the schema file directly instead of printing to stdout

Global Flags:
  -c, --config string   Path to grove.yml config file
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging
</div>

#### docgen schema generate

<div class="terminal">
Executes 'go generate ./...' in the current directory.

This command provides a standardized way to trigger schema generation.
It relies on 'go:generate' directives within the Go source code to execute the actual schema generation tools.

Usage:
  docgen schema generate [flags]

Flags:
  -h, --help   help for generate

Global Flags:
  -c, --config string   Path to grove.yml config file
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging
</div>

### docgen sync

<div class="terminal">
Sync commands allow you to copy documentation files between your notebook's
docgen/docs directory and the repository's docs directory.

Use 'sync to-repo' to publish finalized docs from notebook to repository.
Use 'sync from-repo' to import existing docs from repository to notebook.

Usage:
  docgen sync [command]

Available Commands:
  from-repo   Copy docs from repository to notebook
  to-repo     Copy generated docs from notebook to repository

Flags:
  -h, --help   help for sync

Global Flags:
  -c, --config string   Path to grove.yml config file
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging

Use "docgen sync [command] --help" for more information about a command.
</div>

### docgen version

<div class="terminal">
Print the version information for this binary

Usage:
  docgen version [flags]

Flags:
  -h, --help   help for version
      --json   Output version information in JSON format

Global Flags:
  -c, --config string   Path to grove.yml config file
  -v, --verbose         Enable verbose logging
</div>

### docgen watch

<div class="terminal">
Watches all docgen directories in configured ecosystems and
rebuilds changed packages incrementally. Astro's dev server will
pick up the changes automatically via HMR.

Example:
  docgen watch --website-dir . --mode dev --quiet

The watch command will:
1. Discover all packages with docgen enabled in configured ecosystems
2. Watch their notebook docgen directories for changes
3. On file change, rebuild only the affected package
4. Write output directly to the Astro content directories

Usage:
  docgen watch [flags]

Flags:
      --debounce int         Debounce interval in milliseconds (default 100)
  -h, --help                 help for watch
      --mode string          Build mode: dev or prod (default "dev")
      --quiet                Minimal output (for concurrent use with astro)
      --website-dir string   Path to grove-website root (default ".")

Global Flags:
  -c, --config string   Path to grove.yml config file
      --json            Output in JSON format
  -v, --verbose         Enable verbose logging
</div>

