package commands

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/supah-seo/supah-seo/internal/auth"
	"github.com/supah-seo/supah-seo/internal/common/config"
	"github.com/supah-seo/supah-seo/pkg/output"
)

// NewAuthCmd returns the auth command group.
func NewAuthCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication for external services",
		Long:  `Login, logout, and check authentication status for services like Google Search Console.`,
	}

	cmd.AddCommand(
		newAuthLoginCmd(format, verbose),
		newAuthStatusCmd(format, verbose),
		newAuthLogoutCmd(format, verbose),
	)

	return cmd
}

func newAuthLoginCmd(format *string, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "login <service>",
		Short: "Authenticate with a service (e.g. gsc)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service := args[0]
			switch service {
			case "gsc":
				return loginGSC(format, verbose)
			default:
				return output.PrintCodedError(output.ErrAuthFailed, "unsupported service", fmt.Errorf("unknown service: %s", service), nil, output.Format(*format))
			}
		},
	}
}

func newAuthStatusCmd(format *string, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status for all services",
		RunE: func(cmd *cobra.Command, args []string) error {
			store := auth.NewFileTokenStore()
			services := []string{"gsc"}

			statuses := make([]auth.Status, 0, len(services))
			for _, svc := range services {
				st, err := store.Status(svc)
				if err != nil {
					return output.PrintCodedError(output.ErrAuthFailed, "failed to check auth status", err, nil, output.Format(*format))
				}
				statuses = append(statuses, st)
			}

			return output.PrintSuccess(statuses, map[string]any{
				"verbose": *verbose,
			}, output.Format(*format))
		},
	}
}

func newAuthLogoutCmd(format *string, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "logout <service>",
		Short: "Remove stored credentials for a service (e.g. gsc)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service := args[0]
			store := auth.NewFileTokenStore()

			if err := store.Delete(service); err != nil {
				return output.PrintCodedError(output.ErrAuthFailed, "failed to remove credentials", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(map[string]any{
				"service": service,
				"status":  "logged_out",
			}, map[string]any{
				"verbose": *verbose,
			}, output.Format(*format))
		},
	}
}

func loginGSC(format *string, verbose *bool) error {
	cfg, err := config.Load()
	if err != nil {
		return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
	}

	if cfg.GSCClientID == "" || cfg.GSCClientSecret == "" {
		return output.PrintCodedError(output.ErrAuthRequired, "GSC client credentials not configured",
			fmt.Errorf("set gsc_client_id and gsc_client_secret via 'supah-seo config set'"), nil, output.Format(*format))
	}

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return output.PrintCodedError(output.ErrAuthFailed, "failed to generate state token", err, nil, output.Format(*format))
	}
	state := hex.EncodeToString(stateBytes)

	// Start local callback server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return output.PrintCodedError(output.ErrAuthFailed, "failed to start local callback server", err, nil, output.Format(*format))
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	authParams := url.Values{
		"client_id":     {cfg.GSCClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {"https://www.googleapis.com/auth/webmasters.readonly openid"},
		"state":         {state},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
	}
	authURL := "https://accounts.google.com/o/oauth2/v2/auth?" + authParams.Encode()

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "invalid state", http.StatusBadRequest)
			errCh <- fmt.Errorf("state mismatch in OAuth callback")
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- fmt.Errorf("no authorization code received")
			return
		}
		_, _ = fmt.Fprintf(w, "<html><body><h1>Authentication successful</h1><p>You may close this window.</p></body></html>")
		codeCh <- code
	})

	srv := &http.Server{Handler: mux}

	go func() {
		if serveErr := srv.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			errCh <- serveErr
		}
	}()

	openedBrowser, browserErr := openBrowser(authURL)

	message := "Open the URL in your browser to authorize Supah SEO with Google Search Console"
	if openedBrowser {
		message = "Opened your browser for authorization. If it didn't open, use the auth_url below."
	} else if browserErr != nil {
		message = fmt.Sprintf("Could not automatically open browser (%v). Copy auth_url below into your browser.", browserErr)
	}

	// Print the auth URL for the user
	_ = output.PrintSuccess(map[string]any{
		"status":         "awaiting_authorization",
		"auth_url":       authURL,
		"message":        message,
		"browser_opened": openedBrowser,
		"browser_open_err": func() string {
			if browserErr == nil {
				return ""
			}
			return browserErr.Error()
		}(),
	}, map[string]any{
		"verbose": *verbose,
	}, output.Format(*format))

	// Wait for callback or timeout
	timeout := time.After(5 * time.Minute)
	var code string
	select {
	case code = <-codeCh:
	case cbErr := <-errCh:
		_ = srv.Shutdown(context.Background())
		return output.PrintCodedError(output.ErrAuthFailed, "OAuth callback failed", cbErr, nil, output.Format(*format))
	case <-timeout:
		_ = srv.Shutdown(context.Background())
		return output.PrintCodedError(output.ErrAuthFailed, "authorization timed out", fmt.Errorf("no callback received within 5 minutes"), nil, output.Format(*format))
	}

	_ = srv.Shutdown(context.Background())

	// Exchange the code for a token using the GSC client
	token, err := exchangeGSCCode(cfg, code, redirectURI)
	if err != nil {
		return output.PrintCodedError(output.ErrAuthFailed, "failed to exchange authorization code", err, nil, output.Format(*format))
	}

	store := auth.NewFileTokenStore()
	if err := store.Save("gsc", token); err != nil {
		return output.PrintCodedError(output.ErrAuthFailed, "failed to save token", err, nil, output.Format(*format))
	}

	return output.PrintSuccess(map[string]any{
		"service": "gsc",
		"status":  "authenticated",
	}, map[string]any{
		"verbose": *verbose,
	}, output.Format(*format))
}

func exchangeGSCCode(cfg *config.Config, code, redirectURI string) (auth.TokenRecord, error) {
	resp, err := http.PostForm("https://oauth2.googleapis.com/token", map[string][]string{
		"client_id":     {cfg.GSCClientID},
		"client_secret": {cfg.GSCClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	})
	if err != nil {
		return auth.TokenRecord{}, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return auth.TokenRecord{}, fmt.Errorf("token exchange returned status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}

	if err := decodeJSON(resp.Body, &tokenResp); err != nil {
		return auth.TokenRecord{}, fmt.Errorf("decoding token response: %w", err)
	}

	expiresAt := ""
	if tokenResp.ExpiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339)
	}

	return auth.TokenRecord{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresAt:    expiresAt,
		Scope:        tokenResp.Scope,
	}, nil
}

// errNoDisplay is returned when no GUI display is available on Linux.
var errNoDisplay = fmt.Errorf("no GUI display available (DISPLAY and WAYLAND_DISPLAY are unset); open the URL manually in a browser")

// hasDisplay reports whether a graphical display server is reachable.
// It checks the DISPLAY and WAYLAND_DISPLAY environment variables.
var hasDisplay = func() bool {
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

func openBrowser(targetURL string) (bool, error) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", targetURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
	default:
		// On Linux/BSD, verify a display server is available before
		// attempting xdg-open; headless environments would hang or fail.
		if !hasDisplay() {
			return false, errNoDisplay
		}
		cmd = exec.Command("xdg-open", targetURL)
	}

	if err := cmd.Start(); err != nil {
		return false, err
	}

	go func() {
		_ = cmd.Wait()
	}()

	return true, nil
}

func decodeJSON(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}
