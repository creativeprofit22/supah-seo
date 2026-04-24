package commands

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/supah-seo/supah-seo/pkg/output"
)

// BatchRow is one prospect from the input CSV.
type BatchRow struct {
	URL           string
	ProspectName  string
	Industry      string
	Location      string
	CurrentAgency string
	AvgTicket     float64
	CloseRate     float64
	CTAURL        string
	Pricing       string
}

// BatchResult captures the outcome of a single prospect run.
type BatchResult struct {
	ProspectName string   `json:"prospect_name"`
	URL          string   `json:"url"`
	Directory    string   `json:"directory"`
	ReportFiles  []string `json:"report_files,omitempty"`
	Success      bool     `json:"success"`
	Error        string   `json:"error,omitempty"`
	DurationMs   int64    `json:"duration_ms"`
}

// NewBatchCmd returns the batch command.
func NewBatchCmd(format *string, verbose *bool) *cobra.Command {
	var (
		csvPath       string
		baseDir       string
		workers       int
		logoPath      string
		agencyName    string
		depth         int
		maxPages      int
		dryRun        bool
		skipLabs      bool
		skipBacklinks bool
	)

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Run the full audit pipeline over a CSV of prospects",
		Long: `Reads a CSV file of prospects and runs crawl, audit, PSI, Labs, backlinks,
analyze and render for each row. Work is parallelised with a configurable
concurrency limit. Each prospect gets its own subdirectory under --output-dir
and its own set of HTML reports.

CSV columns (in order):
  url,prospect_name,industry,location,current_agency,avg_ticket,close_rate,cta_url,pricing

Header row is required. Missing optional columns can be left blank.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if csvPath == "" {
				return output.PrintCodedError(output.ErrInvalidURL, "missing required --csv flag", nil, nil, output.Format(*format))
			}
			rows, err := readBatchCSV(csvPath)
			if err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to read csv", err, map[string]any{"path": csvPath}, output.Format(*format))
			}
			if len(rows) == 0 {
				return output.PrintCodedError(output.ErrReportWriteFailed, "csv has no data rows", nil, nil, output.Format(*format))
			}

			if baseDir == "" {
				baseDir = filepath.Join(os.Getenv("HOME"), "supah-audits", "batch-"+time.Now().Format("2006-01-02"))
			}
			if err := os.MkdirAll(baseDir, 0o755); err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "failed to create output dir", err, nil, output.Format(*format))
			}

			if dryRun {
				estCostPerProspect := 0.05
				if !skipLabs {
					estCostPerProspect += 0.03
				}
				if !skipBacklinks {
					estCostPerProspect += 0.02
				}
				return output.PrintSuccess(map[string]any{
					"prospects":              len(rows),
					"workers":                workers,
					"base_dir":               baseDir,
					"estimated_cost_per_row": estCostPerProspect,
					"estimated_total_cost":   float64(len(rows)) * estCostPerProspect,
					"currency":               "USD",
				}, map[string]any{"dry_run": true}, output.Format(*format))
			}

			bin, err := exec.LookPath("supah-seo")
			if err != nil {
				// Fall back to the absolute path of the running binary.
				bin, err = os.Executable()
				if err != nil {
					return output.PrintCodedError(output.ErrReportWriteFailed, "failed to locate supah-seo binary", err, nil, output.Format(*format))
				}
			}

			results := runBatch(bin, rows, baseDir, workers, agencyName, logoPath, depth, maxPages, skipLabs, skipBacklinks, *verbose)

			succeeded := 0
			for _, r := range results {
				if r.Success {
					succeeded++
				}
			}

			return output.PrintSuccess(map[string]any{
				"total":     len(results),
				"succeeded": succeeded,
				"failed":    len(results) - succeeded,
				"base_dir":  baseDir,
				"results":   results,
			}, map[string]any{"verbose": *verbose}, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&csvPath, "csv", "", "Path to the input CSV file (required)")
	cmd.Flags().StringVar(&baseDir, "output-dir", "", "Base directory for prospect subfolders (default: ~/supah-audits/batch-YYYY-MM-DD)")
	cmd.Flags().IntVar(&workers, "workers", 3, "Number of prospects to process in parallel")
	cmd.Flags().StringVar(&logoPath, "logo", "", "Agency logo to embed in all reports")
	cmd.Flags().StringVar(&agencyName, "agency-name", "Douro Digital", "Agency name for report header")
	cmd.Flags().IntVar(&depth, "depth", 2, "Crawl depth per prospect")
	cmd.Flags().IntVar(&maxPages, "max-pages", 30, "Max pages crawled per prospect")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without running")
	cmd.Flags().BoolVar(&skipLabs, "skip-labs", false, "Skip DataForSEO Labs calls (saves ~$0.03/prospect, thinner reports)")
	cmd.Flags().BoolVar(&skipBacklinks, "skip-backlinks", false, "Skip backlinks summary (saves ~$0.02/prospect)")

	return cmd
}

func readBatchCSV(path string) ([]BatchRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true

	// Read header.
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	required := []string{"url"}
	for _, req := range required {
		if _, ok := idx[req]; !ok {
			return nil, fmt.Errorf("csv is missing required column %q", req)
		}
	}

	var rows []BatchRow
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}
		get := func(col string) string {
			i, ok := idx[col]
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		urlVal := get("url")
		if urlVal == "" {
			continue
		}
		row := BatchRow{
			URL:           urlVal,
			ProspectName:  get("prospect_name"),
			Industry:      get("industry"),
			Location:      get("location"),
			CurrentAgency: get("current_agency"),
			CTAURL:        get("cta_url"),
			Pricing:       get("pricing"),
		}
		if v := get("avg_ticket"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				row.AvgTicket = f
			}
		}
		if v := get("close_rate"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				row.CloseRate = f
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func runBatch(bin string, rows []BatchRow, baseDir string, workers int, agencyName, logoPath string, depth, maxPages int, skipLabs, skipBacklinks bool, verbose bool) []BatchResult {
	if workers < 1 {
		workers = 1
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	results := make([]BatchResult, len(rows))

	for i, row := range rows {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, row BatchRow) {
			defer wg.Done()
			defer func() { <-sem }()
			start := time.Now()
			res := runSingleProspect(bin, row, baseDir, agencyName, logoPath, depth, maxPages, skipLabs, skipBacklinks, verbose)
			res.DurationMs = time.Since(start).Milliseconds()
			results[i] = res
		}(i, row)
	}
	wg.Wait()
	return results
}

func runSingleProspect(bin string, row BatchRow, baseDir, agencyName, logoPath string, depth, maxPages int, skipLabs, skipBacklinks bool, verbose bool) BatchResult {
	name := row.ProspectName
	if name == "" {
		name = domainOnly(row.URL)
	}
	slug := batchSlugify(name)
	dir := filepath.Join(baseDir, slug)

	res := BatchResult{
		ProspectName: name,
		URL:          row.URL,
		Directory:    dir,
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		res.Error = fmt.Sprintf("mkdir: %v", err)
		return res
	}

	run := func(args ...string) error {
		cmd := exec.Command(bin, args...)
		cmd.Dir = dir
		cmd.Env = os.Environ()
		if verbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		return cmd.Run()
	}

	steps := [][]string{
		{"init", "--url", row.URL},
		{"audit", "run", "--url", row.URL, "--depth", strconv.Itoa(depth), "--max-pages", strconv.Itoa(maxPages)},
	}

	if !skipLabs {
		loc := row.Location
		if loc == "" {
			loc = "United States"
		}
		steps = append(steps,
			[]string{"labs", "ranked-keywords", "--target", domainOnly(row.URL), "--location", loc, "--limit", "50"},
			[]string{"labs", "competitors", "--target", domainOnly(row.URL), "--location", loc, "--limit", "10"},
		)
	}
	if !skipBacklinks {
		steps = append(steps, []string{"backlinks", "summary", "--target", domainOnly(row.URL)})
	}
	steps = append(steps, []string{"analyze"})

	renderArgs := []string{"report", "render",
		"--agency-name", agencyName,
		"--prospect-name", name,
	}
	if logoPath != "" {
		renderArgs = append(renderArgs, "--logo", logoPath)
	}
	if row.Industry != "" {
		renderArgs = append(renderArgs, "--industry", row.Industry)
	}
	if row.Location != "" {
		renderArgs = append(renderArgs, "--location", row.Location)
	}
	if row.CurrentAgency != "" {
		renderArgs = append(renderArgs, "--current-agency", row.CurrentAgency)
	}
	if row.CTAURL != "" {
		renderArgs = append(renderArgs, "--cta-url", row.CTAURL)
	}
	if row.AvgTicket > 0 {
		renderArgs = append(renderArgs, "--avg-ticket", strconv.FormatFloat(row.AvgTicket, 'f', 2, 64))
	}
	if row.CloseRate > 0 {
		renderArgs = append(renderArgs, "--close-rate", strconv.FormatFloat(row.CloseRate, 'f', 4, 64))
	}
	if row.Pricing != "" {
		renderArgs = append(renderArgs, "--pricing", row.Pricing)
	}
	steps = append(steps, renderArgs)

	for _, step := range steps {
		if err := run(step...); err != nil {
			res.Error = fmt.Sprintf("%s: %v", strings.Join(step[:min(2, len(step))], " "), err)
			return res
		}
	}

	// List generated reports.
	reportsDir := filepath.Join(dir, "reports")
	if entries, err := os.ReadDir(reportsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".html") {
				res.ReportFiles = append(res.ReportFiles, filepath.Join(reportsDir, e.Name()))
			}
		}
	}

	res.Success = true
	return res
}

func domainOnly(u string) string {
	s := strings.TrimPrefix(u, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "www.")
	if i := strings.Index(s, "/"); i > 0 {
		s = s[:i]
	}
	return s
}

func batchSlugify(s string) string {
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
		return "prospect"
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
