package commands

import (
	"github.com/supah-seo/supah-seo/internal/common/config"
	"github.com/supah-seo/supah-seo/internal/provider"
	_ "github.com/supah-seo/supah-seo/internal/provider/local"
	"github.com/supah-seo/supah-seo/pkg/output"
	"github.com/spf13/cobra"
)

// NewProviderCmd returns the provider command group.
func NewProviderCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Provider management commands",
	}

	cmd.AddCommand(
		newProviderListCmd(format, verbose),
		newProviderUseCmd(format, verbose),
	)
	return cmd
}

func newProviderListCmd(format *string, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			available := provider.Available()
			type providerInfo struct {
				Name   string `json:"name"`
				Active bool   `json:"active"`
			}

			providers := make([]providerInfo, 0, len(available))
			for _, name := range available {
				providers = append(providers, providerInfo{
					Name:   name,
					Active: name == cfg.ActiveProvider,
				})
			}

			return output.PrintSuccess(providers, map[string]any{
				"count":   len(providers),
				"verbose": *verbose,
			}, output.Format(*format))
		},
	}
}

func newProviderUseCmd(format *string, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set the active provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Validate the provider exists
			if _, err := provider.NewFetcher(name); err != nil {
				return output.PrintCodedError(output.ErrProviderNotFound, "invalid provider", err, nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			cfg.ActiveProvider = name
			if err := cfg.Save(); err != nil {
				return output.PrintCodedError(output.ErrConfigSaveFailed, "failed to save config", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(map[string]any{
				"active_provider": name,
				"status":          "ok",
			}, map[string]any{
				"verbose": *verbose,
			}, output.Format(*format))
		},
	}
}
