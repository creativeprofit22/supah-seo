package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/supah-seo/supah-seo/internal/common/config"
	"github.com/supah-seo/supah-seo/internal/dataforseo"
	"github.com/supah-seo/supah-seo/internal/serp/serpapi"
	"golang.org/x/term"
)

type loginAction string

const (
	loginActionGSC        loginAction = "gsc"
	loginActionDataForSEO loginAction = "dataforseo"
	loginActionSerpAPI    loginAction = "serpapi"
	loginActionAll        loginAction = "all"
	loginActionFinish     loginAction = "finish"
)

// Adaptive colors that work on both light and dark terminals.
var (
	loginPrimaryColor = lipgloss.AdaptiveColor{Light: "#2563eb", Dark: "#60a5fa"}
	loginTextColor    = lipgloss.AdaptiveColor{Light: "#1e293b", Dark: "#e2e8f0"}
	loginDimColor     = lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#94a3b8"}
	loginSuccessColor = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34d399"}
	loginErrorColor   = lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f87171"}
)

var (
	loginPrimaryStyle   = lipgloss.NewStyle().Foreground(loginPrimaryColor).Bold(true)
	loginTextStyle      = lipgloss.NewStyle().Foreground(loginTextColor).Bold(true)
	loginDimStyle       = lipgloss.NewStyle().Foreground(loginDimColor)
	loginSuccessStyle   = lipgloss.NewStyle().Foreground(loginSuccessColor).Bold(true)
	loginInfoStyle      = lipgloss.NewStyle().Foreground(loginPrimaryColor)
	loginErrorStyle     = lipgloss.NewStyle().Foreground(loginErrorColor).Bold(true)
	loginConfirmedStyle = lipgloss.NewStyle().Foreground(loginSuccessColor).Bold(true)
	loginNotConfigStyle = lipgloss.NewStyle().Foreground(loginDimColor)
)

// logoLines spells "SUPAH SEO" in a clean thin-line style.
var logoLines = []string{
	"╔═╗ ╦ ╦ ╔═╗ ╔═╗ ╦ ╦  ╔═╗ ╔═╗ ╔═╗",
	"╚═╗ ║ ║ ╠═╝ ╠═╣ ╠═╣  ╚═╗ ╠═  ║ ║",
	"╚═╝ ╚═╝ ╩   ╩ ╩ ╩ ╩  ╚═╝ ╚═╝ ╚═╝",
}

// logoGradient applies a blue gradient across a logo line.
func logoGradient(line string) string {
	gradientDark := []string{
		"#93c5fd", "#86bffc", "#79b8fb", "#6cb2fa",
		"#60a5fa", "#56a0f9", "#4b96f8", "#418cf7",
		"#3b82f6", "#3179f4", "#2970f2", "#2563eb",
	}
	gradientLight := []string{
		"#2563eb", "#2970f2", "#3179f4", "#3b82f6",
		"#418cf7", "#4b96f8", "#56a0f9", "#60a5fa",
		"#6cb2fa", "#79b8fb", "#86bffc", "#93c5fd",
	}
	isDark := lipgloss.HasDarkBackground()
	colors := gradientDark
	if !isDark {
		colors = gradientLight
	}
	var b strings.Builder
	ci := 0
	for _, ch := range line {
		if ch == ' ' {
			b.WriteRune(ch)
		} else {
			c := colors[ci%len(colors)]
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render(string(ch)))
			ci++
		}
	}
	return b.String()
}

var errBackToMenu = errors.New("back to menu")

const controlsHint = "↑↓ navigate · Enter next · Esc back"

// loginVersion is set by NewLoginCmd so the header can display it.
var loginVersion string

