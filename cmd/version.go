package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/mattsolo1/grove-core/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information for this binary",
		RunE: func(cmd *cobra.Command, args []string) error {
			
			info := version.GetInfo()

			if jsonOutput {
				jsonData, err := json.MarshalIndent(info, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal version info to JSON: %w", err)
				}
				ulog.Info("Version information").
					Field("version", info.Version).
					Field("commit", info.Commit).
					Field("build_date", info.BuildDate).
					PrettyOnly().
					Pretty(string(jsonData)).
					Emit()
			} else {
				ulog.Info("Version information").
					Field("version", info.Version).
					Field("commit", info.Commit).
					Field("build_date", info.BuildDate).
					PrettyOnly().
					Pretty(info.String()).
					Emit()
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output version information in JSON format")

	return cmd
}
