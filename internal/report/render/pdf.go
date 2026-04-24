package render

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ChromeBinary returns the path of a usable headless-capable Chrome/Chromium binary,
// or empty string if none is found on this system.
func ChromeBinary() string {
	candidates := []string{
		"google-chrome", "google-chrome-stable",
		"chromium", "chromium-browser",
		"chrome",
	}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		)
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		)
	}
	// WSL users often want the Windows Chrome binary exposed through /mnt/c.
	if runtime.GOOS == "linux" {
		candidates = append(candidates,
			"/mnt/c/Program Files/Google/Chrome/Application/chrome.exe",
			"/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe",
		)
	}

	for _, c := range candidates {
		if strings.Contains(c, "/") || strings.Contains(c, `\`) {
			if _, err := os.Stat(c); err == nil {
				return c
			}
			continue
		}
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

// HTMLToPDF converts an HTML file at inPath into a PDF at outPath using headless Chrome.
// Returns the chosen binary and any error.
//
// On WSL, if the Chrome binary is a Windows .exe (e.g. chrome.exe under /mnt/c),
// the input/output paths are translated to Windows paths via wslpath so Chrome
// can actually read and write them.
func HTMLToPDF(inPath, outPath string) (string, error) {
	bin := ChromeBinary()
	if bin == "" {
		return "", fmt.Errorf("no Chrome or Chromium binary found on PATH. Install Google Chrome, Chromium, or set one on PATH, then retry")
	}
	absIn, err := filepath.Abs(inPath)
	if err != nil {
		return bin, fmt.Errorf("resolve input path: %w", err)
	}
	absOut, err := filepath.Abs(outPath)
	if err != nil {
		return bin, fmt.Errorf("resolve output path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absOut), 0o755); err != nil {
		return bin, err
	}

	usingWinExe := strings.HasSuffix(strings.ToLower(bin), ".exe")

	inputArg := fileURL(absIn)
	outputArg := absOut
	if usingWinExe {
		winIn, err := wslToWindows(absIn)
		if err == nil && winIn != "" {
			inputArg = "file:///" + strings.ReplaceAll(winIn, `\`, "/")
		}
		if winOut, err := wslToWindows(absOut); err == nil && winOut != "" {
			outputArg = winOut
		}
	}

	// Try the modern --headless=new syntax first; fall back if the binary is older.
	args := []string{
		"--headless=new",
		"--disable-gpu",
		"--no-sandbox",
		"--no-pdf-header-footer",
		"--print-to-pdf=" + outputArg,
		inputArg,
	}
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		args[0] = "--headless"
		cmd2 := exec.Command(bin, args...)
		if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
			return bin, fmt.Errorf("chrome print-to-pdf failed (new: %v / classic: %v): %s", err, err2, strings.TrimSpace(string(append(out, out2...))))
		}
	}
	if fi, err := os.Stat(absOut); err != nil || fi.Size() == 0 {
		return bin, fmt.Errorf("PDF was not produced or is empty at %s", absOut)
	}
	return bin, nil
}

func fileURL(abs string) string {
	if strings.HasPrefix(abs, "/") {
		return "file://" + abs
	}
	return "file:///" + strings.ReplaceAll(abs, `\`, "/")
}

// wslToWindows converts a /mnt/c/... WSL path to a C:\... Windows path.
// Uses `wslpath -w` when available, otherwise does a best-effort prefix swap.
func wslToWindows(linuxPath string) (string, error) {
	if runtime.GOOS != "linux" {
		return linuxPath, nil
	}
	if p, err := exec.LookPath("wslpath"); err == nil {
		out, err := exec.Command(p, "-w", linuxPath).Output()
		if err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}
	// Fallback: hand-translate /mnt/<drive>/... → <drive>:\...
	if strings.HasPrefix(linuxPath, "/mnt/") && len(linuxPath) > 6 {
		drive := strings.ToUpper(string(linuxPath[5]))
		rest := linuxPath[6:]
		return drive + ":" + strings.ReplaceAll(rest, "/", `\`), nil
	}
	return linuxPath, fmt.Errorf("cannot translate path %q to Windows form", linuxPath)
}
