package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Config stores local Supah SEO settings.
type Config struct {
	ActiveProvider       string  `json:"active_provider"`
	APIKey               string  `json:"api_key"`
	BaseURL              string  `json:"base_url"`
	OrganizationID       string  `json:"organization_id"`
	SERPProvider         string  `json:"serp_provider"`
	SERPAPIKey           string  `json:"serp_api_key"`
	DataForSEOLogin      string  `json:"dataforseo_login"`
	DataForSEOPassword   string  `json:"dataforseo_password"`
	ApprovalThresholdUSD float64 `json:"approval_threshold_usd"`
	GSCProperty          string  `json:"gsc_property"`
	GSCClientID          string  `json:"gsc_client_id"`
	GSCClientSecret      string  `json:"gsc_client_secret"`
	PSIAPIKey            string  `json:"psi_api_key"`

	// Branding profiles. Each profile bundles agency identity fields
	// so reports ship with matched name + logo + CTA, avoiding mixed-brand
	// white-label mistakes. DefaultProfile names the profile used when
	// --profile is not passed.
	DefaultProfile string             `json:"default_profile,omitempty"`
	Profiles       map[string]Profile `json:"profiles,omitempty"`
}

// Profile bundles branding fields applied to generated reports.
type Profile struct {
	AgencyName string `json:"agency_name"`
	Logo       string `json:"logo,omitempty"`
	CTAURL     string `json:"cta_url,omitempty"`
	CTALabel   string `json:"cta_label,omitempty"`
}

// ResolveProfile returns the named profile, or the DefaultProfile if name is
// empty, or the zero Profile if no default is set. Returns an error only when
// a specific profile name is requested but not defined in the config.
func (c *Config) ResolveProfile(name string) (Profile, error) {
	if name == "" {
		name = c.DefaultProfile
	}
	if name == "" {
		return Profile{}, nil
	}
	p, ok := c.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("profile %q not found in config (available: %s)", name, strings.Join(profileNames(c.Profiles), ", "))
	}
	return p, nil
}

func profileNames(m map[string]Profile) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

var (
	resolvedConfigPath string
	resolvePathOnce    sync.Once
)

func resetPathCacheForTest() {
	resolvedConfigPath = ""
	resolvePathOnce = sync.Once{}
}

// Path returns the resolved config file path.
func Path() string {
	resolvePathOnce.Do(func() {
		if p := os.Getenv("SUPAHSEO_CONFIG"); p != "" {
			clean := filepath.Clean(p)
			if filepath.IsAbs(clean) && strings.HasSuffix(clean, ".json") && !strings.Contains(clean, "..") {
				resolvedConfigPath = clean
				return
			}
		}

		home, err := os.UserHomeDir()
		if err != nil {
			resolvedConfigPath = "config.json"
			return
		}

		resolvedConfigPath = filepath.Join(home, ".config", "supah-seo", "config.json")
	})

	return resolvedConfigPath
}

// Load reads config from disk. Missing file returns an empty config.
func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		cfg := NewDefault()
		cfg.applyEnvOverrides()
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	cfg.applyEnvOverrides()
	return &cfg, nil
}

// NewDefault returns default placeholder values for scaffold phase.
func NewDefault() *Config {
	return &Config{
		ActiveProvider:       "local",
		BaseURL:              "",
		SERPProvider:         "serpapi",
		ApprovalThresholdUSD: 0,
	}
}

