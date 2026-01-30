package cmd

import (
	"path/filepath"
	"strings"

	"github.com/grovetools/docgen/pkg/logo"
	"github.com/spf13/cobra"
)

func newLogoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logo",
		Short: "Logo asset generation commands",
		Long:  `Commands for generating logo assets, including combined logo+text SVGs.`,
	}

	cmd.AddCommand(newLogoGenerateCmd())

	return cmd
}

func newLogoGenerateCmd() *cobra.Command {
	var (
		text      string
		textColor string
		fontPath  string
		fontSize  float64
		spacing   float64
		textScale float64
		width     float64
		output    string
	)

	cmd := &cobra.Command{
		Use:   "generate <input-svg>",
		Short: "Generate a combined logo+text SVG with text converted to paths",
		Long: `Creates a new SVG that combines an existing logo with text below it.
The text is converted to SVG paths so it renders correctly without requiring
the font to be installed on the viewer's system.

This is useful for generating README-friendly logos that include the product name,
since GitHub READMEs cannot rely on custom fonts being available.

Example:
  docgen logo generate logo-dark.svg --text "grove flow" --font /path/to/FiraCode.ttf --color "#589ac7" -o logo-with-text-dark.svg`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputPath := args[0]

			// Default output path based on input
			if output == "" {
				ext := filepath.Ext(inputPath)
				base := strings.TrimSuffix(inputPath, ext)
				output = base + "-with-text" + ext
			}

			cfg := logo.Config{
				InputPath:  inputPath,
				OutputPath: output,
				Text:       text,
				TextColor:  textColor,
				FontPath:   fontPath,
				FontSize:   fontSize,
				Spacing:    spacing,
				TextScale:  textScale,
				Width:      width,
			}

			gen := logo.New(getLogger())
			if err := gen.Generate(cfg); err != nil {
				return err
			}

			ulog.Success("Generated logo with text paths").Field("output", output).Emit()
			return nil
		},
	}

	cmd.Flags().StringVar(&text, "text", "", "Text to display below the logo (required)")
	cmd.Flags().StringVar(&textColor, "color", "#589ac7", "Text color (hex)")
	cmd.Flags().StringVar(&fontPath, "font", "", "Path to TTF/OTF font file (required)")
	cmd.Flags().Float64Var(&fontSize, "size", 48, "Font size in pixels")
	cmd.Flags().Float64Var(&spacing, "spacing", 20, "Spacing between logo and text in pixels")
	cmd.Flags().Float64Var(&textScale, "text-scale", 0.8, "Text width as proportion of logo width (e.g., 1.0 = same width)")
	cmd.Flags().Float64Var(&width, "width", 200, "Output SVG width in pixels")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output path (defaults to input-with-text.svg)")

	cmd.MarkFlagRequired("text")
	cmd.MarkFlagRequired("font")

	return cmd
}
