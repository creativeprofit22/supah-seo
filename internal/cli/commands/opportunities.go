package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/supah-seo/supah-seo/internal/auth"
	"github.com/supah-seo/supah-seo/internal/common/cache"
	"github.com/supah-seo/supah-seo/internal/common/config"
	"github.com/supah-seo/supah-seo/internal/common/cost"
	"github.com/supah-seo/supah-seo/internal/gsc"
	"github.com/supah-seo/supah-seo/internal/opportunities"
	"github.com/supah-seo/supah-seo/internal/serp"
	serpdforseo "github.com/supah-seo/supah-seo/internal/serp/dataforseo"
	"github.com/supah-seo/supah-seo/internal/serp/serpapi"
	"github.com/supah-seo/supah-seo/internal/state"
	"github.com/supah-seo/supah-seo/pkg/output"
)

// NewOpportunitiesCmd returns the opportunities command group.
func NewOpportunitiesCmd(format *string, verbose *bool) *cobra.Command {
	var startDate, endDate string
	var rowLimit int
	var withSERP bool
	var serpQueries int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "opportunities",
		Short: "Find SEO opportunities by combining GSC data with optional SERP analysis",
		Long: `Analyze Google Search Console data to find SEO improvement opportunities.
Optionally enrich with live SERP data for validation (paid, supports --dry-run).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			// GSC auth check
			store := auth.NewFileTokenStore()
			st, err := store.Status("gsc")
			if err != nil {
				return output.PrintCodedError(output.ErrAuthFailed, "failed to check auth status", err, nil, output.Format(*format))
			}
			if !st.Authenticated {
				return output.PrintCodedError(output.ErrAuthRequired, "not authenticated with GSC",
					fmt.Errorf("run 'supah-seo auth login gsc' first (token may be missing or expired)"), nil, output.Format(*format))
			}
			token, err := store.Load("gsc")
			if err != nil {
				return output.PrintCodedError(output.ErrAuthRequired, "not authenticated with GSC",
					fmt.Errorf("run 'supah-seo auth login gsc' first"), nil, output.Format(*format))
			}

			if cfg.GSCProperty == "" {
				return output.PrintCodedError(output.ErrGSCFailed, "no GSC property configured",
					fmt.Errorf("run 'supah-seo gsc sites use <url>' or set gsc_property in config"), nil, output.Format(*format))
			}

			if startDate == "" {
				startDate = time.Now().AddDate(0, 0, -28).Format("2006-01-02")
			}
			if endDate == "" {
				endDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
			}

			meta := map[string]any{
				"source":     "gsc",
				"start_date": startDate,
				"end_date":   endDate,
				"with_serp":  withSERP,
				"dry_run":    dryRun,
				"verbose":    *verbose,
			}

			// Cost estimation for SERP enrichment
			if withSERP {
				usingDataForSEO := cfg.SERPProvider == "dataforseo" ||
					(cfg.SERPProvider == "" && cfg.DataForSEOLogin != "" && cfg.DataForSEOPassword != "")

				if !usingDataForSEO && cfg.SERPAPIKey == "" {
					return output.PrintCodedError(output.ErrSERPFailed, "no SERP provider configured",
						fmt.Errorf("run 'supah-seo login' to configure DataForSEO, or set serp_api_key for SerpAPI"), nil, output.Format(*format))
				}

				var unitCost float64
				var basisStr string
				if usingDataForSEO {
					unitCost = 0.002
					basisStr = fmt.Sprintf("dataforseo: %d queries @ $0.002/query (live mode)", serpQueries)
				} else {
					unitCost = 0.01
					basisStr = fmt.Sprintf("serpapi: %d queries @ $0.01/query", serpQueries)
				}

				estimate, err := cost.BuildEstimate(cost.EstimateInput{
					UnitCostUSD: unitCost,
					Units:       serpQueries,
					Basis:       basisStr,
				})
				if err != nil {
					return output.PrintCodedError(output.ErrEstimateFailed, "failed to estimate cost", err, nil, output.Format(*format))
				}

				approval := cost.EvaluateApproval(estimate, cfg.ApprovalThresholdUSD)
				meta["estimated_cost"] = estimate.Amount
				meta["currency"] = estimate.Currency
				meta["requires_approval"] = approval.RequiresApproval

				if dryRun {
					return output.PrintSuccess(map[string]any{
						"status":       "dry_run",
						"serp_queries": serpQueries,
					}, meta, output.Format(*format))
				}

				if approval.RequiresApproval {
					meta["reason"] = approval.Reason
					return output.PrintCodedError(output.ErrApprovalRequired, "cost exceeds approval threshold",
						fmt.Errorf("%s", approval.Reason), meta, output.Format(*format))
				}
			}

			// Fetch GSC opportunity seeds
			gscClient := gsc.NewClient(token.AccessToken)
			seeds, err := gscClient.QueryOpportunities(cfg.GSCProperty, startDate, endDate, rowLimit, "web")
			if err != nil {
				return output.PrintCodedError(output.ErrGSCFailed, "failed to query GSC opportunities", err, nil, output.Format(*format))
			}

			mergeInput := opportunities.MergeInput{
				GSCSeeds: seeds,
			}

			// Load Labs keyword data from state if available
			var labsMap map[string]opportunities.LabsKeywordInfo
			if state.Exists(".") {
				if st, loadErr := state.Load("."); loadErr == nil && st.Labs != nil && len(st.Labs.Keywords) > 0 {
					labsMap = make(map[string]opportunities.LabsKeywordInfo, len(st.Labs.Keywords))
					for _, kw := range st.Labs.Keywords {
						labsMap[kw.Keyword] = opportunities.LabsKeywordInfo{
							SearchVolume: kw.SearchVolume,
							Difficulty:   kw.Difficulty,
							Intent:       kw.Intent,
						}
					}
					mergeInput.LabsKeywords = labsMap
				}
			}
			meta["labs_enriched"] = len(labsMap) > 0

			// Optional SERP enrichment
			if withSERP && len(seeds) > 0 {
				serpResults, serpMeta, serpErr := fetchSERPForSeeds(cfg, seeds, serpQueries)
				if serpErr != nil {
					return output.PrintCodedError(output.ErrSERPFailed, "SERP enrichment failed", serpErr, nil, output.Format(*format))
				}
				mergeInput.SERPResults = serpResults
				for k, v := range serpMeta {
					meta[k] = v
				}
			}

			opps := opportunities.Merge(mergeInput)
			meta["count"] = len(opps)

			return output.PrintSuccess(opps, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD, default: 28 days ago)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD, default: yesterday)")
	cmd.Flags().IntVar(&rowLimit, "limit", 1000, "Maximum GSC rows to return")
	cmd.Flags().BoolVar(&withSERP, "with-serp", false, "Enrich with live SERP data (paid)")
	cmd.Flags().IntVar(&serpQueries, "serp-queries", 10, "Number of top seeds to validate via SERP")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing SERP queries")

	return cmd
}

func fetchSERPForSeeds(cfg *config.Config, seeds []gsc.OpportunitySeed, maxQueries int) (map[string]*serp.AnalyzeResponse, map[string]any, error) {
	var provider serp.Provider
	usingDataForSEO := cfg.SERPProvider == "dataforseo" ||
		((cfg.SERPProvider == "" || cfg.SERPProvider == "serpapi") && cfg.DataForSEOLogin != "" && cfg.DataForSEOPassword != "")
	if usingDataForSEO {
		provider = serpdforseo.New(cfg.DataForSEOLogin, cfg.DataForSEOPassword)
	} else {
		provider = serpapi.New(cfg.SERPAPIKey)
	}
	cacheStore := cache.NewFileStore()
	results := make(map[string]*serp.AnalyzeResponse)
	meta := map[string]any{}

	// Deduplicate queries
	seen := map[string]bool{}
	var uniqueQueries []string
	for _, seed := range seeds {
		if !seen[seed.Query] && len(uniqueQueries) < maxQueries {
			seen[seed.Query] = true
			uniqueQueries = append(uniqueQueries, seed.Query)
		}
	}

	cacheHits := 0
	for _, q := range uniqueQueries {
		req := serp.AnalyzeRequest{Query: q}
		providerName := provider.Name()
		cacheKey := map[string]any{"provider": providerName, "request": req}

		// Try cache first
		if rec, hit, err := cacheStore.Get(providerName, cacheKey); hit && err == nil {
			var resp serp.AnalyzeResponse
			if jsonErr := json.Unmarshal(rec.Payload, &resp); jsonErr == nil {
				results[q] = &resp
				cacheHits++
				continue
			}
		}

		resp, err := provider.Analyze(req)
		if err != nil {
			return nil, nil, fmt.Errorf("SERP query %q failed: %w", q, err)
		}
		results[q] = resp

		// Cache
		fetchedAt := time.Now().Format(time.RFC3339)
		if payload, jsonErr := json.Marshal(resp); jsonErr == nil {
			_ = cacheStore.Set(providerName, cacheKey, cache.Record{
				Payload:    payload,
				Source:     providerName,
				FetchedAt:  fetchedAt,
				TTLSeconds: 3600,
			})
		}
	}

	meta["serp_queries_executed"] = len(uniqueQueries) - cacheHits
	meta["serp_cache_hits"] = cacheHits
	meta["fetched_at"] = time.Now().Format(time.RFC3339)

	return results, meta, nil
}
