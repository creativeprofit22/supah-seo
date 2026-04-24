package render

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed templates/*.tmpl.html
var templateFS embed.FS

// Target selects which template variant to render.
type Target string

const (
	TargetClient Target = "client"
	TargetAgency Target = "agency"
)

// Render renders the view into the chosen template and returns HTML bytes.
func Render(view ReportView, target Target) ([]byte, error) {
	name := string(target) + ".tmpl.html"
	tpl, err := template.New(name).Funcs(funcMap()).ParseFS(templateFS, "templates/"+name)
	if err != nil {
		return nil, fmt.Errorf("parse template %q: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, view); err != nil {
		return nil, fmt.Errorf("execute template %q: %w", name, err)
	}
	return buf.Bytes(), nil
}

// WriteFiles renders both templates (or the requested set) and writes them to dir.
// Returns the list of paths written.
func WriteFiles(view ReportView, dir string, targets []Target) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	stamp := view.GeneratedAt.Format("2006-01-02T15-04-05")
	slug := slugify(view.ProspectName)
	var written []string
	for _, t := range targets {
		b, err := Render(view, t)
		if err != nil {
			return written, err
		}
		p := filepath.Join(dir, fmt.Sprintf("%s-%s-%s.html", slug, string(t), stamp))
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return written, err
		}
		written = append(written, p)
	}
	return written, nil
}

// LoadLogoBase64 reads a PNG/JPEG logo from disk and returns a base64 data URL payload.
// Returns empty string if path is empty or the file cannot be read.
func LoadLogoBase64(path string) string {
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	mime := "image/png"
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		mime = "image/jpeg"
	case strings.HasSuffix(lower, ".svg"):
		mime = "image/svg+xml"
	}
	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(b))
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"money": func(n float64) string {
			if n >= 1000 {
				return fmt.Sprintf("$%s", humanInt(int64(n)))
			}
			return fmt.Sprintf("$%d", int64(n))
		},
		"pct": func(f float64) string {
			return fmt.Sprintf("%.0f%%", f*100)
		},
		"date": func(t time.Time) string {
			return t.Format("2 January 2006")
		},
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
		"title": func(s string) string {
			if s == "" {
				return s
			}
			return strings.ToUpper(s[:1]) + s[1:]
		},
		"join":     strings.Join,
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		"safeURL":  func(s string) template.URL { return template.URL(s) },
		"add1":     func(i int) int { return i + 1 },
	}
}

func humanInt(n int64) string {
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if out == "" {
		return "report"
	}
	return out
}
