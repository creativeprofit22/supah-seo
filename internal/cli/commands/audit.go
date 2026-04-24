package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/supah-seo/supah-seo/internal/audit"
	"github.com/supah-seo/supah-seo/internal/common/config"
	"github.com/supah-seo/supah-seo/internal/crawl"
	"github.com/supah-seo/supah-seo/internal/provider"
	_ "github.com/supah-seo/supah-seo/internal/provider/local"
	"github.com/supah-seo/supah-seo/internal/psi"
	"github.com/supah-seo/supah-seo/internal/state"
	"github.com/supah-seo/supah-seo/pkg/output"
)

const psiPageCap = 5

// NewAuditCmd returns the audit command group.
func NewAuditCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "SEO audit commands",
	}

	cmd.AddCommand(newAuditRunCmd(format, verbose))
	return cmd
}

func newAuditRunCmd(format *string, verbose *bool) *cobra.Command {
	var (
		targetURL string
		depth     int
		maxPages  int
		skipPSI   bool
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Crawl a website and run an SEO audit",
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

			// Save to .supah-seo/state.json if project is initialized
			if state.Exists(".") {
				st, loadErr := state.Load(".")
				if loadErr == nil {
					findings := make([]state.Finding, 0, len(auditResult.Issues))
					for _, issue := range auditResult.Issues {
						findings = append(findings, state.Finding{
							Rule:    issue.Rule,
							URL:     issue.URL,
							Value:   issue.Message,
							Verdict: string(issue.Severity),
							Why:     issue.Why,
							Fix:     issue.Fix,
						})
					}
					st.UpdateAudit(auditResult.Score, auditResult.PageCount, findings)
					st.AddHistory("audit", fmt.Sprintf("score=%.1f issues=%d pages=%d", auditResult.Score, len(auditResult.Issues), auditResult.PageCount))
					_ = st.Save(".")

					// Automatically run PSI on top pages unless skipped
					if !skipPSI {
						pages := selectTopPages(st, crawlResult)
						if len(pages) > 0 {
							_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Running PageSpeed Insights on top %d page(s)...\n", len(pages))
							psiResults := runPSIForPages(cmd.Context(), pages, cmd.ErrOrStderr())
							if len(psiResults) > 0 {
								// Merge with any existing PSI results
								merged := mergePSIResults(st, psiResults)
								st.UpsertPSI(merged)
								st.AddHistory("psi", fmt.Sprintf("pages=%d strategy=mobile", len(psiResults)))
								_ = st.Save(".")
							}
						}
					}
				}
			}

			return output.PrintSuccess(auditResult, map[string]any{
				"pages_audited": auditResult.PageCount,
				"total_issues":  len(auditResult.Issues),
				"verbose":       *verbose,
			}, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&targetURL, "url", "", "Target URL to audit (required)")
	cmd.Flags().IntVar(&depth, "depth", 2, "Maximum crawl depth")
	cmd.Flags().IntVar(&maxPages, "max-pages", 50, "Maximum number of pages to crawl")
	cmd.Flags().BoolVar(&skipPSI, "skip-psi", false, "Skip automatic PageSpeed Insights pass")

	return cmd
}

// selectTopPages picks up to psiPageCap pages to analyse with PSI.
// If GSC data is present, we use the top pages by impressions.
// Otherwise we fall back to the first crawled pages.
func selectTopPages(st *state.State, crawlResult crawl.Result) []string {
	if st.GSC != nil && len(st.GSC.TopPages) > 0 {
		type row struct {
			url         string
			impressions float64
		}
		rows := make([]row, 0, len(st.GSC.TopPages))
		for _, p := range st.GSC.TopPages {
			if p.Key != "" {
				rows = append(rows, row{url: p.Key, impressions: p.Impressions})
			}
		}
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].impressions > rows[j].impressions
		})
		cap := psiPageCap
		if len(rows) < cap {
			cap = len(rows)
		}
		urls := make([]string, cap)
		for i := 0; i < cap; i++ {
			urls[i] = rows[i].url
		}
		return urls
	}

	// Fall back to crawled pages
	cap := psiPageCap
	if len(crawlResult.Pages) < cap {
		cap = len(crawlResult.Pages)
	}
	urls := make([]string, cap)
	for i := 0; i < cap; i++ {
		urls[i] = crawlResult.Pages[i].URL
	}
	return urls
}

// runPSIForPages calls PSI for each URL, adds a delay between calls, and
// returns all successful results. Per-page errors are logged but do not abort.
// Auth resolution: API key → GSC OAuth token → unauthenticated (same as psi run).
func runPSIForPages(ctx context.Context, pages []string, errW io.Writer) []state.PSIResult {
	if errW == nil {
		errW = os.Stderr
	}

	// Resolve auth: API key → GSC OAuth → unauthenticated
	apiKey := os.Getenv("SUPAHSEO_PSI_API_KEY")
	if apiKey == "" {
		if cfg, err := config.Load(); err == nil {
			apiKey = cfg.PSIAPIKey
		}
	}

	var client *psi.Client
	authenticated := false
	if apiKey != "" {
		client = psi.NewClient(apiKey, nil)
		authenticated = true
	} else if token, err := resolveGSCAccessToken(); err == nil && token != "" {
		client = psi.NewClientWithToken(token, nil)
		authenticated = true
	} else {
		client = psi.NewClient("", nil)
	}

	delay := 2 * time.Second
	if authenticated {
		delay = 500 * time.Millisecond
	}

	results := make([]state.PSIResult, 0, len(pages))
	for i, u := range pages {
		if i > 0 {
			select {
			case <-ctx.Done():
				return results
			case <-time.After(delay):
			}
		}

		r, err := client.Run(u, "mobile")
		if err != nil {
			_, _ = fmt.Fprintf(errW, "PSI skipped %s: %v\n", u, err)
			continue
		}
		results = append(results, state.PSIResult{
			URL:              r.URL,
			PerformanceScore: r.PerformanceScore,
			LCP:              r.LCP,
			CLS:              r.CLS,
			Strategy:         r.Strategy,
		})
	}
	return results
}

// mergePSIResults keeps existing PSI entries for URLs not in the new batch,
// then appends (or replaces) with the freshly fetched results.
func mergePSIResults(st *state.State, fresh []state.PSIResult) state.PSIData {
	freshURLs := make(map[string]struct{}, len(fresh))
	for _, r := range fresh {
		freshURLs[r.URL] = struct{}{}
	}

	var kept []state.PSIResult
	if st.PSI != nil {
		for _, old := range st.PSI.Pages {
			if _, replaced := freshURLs[old.URL]; !replaced {
				kept = append(kept, old)
			}
		}
	}

	return state.PSIData{
		Pages: append(kept, fresh...),
	}
}
