package commands

import (
	"runtime"
	"time"

	"github.com/supah-seo/supah-seo/pkg/output"
	"github.com/spf13/cobra"
)

// NewVersionCmd returns the version command.
func NewVersionCmd(version string, format *string, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Supah SEO version and build metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			data := map[string]any{
				"version": version,
				"go":      runtime.Version(),
			}
			metadata := map[string]any{
				"verbose":      *verbose,
				"generated_at": time.Now().UTC().Format(time.RFC3339),
			}
			return output.PrintSuccess(data, metadata, output.Format(*format))
		},
	}
}
