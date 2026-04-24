package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/supah-seo/supah-seo/internal/audit"
	"github.com/supah-seo/supah-seo/internal/common/config"
	"github.com/supah-seo/supah-seo/internal/crawl"
	"github.com/supah-seo/supah-seo/internal/provider"
	_ "github.com/supah-seo/supah-seo/internal/provider/local"
	"github.com/supah-seo/supah-seo/internal/report"
	"github.com/supah-seo/supah-seo/internal/report/brief"
	"github.com/supah-seo/supah-seo/internal/report/diff"
	"github.com/supah-seo/supah-seo/internal/report/render"
	"github.com/supah-seo/supah-seo/internal/state"
	"github.com/supah-seo/supah-seo/pkg/output"
)

// NewReportCmd returns the report command group.
func NewReportCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Report generation and listing commands",
	}

	cmd.AddCommand(
		newReportGenerateCmd(format, verbose),
		newReportListCmd(format, verbose),
		newReportRenderCmd(format, verbose),
		newReportBriefCmd(format, verbose),
		newReportCompareCmd(format, verbose),
	)
	return cmd
}

func newReportCompareCmd(format *string, verbose *bool) *cobra.Command {
	var (
		fromPath     string
		toPath       string
		fromLabel    string
		toLabel      string
		outputFile   string
		agencyName   string
		logoPath     string
		prospectName string
	)

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Generate an HTML diff report between two state snapshots",
		Long: `Compares two .supah-seo/state.json snapshots and renders an HTML report
showing score deltas, findings resolved and introduced, keyword ranking
movement, backlink growth, and PageSpeed changes. Typical workflow:

  supah-seo snapshot create --label baseline   # at kick-off
  # ... 30/60/90 days of work ...
  supah-seo snapshot create --label month-3    # fresh audit first
  supah-seo report compare --from baseline --to month-3

You can also pass 'current' to --to to compare against the live state.json
without taking a fresh snapshot.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromPath == "" {
				return output.PrintCodedError(output.ErrReportWriteFailed, "missing --from flag (snapshot label or path)", nil, nil, output.Format(*format))
			}
			if toPath == "" {
				toPath = "current"
			}

			fromResolved, err := resolveSnapshot(fromPath)
			if err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to resolve --from", err, map[string]any{"value": fromPath}, output.Format(*format))
			}
			toResolved, err := resolveSnapshot(toPath)
			if err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to resolve --to", err, map[string]any{"value": toPath}, output.Format(*format))
			}

			fromState, err := diff.LoadSnapshot(fromResolved)
			if err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to load --from snapshot", err, nil, output.Format(*format))
			}
			toState, err := diff.LoadSnapshot(toResolved)
			if err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to load --to snapshot", err, nil, output.Format(*format))
			}

			if fromLabel == "" {
				fromLabel = labelFromPath(fromPath)
			}
			if toLabel == "" {
				toLabel = labelFromPath(toPath)
			}

			view := diff.Compute(fromState, toState, diff.Options{
				ProspectName:  prospectName,
				AgencyName:    agencyName,
				AgencyLogoB64: render.LoadLogoBase64(logoPath),
				FromLabel:     fromLabel,
				ToLabel:       toLabel,
			})

			html, err := diff.Render(view)
			if err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to render compare template", err, nil, output.Format(*format))
			}

			if outputFile == "" {
				outputFile = filepath.Join("reports", "compare-"+view.GeneratedAt.Format("2006-01-02T15-04-05")+".html")
			}
			if err := os.MkdirAll(filepath.Dir(outputFile), 0o755); err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to create output dir", err, nil, output.Format(*format))
			}
			if err := os.WriteFile(outputFile, html, 0o644); err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to write compare file", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(map[string]any{
				"file":        outputFile,
				"from":        fromResolved,
				"to":          toResolved,
				"score_from":  view.ScoreFrom,
				"score_to":    view.ScoreTo,
				"score_delta": view.ScoreDelta,
			}, map[string]any{"verbose": *verbose}, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&fromPath, "from", "", "Earlier snapshot: label, file path, or 'current' (required)")
	cmd.Flags().StringVar(&toPath, "to", "current", "Later snapshot: label, file path, or 'current' (default)")
	cmd.Flags().StringVar(&fromLabel, "from-label", "", "Display label for the 'from' snapshot (default: derived from --from)")
	cmd.Flags().StringVar(&toLabel, "to-label", "", "Display label for the 'to' snapshot (default: derived from --to)")
	cmd.Flags().StringVar(&outputFile, "out", "", "Output HTML file (default: reports/compare-<timestamp>.html)")
	cmd.Flags().StringVar(&agencyName, "agency-name", "Douro Digital", "Agency name shown in report header")
	cmd.Flags().StringVar(&logoPath, "logo", "", "Path to agency logo (PNG/JPEG), embedded as base64")
	cmd.Flags().StringVar(&prospectName, "prospect-name", "", "Prospect business name")
	return cmd
}

// resolveSnapshot accepts 'current', a filesystem path, or a label/prefix
// that matches a file under .supah-seo/snapshots/ and returns the absolute path.
func resolveSnapshot(ref string) (string, error) {
	if ref == "current" {
		return filepath.Join(state.DirName, state.FileName), nil
	}
	// If it exists as a file path, use it directly.
	if _, err := os.Stat(ref); err == nil {
		return ref, nil
	}
	// Otherwise try matching by label or timestamp prefix in the snapshots dir.
	dir := filepath.Join(state.DirName, "snapshots")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("snapshot %q not found and snapshots dir is unreadable: %w", ref, err)
	}
	var matches []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		if strings.Contains(name, ref) {
			matches = append(matches, filepath.Join(dir, e.Name()))
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no snapshot matches %q under %s", ref, dir)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous: %d snapshots match %q (%v). Use a more specific label or a full path", len(matches), ref, matches)
	}
	return matches[0], nil
}

func labelFromPath(ref string) string {
	if ref == "current" {
		return "Current"
	}
	base := filepath.Base(ref)
	base = strings.TrimSuffix(base, ".json")
	// Try to extract a label suffix from <ts>-<label> format.
	const tsLen = 19
	if len(base) > tsLen && base[tsLen] == '-' {
		return strings.Title(strings.ReplaceAll(base[tsLen+1:], "-", " "))
	}
	return base
}

func newReportBriefCmd(format *string, verbose *bool) *cobra.Command {
	var (
		stateFile    string
		outputFile   string
		prospectName string
		industry     string
		maxBriefs    int
	)

	cmd := &cobra.Command{
		Use:   "brief",
		Short: "Generate content briefs from People Also Ask questions captured in state",
		Long: `Reads PAA questions recorded during SERP analysis and turns each one into
a structured content brief (title, H1, H2 outline, word count target, related
keywords, schema suggestions, and linking hints). Output is a single markdown
file ready to hand to a writer or strategist.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if stateFile == "" {
				stateFile = filepath.Join(state.DirName, state.FileName)
			}
			data, err := os.ReadFile(stateFile)
			if err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to read state file", err, map[string]any{"path": stateFile}, output.Format(*format))
			}
			var s state.State
			if err := json.Unmarshal(data, &s); err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "invalid state json", err, nil, output.Format(*format))
			}

			bundle := brief.Generate(&s, brief.Options{
				ProspectName: prospectName,
				Industry:     industry,
				MaxBriefs:    maxBriefs,
			})
			if bundle.TotalBriefs == 0 {
				return output.PrintCodedError(output.ErrReportWriteFailed, "no PAA questions found in state; run 'supah-seo serp batch' first to capture them", nil, nil, output.Format(*format))
			}

			if outputFile == "" {
				outputFile = filepath.Join("reports", "content-briefs-"+bundle.GeneratedAt.Format("2006-01-02T15-04-05")+".md")
			}
			if err := os.MkdirAll(filepath.Dir(outputFile), 0o755); err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to create output dir", err, nil, output.Format(*format))
			}
			if err := os.WriteFile(outputFile, []byte(bundle.Markdown()), 0o644); err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to write briefs file", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(map[string]any{
				"file":          outputFile,
				"briefs":        bundle.TotalBriefs,
				"total_words":   bundle.TotalWords,
				"prospect_name": bundle.ProspectName,
			}, map[string]any{
				"verbose":     *verbose,
				"source_file": stateFile,
			}, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&stateFile, "state", "", "Path to state.json (default: .supah-seo/state.json)")
	cmd.Flags().StringVar(&outputFile, "out", "", "Output markdown file (default: reports/content-briefs-<timestamp>.md)")
	cmd.Flags().StringVar(&prospectName, "prospect-name", "", "Prospect business name (used in suggested title tags)")
	cmd.Flags().StringVar(&industry, "industry", "", "Industry: generic, car-detailing, trades, professional-services, restaurants, dental")
	cmd.Flags().IntVar(&maxBriefs, "max", 0, "Maximum number of briefs to generate (0 = all)")
	return cmd
}

