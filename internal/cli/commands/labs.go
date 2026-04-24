package commands

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/supah-seo/supah-seo/internal/common/config"
	"github.com/supah-seo/supah-seo/internal/common/cost"
	"github.com/supah-seo/supah-seo/internal/dataforseo"
	"github.com/supah-seo/supah-seo/internal/state"
	"github.com/supah-seo/supah-seo/pkg/output"
)

// NewLabsCmd returns the labs command group.
func NewLabsCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "labs",
		Short: "DataForSEO Labs commands",
		Long:  `Query DataForSEO Labs datasets for domain and keyword intelligence.`,
	}

	cmd.AddCommand(
		newLabsRankedKeywordsCmd(format, verbose),
		newLabsKeywordsCmd(format, verbose),
		newLabsOverviewCmd(format, verbose),
		newLabsCompetitorsCmd(format, verbose),
		newLabsKeywordIdeasCmd(format, verbose),
		newLabsBulkDifficultyCmd(format, verbose),
	)

	return cmd
}

func newLabsRankedKeywordsCmd(format *string, verbose *bool) *cobra.Command {
	var target, location, language string
	var limit, minVolume int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "ranked-keywords",
		Short: "Get keywords a domain or URL ranks for",
		Long:  `Retrieve ranked keywords for a domain or URL from DataForSEO Labs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if target == "" {
				return output.PrintCodedError(output.ErrLabsFailed, "target is required",
					fmt.Errorf("use --target to specify a domain or URL"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrLabsFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: 0.01,
				Units:       1,
				Basis:       "dataforseo labs ranked keywords: 1 task @ $0.01/task",
			})
			if err != nil {
				return output.PrintCodedError(output.ErrEstimateFailed, "failed to estimate cost", err, nil, output.Format(*format))
			}

			approval := cost.EvaluateApproval(estimate, cfg.ApprovalThresholdUSD)

			meta := map[string]any{
				"estimated_cost":    estimate.Amount,
				"currency":          estimate.Currency,
				"requires_approval": approval.RequiresApproval,
				"dry_run":           dryRun,
				"source":            "dataforseo",
				"verbose":           *verbose,
			}

			if dryRun {
				return output.PrintSuccess(map[string]any{
					"target": target,
					"status": "dry_run",
				}, meta, output.Format(*format))
			}

			if approval.RequiresApproval {
				meta["reason"] = approval.Reason
				return output.PrintCodedError(output.ErrApprovalRequired, "cost exceeds approval threshold",
					fmt.Errorf("%s", approval.Reason), meta, output.Format(*format))
			}

			client := dataforseo.New(cfg.DataForSEOLogin, cfg.DataForSEOPassword)

			task := map[string]any{
				"target":        target,
				"location_name": location,
				"language_code": language,
				"limit":         limit,
			}
			if minVolume > 0 {
				task["filters"] = []any{
					[]any{"keyword_data.keyword_info.search_volume", ">", minVolume},
				}
			}

			raw, err := client.Post("/v3/dataforseo_labs/google/ranked_keywords/live", []map[string]any{task})
			if err != nil {
				return output.PrintCodedError(output.ErrLabsFailed, "ranked keywords request failed", err, nil, output.Format(*format))
			}

			var envelope struct {
				StatusCode    int    `json:"status_code"`
				StatusMessage string `json:"status_message"`
				Tasks         []struct {
					StatusCode    int               `json:"status_code"`
					StatusMessage string            `json:"status_message"`
					Result        []json.RawMessage `json:"result"`
				} `json:"tasks"`
			}

			if jsonErr := json.Unmarshal(raw, &envelope); jsonErr != nil {
				return output.PrintCodedError(output.ErrLabsFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
			}

			if envelope.StatusCode != 20000 {
				return output.PrintCodedError(output.ErrLabsFailed,
					fmt.Sprintf("DataForSEO error %d: %s", envelope.StatusCode, envelope.StatusMessage),
					fmt.Errorf("code %d", envelope.StatusCode), nil, output.Format(*format))
			}

			var results []json.RawMessage
			if len(envelope.Tasks) > 0 {
				results = envelope.Tasks[0].Result
			}

			// Persist to state if project is initialized.
			if state.Exists(".") && len(results) > 0 {
				type labsRankedResult struct {
					Items []struct {
						KeywordData struct {
							Keyword     string `json:"keyword"`
							KeywordInfo struct {
								SearchVolume int     `json:"search_volume"`
								CPC          float64 `json:"cpc"`
							} `json:"keyword_info"`
							KeywordProperties struct {
								Difficulty float64 `json:"keyword_difficulty"`
							} `json:"keyword_properties"`
							SearchIntentInfo struct {
								MainIntent string `json:"main_intent"`
							} `json:"search_intent_info"`
						} `json:"keyword_data"`
						RankedSerpElement struct {
							SerpItem struct {
								RankGroup int `json:"rank_group"`
							} `json:"serp_item"`
						} `json:"ranked_serp_element"`
					} `json:"items"`
				}

				var parsed labsRankedResult
				if err := json.Unmarshal(results[0], &parsed); err == nil {
					var keywords []state.LabsKeyword
					for _, item := range parsed.Items {
						keywords = append(keywords, state.LabsKeyword{
							Keyword:      item.KeywordData.Keyword,
							SearchVolume: item.KeywordData.KeywordInfo.SearchVolume,
							Difficulty:   item.KeywordData.KeywordProperties.Difficulty,
							CPC:          item.KeywordData.KeywordInfo.CPC,
							Intent:       item.KeywordData.SearchIntentInfo.MainIntent,
							Position:     item.RankedSerpElement.SerpItem.RankGroup,
						})
					}

					if st, loadErr := state.Load("."); loadErr == nil {
						if st.Labs == nil {
							st.Labs = &state.LabsData{}
						}
						st.Labs.LastRun = time.Now().UTC().Format(time.RFC3339)
						st.Labs.Target = target
						st.Labs.Keywords = keywords
						st.AddHistory("labs", fmt.Sprintf("ranked-keywords for %s: %d keywords", target, len(keywords)))
						_ = st.Save(".")
					}
				}
			}

			meta["fetched_at"] = time.Now().Format(time.RFC3339)
			meta["target"] = target

			return output.PrintSuccess(results, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Domain or URL to analyze (required)")
	cmd.Flags().StringVar(&location, "location", "Australia", "Location name (e.g. 'Australia')")
	cmd.Flags().StringVar(&language, "language", "en", "Language name (e.g. 'en')")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of results")
	cmd.Flags().IntVar(&minVolume, "min-volume", 0, "Minimum monthly search volume filter")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

func newLabsKeywordsCmd(format *string, verbose *bool) *cobra.Command {
	var target, location, language string
	var limit int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "keywords",
		Short: "Get keyword ideas relevant to a domain",
		Long:  `Retrieve keywords for a site from DataForSEO Labs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if target == "" {
				return output.PrintCodedError(output.ErrLabsFailed, "target is required",
					fmt.Errorf("use --target to specify a domain"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrLabsFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: 0.01,
				Units:       1,
				Basis:       "dataforseo labs keywords for site: 1 task @ $0.01/task",
			})
			if err != nil {
				return output.PrintCodedError(output.ErrEstimateFailed, "failed to estimate cost", err, nil, output.Format(*format))
			}

			approval := cost.EvaluateApproval(estimate, cfg.ApprovalThresholdUSD)

			meta := map[string]any{
				"estimated_cost":    estimate.Amount,
				"currency":          estimate.Currency,
				"requires_approval": approval.RequiresApproval,
				"dry_run":           dryRun,
				"source":            "dataforseo",
				"verbose":           *verbose,
			}

			if dryRun {
				return output.PrintSuccess(map[string]any{
					"target": target,
					"status": "dry_run",
				}, meta, output.Format(*format))
			}

			if approval.RequiresApproval {
				meta["reason"] = approval.Reason
				return output.PrintCodedError(output.ErrApprovalRequired, "cost exceeds approval threshold",
					fmt.Errorf("%s", approval.Reason), meta, output.Format(*format))
			}

			locationCodes := map[string]int{
				"Australia":      2036,
				"United States":  2840,
				"United Kingdom": 2826,
				"Canada":         2124,
				"New Zealand":    2554,
			}
			locationCode, ok := locationCodes[location]
			if !ok {
				return output.PrintCodedError(output.ErrLabsFailed, "unsupported location",
					fmt.Errorf("supported locations: Australia, United States, United Kingdom, Canada, New Zealand"), nil, output.Format(*format))
			}

			client := dataforseo.New(cfg.DataForSEOLogin, cfg.DataForSEOPassword)

			task := map[string]any{
				"target":        target,
				"location_code": locationCode,
				"language_code": language,
				"limit":         limit,
			}

			raw, err := client.Post("/v3/dataforseo_labs/google/keywords_for_site/live", []map[string]any{task})
			if err != nil {
				return output.PrintCodedError(output.ErrLabsFailed, "keywords for site request failed", err, nil, output.Format(*format))
			}

			var envelope struct {
				StatusCode    int    `json:"status_code"`
				StatusMessage string `json:"status_message"`
				Tasks         []struct {
					StatusCode    int               `json:"status_code"`
					StatusMessage string            `json:"status_message"`
					Result        []json.RawMessage `json:"result"`
				} `json:"tasks"`
			}

			if jsonErr := json.Unmarshal(raw, &envelope); jsonErr != nil {
				return output.PrintCodedError(output.ErrLabsFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
			}

			if envelope.StatusCode != 20000 {
				return output.PrintCodedError(output.ErrLabsFailed,
					fmt.Sprintf("DataForSEO error %d: %s", envelope.StatusCode, envelope.StatusMessage),
					fmt.Errorf("code %d", envelope.StatusCode), nil, output.Format(*format))
			}

			var results []json.RawMessage
			if len(envelope.Tasks) > 0 {
				results = envelope.Tasks[0].Result
			}

			meta["fetched_at"] = time.Now().Format(time.RFC3339)
			meta["target"] = target

			return output.PrintSuccess(results, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Domain to analyze (required)")
	cmd.Flags().StringVar(&location, "location", "Australia", "Location name (e.g. 'Australia', 'United States')")
	cmd.Flags().StringVar(&language, "language", "en", "Language code (e.g. 'en')")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of results")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

func newLabsOverviewCmd(format *string, verbose *bool) *cobra.Command {
	var target, location, language string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "overview",
		Short: "Get domain ranking overview",
		Long:  `Retrieve domain rank overview from DataForSEO Labs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if target == "" {
				return output.PrintCodedError(output.ErrLabsFailed, "target is required",
					fmt.Errorf("use --target to specify a domain"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrLabsFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: 0.01,
				Units:       1,
				Basis:       "dataforseo labs domain rank overview: 1 task @ $0.01/task",
			})
			if err != nil {
				return output.PrintCodedError(output.ErrEstimateFailed, "failed to estimate cost", err, nil, output.Format(*format))
			}

			approval := cost.EvaluateApproval(estimate, cfg.ApprovalThresholdUSD)

			meta := map[string]any{
				"estimated_cost":    estimate.Amount,
				"currency":          estimate.Currency,
				"requires_approval": approval.RequiresApproval,
				"dry_run":           dryRun,
				"source":            "dataforseo",
				"verbose":           *verbose,
			}

			if dryRun {
				return output.PrintSuccess(map[string]any{
					"target": target,
					"status": "dry_run",
				}, meta, output.Format(*format))
			}

			if approval.RequiresApproval {
				meta["reason"] = approval.Reason
				return output.PrintCodedError(output.ErrApprovalRequired, "cost exceeds approval threshold",
					fmt.Errorf("%s", approval.Reason), meta, output.Format(*format))
			}

			client := dataforseo.New(cfg.DataForSEOLogin, cfg.DataForSEOPassword)

			task := map[string]any{
				"target":        target,
				"location_name": location,
				"language_code": language,
			}

			raw, err := client.Post("/v3/dataforseo_labs/google/domain_rank_overview/live", []map[string]any{task})
			if err != nil {
				return output.PrintCodedError(output.ErrLabsFailed, "domain rank overview request failed", err, nil, output.Format(*format))
			}

			var envelope struct {
				StatusCode    int    `json:"status_code"`
				StatusMessage string `json:"status_message"`
				Tasks         []struct {
					StatusCode    int               `json:"status_code"`
					StatusMessage string            `json:"status_message"`
					Result        []json.RawMessage `json:"result"`
				} `json:"tasks"`
			}

			if jsonErr := json.Unmarshal(raw, &envelope); jsonErr != nil {
				return output.PrintCodedError(output.ErrLabsFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
			}

			if envelope.StatusCode != 20000 {
				return output.PrintCodedError(output.ErrLabsFailed,
					fmt.Sprintf("DataForSEO error %d: %s", envelope.StatusCode, envelope.StatusMessage),
					fmt.Errorf("code %d", envelope.StatusCode), nil, output.Format(*format))
			}

			var results []json.RawMessage
			if len(envelope.Tasks) > 0 {
				results = envelope.Tasks[0].Result
			}

			meta["fetched_at"] = time.Now().Format(time.RFC3339)
			meta["target"] = target

			return output.PrintSuccess(results, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Domain to analyze (required)")
	cmd.Flags().StringVar(&location, "location", "Australia", "Location name (e.g. 'Australia')")
	cmd.Flags().StringVar(&language, "language", "en", "Language name (e.g. 'en')")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

func newLabsCompetitorsCmd(format *string, verbose *bool) *cobra.Command {
	var target, location, language string
	var limit int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "competitors",
		Short: "Get competing domains",
		Long:  `Retrieve competitor domains from DataForSEO Labs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if target == "" {
				return output.PrintCodedError(output.ErrLabsFailed, "target is required",
					fmt.Errorf("use --target to specify a domain"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrLabsFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: 0.01,
				Units:       1,
				Basis:       "dataforseo labs competitors domain: 1 task @ $0.01/task",
			})
			if err != nil {
				return output.PrintCodedError(output.ErrEstimateFailed, "failed to estimate cost", err, nil, output.Format(*format))
			}

			approval := cost.EvaluateApproval(estimate, cfg.ApprovalThresholdUSD)

			meta := map[string]any{
				"estimated_cost":    estimate.Amount,
				"currency":          estimate.Currency,
				"requires_approval": approval.RequiresApproval,
				"dry_run":           dryRun,
				"source":            "dataforseo",
				"verbose":           *verbose,
			}

			if dryRun {
				return output.PrintSuccess(map[string]any{
					"target": target,
					"status": "dry_run",
				}, meta, output.Format(*format))
			}

			if approval.RequiresApproval {
				meta["reason"] = approval.Reason
				return output.PrintCodedError(output.ErrApprovalRequired, "cost exceeds approval threshold",
					fmt.Errorf("%s", approval.Reason), meta, output.Format(*format))
			}

			client := dataforseo.New(cfg.DataForSEOLogin, cfg.DataForSEOPassword)

			task := map[string]any{
				"target":        target,
				"location_name": location,
				"language_code": language,
				"limit":         limit,
			}

			raw, err := client.Post("/v3/dataforseo_labs/google/competitors_domain/live", []map[string]any{task})
			if err != nil {
				return output.PrintCodedError(output.ErrLabsFailed, "competitors domain request failed", err, nil, output.Format(*format))
			}

			var envelope struct {
				StatusCode    int    `json:"status_code"`
				StatusMessage string `json:"status_message"`
				Tasks         []struct {
					StatusCode    int               `json:"status_code"`
					StatusMessage string            `json:"status_message"`
					Result        []json.RawMessage `json:"result"`
				} `json:"tasks"`
			}

			if jsonErr := json.Unmarshal(raw, &envelope); jsonErr != nil {
				return output.PrintCodedError(output.ErrLabsFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
			}

			if envelope.StatusCode != 20000 {
				return output.PrintCodedError(output.ErrLabsFailed,
					fmt.Sprintf("DataForSEO error %d: %s", envelope.StatusCode, envelope.StatusMessage),
					fmt.Errorf("code %d", envelope.StatusCode), nil, output.Format(*format))
			}

			var results []json.RawMessage
			if len(envelope.Tasks) > 0 {
				results = envelope.Tasks[0].Result
			}

			// Persist competitors to state if project is initialized.
			if state.Exists(".") && len(results) > 0 {
				type labsCompetitorResult struct {
					Items []struct {
						Domain string `json:"domain"`
					} `json:"items"`
				}

				var parsed labsCompetitorResult
				if err := json.Unmarshal(results[0], &parsed); err == nil {
					var domains []string
					for _, item := range parsed.Items {
						if item.Domain != "" {
							domains = append(domains, item.Domain)
						}
					}

					if st, loadErr := state.Load("."); loadErr == nil {
						if st.Labs == nil {
							st.Labs = &state.LabsData{}
						}
						st.Labs.LastRun = time.Now().UTC().Format(time.RFC3339)
						st.Labs.Target = target
						st.Labs.Competitors = domains
						st.AddHistory("labs", fmt.Sprintf("competitors for %s: %d domains", target, len(domains)))
						_ = st.Save(".")
					}
				}
			}

			meta["fetched_at"] = time.Now().Format(time.RFC3339)
			meta["target"] = target

			return output.PrintSuccess(results, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Domain to analyze (required)")
	cmd.Flags().StringVar(&location, "location", "Australia", "Location name (e.g. 'Australia')")
	cmd.Flags().StringVar(&language, "language", "en", "Language name (e.g. 'en')")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of results")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

func newLabsKeywordIdeasCmd(format *string, verbose *bool) *cobra.Command {
	var keyword, location, language string
	var limit int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "keyword-ideas",
		Short: "Get keyword ideas from a seed keyword",
		Long:  `Retrieve keyword ideas from DataForSEO Labs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if keyword == "" {
				return output.PrintCodedError(output.ErrLabsFailed, "keyword is required",
					fmt.Errorf("use --keyword to specify a seed keyword"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrLabsFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: 0.01,
				Units:       1,
				Basis:       "dataforseo labs keyword ideas: 1 task @ $0.01/task",
			})
			if err != nil {
				return output.PrintCodedError(output.ErrEstimateFailed, "failed to estimate cost", err, nil, output.Format(*format))
			}

			approval := cost.EvaluateApproval(estimate, cfg.ApprovalThresholdUSD)

			meta := map[string]any{
				"estimated_cost":    estimate.Amount,
				"currency":          estimate.Currency,
				"requires_approval": approval.RequiresApproval,
				"dry_run":           dryRun,
				"source":            "dataforseo",
				"verbose":           *verbose,
			}

			if dryRun {
				return output.PrintSuccess(map[string]any{
					"keyword": keyword,
					"status":  "dry_run",
				}, meta, output.Format(*format))
			}

			if approval.RequiresApproval {
				meta["reason"] = approval.Reason
				return output.PrintCodedError(output.ErrApprovalRequired, "cost exceeds approval threshold",
					fmt.Errorf("%s", approval.Reason), meta, output.Format(*format))
			}

			client := dataforseo.New(cfg.DataForSEOLogin, cfg.DataForSEOPassword)

			task := map[string]any{
				"keywords":      []string{keyword},
				"location_name": location,
				"language_code": language,
				"limit":         limit,
			}

			raw, err := client.Post("/v3/dataforseo_labs/google/keyword_ideas/live", []map[string]any{task})
			if err != nil {
				return output.PrintCodedError(output.ErrLabsFailed, "keyword ideas request failed", err, nil, output.Format(*format))
			}

			var envelope struct {
				StatusCode    int    `json:"status_code"`
				StatusMessage string `json:"status_message"`
				Tasks         []struct {
					StatusCode    int               `json:"status_code"`
					StatusMessage string            `json:"status_message"`
					Result        []json.RawMessage `json:"result"`
				} `json:"tasks"`
			}

			if jsonErr := json.Unmarshal(raw, &envelope); jsonErr != nil {
				return output.PrintCodedError(output.ErrLabsFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
			}

			if envelope.StatusCode != 20000 {
				return output.PrintCodedError(output.ErrLabsFailed,
					fmt.Sprintf("DataForSEO error %d: %s", envelope.StatusCode, envelope.StatusMessage),
					fmt.Errorf("code %d", envelope.StatusCode), nil, output.Format(*format))
			}

			var results []json.RawMessage
			if len(envelope.Tasks) > 0 {
				results = envelope.Tasks[0].Result
			}

			meta["fetched_at"] = time.Now().Format(time.RFC3339)
			meta["keyword"] = keyword

			return output.PrintSuccess(results, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&keyword, "keyword", "", "Seed keyword to analyze (required)")
	cmd.Flags().StringVar(&location, "location", "Australia", "Location name (e.g. 'Australia')")
	cmd.Flags().StringVar(&language, "language", "en", "Language name (e.g. 'en')")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of results")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

func newLabsBulkDifficultyCmd(format *string, verbose *bool) *cobra.Command {
	var keywordsFlag, location, language string
	var fromGSC, dryRun bool

	locationCodes := map[string]int{
		"Australia":      2036,
		"United States":  2840,
		"United Kingdom": 2826,
		"Canada":         2124,
		"New Zealand":    2554,
	}

	cmd := &cobra.Command{
		Use:   "bulk-difficulty",
		Short: "Get bulk keyword difficulty scores",
		Long:  `Retrieve keyword difficulty scores in bulk from DataForSEO Labs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve keyword list.
			var keywords []string

			if fromGSC {
				if !state.Exists(".") {
					return output.PrintCodedError(output.ErrLabsFailed, "no project state found",
						fmt.Errorf("run 'supah-seo init' to initialise the project first"), nil, output.Format(*format))
				}
				st, err := state.Load(".")
				if err != nil {
					return output.PrintCodedError(output.ErrLabsFailed, "failed to load state", err, nil, output.Format(*format))
				}
				if st.GSC == nil || len(st.GSC.TopKeywords) == 0 {
					return output.PrintCodedError(output.ErrLabsFailed, "no GSC keyword data in state",
						fmt.Errorf("run 'supah-seo gsc query keywords' first to populate GSC data"), nil, output.Format(*format))
				}
				for _, row := range st.GSC.TopKeywords {
					if row.Key != "" {
						keywords = append(keywords, row.Key)
					}
				}
			} else {
				if keywordsFlag == "" {
					return output.PrintCodedError(output.ErrLabsFailed, "keywords are required",
						fmt.Errorf("use --keywords or --from-gsc to provide keywords"), nil, output.Format(*format))
				}
				for _, kw := range strings.Split(keywordsFlag, ",") {
					kw = strings.TrimSpace(kw)
					if kw != "" {
						keywords = append(keywords, kw)
					}
				}
			}

			if len(keywords) == 0 {
				return output.PrintCodedError(output.ErrLabsFailed, "no keywords to process",
					fmt.Errorf("keyword list is empty"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrLabsFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			locationCode, ok := locationCodes[location]
			if !ok {
				return output.PrintCodedError(output.ErrLabsFailed, "unsupported location",
					fmt.Errorf("supported locations: Australia, United States, United Kingdom, Canada, New Zealand"), nil, output.Format(*format))
			}

			// Split into batches of 1000.
			const batchSize = 1000
			var batches [][]string
			for i := 0; i < len(keywords); i += batchSize {
				end := i + batchSize
				if end > len(keywords) {
					end = len(keywords)
				}
				batches = append(batches, keywords[i:end])
			}

			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: 0.001,
				Units:       1,
				Basis:       fmt.Sprintf("dataforseo labs bulk keyword difficulty: 1 task, %d keywords", len(keywords)),
			})
			if err != nil {
				return output.PrintCodedError(output.ErrEstimateFailed, "failed to estimate cost", err, nil, output.Format(*format))
			}

			approval := cost.EvaluateApproval(estimate, cfg.ApprovalThresholdUSD)

			meta := map[string]any{
				"estimated_cost":    estimate.Amount,
				"currency":          estimate.Currency,
				"requires_approval": approval.RequiresApproval,
				"dry_run":           dryRun,
				"source":            "dataforseo",
				"verbose":           *verbose,
				"keyword_count":     len(keywords),
				"batches":           len(batches),
			}

			if dryRun {
				return output.PrintSuccess(map[string]any{
					"keywords": len(keywords),
					"batches":  len(batches),
					"status":   "dry_run",
				}, meta, output.Format(*format))
			}

			if approval.RequiresApproval {
				meta["reason"] = approval.Reason
				return output.PrintCodedError(output.ErrApprovalRequired, "cost exceeds approval threshold",
					fmt.Errorf("%s", approval.Reason), meta, output.Format(*format))
			}

			client := dataforseo.New(cfg.DataForSEOLogin, cfg.DataForSEOPassword)

			type difficultyItem struct {
				Keyword           string `json:"keyword"`
				KeywordDifficulty int    `json:"keyword_difficulty"`
			}

			var allItems []difficultyItem

			for _, batch := range batches {
				task := map[string]any{
					"keywords":      batch,
					"location_code": locationCode,
					"language_code": language,
				}

				raw, reqErr := client.Post("/v3/dataforseo_labs/google/bulk_keyword_difficulty/live", []map[string]any{task})
				if reqErr != nil {
					return output.PrintCodedError(output.ErrLabsFailed, "bulk keyword difficulty request failed", reqErr, nil, output.Format(*format))
				}

				var envelope struct {
					StatusCode    int    `json:"status_code"`
					StatusMessage string `json:"status_message"`
					Tasks         []struct {
						StatusCode    int    `json:"status_code"`
						StatusMessage string `json:"status_message"`
						Result        []struct {
							Items []difficultyItem `json:"items"`
						} `json:"result"`
					} `json:"tasks"`
				}

				if jsonErr := json.Unmarshal(raw, &envelope); jsonErr != nil {
					return output.PrintCodedError(output.ErrLabsFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
				}

				if envelope.StatusCode != 20000 {
					return output.PrintCodedError(output.ErrLabsFailed,
						fmt.Sprintf("DataForSEO error %d: %s", envelope.StatusCode, envelope.StatusMessage),
						fmt.Errorf("code %d", envelope.StatusCode), nil, output.Format(*format))
				}

				for _, t := range envelope.Tasks {
					for _, r := range t.Result {
						allItems = append(allItems, r.Items...)
					}
				}
			}

			// Persist to state if project is initialised.
			if state.Exists(".") && len(allItems) > 0 {
				if st, loadErr := state.Load("."); loadErr == nil {
					if st.Labs == nil {
						st.Labs = &state.LabsData{}
					}

					// Build an index of existing keywords for fast lookup.
					existing := make(map[string]int, len(st.Labs.Keywords))
					for i, kw := range st.Labs.Keywords {
						existing[kw.Keyword] = i
					}

					for _, item := range allItems {
						if idx, found := existing[item.Keyword]; found {
							st.Labs.Keywords[idx].Difficulty = float64(item.KeywordDifficulty)
						} else {
							st.Labs.Keywords = append(st.Labs.Keywords, state.LabsKeyword{
								Keyword:    item.Keyword,
								Difficulty: float64(item.KeywordDifficulty),
							})
							existing[item.Keyword] = len(st.Labs.Keywords) - 1
						}
					}

					st.Labs.LastRun = time.Now().UTC().Format(time.RFC3339)
					st.AddHistory("labs", fmt.Sprintf("bulk-difficulty for %d keywords", len(keywords)))
					_ = st.Save(".")
				}
			}

			meta["fetched_at"] = time.Now().Format(time.RFC3339)

			return output.PrintSuccess(allItems, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&keywordsFlag, "keywords", "", "Comma-separated list of keywords")
	cmd.Flags().BoolVar(&fromGSC, "from-gsc", false, "Load keywords from GSC state (.supah-seo/state.json)")
	cmd.Flags().StringVar(&location, "location", "Australia", "Location name (e.g. 'Australia', 'United States')")
	cmd.Flags().StringVar(&language, "language", "en", "Language code (e.g. 'en')")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}
