package cmd

import (
	"os"

	"github.com/grovetools/docgen/pkg/generator"
	"github.com/spf13/cobra"
)

func newProposeCmd() *cobra.Command {
	var (
		model      string
		cacheTTL   string
		outputDir  string
		usageJSON  string
		dryRun     bool
		fresh      bool
		followup   string
		transcript string
	)

	cmd := &cobra.Command{
		Use:   "propose",
		Short: "Propose an updated docs outline (sections + prompts) as a reviewable bundle",
		Long: `Run a "turn 0" for docs regeneration: warm the SAME cached cx-context prefix
the docs fan-out uses, send ONE request that proposes an updated documentation
outline, and write a reviewable proposal bundle to --output-dir.

The request rides the byte-identical shared prefix that 'docgen generate' warms,
so after you review and edit the proposed prompts + config, a later
'docgen generate' (same repo, same claude model, within the cache TTL) cache-READs
the prefix this proposal already warmed instead of re-paying for it. Because
review takes time, --cache-ttl defaults to 1h.

The request SUFFIX (never the cached prefix) carries the repo's current
docgen.config.yml, its current prompt files, and its README template, plus an
instruction to propose: an updated section list (adds/removes/merges with
reasons), a full draft prompt for every prose section, and an overall rationale.

The bundle written to --output-dir:
  PROPOSAL.md                  rationale + proposed outline table
  proposed.docgen.config.yml   a complete, valid config (current settings kept)
  prompts/<nn>-<name>.md       one draft prompt per prose section

A transcript.json recording the exact turns is written alongside the bundle, so
a later --followup can replay the dialogue and refine the proposal.

The live notebook config/prompts are never overwritten.

Claude models only — a non-claude --model errors, because the point is the shared
cache.

Modes:
  (default)   evolve the current outline: the suffix carries the current config,
              prompts, and README template.
  --fresh     green-field outline from the code alone: the suffix carries only
              the current settings (sections/prompts/README withheld) so the
              proposal is not anchored to today's outline. Excludes --followup.
  --followup  refine a PRIOR proposal in a second turn: replay --transcript's
              turns (same --model required) and add the given feedback as a new
              user turn, re-emitting the complete proposal. Excludes --fresh.

Examples:
  docgen propose --output-dir ./proposal --model claude-haiku-4-5
  docgen propose --output-dir ./proposal --model claude-haiku-4-5 --cache-ttl 1h
  docgen propose --output-dir ./proposal --dry-run    # assemble suffix, no API call
  docgen propose --output-dir ./p2 --model claude-haiku-4-5 --fresh   # green-field outline
  docgen propose --output-dir ./p2 --model claude-haiku-4-5 \
    --followup "merge the CLI pages" --transcript ./proposal/transcript.json`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			gen := generator.New(getLogger())

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			return gen.Propose(cwd, generator.ProposeOptions{
				Model:          model,
				CacheTTL:       cacheTTL,
				OutputDir:      outputDir,
				UsageJSONPath:  usageJSON,
				DryRun:         dryRun,
				Fresh:          fresh,
				Followup:       followup,
				TranscriptPath: transcript,
			})
		},
	}

	cmd.Flags().StringVar(&model, "model", "", "Claude model whose cache the proposal warms (must match a later generate); empty ⇒ settings.model")
	cmd.Flags().StringVar(&cacheTTL, "cache-ttl", "", "Shared-prefix cache TTL: 5m or 1h (default 1h — review outlasts 5m)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Directory to write the proposal bundle to (required)")
	cmd.Flags().StringVar(&usageJSON, "usage-json", "", "Write a machine-readable cache/usage report (JSON) to this file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Assemble and save the request suffix without any API call")
	cmd.Flags().BoolVar(&fresh, "fresh", false, "Green-field: propose from the code alone (withhold the current sections/prompts/README); excludes --followup")
	cmd.Flags().StringVar(&followup, "followup", "", "Reviewer feedback that refines a prior proposal in a second turn (requires --transcript); excludes --fresh")
	cmd.Flags().StringVar(&transcript, "transcript", "", "Path to a prior run's transcript.json to replay for --followup")

	_ = cmd.MarkFlagRequired("output-dir")

	return cmd
}