func newReportGenerateCmd(format *string, verbose *bool) *cobra.Command {
	var (
		targetURL string
		depth     int
		maxPages  int
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Crawl, audit, and generate a stored report",
		RunE: func(cmd *cobra.Command, args []string) error {
			if targetURL == "" {
				return output.PrintCodedError(output.ErrInvalidURL, "missing required --url flag", nil, nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			fetcher, err := provider.NewFetcher(cfg.ActiveProvider)
			if err != nil {
				return output.PrintCodedError(output.ErrProviderNotFound, "failed to create provider", err, nil, output.Format(*format))
			}

			crawlSvc := crawl.NewService(fetcher)
			crawlResult, err := crawlSvc.Run(cmd.Context(), crawl.Request{
				TargetURL: targetURL,
				Depth:     depth,
				MaxPages:  maxPages,
			})
			if err != nil {
				code := output.ErrCrawlFailed
				if errors.Is(err, context.DeadlineExceeded) {
					code = output.ErrFetchTimeout
				} else if errors.Is(err, context.Canceled) {
					code = output.ErrCancelled
				}
				return output.PrintCodedError(code, "crawl failed", err, nil, output.Format(*format))
			}

			auditSvc := audit.NewService()
			auditResult, err := auditSvc.Run(cmd.Context(), audit.Request{
				CrawlResult: crawlResult,
			})
			if err != nil {
				return output.PrintCodedError(output.ErrAuditFailed, "audit failed", err, nil, output.Format(*format))
			}

			reportSvc := report.NewService()
			reportResult, err := reportSvc.Generate(cmd.Context(), report.Request{
				AuditResult: auditResult,
			})
			if err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "report generation failed", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(reportResult, map[string]any{
				"verbose": *verbose,
			}, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&targetURL, "url", "", "Target URL to report on (required)")
	cmd.Flags().IntVar(&depth, "depth", 2, "Maximum crawl depth")
	cmd.Flags().IntVar(&maxPages, "max-pages", 50, "Maximum number of pages to crawl")

	return cmd
}

func newReportListCmd(format *string, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stored reports",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := report.NewService()
			reports, err := svc.List(cmd.Context(), "")
			if err != nil {
				return output.PrintCodedError(output.ErrReportListFailed, "failed to list reports", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(reports, map[string]any{
				"count":   len(reports),
				"verbose": *verbose,
			}, output.Format(*format))
		},
	}
}

func newReportRenderCmd(format *string, verbose *bool) *cobra.Command {
	var (
		stateFile     string
		outputDir     string
		template      string
		agencyName    string
		logoPath      string
		ctaURL        string
		ctaLabel      string
		prospectName  string
		location      string
		currentAgency string
		avgTicket     float64
		closeRate     float64
		pricingCSV    string
		industry      string
		emitPDF       bool
	)

	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render an HTML report from the current project state",
		Long: `Reads .supah-seo/state.json in the current directory (or --state)
and renders an HTML audit report. Supports two templates: client (sales-facing)
and agency (internal). Output is a single self-contained HTML file per template.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Locate state file
			if stateFile == "" {
				stateFile = filepath.Join(state.DirName, state.FileName)
			}
			data, err := os.ReadFile(stateFile)
			if err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to read state file", err, map[string]any{"path": stateFile}, output.Format(*format))
			}
			var s state.State
			if err := json.Unmarshal(data, &s); err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "invalid state json", err, nil, output.Format(*format))
			}

			// Pick templates
			targets, err := parseTemplateTargets(template)
			if err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, err.Error(), err, nil, output.Format(*format))
			}

			// Resolve output dir: default to {project}/reports/
			if outputDir == "" {
				outputDir = "reports"
			}

			// Build options
			opts := render.Options{
				AgencyName:    agencyName,
				AgencyLogoB64: render.LoadLogoBase64(logoPath),
				CTAURL:        ctaURL,
				CTALabel:      ctaLabel,
				ProspectName:  prospectName,
				Location:      location,
				CurrentAgency: currentAgency,
				AvgTicket:     avgTicket,
				CloseRate:     closeRate,
				Industry:      industry,
				Pricing:       parsePricingCSV(pricingCSV),
			}

			view := render.Build(&s, opts)
			written, err := render.WriteFiles(view, outputDir, targets)
			if err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "render failed", err, nil, output.Format(*format))
			}

			var pdfPaths []string
			var pdfErr string
			if emitPDF {
				for _, htmlPath := range written {
					pdfPath := strings.TrimSuffix(htmlPath, ".html") + ".pdf"
					if _, err := render.HTMLToPDF(htmlPath, pdfPath); err != nil {
						pdfErr = err.Error()
						break
					}
					pdfPaths = append(pdfPaths, pdfPath)
				}
			}

			payload := map[string]any{
				"files":     written,
				"site":      s.Site,
				"templates": targets,
			}
			if len(pdfPaths) > 0 {
				payload["pdf_files"] = pdfPaths
			}
			meta := map[string]any{
				"verbose":     *verbose,
				"source_file": stateFile,
				"output_dir":  outputDir,
			}
			if pdfErr != "" {
				meta["pdf_error"] = pdfErr
			}
			return output.PrintSuccess(payload, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&stateFile, "state", "", "Path to state.json (default: .supah-seo/state.json)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Directory to write reports into (default: ./reports)")
	cmd.Flags().StringVar(&template, "template", "both", "Which template to render: client, agency, or both")
	cmd.Flags().StringVar(&agencyName, "agency-name", "Douro Digital", "Agency name shown in report header")
	cmd.Flags().StringVar(&logoPath, "logo", "", "Path to agency logo (PNG/JPEG), embedded as base64")
	cmd.Flags().StringVar(&ctaURL, "cta-url", "", "CTA URL shown at the bottom of the client report")
	cmd.Flags().StringVar(&ctaLabel, "cta-label", "Book a strategy call", "CTA button label")
	cmd.Flags().StringVar(&prospectName, "prospect-name", "", "Prospect business name")
	cmd.Flags().StringVar(&location, "location", "", "Prospect service area / location")
	cmd.Flags().StringVar(&currentAgency, "current-agency", "", "Name of existing incumbent agency if known")
	cmd.Flags().Float64Var(&avgTicket, "avg-ticket", 0, "Average job value for revenue modelling (default 180)")
	cmd.Flags().Float64Var(&closeRate, "close-rate", 0, "Enquiry-to-booking close rate 0-1 (default 0.25)")
	cmd.Flags().StringVar(&pricingCSV, "pricing", "", "Optional pricing gap entries as pipe-separated tuples: 'Service|Their|Market|GapNote;...'")
	cmd.Flags().StringVar(&industry, "industry", "generic", "Industry template: generic, car-detailing, trades, professional-services, restaurants, dental")
	cmd.Flags().BoolVar(&emitPDF, "pdf", false, "Also emit PDF versions of each rendered HTML (requires Chrome or Chromium on PATH)")

	return cmd
}

func parseTemplateTargets(t string) ([]render.Target, error) {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "", "both":
		return []render.Target{render.TargetClient, render.TargetAgency}, nil
	case "client":
		return []render.Target{render.TargetClient}, nil
	case "agency":
		return []render.Target{render.TargetAgency}, nil
	default:
		return nil, fmt.Errorf("invalid --template %q (must be client, agency, or both)", t)
	}
}

func parsePricingCSV(csv string) []render.PricingGap {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	var out []render.PricingGap
	for _, row := range strings.Split(csv, ";") {
		parts := strings.Split(row, "|")
		if len(parts) < 4 {
			continue
		}
		out = append(out, render.PricingGap{
			Service:       strings.TrimSpace(parts[0]),
			TheirPrice:    strings.TrimSpace(parts[1]),
			MarketPrice:   strings.TrimSpace(parts[2]),
			MonthlyMisses: strings.TrimSpace(parts[3]),
		})
	}
	return out
}
