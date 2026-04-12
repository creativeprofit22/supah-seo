package commands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/supah-seo/supah-seo/internal/common/config"
)

func TestBuildLoginSummaryLines_NotConfigured(t *testing.T) {
	cfg := &config.Config{}

	lines := buildLoginSummaryLines(cfg)

	if len(lines) != 3 {
		t.Fatalf("expected 3 summary lines, got %d", len(lines))
	}

	for _, line := range lines {
		if !strings.Contains(line, "not configured") {
			t.Fatalf("expected line to show not configured, got %q", line)
		}
	}
}

func TestBuildLoginSummaryLines_Configured(t *testing.T) {
	cfg := &config.Config{
		GSCClientID:        "1234567890",
		GSCClientSecret:    "supersecretvalue",
		DataForSEOLogin:    "user@example.com",
		DataForSEOPassword: "password123",
		SERPAPIKey:         "serpapikeyvalue",
		SERPProvider:       "dataforseo",
	}

	lines := buildLoginSummaryLines(cfg)

	if len(lines) != 4 {
		t.Fatalf("expected 4 summary lines, got %d", len(lines))
	}

	if !strings.Contains(lines[0], "1234****") || !strings.Contains(lines[0], "✓") {
		t.Fatalf("expected redacted GSC client id with checkmark in first line, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "user@example.com") || !strings.Contains(lines[1], "✓") {
		t.Fatalf("expected DataForSEO login with checkmark in second line, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "serp****") || !strings.Contains(lines[2], "✓") {
		t.Fatalf("expected redacted SerpAPI key with checkmark in third line, got %q", lines[2])
	}
	if !strings.Contains(lines[3], "SERP provider: dataforseo") {
		t.Fatalf("expected serp provider line, got %q", lines[3])
	}
}

func TestSanitizeVerifyError_NilError(t *testing.T) {
	if got := sanitizeVerifyError(nil); got != "" {
		t.Fatalf("expected empty string for nil error, got %q", got)
	}
}

func TestSanitizeVerifyError_NoSecrets(t *testing.T) {
	err := fmt.Errorf("connection refused")
	got := sanitizeVerifyError(err)
	if got != "connection refused" {
		t.Fatalf("expected plain message, got %q", got)
	}
}

func TestSanitizeVerifyError_RedactsSecrets(t *testing.T) {
	err := fmt.Errorf("401 Unauthorized for user@example.com with password mypassword123")
	got := sanitizeVerifyError(err, "user@example.com", "mypassword123")
	if strings.Contains(got, "user@example.com") {
		t.Fatalf("login should be redacted, got %q", got)
	}
	if strings.Contains(got, "mypassword123") {
		t.Fatalf("password should be redacted, got %q", got)
	}
	if !strings.Contains(got, "****") {
		t.Fatalf("expected redaction placeholder, got %q", got)
	}
	expected := "401 Unauthorized for **** with password ****"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestSanitizeVerifyError_EmptySecretIgnored(t *testing.T) {
	err := fmt.Errorf("timeout")
	got := sanitizeVerifyError(err, "", "  ")
	if got != "timeout" {
		t.Fatalf("expected unchanged message, got %q", got)
	}
}

func TestSummaryLine_Configured(t *testing.T) {
	line := summaryLine("TestService", true, "val123")
	if !strings.Contains(line, "✓") {
		t.Fatalf("expected checkmark in configured line, got %q", line)
	}
	if !strings.Contains(line, "TestService") {
		t.Fatalf("expected service name in line, got %q", line)
	}
	if !strings.Contains(line, "val123") {
		t.Fatalf("expected value in line, got %q", line)
	}
}

func TestSummaryLine_NotConfigured(t *testing.T) {
	line := summaryLine("TestService", false, "")
	if strings.Contains(line, "✓") {
		t.Fatalf("should not have checkmark for unconfigured service, got %q", line)
	}
	if !strings.Contains(line, "not configured") {
		t.Fatalf("expected not configured in line, got %q", line)
	}
}
