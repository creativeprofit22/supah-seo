package commands

import (
	"runtime"
	"strings"
	"testing"
)

func TestOpenBrowser_HeadlessLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("headless display check only applies on Linux")
	}

	// Simulate headless: override hasDisplay to return false.
	orig := hasDisplay
	hasDisplay = func() bool { return false }
	defer func() { hasDisplay = orig }()

	opened, err := openBrowser("http://example.com")
	if opened {
		t.Fatal("expected opened=false on headless Linux")
	}
	if err == nil {
		t.Fatal("expected an error on headless Linux")
	}
	if !strings.Contains(err.Error(), "no GUI display available") {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestOpenBrowser_LinuxWithDisplay(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("display check only applies on Linux")
	}

	// Simulate display available.
	orig := hasDisplay
	hasDisplay = func() bool { return true }
	defer func() { hasDisplay = orig }()

	// xdg-open may or may not be installed, but the display guard should pass.
	opened, err := openBrowser("http://example.com")
	// If xdg-open is not installed, Start will fail but the error should NOT
	// be the headless error.
	if err != nil && strings.Contains(err.Error(), "no GUI display available") {
		t.Fatalf("should not get headless error when display is set: %s", err.Error())
	}
	// If xdg-open succeeded, opened should be true.
	if err == nil && !opened {
		t.Fatal("expected opened=true when display is available and command succeeds")
	}
}

func TestOpenBrowser_ManualFallbackMessage(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("headless fallback messaging only applies on Linux")
	}

	// Simulate headless environment.
	orig := hasDisplay
	hasDisplay = func() bool { return false }
	defer func() { hasDisplay = orig }()

	opened, browserErr := openBrowser("http://example.com/auth")

	// Reproduce the caller logic from runAuthLogin / loginGSC.
	var message string
	if opened {
		message = "Opened your browser for authorization."
	} else if browserErr != nil {
		message = "Could not automatically open browser (" + browserErr.Error() + "). Copy auth_url below into your browser."
	} else {
		message = "Open the URL in your browser to authorize"
	}

	if !strings.Contains(message, "no GUI display available") {
		t.Fatalf("fallback message should mention missing display, got: %s", message)
	}
	if !strings.Contains(message, "Copy auth_url") {
		t.Fatalf("fallback message should instruct user to copy URL, got: %s", message)
	}
}
