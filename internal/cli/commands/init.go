package commands

import (
	"github.com/supah-seo/supah-seo/internal/state"
	"github.com/supah-seo/supah-seo/pkg/output"
	"github.com/spf13/cobra"
)

// NewInitCmd creates the init command.
func NewInitCmd(format *string, verbose *bool) *cobra.Command {
	var siteURL string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a .supah-seo project for a site",
		RunE: func(cmd *cobra.Command, args []string) error {
			if siteURL == "" {
				return output.PrintCodedError(output.ErrInvalidURL, "--url is required", nil, nil, output.Format(*format))
			}

			s, err := state.Init(".", siteURL)
			if err != nil {
				return output.PrintErrorResponse(err.Error(), err, nil, output.Format(*format))
			}

			return output.PrintSuccess(map[string]interface{}{
				"site":        s.Site,
				"initialized": s.Initialized,
				"state_file":  state.Path("."),
			}, nil, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&siteURL, "url", "", "Site URL to track")
	return cmd
}