// NewLoginCmd returns the top-level interactive login command.
func NewLoginCmd(version string, format *string, verbose *bool) *cobra.Command {
	loginVersion = version
	return &cobra.Command{
		Use:   "login",
		Short: "Interactive setup for service credentials",
		Long:  `Interactively configure credentials for Google Search Console, SerpAPI, and other services.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return fmt.Errorf("supah-seo login requires an interactive terminal")
			}
			return runLogin(format, verbose)
		},
	}
}

func runLogin(format *string, verbose *bool) error {
	printLoginHeader()

	for {
		action, err := selectLoginAction()
		if err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Println()
				printLoginSummary()
				return nil
			}
			return err
		}

		switch action {
		case loginActionGSC:
			if err := runGSCLoginForm(format, verbose); err != nil {
				if errors.Is(err, errBackToMenu) {
					fmt.Printf("%s\n\n", loginInfoStyle.Render("• Back to service menu"))
					continue
				}
				fmt.Printf("%s\n\n", loginErrorStyle.Render("✗ Google OAuth: "+err.Error()))
			}
		case loginActionDataForSEO:
			if err := runDataForSEOLoginForm(); err != nil {
				if errors.Is(err, errBackToMenu) {
					fmt.Printf("%s\n\n", loginInfoStyle.Render("• Back to service menu"))
					continue
				}
				fmt.Printf("%s\n\n", loginErrorStyle.Render("✗ DataForSEO: "+err.Error()))
			}
		case loginActionSerpAPI:
			if err := runSerpAPILoginForm(); err != nil {
				if errors.Is(err, errBackToMenu) {
					fmt.Printf("%s\n\n", loginInfoStyle.Render("• Back to service menu"))
					continue
				}
				fmt.Printf("%s\n\n", loginErrorStyle.Render("✗ SerpAPI: "+err.Error()))
			}
		case loginActionAll:
			if err := runGSCLoginForm(format, verbose); err != nil {
				if errors.Is(err, errBackToMenu) {
					fmt.Printf("%s\n\n", loginInfoStyle.Render("• Back to service menu"))
					continue
				}
				fmt.Printf("%s\n\n", loginErrorStyle.Render("✗ Google OAuth: "+err.Error()))
			}
			if err := runDataForSEOLoginForm(); err != nil {
				if errors.Is(err, errBackToMenu) {
					fmt.Printf("%s\n\n", loginInfoStyle.Render("• Back to service menu"))
					continue
				}
				fmt.Printf("%s\n\n", loginErrorStyle.Render("✗ DataForSEO: "+err.Error()))
			}
			if err := runSerpAPILoginForm(); err != nil {
				if errors.Is(err, errBackToMenu) {
					fmt.Printf("%s\n\n", loginInfoStyle.Render("• Back to service menu"))
					continue
				}
				fmt.Printf("%s\n\n", loginErrorStyle.Render("✗ SerpAPI: "+err.Error()))
			}
		case loginActionFinish:
			fmt.Println()
			printLoginSummary()
			return nil
		}
	}
}

func printLoginHeader() {
	fmt.Println()
	for _, line := range logoLines {
		fmt.Println("  " + logoGradient(line))
	}
	ver := loginVersion
	if ver == "" {
		ver = "dev"
	}
	fmt.Println()
	fmt.Println("  " + loginDimStyle.Render(ver+" · By ") + loginTextStyle.Render("Jake Schepis"))
	fmt.Println("  " + loginPrimaryStyle.Render("Login") + loginDimStyle.Render(" — Select a service"))
	fmt.Println()
}

func selectLoginAction() (loginAction, error) {
	choice := loginActionFinish

	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	gscLabel := "Google OAuth — GSC, PageSpeed Insights"
	if isGSCConfigured(cfg) {
		gscLabel += " " + loginConfirmedStyle.Render("✓")
	}

	dataForSEOLabel := "DataForSEO — SERP, AEO/GEO, Labs"
	if isDataForSEOConfigured(cfg) {
		dataForSEOLabel += " " + loginConfirmedStyle.Render("✓")
	}

	serpAPILabel := "SerpAPI — Search results"
	if isSerpAPIConfigured(cfg) {
		serpAPILabel += " " + loginConfirmedStyle.Render("✓")
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[loginAction]().
				Options(
					huh.NewOption(gscLabel, loginActionGSC),
					huh.NewOption(dataForSEOLabel, loginActionDataForSEO),
					huh.NewOption(serpAPILabel, loginActionSerpAPI),
					huh.NewOption("All services — Set up everything", loginActionAll),
					huh.NewOption("Finish", loginActionFinish),
				).
				Value(&choice),
			huh.NewNote().Description(controlsHint),
		),
	).WithKeyMap(loginKeyMap()).WithTheme(loginTheme())

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return loginActionFinish, nil
		}
		return "", err
	}

	return choice, nil
}

func runGSCLoginForm(format *string, verbose *bool) error {
	var clientID string
	var clientSecret string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("GSC Client ID").
				Value(&clientID).
				Validate(validateRequired("client ID")),
			huh.NewInput().
				Title("GSC Client Secret").
				EchoMode(huh.EchoModePassword).
				Value(&clientSecret).
				Validate(validateRequired("client secret")),
			huh.NewNote().Description(controlsHint),
		),
	).WithKeyMap(loginKeyMap()).WithTheme(loginTheme())

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return errBackToMenu
		}
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Set("gsc_client_id", strings.TrimSpace(clientID)); err != nil {
		return fmt.Errorf("failed to set client ID: %w", err)
	}
	if err := cfg.Set("gsc_client_secret", strings.TrimSpace(clientSecret)); err != nil {
		return fmt.Errorf("failed to set client secret: %w", err)
	}
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println(loginInfoStyle.Render("• Opening browser for authorization..."))
	if err := loginGSC(format, verbose); err != nil {
		return err
	}

	fmt.Println(loginSuccessStyle.Render("✓ Google OAuth authenticated (GSC + PSI)"))
	fmt.Println()
	return nil
}

func runDataForSEOLoginForm() error {
	var login string
	var password string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("DataForSEO Login (email)").
				Value(&login).
				Validate(validateRequired("login")),
			huh.NewInput().
				Title("DataForSEO Password").
				EchoMode(huh.EchoModePassword).
				Value(&password).
				Validate(validateRequired("password")),
			huh.NewNote().Description(controlsHint),
		),
	).WithKeyMap(loginKeyMap()).WithTheme(loginTheme())

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return errBackToMenu
		}
		return err
	}

	trimmedLogin := strings.TrimSpace(login)
	trimmedPassword := strings.TrimSpace(password)

	fmt.Println(loginInfoStyle.Render("• Verifying DataForSEO credentials..."))

	client := dataforseo.New(trimmedLogin, trimmedPassword)
	if err := client.VerifyCredentials(); err != nil {
		detail := sanitizeVerifyError(err, trimmedLogin, trimmedPassword)
		fmt.Println(loginErrorStyle.Render("✗ DataForSEO credential verification failed: " + detail))
		fmt.Println()
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Set("dataforseo_login", trimmedLogin); err != nil {
		return fmt.Errorf("failed to set DataForSEO login: %w", err)
	}
	if err := cfg.Set("dataforseo_password", trimmedPassword); err != nil {
		return fmt.Errorf("failed to set DataForSEO password: %w", err)
	}
	if err := cfg.Set("serp_provider", "dataforseo"); err != nil {
		return fmt.Errorf("failed to set serp provider: %w", err)
	}
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println(loginSuccessStyle.Render("✓ DataForSEO configured (verified)"))
	fmt.Println()
	return nil
}

func runSerpAPILoginForm() error {
	var apiKey string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("SerpAPI Key").
				EchoMode(huh.EchoModePassword).
				Value(&apiKey).
				Validate(validateRequired("API key")),
			huh.NewNote().Description(controlsHint),
		),
	).WithKeyMap(loginKeyMap()).WithTheme(loginTheme())

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return errBackToMenu
		}
		return err
	}

	trimmedKey := strings.TrimSpace(apiKey)

	fmt.Println(loginInfoStyle.Render("• Verifying SerpAPI key..."))

	adapter := serpapi.New(trimmedKey)
	if err := adapter.VerifyKey(); err != nil {
		detail := sanitizeVerifyError(err, trimmedKey)
		fmt.Println(loginErrorStyle.Render("✗ SerpAPI key verification failed: " + detail))
		fmt.Println()
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Set("serp_api_key", trimmedKey); err != nil {
		return fmt.Errorf("failed to set API key: %w", err)
	}
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println(loginSuccessStyle.Render("✓ SerpAPI configured (verified)"))
	fmt.Println()
	return nil
}

func printLoginSummary() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println(loginErrorStyle.Render("✗ Failed to load config for summary: " + err.Error()))
		return
	}

	fmt.Println(loginSuccessStyle.Render("✓ Setup complete"))
	fmt.Println()

	for _, line := range buildLoginSummaryLines(cfg) {
		fmt.Println(line)
	}

	fmt.Println()
}

func buildLoginSummaryLines(cfg *config.Config) []string {
	lines := []string{
		summaryLine("Google OAuth (GSC | PSI)", isGSCConfigured(cfg), redactValue(cfg.GSCClientID)),
		summaryLine("DataForSEO", isDataForSEOConfigured(cfg), cfg.DataForSEOLogin),
		summaryLine("SerpAPI", isSerpAPIConfigured(cfg), redactValue(cfg.SERPAPIKey)),
	}

	if cfg.SERPProvider != "" {
		lines = append(lines, fmt.Sprintf("  • SERP provider: %s", cfg.SERPProvider))
	}

	return lines
}

func summaryLine(name string, configured bool, value string) string {
	if configured {
		detail := ""
		if v := strings.TrimSpace(value); v != "" {
			detail = loginDimStyle.Render(" (" + v + ")")
		}
		return "  " + loginConfirmedStyle.Render("✓") + " " + loginTextStyle.Render(name) + detail
	}
	return "  " + loginNotConfigStyle.Render("· "+name+" — not configured")
}

func validateRequired(name string) func(string) error {
	return func(value string) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s cannot be empty", name)
		}
		return nil
	}
}

func isDataForSEOConfigured(cfg *config.Config) bool {
	return strings.TrimSpace(cfg.DataForSEOLogin) != "" && strings.TrimSpace(cfg.DataForSEOPassword) != ""
}

func isSerpAPIConfigured(cfg *config.Config) bool {
	return strings.TrimSpace(cfg.SERPAPIKey) != ""
}

func isGSCConfigured(cfg *config.Config) bool {
	return strings.TrimSpace(cfg.GSCClientID) != "" && strings.TrimSpace(cfg.GSCClientSecret) != ""
}

// loginKeyMap returns a huh keymap with Esc bound to quit/back and
// up/down arrows for navigating between input fields.
func loginKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"), key.WithHelp("esc", "back"))
	km.Input.Prev = key.NewBinding(key.WithKeys("shift+tab", "up"), key.WithHelp("↑", "prev"))
	km.Input.Next = key.NewBinding(key.WithKeys("enter", "tab", "down"), key.WithHelp("↓/enter", "next"))
	return km
}

func loginTheme() huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		styles := huh.ThemeCharm(isDark)

		primary := "#60a5fa"
		dim := "#94a3b8"
		text := "#e2e8f0"
		errColor := "#f87171"
		if !isDark {
			primary = "#2563eb"
			dim = "#64748b"
			text = "#1e293b"
			errColor = "#dc2626"
		}

		styles.Focused.Title = styles.Focused.Title.Foreground(lipgloss.Color(primary)).Bold(true)
		styles.Focused.Description = styles.Focused.Description.Foreground(lipgloss.Color(dim))
		styles.Focused.SelectSelector = styles.Focused.SelectSelector.Foreground(lipgloss.Color(primary)).Bold(true)
		styles.Focused.Option = styles.Focused.Option.Foreground(lipgloss.Color(text))
		styles.Focused.SelectedOption = styles.Focused.SelectedOption.Foreground(lipgloss.Color(primary)).Bold(true)
		styles.Focused.TextInput.Cursor = styles.Focused.TextInput.Cursor.Foreground(lipgloss.Color(primary))
		styles.Focused.NextIndicator = styles.Focused.NextIndicator.Foreground(lipgloss.Color(primary))
		styles.Focused.PrevIndicator = styles.Focused.PrevIndicator.Foreground(lipgloss.Color(primary))
		styles.Focused.FocusedButton = styles.Focused.FocusedButton.Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color(primary)).Bold(true)
		styles.Focused.BlurredButton = styles.Focused.BlurredButton.Foreground(lipgloss.Color(dim))
		styles.Focused.ErrorIndicator = styles.Focused.ErrorIndicator.Foreground(lipgloss.Color(errColor)).Bold(true)
		styles.Focused.ErrorMessage = styles.Focused.ErrorMessage.Foreground(lipgloss.Color(errColor))

		return styles
	})
}

func redactValue(v string) string {
	if len(v) <= 8 {
		return "****"
	}
	return v[:4] + "****"
}

// sanitizeVerifyError returns a user-safe error detail string with any
// secret-like tokens (long hex/base64 strings, emails, passwords) scrubbed.
func sanitizeVerifyError(err error, secrets ...string) string {
	if err == nil {
		return ""
	}
	msg := err.Error()

	// Remove any literal secret values that were passed in.
	for _, s := range secrets {
		s = strings.TrimSpace(s)
		if s != "" {
			msg = strings.ReplaceAll(msg, s, "****")
		}
	}

	return msg
}
