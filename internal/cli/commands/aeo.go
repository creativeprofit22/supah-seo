package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/supah-seo/supah-seo/internal/common/config"
	"github.com/supah-seo/supah-seo/internal/common/cost"
	"github.com/supah-seo/supah-seo/internal/dataforseo"
	"github.com/supah-seo/supah-seo/pkg/output"
)

// NewAEOCmd returns the aeo command group.
func NewAEOCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aeo",
		Short: "Answer Engine Optimization commands",
		Long:  `Query AI engines and get keyword data for Answer Engine Optimization (AEO). Powered by DataForSEO.`,
	}

	cmd.AddCommand(
		newAEOResponsesCmd(format, verbose),
		newAEOKeywordsCmd(format, verbose),
	)

	return cmd
}

func newAEOResponsesCmd(format *string, verbose *bool) *cobra.Command {
	var prompt, model string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "responses",
		Short: "Query an AI engine and see what it says about your brand or topic",
		Long: `Send a prompt to an AI engine (ChatGPT, Claude, Gemini, or Perplexity) and see the full response.
Useful for understanding how AI tools describe your brand, products, or keywords.

Supported models: chatgpt, claude, gemini, perplexity`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if prompt == "" {
				return output.PrintCodedError(output.ErrAEOFailed, "prompt is required",
					fmt.Errorf("use --prompt to specify a prompt"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrAEOFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			// Estimate: ~$0.003/query (LLM pass-through pricing varies by model)
			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: 0.003,
				Units:       1,
				Basis:       fmt.Sprintf("dataforseo llm responses: 1 query via %s @ ~$0.003/query", model),
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
				"model":             model,
				"source":            "dataforseo",
				"verbose":           *verbose,
			}

			if dryRun {
				return output.PrintSuccess(map[string]any{
					"prompt": prompt,
					"model":  model,
					"status": "dry_run",
				}, meta, output.Format(*format))
			}

			if approval.RequiresApproval {
				meta["reason"] = approval.Reason
				return output.PrintCodedError(output.ErrApprovalRequired, "cost exceeds approval threshold",
					fmt.Errorf("%s", approval.Reason), meta, output.Format(*format))
			}

			// Validate model before making any API calls.
			validModels := map[string]bool{"chatgpt": true, "claude": true, "gemini": true, "perplexity": true}
			if !validModels[model] {
				return output.PrintCodedError(output.ErrAEOFailed,
					fmt.Sprintf("unsupported model %q", model),
					fmt.Errorf("valid values: chatgpt, claude, gemini, perplexity"), nil, output.Format(*format))
			}

			client := dataforseo.New(cfg.DataForSEOLogin, cfg.DataForSEOPassword)

			endpoint, endpointErr := aeoEndpointForModel(model)
			if endpointErr != nil {
				return output.PrintCodedError(output.ErrAEOFailed, "invalid model", endpointErr, nil, output.Format(*format))
			}
			reqBody := []map[string]any{
				{"prompt": prompt},
			}

			raw, err := client.Post(endpoint, reqBody)
			if err != nil {
				return output.PrintCodedError(output.ErrAEOFailed, "LLM responses request failed", err, nil, output.Format(*format))
			}

			var envelope struct {
				StatusCode    int    `json:"status_code"`
				StatusMessage string `json:"status_message"`
				Tasks         []struct {
					StatusCode    int    `json:"status_code"`
					StatusMessage string `json:"status_message"`
					Result        []struct {
						Prompt   string `json:"prompt"`
						Response string `json:"response"`
						Model    string `json:"model"`
					} `json:"result"`
				} `json:"tasks"`
			}

			if jsonErr := json.Unmarshal(raw, &envelope); jsonErr != nil {
				return output.PrintCodedError(output.ErrAEOFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
			}

			if envelope.StatusCode != 20000 {
				return output.PrintCodedError(output.ErrAEOFailed,
					fmt.Sprintf("DataForSEO error %d: %s", envelope.StatusCode, envelope.StatusMessage),
					fmt.Errorf("code %d", envelope.StatusCode), nil, output.Format(*format))
			}
			if len(envelope.Tasks) == 0 || len(envelope.Tasks[0].Result) == 0 {
				return output.PrintCodedError(output.ErrAEOFailed, "no results returned",
					fmt.Errorf("dataforseo returned empty task result"), nil, output.Format(*format))
			}

			result := envelope.Tasks[0].Result[0]
			meta["fetched_at"] = time.Now().Format(time.RFC3339)

			return output.PrintSuccess(map[string]any{
				"prompt":   result.Prompt,
				"response": result.Response,
				"model":    result.Model,
			}, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt to send to the AI engine (required)")
	cmd.Flags().StringVar(&model, "model", "chatgpt", "AI model: chatgpt, claude, gemini, perplexity")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

func newAEOKeywordsCmd(format *string, verbose *bool) *cobra.Command {
	var keyword, location, language string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "keywords",
		Short: "Get AI search volume for keywords",
		Long:  `Retrieve AI search volume data showing how often keywords are used in AI tools like ChatGPT and Gemini.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if keyword == "" {
				return output.PrintCodedError(output.ErrAEOFailed, "keyword is required",
					fmt.Errorf("use --keyword to specify a keyword"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrAEOFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			// Estimate: $0.01/task
			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: 0.01,
				Units:       1,
				Basis:       "dataforseo ai keyword data: 1 task @ $0.01/task",
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
				"keyword": keyword,
			}
			if location != "" {
				task["location_name"] = location
			}
			if language != "" {
				task["language_code"] = language
			}

			raw, err := client.Post("/v3/ai_optimization/ai_keyword_data/search_volume/live", []map[string]any{task})
			if err != nil {
				return output.PrintCodedError(output.ErrAEOFailed, "AI keyword data request failed", err, nil, output.Format(*format))
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
				return output.PrintCodedError(output.ErrAEOFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
			}

			if envelope.StatusCode != 20000 {
				return output.PrintCodedError(output.ErrAEOFailed,
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

	cmd.Flags().StringVar(&keyword, "keyword", "", "Keyword to get AI search volume for (required)")
	cmd.Flags().StringVar(&location, "location", "", "Location name (e.g. 'United States')")
	cmd.Flags().StringVar(&language, "language", "", "Language code (e.g. 'en')")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

// aeoEndpointForModel returns the DataForSEO LLM responses endpoint for the given model name.
func aeoEndpointForModel(model string) (string, error) {
	switch model {
	case "chatgpt":
		return "/v3/ai_optimization/chat_gpt/llm_responses/live", nil
	case "claude":
		return "/v3/ai_optimization/claude/llm_responses/live", nil
	case "gemini":
		return "/v3/ai_optimization/gemini/llm_responses/live", nil
	case "perplexity":
		return "/v3/ai_optimization/perplexity/llm_responses/live", nil
	default:
		return "", fmt.Errorf("unsupported model %q: valid values: chatgpt, claude, gemini, perplexity", model)
	}
}
