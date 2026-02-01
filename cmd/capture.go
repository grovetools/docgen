package cmd

import (
	"fmt"
	"os/exec"

	"github.com/grovetools/docgen/pkg/capture"
	"github.com/spf13/cobra"
)

func newCaptureCmd() *cobra.Command {
	var output string
	var depth int
	var format string

	cmd := &cobra.Command{
		Use:   "capture <binary>",
		Short: "Capture help output for all commands in a binary",
		Long: `Recursively executes a binary with --help to capture and compile a complete command reference.

This is useful for generating documentation for CLI tools that use Cobra or similar frameworks.
It parses the "COMMANDS" section of the help output to discover subcommands.

Output formats:
  markdown  Plain text in markdown code blocks (default)
  html      Styled HTML with terminal colors preserved

Examples:
  docgen capture nb --output docs/commands.md
  docgen capture grove -o commands.html --format html
  docgen capture grove -o commands.md --depth 3`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			binary := args[0]

			// Verify binary exists
			if _, err := exec.LookPath(binary); err != nil {
				return fmt.Errorf("binary '%s' not found in PATH", binary)
			}

			// Determine format
			var captureFormat capture.Format
			switch format {
			case "html":
				captureFormat = capture.FormatHTML
				if output == "" {
					output = "commands.html"
				}
			default:
				captureFormat = capture.FormatMarkdown
				if output == "" {
					output = "commands.md"
				}
			}

			ulog.Info("Capturing command reference").
				Field("binary", binary).
				Field("format", format).
				Field("output", output).
				Emit()

			capturer := capture.New(getLogger())
			opts := capture.Options{
				MaxDepth: depth,
				Format:   captureFormat,
			}

			if err := capturer.Capture(binary, output, opts); err != nil {
				return err
			}

			ulog.Success("Command reference generated").
				Field("file", output).
				Emit()

			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file (default: commands.md or commands.html)")
	cmd.Flags().IntVarP(&depth, "depth", "d", 5, "Maximum recursion depth")
	cmd.Flags().StringVarP(&format, "format", "f", "markdown", "Output format: markdown, html")

	return cmd
}