// Save persists config to disk with restrictive permissions.
func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(Path()), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	body, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(Path(), body, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// Set updates a named key.
func (c *Config) Set(key, value string) error {
	switch strings.ToLower(key) {
	case "active_provider", "active-provider", "provider":
		c.ActiveProvider = value
	case "api_key", "api-key", "apikey":
		c.APIKey = value
	case "base_url", "base-url":
		c.BaseURL = value
	case "organization_id", "organization-id", "org_id", "org-id":
		c.OrganizationID = value
	case "serp_provider", "serp-provider":
		c.SERPProvider = value
	case "serp_api_key", "serp-api-key", "serpapikey":
		c.SERPAPIKey = value
	case "dataforseo_login", "dataforseo-login":
		c.DataForSEOLogin = value
	case "dataforseo_password", "dataforseo-password":
		c.DataForSEOPassword = value
	case "approval_threshold_usd", "approval-threshold-usd":
		threshold, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid approval_threshold_usd: %w", err)
		}
		c.ApprovalThresholdUSD = threshold
	case "gsc_property", "gsc-property":
		c.GSCProperty = value
	case "gsc_client_id", "gsc-client-id":
		c.GSCClientID = value
	case "gsc_client_secret", "gsc-client-secret":
		c.GSCClientSecret = value
	case "psi_api_key", "psi-api-key":
		c.PSIAPIKey = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return nil
}

// Get reads a named key and redacts secrets where needed.
func (c *Config) Get(key string) (string, error) {
	switch strings.ToLower(key) {
	case "active_provider", "active-provider", "provider":
		return c.ActiveProvider, nil
	case "api_key", "api-key", "apikey":
		return redact(c.APIKey), nil
	case "base_url", "base-url":
		return c.BaseURL, nil
	case "organization_id", "organization-id", "org_id", "org-id":
		return c.OrganizationID, nil
	case "serp_provider", "serp-provider":
		return c.SERPProvider, nil
	case "serp_api_key", "serp-api-key", "serpapikey":
		return redact(c.SERPAPIKey), nil
	case "dataforseo_login", "dataforseo-login":
		return c.DataForSEOLogin, nil
	case "dataforseo_password", "dataforseo-password":
		return redact(c.DataForSEOPassword), nil
	case "approval_threshold_usd", "approval-threshold-usd":
		return strconv.FormatFloat(c.ApprovalThresholdUSD, 'f', -1, 64), nil
	case "gsc_property", "gsc-property":
		return c.GSCProperty, nil
	case "gsc_client_id", "gsc-client-id":
		return redact(c.GSCClientID), nil
	case "gsc_client_secret", "gsc-client-secret":
		return redact(c.GSCClientSecret), nil
	case "psi_api_key", "psi-api-key":
		return redact(c.PSIAPIKey), nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

// Redacted returns map output safe for display.
func (c *Config) Redacted() map[string]any {
	out := map[string]any{
		"active_provider":        c.ActiveProvider,
		"api_key":                redact(c.APIKey),
		"base_url":               c.BaseURL,
		"organization_id":        c.OrganizationID,
		"serp_provider":          c.SERPProvider,
		"serp_api_key":           redact(c.SERPAPIKey),
		"dataforseo_login":       c.DataForSEOLogin,
		"dataforseo_password":    redact(c.DataForSEOPassword),
		"approval_threshold_usd": c.ApprovalThresholdUSD,
		"gsc_property":           c.GSCProperty,
		"gsc_client_id":          redact(c.GSCClientID),
		"gsc_client_secret":      redact(c.GSCClientSecret),
		"psi_api_key":            redact(c.PSIAPIKey),
	}
	if c.DefaultProfile != "" || len(c.Profiles) > 0 {
		out["default_profile"] = c.DefaultProfile
		out["profiles"] = c.Profiles
	}
	return out
}

func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("SUPAHSEO_PROVIDER"); v != "" {
		c.ActiveProvider = v
	}
	if v := os.Getenv("SUPAHSEO_API_KEY"); v != "" {
		c.APIKey = v
	}
	if v := os.Getenv("SUPAHSEO_BASE_URL"); v != "" {
		c.BaseURL = v
	}
	if v := os.Getenv("SUPAHSEO_ORGANIZATION_ID"); v != "" {
		c.OrganizationID = v
	}
	if v := os.Getenv("SUPAHSEO_SERP_PROVIDER"); v != "" {
		c.SERPProvider = v
	}
	if v := os.Getenv("SUPAHSEO_SERP_API_KEY"); v != "" {
		c.SERPAPIKey = v
	}
	if v := os.Getenv("SUPAHSEO_DATAFORSEO_LOGIN"); v != "" {
		c.DataForSEOLogin = v
	}
	if v := os.Getenv("SUPAHSEO_DATAFORSEO_PASSWORD"); v != "" {
		c.DataForSEOPassword = v
	}
	if v := os.Getenv("SUPAHSEO_APPROVAL_THRESHOLD_USD"); v != "" {
		if threshold, err := strconv.ParseFloat(v, 64); err == nil {
			c.ApprovalThresholdUSD = threshold
		}
	}
	if v := os.Getenv("SUPAHSEO_GSC_PROPERTY"); v != "" {
		c.GSCProperty = v
	}
	if v := os.Getenv("SUPAHSEO_GSC_CLIENT_ID"); v != "" {
		c.GSCClientID = v
	}
	if v := os.Getenv("SUPAHSEO_GSC_CLIENT_SECRET"); v != "" {
		c.GSCClientSecret = v
	}
	if v := os.Getenv("SUPAHSEO_PSI_API_KEY"); v != "" {
		c.PSIAPIKey = v
	}
	if v := os.Getenv("SUPAHSEO_PROFILE"); v != "" {
		c.DefaultProfile = v
	}
}

func redact(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "****" + value[len(value)-4:]
}
