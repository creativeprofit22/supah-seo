package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/supah-seo/supah-seo/internal/common/cache"
	"github.com/supah-seo/supah-seo/internal/common/config"
	"github.com/supah-seo/supah-seo/internal/common/cost"
	"github.com/supah-seo/supah-seo/internal/serp"
	serpdforseo "github.com/supah-seo/supah-seo/internal/serp/dataforseo"
	"github.com/supah-seo/supah-seo/internal/serp/serpapi"
	"github.com/supah-seo/supah-seo/internal/state"
	"github.com/supah-seo/supah-seo/pkg/output"
	"github.com/spf13/cobra"
)

// NewSERPCmd returns the serp command group.
func NewSERPCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serp",
		Short: "SERP analysis commands",
		Long:  `Analyze search engine results pages using paid SERP providers. Supports --dry-run and cost estimation.`,
	}

	cmd.AddCommand(
		newSERPAnalyzeCmd(format, verbose),
		newSERPCompareCmd(format, verbose),
		newSERPBatchCmd(format, verbose),
	)

	return cmd
}

func newSERPAnalyzeCmd(format *string, verbose *bool) *cobra.Command {
	var query, location, language string
	var numResults int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze SERP for a query",
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" {
				return output.PrintCodedError(output.ErrSERPFailed, "query is required",
					fmt.Errorf("use --query to specify a search query"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			provider, err := serpProvider(cfg)
			if err != nil {
				return output.PrintCodedError(output.ErrSERPFailed, "failed to initialize SERP provider", err, nil, output.Format(*format))
			}

			req := serp.AnalyzeRequest{
				Query:      query,
				Location:   location,
				Language:   language,
				NumResults: numResults,
			}

			// Compute cost estimate
			estimate, err := provider.Estimate(req)
			if err != nil {
				return output.PrintCodedError(output.ErrEstimateFailed, "failed to estimate cost", err, nil, output.Format(*format))
			}

			approval := cost.EvaluateApproval(estimate, cfg.ApprovalThresholdUSD)

			meta := map[string]any{
				"estimated_cost":    estimate.Amount,
				"currency":          estimate.Currency,
				"basis":             estimate.Basis,
				"requires_approval": approval.RequiresApproval,
				"dry_run":           dryRun,
				"source":            provider.Name(),
				"verbose":           *verbose,
			}

			if dryRun {
				return output.PrintSuccess(map[string]any{
					"query":    query,
					"provider": provider.Name(),
					"status":   "dry_run",
				}, meta, output.Format(*format))
			}

			if approval.RequiresApproval {
				meta["reason"] = approval.Reason
				return output.PrintCodedError(output.ErrApprovalRequired, "cost exceeds approval threshold",
					fmt.Errorf("%s", approval.Reason), meta, output.Format(*format))
			}

			// Check cache
			cacheStore := cache.NewFileStore()
			cacheKey := map[string]any{"provider": provider.Name(), "request": req}
			if rec, hit, cacheErr := cacheStore.Get(provider.Name(), cacheKey); hit && cacheErr == nil {
				var resp serp.AnalyzeResponse
				if jsonErr := json.Unmarshal(rec.Payload, &resp); jsonErr == nil {
					meta["cached"] = true
					meta["fetched_at"] = rec.FetchedAt
					return output.PrintSuccess(resp, meta, output.Format(*format))
				}
			}

			// Execute
			resp, err := provider.Analyze(req)
			if err != nil {
				return output.PrintCodedError(output.ErrSERPFailed, "SERP analysis failed", err, nil, output.Format(*format))
			}

			// Cache result
			fetchedAt := time.Now().Format(time.RFC3339)
			if payload, jsonErr := json.Marshal(resp); jsonErr == nil {
				_ = cacheStore.Set(provider.Name(), cacheKey, cache.Record{
					Payload:    payload,
					Source:     provider.Name(),
					FetchedAt:  fetchedAt,
					TTLSeconds: 3600,
				})
			}

			meta["cached"] = false
			meta["fetched_at"] = fetchedAt

			// Persist to project state if it exists.
			if state.Exists(".") {
				if st, loadErr := state.Load("."); loadErr == nil {
					persistSERPToState(st, resp)
					_ = st.Save(".")
				}
			}

			return output.PrintSuccess(resp, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&query, "query", "", "Search query to analyze (required)")
	cmd.Flags().StringVar(&location, "location", "", "Search location")
	cmd.Flags().StringVar(&language, "language", "", "Search language code")
	cmd.Flags().IntVar(&numResults, "num", 10, "Number of results to fetch")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

func newSERPCompareCmd(format *string, verbose *bool) *cobra.Command {
	var queries []string
	var location, language string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare SERP results for multiple queries",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(queries) < 2 {
				return output.PrintCodedError(output.ErrSERPFailed, "at least 2 queries required",
					fmt.Errorf("use --query multiple times to specify queries"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			provider, err := serpProvider(cfg)
			if err != nil {
				return output.PrintCodedError(output.ErrSERPFailed, "failed to initialize SERP provider", err, nil, output.Format(*format))
			}

			// Estimate total cost
			totalEstimate := cost.Estimate{Currency: cost.CurrencyUSD}
			for _, q := range queries {
				est, err := provider.Estimate(serp.AnalyzeRequest{Query: q, Location: location, Language: language})
				if err != nil {
					return output.PrintCodedError(output.ErrEstimateFailed, "failed to estimate cost", err, nil, output.Format(*format))
				}
				totalEstimate.Amount += est.Amount
				totalEstimate.Basis = est.Basis
			}

			approval := cost.EvaluateApproval(totalEstimate, cfg.ApprovalThresholdUSD)

			meta := map[string]any{
				"estimated_cost":    totalEstimate.Amount,
				"currency":          totalEstimate.Currency,
				"requires_approval": approval.RequiresApproval,
				"dry_run":           dryRun,
				"source":            provider.Name(),
				"query_count":       len(queries),
				"verbose":           *verbose,
			}

			if dryRun {
				return output.PrintSuccess(map[string]any{
					"queries":  queries,
					"provider": provider.Name(),
					"status":   "dry_run",
				}, meta, output.Format(*format))
			}

			if approval.RequiresApproval {
				meta["reason"] = approval.Reason
				return output.PrintCodedError(output.ErrApprovalRequired, "cost exceeds approval threshold",
					fmt.Errorf("%s", approval.Reason), meta, output.Format(*format))
			}

			results := make(map[string]*serp.AnalyzeResponse, len(queries))
			for _, q := range queries {
				resp, err := provider.Analyze(serp.AnalyzeRequest{
					Query:    q,
					Location: location,
					Language: language,
				})
				if err != nil {
					return output.PrintCodedError(output.ErrSERPFailed, fmt.Sprintf("SERP analysis failed for query %q", q), err, nil, output.Format(*format))
				}
				results[q] = resp
			}

			meta["cached"] = false
			meta["fetched_at"] = time.Now().Format(time.RFC3339)

			return output.PrintSuccess(results, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringArrayVar(&queries, "query", nil, "Search queries to compare (use multiple times)")
	cmd.Flags().StringVar(&location, "location", "", "Search location")
	cmd.Flags().StringVar(&language, "language", "", "Search language code")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

func newSERPBatchCmd(format *string, verbose *bool) *cobra.Command {
	var keywordsFlag, location, language string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Batch SERP analysis using DataForSEO Standard (POST-GET) method",
		Long: `Submit up to 100 keywords in a single API call using DataForSEO's Standard queue.
Costs $0.0006/keyword (vs $0.002 live) — 70% cheaper for bulk analysis.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if keywordsFlag == "" {
				return output.PrintCodedError(output.ErrSERPFailed, "keywords are required",
					fmt.Errorf("use --keywords to specify a comma-separated list of keywords"), nil, output.Format(*format))
			}

			// Split and trim keywords.
			raw := strings.Split(keywordsFlag, ",")
			var keywords []string
			for _, kw := range raw {
				kw = strings.TrimSpace(kw)
				if kw != "" {
					keywords = append(keywords, kw)
				}
			}
			if len(keywords) == 0 {
				return output.PrintCodedError(output.ErrSERPFailed, "no valid keywords provided",
					fmt.Errorf("--keywords must contain at least one non-empty keyword"), nil, output.Format(*format))
			}

			// Cap at 100 keywords per DataForSEO batch limit.
			if len(keywords) > 100 {
				keywords = keywords[:100]
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrSERPFailed, "DataForSEO credentials required",
					fmt.Errorf("dataforseo_login and dataforseo_password not configured; run 'supah-seo login' to set them"), nil, output.Format(*format))
			}

			adapter := serpdforseo.New(cfg.DataForSEOLogin, cfg.DataForSEOPassword)

			estimate, err := adapter.BatchEstimate(len(keywords))
			if err != nil {
				return output.PrintCodedError(output.ErrEstimateFailed, "failed to estimate cost", err, nil, output.Format(*format))
			}

			approval := cost.EvaluateApproval(estimate, cfg.ApprovalThresholdUSD)

			meta := map[string]any{
				"estimated_cost":    estimate.Amount,
				"currency":          estimate.Currency,
				"basis":             estimate.Basis,
				"requires_approval": approval.RequiresApproval,
				"dry_run":           dryRun,
				"source":            "dataforseo",
				"keyword_count":     len(keywords),
				"verbose":           *verbose,
			}

			if dryRun {
				return output.PrintSuccess(map[string]any{
					"keywords": keywords,
					"provider": "dataforseo",
					"method":   "standard_queue",
					"status":   "dry_run",
				}, meta, output.Format(*format))
			}

			if approval.RequiresApproval {
				meta["reason"] = approval.Reason
				return output.PrintCodedError(output.ErrApprovalRequired, "cost exceeds approval threshold",
					fmt.Errorf("%s", approval.Reason), meta, output.Format(*format))
			}

			// Build requests.
			reqs := make([]serp.AnalyzeRequest, len(keywords))
			for i, kw := range keywords {
				reqs[i] = serp.AnalyzeRequest{
					Query:    kw,
					Location: location,
					Language: language,
				}
			}

			responses, err := adapter.AnalyzeBatch(reqs)
			if err != nil && responses == nil {
				return output.PrintCodedError(output.ErrSERPFailed, "batch SERP analysis failed", err, nil, output.Format(*format))
			}

			if err != nil {
				meta["warning"] = err.Error()
			}

			meta["cached"] = false
			meta["fetched_at"] = time.Now().Format(time.RFC3339)

			// Persist to project state if it exists.
			if state.Exists(".") {
				if st, loadErr := state.Load("."); loadErr == nil {
					for _, resp := range responses {
						if resp != nil {
							persistSERPToState(st, resp)
						}
					}
					_ = st.Save(".")
				}
			}

			return output.PrintSuccess(responses, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&keywordsFlag, "keywords", "", "Comma-separated list of keywords to analyze (required, max 100)")
	cmd.Flags().StringVar(&location, "location", "", "Search location (defaults to adapter default)")
	cmd.Flags().StringVar(&language, "language", "", "Search language code (defaults to en)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

// persistSERPToState converts an AnalyzeResponse and upserts it into project state.
func persistSERPToState(st *state.State, resp *serp.AnalyzeResponse) {
	// Map features.
	features := make([]state.SERPFeatureRecord, len(resp.Features))
	for i, f := range resp.Features {
		features[i] = state.SERPFeatureRecord{
			Type:     string(f.Type),
			Position: f.Position,
			Title:    f.Title,
			URL:      f.URL,
			Domain:   f.Domain,
		}
	}

	// Map related questions to strings.
	questions := make([]string, len(resp.RelatedQuestions))
	for i, rq := range resp.RelatedQuestions {
		questions[i] = rq.Question
	}

	// Extract top domains from first 3 organic results.
	var topDomains []string
	for i, r := range resp.OrganicResults {
		if i >= 3 {
			break
		}
		d := r.Domain
		if d == "" {
			if u, err := url.Parse(r.Link); err == nil {
				d = u.Hostname()
			}
		}
		if d != "" {
			topDomains = append(topDomains, d)
		}
	}

	// Determine our position by matching site domain.
	ourPos := -1
	siteDomain := ""
	if st.Site != "" {
		if u, err := url.Parse(st.Site); err == nil {
			siteDomain = strings.TrimPrefix(u.Hostname(), "www.")
		}
	}
	if siteDomain != "" {
		for _, r := range resp.OrganicResults {
			d := r.Domain
			if d == "" {
				if u, err := url.Parse(r.Link); err == nil {
					d = u.Hostname()
				}
			}
			if strings.TrimPrefix(d, "www.") == siteDomain {
				ourPos = r.Position
				break
			}
		}
	}

	qr := state.SERPQueryResult{
		Query:            resp.Query,
		HasAIOverview:    resp.HasAIOverview,
		Features:         features,
		RelatedQuestions: questions,
		TopDomains:       topDomains,
		OurPosition:      ourPos,
	}

	// Initialize SERP data if nil.
	if st.SERP == nil {
		st.SERP = &state.SERPData{}
	}
	st.SERP.LastRun = time.Now().UTC().Format(time.RFC3339)

	// Upsert: replace existing query or append.
	found := false
	for i, existing := range st.SERP.Queries {
		if existing.Query == resp.Query {
			st.SERP.Queries[i] = qr
			found = true
			break
		}
	}
	if !found {
		st.SERP.Queries = append(st.SERP.Queries, qr)
	}

	st.AddHistory("serp", fmt.Sprintf("analyzed query %q, %d features detected", resp.Query, len(features)))
}

func serpProvider(cfg *config.Config) (serp.Provider, error) {
	switch cfg.SERPProvider {
	case "dataforseo":
		if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
			return nil, fmt.Errorf("dataforseo_login and dataforseo_password not configured; run 'supah-seo login' to set them")
		}
		return serpdforseo.New(cfg.DataForSEOLogin, cfg.DataForSEOPassword), nil
	case "serpapi", "":
		// Fall back to DataForSEO when login is set and provider is unset/default
		if (cfg.SERPProvider == "" || cfg.SERPProvider == "serpapi") && cfg.DataForSEOLogin != "" && cfg.DataForSEOPassword != "" {
			return serpdforseo.New(cfg.DataForSEOLogin, cfg.DataForSEOPassword), nil
		}
		if cfg.SERPAPIKey == "" {
			return nil, fmt.Errorf("serp_api_key not configured; set via 'supah-seo config set serp_api_key <key>' or run 'supah-seo login' to configure DataForSEO")
		}
		return serpapi.New(cfg.SERPAPIKey), nil
	default:
		return nil, fmt.Errorf("unsupported SERP provider: %s", cfg.SERPProvider)
	}
}
