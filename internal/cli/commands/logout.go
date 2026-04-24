package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/supah-seo/supah-seo/internal/auth"
	"github.com/supah-seo/supah-seo/internal/common/config"
)

// NewLogoutCmd returns the top-level logout command that clears all credentials.
func NewLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear all stored credentials and API keys",
		Long:  `Remove all stored OAuth tokens and sensitive configuration keys (SerpAPI key, GSC credentials).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogout()
		},
	}
}

func runLogout() error {
	// Delete OAuth tokens
	store := auth.NewFileTokenStore()
	services := []string{"gsc"}
	for _, svc := range services {
		if err := store.Delete(svc); err != nil {
			return fmt.Errorf("failed to remove %s token: %w", svc, err)
		}
	}

	// Clear sensitive config keys
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	_ = cfg.Set("serp_api_key", "")
	_ = cfg.Set("dataforseo_login", "")
	_ = cfg.Set("dataforseo_password", "")
	_ = cfg.Set("gsc_client_id", "")
	_ = cfg.Set("gsc_client_secret", "")

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("  ✓ All credentials cleared")
	return nil
}
