package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/supah-seo/supah-seo/internal/common/config"
	"github.com/supah-seo/supah-seo/internal/common/cost"
	"github.com/supah-seo/supah-seo/internal/dataforseo"
	"github.com/supah-seo/supah-seo/pkg/output"
	"github.com/spf13/cobra"
)

// NewGEOCmd returns the geo command group.
func NewGEOCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "geo",
		Short: "Generative Engine Optimization commands",
		Long:  `Track how your domain and brand appear in AI-generated responses. Powered by DataForSEO.`,
	}

	cmd.AddCommand(
		newGEOMentionsCmd(format, verbose),
		newGEOTopPagesCmd(format, verbose),
	)

	return cmd
}

func newGEOMentionsCmd(format *string, verbose *bool) *cobra.Command {
	var domain, keyword, platform string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "mentions",
		Short: "Track how often your domain or brand is mentioned in AI responses",
		Long: `Query DataForSEO's LLM Mentions API to see how often your domain appears in AI-generated search results.
Returns mention count, AI search volume, impressions, and 12-month trending data.

Note: DataForSEO LLM Mentions requires a $100/month minimum commitment on your DataForSEO account.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if keyword == "" {
				return output.PrintCodedError(output.ErrGEOFailed, "keyword is required",
					fmt.Errorf("use --keyword to specify a keyword"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrGEOFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			// Estimate: per-row billing; approximate $0.01/keyword task
			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: 0.01,
				Units:       1,
				Basis:       "dataforseo llm mentions: 1 task @ ~$0.01/task (per-row billing; $100/mo minimum commitment required)",
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
				"note":              "LLM Mentions requires a $100/month minimum commitment on your DataForSEO account",
				"verbose":           *verbose,
			}

			if dryRun {
				return output.PrintSuccess(map[string]any{
					"keyword":  keyword,
					"domain":   domain,
					"platform": platform,
					"status":   "dry_run",
				}, meta, output.Format(*format))
			}

			if approval.RequiresApproval {
				meta["reason"] = approval.Reason
				return output.PrintCodedError(output.ErrApprovalRequired, "cost exceeds approval threshold",
					fmt.Errorf("%s", approval.Reason), meta, output.Format(*format))
			}

			client := dataforseo.New(cfg.DataForSEOLogin, cfg.DataForSEOPassword)

			task := map[string]any{
				"keyword": keyword,
			}
			if domain != "" {
				task["domain"] = domain
			}
			if platform != "" {
				task["se_type"] = platform
			}

			raw, err := client.Post("/v3/ai_optimization/llm_mentions/search/live", []map[string]any{task})
			if err != nil {
				return output.PrintCodedError(output.ErrGEOFailed, "LLM mentions request failed", err, nil, output.Format(*format))
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
				return output.PrintCodedError(output.ErrGEOFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
			}

			if envelope.StatusCode != 20000 {
				return output.PrintCodedError(output.ErrGEOFailed,
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

	cmd.Flags().StringVar(&keyword, "keyword", "", "Keyword to track AI mentions for (required)")
	cmd.Flags().StringVar(&domain, "domain", "", "Filter results to a specific domain")
	cmd.Flags().StringVar(&platform, "platform", "", "Search engine type: google or bing")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

func newGEOTopPagesCmd(format *string, verbose *bool) *cobra.Command {
	var keyword, domain string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "top-pages",
		Short: "Show which pages are most cited by AI engines for a keyword",
		Long: `Query DataForSEO's LLM Mentions Top Pages endpoint to see which URLs appear most often
in AI-generated responses for a given keyword, optionally filtered by domain.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if keyword == "" {
				return output.PrintCodedError(output.ErrGEOFailed, "keyword is required",
					fmt.Errorf("use --keyword to specify a keyword"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrGEOFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			// Estimate: per-row billing; approximate $0.01/keyword task
			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: 0.01,
				Units:       1,
				Basis:       "dataforseo llm mentions top pages: 1 task @ ~$0.01/task",
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
					"domain":  domain,
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
				"keyword": keyword,
			}
			if domain != "" {
				task["domain"] = domain
			}

			raw, err := client.Post("/v3/ai_optimization/llm_mentions/top_pages/live", []map[string]any{task})
			if err != nil {
				return output.PrintCodedError(output.ErrGEOFailed, "LLM mentions top pages request failed", err, nil, output.Format(*format))
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
				return output.PrintCodedError(output.ErrGEOFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
			}

			if envelope.StatusCode != 20000 {
				return output.PrintCodedError(output.ErrGEOFailed,
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

	cmd.Flags().StringVar(&keyword, "keyword", "", "Keyword to find top AI-cited pages for (required)")
	cmd.Flags().StringVar(&domain, "domain", "", "Filter to pages from a specific domain")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}
