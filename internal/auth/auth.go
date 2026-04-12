package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/supah-seo/supah-seo/internal/common/config"
)

// TokenRecord stores an OAuth2 token on disk.
type TokenRecord struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// Status describes the current authentication state for a service.
type Status struct {
	Service       string `json:"service"`
	Authenticated bool   `json:"authenticated"`
	ExpiresAt     string `json:"expires_at,omitempty"`
	TokenPath     string `json:"token_path"`
}

// TokenStore defines the interface for token persistence.
type TokenStore interface {
	Save(service string, token TokenRecord) error
	Load(service string) (TokenRecord, error)
	Delete(service string) error
	Status(service string) (Status, error)
}

// FileTokenStore persists tokens to disk under the Supah SEO config directory.
type FileTokenStore struct {
	baseDir string
	nowFunc func() time.Time
}

// NewFileTokenStore creates a store under ~/.config/supah-seo/auth/.
func NewFileTokenStore() *FileTokenStore {
	baseDir := filepath.Join(filepath.Dir(config.Path()), "auth")
	return &FileTokenStore{
		baseDir: baseDir,
		nowFunc: time.Now,
	}
}

// Save writes a token record for the given service.
func (s *FileTokenStore) Save(service string, token TokenRecord) error {
	path := s.tokenPath(service)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating auth dir: %w", err)
	}

	body, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding token: %w", err)
	}

	if err := os.WriteFile(path, body, 0o600); err != nil {
		return fmt.Errorf("writing token: %w", err)
	}

	return nil
}

// Load reads a stored token for the given service.
func (s *FileTokenStore) Load(service string) (TokenRecord, error) {
	path := s.tokenPath(service)

	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return TokenRecord{}, fmt.Errorf("not authenticated with %s", service)
	}
	if err != nil {
		return TokenRecord{}, fmt.Errorf("reading token: %w", err)
	}

	var token TokenRecord
	if err := json.Unmarshal(body, &token); err != nil {
		return TokenRecord{}, fmt.Errorf("decoding token: %w", err)
	}

	return token, nil
}

// Delete removes a stored token for the given service.
func (s *FileTokenStore) Delete(service string) error {
	path := s.tokenPath(service)

	if err := os.Remove(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("removing token: %w", err)
	}

	return nil
}

// Status checks the current auth state for a service.
func (s *FileTokenStore) Status(service string) (Status, error) {
	path := s.tokenPath(service)
	st := Status{
		Service:   service,
		TokenPath: path,
	}

	token, err := s.Load(service)
	if err != nil {
		st.Authenticated = false
		return st, nil
	}

	st.Authenticated = true
	st.ExpiresAt = token.ExpiresAt

	if token.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, token.ExpiresAt)
		if err == nil && s.nowFunc().After(expiresAt) {
			st.Authenticated = false
		}
	}

	return st, nil
}

func (s *FileTokenStore) tokenPath(service string) string {
	return filepath.Join(s.baseDir, service+".json")
}

// RefreshGSCToken uses the stored refresh token to obtain a new access token
// from Google's OAuth2 token endpoint. It updates and persists the token on success.
func (s *FileTokenStore) RefreshGSCToken(clientID, clientSecret string) (TokenRecord, error) {
	token, err := s.Load("gsc")
	if err != nil {
		return TokenRecord{}, fmt.Errorf("loading token for refresh: %w", err)
	}
	if token.RefreshToken == "" {
		return TokenRecord{}, fmt.Errorf("no refresh token available — re-authenticate with 'supah-seo login'")
	}

	data := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {token.RefreshToken},
		"grant_type":    {"refresh_token"},
	}

	resp, err := http.Post("https://oauth2.googleapis.com/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return TokenRecord{}, fmt.Errorf("refresh request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenRecord{}, fmt.Errorf("reading refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return TokenRecord{}, fmt.Errorf("token refresh failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return TokenRecord{}, fmt.Errorf("decoding refresh response: %w", err)
	}

	token.AccessToken = result.AccessToken
	token.TokenType = result.TokenType
	if result.Scope != "" {
		token.Scope = result.Scope
	}
	token.ExpiresAt = s.nowFunc().Add(time.Duration(result.ExpiresIn) * time.Second).Format(time.RFC3339)

	if err := s.Save("gsc", token); err != nil {
		return TokenRecord{}, fmt.Errorf("persisting refreshed token: %w", err)
	}

	return token, nil
}
