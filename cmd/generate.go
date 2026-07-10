package cmd

import (
	"os"

	"github.com/grovetools/docgen/pkg/generator"
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	var (
		sections  []string
		model     string
		cacheTTL  string
		usageJSON string
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate documentation for the current package",
		Long: `Reads the docs/docgen.config.yml in the current directory, builds context, calls an LLM for each section, and writes the output to docs/.

When --model is a Claude model (or settings.cache_fanout is set), the repo's cx
context is built once and cached as a shared Anthropic prompt prefix; each
section's request rides that cached prefix (cache_read ≈ 0.1x cost) instead of
shelling grove llm request. Non-Claude models keep the standard path.

Examples:
  docgen generate                                  # Generate all sections
  docgen generate --section introduction           # Generate only introduction
  docgen generate -s intro -s core                 # Generate multiple specific sections
  docgen generate --model claude-haiku-4-5         # Claude cache fan-out for all sections
  docgen generate --model claude-haiku-4-5 --cache-ttl 1h`,
		// A generation failure is a runtime error, not a usage error — dumping
		// the flag reference after "15 section(s) failed" buries the cause.
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			gen := generator.New(getLogger())

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			opts := generator.GenerateOptions{
				Sections:      sections,
				Model:         model,
				CacheTTL:      cacheTTL,
				UsageJSONPath: usageJSON,
			}
			return gen.GenerateWithOptions(cwd, opts)
		},
	}

	cmd.Flags().StringSliceVarP(&sections, "section", "s", nil, "Generate only specified sections (by name)")
	cmd.Flags().StringVar(&model, "model", "", "Override the model for all sections; a claude-* model enables the shared-prefix cache fan-out")
	cmd.Flags().StringVar(&cacheTTL, "cache-ttl", "", "Cache TTL for the fan-out shared prefix: 5m (default) or 1h")
	cmd.Flags().StringVar(&usageJSON, "usage-json", "", "Write a machine-readable per-section cache/usage report (JSON) to this file at end of run")

	return cmd
}
