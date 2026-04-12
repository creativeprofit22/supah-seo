package cli

import (
	"os"

	"github.com/supah-seo/supah-seo/internal/cli/commands"
	"github.com/spf13/cobra"
)

var (
	outputFormat string
	verbose      bool
)

// Execute runs the root command.
func Execute(version string) error {
	root := newRootCmd(version)
	// Commands print their own structured error envelopes via PrintCodedError.
	// SilenceErrors prevents Cobra from printing raw errors.
	// We return the error for exit code purposes only — no double printing.
	return root.Execute()
}

func newRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "supah-seo",
		Short: "Supah SEO for SEO crawling, auditing, and reporting",
		Long: `supah-seo is a command-line tool for SEO, GEO, and AEO operations.

Crawl websites, run SEO audits, generate reports, and manage providers.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "json", "Output format: json, text, table")
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	cmd.SetErr(os.Stderr)
	cmd.SetOut(os.Stdout)

	cmd.AddCommand(commands.NewVersionCmd(version, &outputFormat, &verbose))
	cmd.AddCommand(commands.NewConfigCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewCrawlCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewAuditCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewReportCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewProviderCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewAuthCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewGSCCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewSERPCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewOpportunitiesCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewAEOCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewGEOCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewLabsCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewBacklinksCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewLoginCmd(version, &outputFormat, &verbose))
	cmd.AddCommand(commands.NewLogoutCmd())
	cmd.AddCommand(commands.NewInitCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewStatusCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewAnalyzeCmd(&outputFormat, &verbose))
	cmd.AddCommand(commands.NewPSICmd(&outputFormat, &verbose))

	return cmd
}
