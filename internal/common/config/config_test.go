package config

import (
	"path/filepath"
	"testing"
)

func TestPathUsesEnvOverride(t *testing.T) {
	resetPathCacheForTest()

	configPath := filepath.Join(t.TempDir(), "supah-seo-config.json")
	t.Setenv("SUPAHSEO_CONFIG", configPath)

	got := Path()
	if got != configPath {
		t.Fatalf("expected %q, got %q", configPath, got)
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	resetPathCacheForTest()

	configPath := filepath.Join(t.TempDir(), "supah-seo-config.json")
	t.Setenv("SUPAHSEO_CONFIG", configPath)

	cfg := NewDefault()
	cfg.ActiveProvider = "cloudflare"
	cfg.APIKey = "secret-token-value"
	cfg.BaseURL = "https://api.example.com"
	cfg.OrganizationID = "org_123"
	cfg.SERPProvider = "serpapi"
	cfg.SERPAPIKey = "serp-secret-key"
	cfg.ApprovalThresholdUSD = 2.5
	cfg.GSCProperty = "https://example.com/"
	cfg.GSCClientID = "client-id-value"
	cfg.GSCClientSecret = "client-secret-value"

	if err := cfg.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.ActiveProvider != cfg.ActiveProvider {
		t.Fatalf("expected active provider %q, got %q", cfg.ActiveProvider, loaded.ActiveProvider)
	}
	if loaded.APIKey != cfg.APIKey {
		t.Fatalf("expected api key %q, got %q", cfg.APIKey, loaded.APIKey)
	}
	if loaded.BaseURL != cfg.BaseURL {
		t.Fatalf("expected base url %q, got %q", cfg.BaseURL, loaded.BaseURL)
	}
	if loaded.OrganizationID != cfg.OrganizationID {
		t.Fatalf("expected organization id %q, got %q", cfg.OrganizationID, loaded.OrganizationID)
	}
	if loaded.SERPProvider != cfg.SERPProvider {
		t.Fatalf("expected serp provider %q, got %q", cfg.SERPProvider, loaded.SERPProvider)
	}
	if loaded.SERPAPIKey != cfg.SERPAPIKey {
		t.Fatalf("expected serp api key %q, got %q", cfg.SERPAPIKey, loaded.SERPAPIKey)
	}
	if loaded.ApprovalThresholdUSD != cfg.ApprovalThresholdUSD {
		t.Fatalf("expected approval threshold %v, got %v", cfg.ApprovalThresholdUSD, loaded.ApprovalThresholdUSD)
	}
	if loaded.GSCProperty != cfg.GSCProperty {
		t.Fatalf("expected gsc property %q, got %q", cfg.GSCProperty, loaded.GSCProperty)
	}
	if loaded.GSCClientID != cfg.GSCClientID {
		t.Fatalf("expected gsc client id %q, got %q", cfg.GSCClientID, loaded.GSCClientID)
	}
	if loaded.GSCClientSecret != cfg.GSCClientSecret {
		t.Fatalf("expected gsc client secret %q, got %q", cfg.GSCClientSecret, loaded.GSCClientSecret)
	}
}

func TestEnvOverridesAndRedaction(t *testing.T) {
	resetPathCacheForTest()

	configPath := filepath.Join(t.TempDir(), "supah-seo-config.json")
	t.Setenv("SUPAHSEO_CONFIG", configPath)
	t.Setenv("SUPAHSEO_SERP_PROVIDER", "serpapi")
	t.Setenv("SUPAHSEO_SERP_API_KEY", "serp-override-key")
	t.Setenv("SUPAHSEO_APPROVAL_THRESHOLD_USD", "5.25")
	t.Setenv("SUPAHSEO_GSC_PROPERTY", "sc-domain:example.com")
	t.Setenv("SUPAHSEO_GSC_CLIENT_ID", "override-client-id")
	t.Setenv("SUPAHSEO_GSC_CLIENT_SECRET", "override-client-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.SERPProvider != "serpapi" {
		t.Fatalf("expected serp provider override, got %q", cfg.SERPProvider)
	}
	if cfg.SERPAPIKey != "serp-override-key" {
		t.Fatalf("expected serp key override, got %q", cfg.SERPAPIKey)
	}
	if cfg.ApprovalThresholdUSD != 5.25 {
		t.Fatalf("expected threshold override 5.25, got %v", cfg.ApprovalThresholdUSD)
	}
	if cfg.GSCProperty != "sc-domain:example.com" {
		t.Fatalf("expected gsc property override, got %q", cfg.GSCProperty)
	}

	redacted := cfg.Redacted()
	if redacted["serp_api_key"] == cfg.SERPAPIKey {
		t.Fatalf("expected serp api key to be redacted")
	}
	if redacted["gsc_client_secret"] == cfg.GSCClientSecret {
		t.Fatalf("expected gsc client secret to be redacted")
	}
}

func TestSetGetExtendedKeys(t *testing.T) {
	cfg := NewDefault()

	if err := cfg.Set("approval_threshold_usd", "1.75"); err != nil {
		t.Fatalf("set threshold failed: %v", err)
	}
	if err := cfg.Set("serp_api_key", "serp-secret"); err != nil {
		t.Fatalf("set serp key failed: %v", err)
	}
	if err := cfg.Set("gsc_client_secret", "gsc-secret"); err != nil {
		t.Fatalf("set gsc secret failed: %v", err)
	}

	threshold, err := cfg.Get("approval_threshold_usd")
	if err != nil {
		t.Fatalf("get threshold failed: %v", err)
	}
	if threshold != "1.75" {
		t.Fatalf("expected threshold 1.75, got %q", threshold)
	}

	serpKey, err := cfg.Get("serp_api_key")
	if err != nil {
		t.Fatalf("get serp key failed: %v", err)
	}
	if serpKey == "serp-secret" {
		t.Fatalf("expected redacted serp key")
	}

	gscSecret, err := cfg.Get("gsc_client_secret")
	if err != nil {
		t.Fatalf("get gsc secret failed: %v", err)
	}
	if gscSecret == "gsc-secret" {
		t.Fatalf("expected redacted gsc secret")
	}
}
