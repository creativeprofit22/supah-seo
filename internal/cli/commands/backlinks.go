package commands

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/supah-seo/supah-seo/internal/backlinks"
	"github.com/supah-seo/supah-seo/internal/common/config"
	"github.com/supah-seo/supah-seo/internal/common/cost"
	"github.com/supah-seo/supah-seo/internal/dataforseo"
	"github.com/supah-seo/supah-seo/internal/state"
	"github.com/supah-seo/supah-seo/pkg/output"
)

// NewBacklinksCmd returns the backlinks command group.
func NewBacklinksCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backlinks",
		Short: "DataForSEO Backlinks commands",
		Long:  `Query the DataForSEO Backlinks API for link intelligence.`,
	}

	cmd.AddCommand(
		newBacklinksSummaryCmd(format, verbose),
		newBacklinksListCmd(format, verbose),
		newBacklinksReferringDomainsCmd(format, verbose),
		newBacklinksCompetitorsCmd(format, verbose),
		newBacklinksGapCmd(format, verbose),
	)

	return cmd
}

// dfseEnvelope is the standard DataForSEO response envelope used for backlinks parsing.
type dfseEnvelope struct {
	StatusCode    int    `json:"status_code"`
	StatusMessage string `json:"status_message"`
	Tasks         []struct {
		StatusCode    int               `json:"status_code"`
		StatusMessage string            `json:"status_message"`
		Result        []json.RawMessage `json:"result"`
	} `json:"tasks"`
}

func newBacklinksSummaryCmd(format *string, verbose *bool) *cobra.Command {
	var target string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Get backlink profile summary for a domain",
		Long:  `Retrieve backlink summary (total backlinks, referring domains, spam score, etc.) from the DataForSEO Backlinks API.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if target == "" {
				return output.PrintCodedError(output.ErrBacklinksFailed, "target is required",
					fmt.Errorf("use --target to specify a domain"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrBacklinksFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: 0.02,
				Units:       1,
				Basis:       "dataforseo backlinks summary: 1 task @ $0.02/task",
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
				"target": target,
			}

			raw, err := client.Post("/v3/backlinks/summary/live", []map[string]any{task})
			if err != nil {
				return output.PrintCodedError(output.ErrBacklinksFailed, "backlinks summary request failed", err, nil, output.Format(*format))
			}

			var envelope dfseEnvelope
			if jsonErr := json.Unmarshal(raw, &envelope); jsonErr != nil {
				return output.PrintCodedError(output.ErrBacklinksFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
			}

			if envelope.StatusCode != 20000 {
				return output.PrintCodedError(output.ErrBacklinksFailed,
					fmt.Sprintf("DataForSEO error %d: %s", envelope.StatusCode, envelope.StatusMessage),
					fmt.Errorf("code %d", envelope.StatusCode), nil, output.Format(*format))
			}

			var results []json.RawMessage
			if len(envelope.Tasks) > 0 {
				results = envelope.Tasks[0].Result
			}

			// Parse into Summary struct and persist to state.
			if len(results) > 0 {
				type summaryResult struct {
					TotalBacklinks           int64   `json:"total_backlinks"`
					TotalReferringDomains    int64   `json:"referring_domains"`
					TotalReferringPages      int64   `json:"referring_pages"`
					BrokenBacklinks          int64   `json:"broken_backlinks"`
					ReferringDomainsNofollow int64   `json:"referring_domains_nofollow"`
					BacklinksSpamScore       float64 `json:"backlinks_spam_score"`
					Rank                     int     `json:"rank"`
					BacklinksNofollow        int64   `json:"backlinks_nofollow"`
				}

				var parsed summaryResult
				if parseErr := json.Unmarshal(results[0], &parsed); parseErr == nil {
					summary := backlinks.Summary{
						Target:                   target,
						TotalBacklinks:           parsed.TotalBacklinks,
						TotalReferringDomains:    parsed.TotalReferringDomains,
						TotalReferringPages:      parsed.TotalReferringPages,
						BrokenBacklinks:          parsed.BrokenBacklinks,
						ReferringDomainsNofollow: parsed.ReferringDomainsNofollow,
						BacklinksSpamScore:       parsed.BacklinksSpamScore,
						Rank:                     parsed.Rank,
						DoFollowLinks:            parsed.TotalBacklinks - parsed.BacklinksNofollow,
						NoFollowLinks:            parsed.BacklinksNofollow,
					}

					if state.Exists(".") {
						if st, loadErr := state.Load("."); loadErr == nil {
							st.Backlinks = &state.BacklinksData{
								LastRun:               time.Now().UTC().Format(time.RFC3339),
								Target:                target,
								TotalBacklinks:        summary.TotalBacklinks,
								TotalReferringDomains: summary.TotalReferringDomains,
								BrokenBacklinks:       summary.BrokenBacklinks,
								Rank:                  summary.Rank,
								DoFollow:              summary.DoFollowLinks,
								NoFollow:              summary.NoFollowLinks,
								SpamScore:             summary.BacklinksSpamScore,
							}
							st.AddHistory("backlinks", fmt.Sprintf("summary for %s: %d backlinks, %d referring domains", target, summary.TotalBacklinks, summary.TotalReferringDomains))
							_ = st.Save(".")
						}
					}

					meta["fetched_at"] = time.Now().Format(time.RFC3339)
					meta["target"] = target

					return output.PrintSuccess(summary, meta, output.Format(*format))
				}
			}

			meta["fetched_at"] = time.Now().Format(time.RFC3339)
			meta["target"] = target

			return output.PrintSuccess(results, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Domain or URL to analyze (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

func newBacklinksListCmd(format *string, verbose *bool) *cobra.Command {
	var target string
	var limit int
	var dofollowOnly, dryRun bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Get list of backlinks for a domain",
		Long:  `Retrieve individual backlinks for a domain from the DataForSEO Backlinks API.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if target == "" {
				return output.PrintCodedError(output.ErrBacklinksFailed, "target is required",
					fmt.Errorf("use --target to specify a domain"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrBacklinksFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			costAmount := 0.02 + 0.00003*float64(limit)
			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: costAmount,
				Units:       1,
				Basis:       fmt.Sprintf("dataforseo backlinks list: $0.02 base + $0.00003 × %d rows", limit),
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
					"limit":  limit,
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
				"target": target,
				"mode":   "as_is",
				"limit":  limit,
			}
			if dofollowOnly {
				task["filters"] = []any{"dofollow", "=", true}
			}

			raw, err := client.Post("/v3/backlinks/backlinks/live", []map[string]any{task})
			if err != nil {
				return output.PrintCodedError(output.ErrBacklinksFailed, "backlinks list request failed", err, nil, output.Format(*format))
			}

			var envelope dfseEnvelope
			if jsonErr := json.Unmarshal(raw, &envelope); jsonErr != nil {
				return output.PrintCodedError(output.ErrBacklinksFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
			}

			if envelope.StatusCode != 20000 {
				return output.PrintCodedError(output.ErrBacklinksFailed,
					fmt.Sprintf("DataForSEO error %d: %s", envelope.StatusCode, envelope.StatusMessage),
					fmt.Errorf("code %d", envelope.StatusCode), nil, output.Format(*format))
			}

			var results []json.RawMessage
			if len(envelope.Tasks) > 0 {
				results = envelope.Tasks[0].Result
			}

			// Parse items into []backlinks.Backlink.
			var items []backlinks.Backlink
			if len(results) > 0 {
				type listResult struct {
					Items []struct {
						DomainFrom       string `json:"domain_from"`
						URLFrom          string `json:"url_from"`
						URLTo            string `json:"url_to"`
						Anchor           string `json:"anchor"`
						DoFollow         bool   `json:"dofollow"`
						PageFromRank     int    `json:"page_from_rank"`
						DomainFromRank   int    `json:"domain_from_rank"`
						IsNew            bool   `json:"is_new"`
						IsLost           bool   `json:"is_lost"`
						FirstVisitedFrom string `json:"first_seen,omitempty"`
						LastVisitedFrom  string `json:"last_seen,omitempty"`
					} `json:"items"`
				}

				var parsed listResult
				if parseErr := json.Unmarshal(results[0], &parsed); parseErr == nil {
					for _, item := range parsed.Items {
						items = append(items, backlinks.Backlink{
							DomainFrom:     item.DomainFrom,
							URLFrom:        item.URLFrom,
							URLTo:          item.URLTo,
							Anchor:         item.Anchor,
							IsDoFollow:     item.DoFollow,
							PageFromRank:   item.PageFromRank,
							DomainFromRank: item.DomainFromRank,
							IsNew:          item.IsNew,
							IsLost:         item.IsLost,
							FirstSeen:      item.FirstVisitedFrom,
							LastSeen:       item.LastVisitedFrom,
						})
					}
				}
			}

			meta["fetched_at"] = time.Now().Format(time.RFC3339)
			meta["target"] = target

			return output.PrintSuccess(items, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Domain or URL to analyze (required)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of backlinks to return (max 1000)")
	cmd.Flags().BoolVar(&dofollowOnly, "dofollow-only", false, "Only return dofollow backlinks")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

func newBacklinksReferringDomainsCmd(format *string, verbose *bool) *cobra.Command {
	var target string
	var limit int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "referring-domains",
		Short: "Get referring domains for a target",
		Long:  `Retrieve the list of referring domains that link to a target from the DataForSEO Backlinks API.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if target == "" {
				return output.PrintCodedError(output.ErrBacklinksFailed, "target is required",
					fmt.Errorf("use --target to specify a domain"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrBacklinksFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			costAmount := 0.02 + 0.00003*float64(limit)
			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: costAmount,
				Units:       1,
				Basis:       fmt.Sprintf("dataforseo backlinks referring domains: $0.02 base + $0.00003 × %d rows", limit),
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
					"limit":  limit,
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
				"target": target,
				"limit":  limit,
			}

			raw, err := client.Post("/v3/backlinks/referring_domains/live", []map[string]any{task})
			if err != nil {
				return output.PrintCodedError(output.ErrBacklinksFailed, "referring domains request failed", err, nil, output.Format(*format))
			}

			var envelope dfseEnvelope
			if jsonErr := json.Unmarshal(raw, &envelope); jsonErr != nil {
				return output.PrintCodedError(output.ErrBacklinksFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
			}

			if envelope.StatusCode != 20000 {
				return output.PrintCodedError(output.ErrBacklinksFailed,
					fmt.Sprintf("DataForSEO error %d: %s", envelope.StatusCode, envelope.StatusMessage),
					fmt.Errorf("code %d", envelope.StatusCode), nil, output.Format(*format))
			}

			var results []json.RawMessage
			if len(envelope.Tasks) > 0 {
				results = envelope.Tasks[0].Result
			}

			var items []backlinks.ReferringDomain
			if len(results) > 0 {
				type refResult struct {
					Items []struct {
						Domain        string `json:"domain"`
						Rank          int    `json:"rank"`
						Backlinks     int64  `json:"backlinks"`
						DoFollowLinks int64  `json:"dofollow"`
						FirstSeen     string `json:"first_seen,omitempty"`
					} `json:"items"`
				}

				var parsed refResult
				if parseErr := json.Unmarshal(results[0], &parsed); parseErr == nil {
					for _, item := range parsed.Items {
						items = append(items, backlinks.ReferringDomain{
							Domain:        item.Domain,
							Rank:          item.Rank,
							Backlinks:     item.Backlinks,
							DoFollowLinks: item.DoFollowLinks,
							FirstSeen:     item.FirstSeen,
						})
					}
				}
			}

			meta["fetched_at"] = time.Now().Format(time.RFC3339)
			meta["target"] = target

			return output.PrintSuccess(items, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Domain or URL to analyze (required)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of referring domains to return")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

func newBacklinksCompetitorsCmd(format *string, verbose *bool) *cobra.Command {
	var target string
	var limit int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "competitors",
		Short: "Get competitor domains by backlink overlap",
		Long:  `Find domains that share backlink sources with a target from the DataForSEO Backlinks API.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if target == "" {
				return output.PrintCodedError(output.ErrBacklinksFailed, "target is required",
					fmt.Errorf("use --target to specify a domain"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrBacklinksFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			costAmount := 0.02 + 0.00003*float64(limit)
			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: costAmount,
				Units:       1,
				Basis:       fmt.Sprintf("dataforseo backlinks competitors: $0.02 base + $0.00003 × %d rows", limit),
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
					"limit":  limit,
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
				"target": target,
				"limit":  limit,
			}

			raw, err := client.Post("/v3/backlinks/competitors/live", []map[string]any{task})
			if err != nil {
				return output.PrintCodedError(output.ErrBacklinksFailed, "backlinks competitors request failed", err, nil, output.Format(*format))
			}

			var envelope dfseEnvelope
			if jsonErr := json.Unmarshal(raw, &envelope); jsonErr != nil {
				return output.PrintCodedError(output.ErrBacklinksFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
			}

			if envelope.StatusCode != 20000 {
				return output.PrintCodedError(output.ErrBacklinksFailed,
					fmt.Sprintf("DataForSEO error %d: %s", envelope.StatusCode, envelope.StatusMessage),
					fmt.Errorf("code %d", envelope.StatusCode), nil, output.Format(*format))
			}

			var results []json.RawMessage
			if len(envelope.Tasks) > 0 {
				results = envelope.Tasks[0].Result
			}

			var items []backlinks.CompetitorBacklinks
			if len(results) > 0 {
				type compResult struct {
					Items []struct {
						Domain                 string `json:"domain"`
						CommonReferringDomains int64  `json:"intersecting_domains"`
						Rank                   int    `json:"rank"`
					} `json:"items"`
				}

				var parsed compResult
				if parseErr := json.Unmarshal(results[0], &parsed); parseErr == nil {
					for _, item := range parsed.Items {
						items = append(items, backlinks.CompetitorBacklinks{
							Domain:                 item.Domain,
							CommonReferringDomains: item.CommonReferringDomains,
							Rank:                   item.Rank,
						})
					}
				}
			}

			meta["fetched_at"] = time.Now().Format(time.RFC3339)
			meta["target"] = target

			return output.PrintSuccess(items, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Domain or URL to analyze (required)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of competitors to return")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}

func newBacklinksGapCmd(format *string, verbose *bool) *cobra.Command {
	var target string
	var competitors string
	var limit int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "gap",
		Short: "Find domains that link to competitors but not to you",
		Long: `Identify link building targets by finding domains that link to your competitors
but not to your site. Uses the DataForSEO Backlinks Domain Intersection API.

If --competitors is not provided, competitors are loaded from state.json
(populated by 'supah-seo labs competitors').`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if target == "" {
				return output.PrintCodedError(output.ErrBacklinksFailed, "target is required",
					fmt.Errorf("use --target to specify your domain"), nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			if cfg.DataForSEOLogin == "" || cfg.DataForSEOPassword == "" {
				return output.PrintCodedError(output.ErrBacklinksFailed, "DataForSEO credentials not configured",
					fmt.Errorf("run 'supah-seo login' and select DataForSEO to configure credentials"), nil, output.Format(*format))
			}

			// Resolve competitor list.
			var compList []string
			if competitors != "" {
				for _, c := range strings.Split(competitors, ",") {
					c = strings.TrimSpace(c)
					if c != "" {
						compList = append(compList, c)
					}
				}
			} else {
				// Load from state.
				if !state.Exists(".") {
					return output.PrintCodedError(output.ErrBacklinksFailed, "no competitors provided and no state.json found",
						fmt.Errorf("use --competitors or run 'supah-seo labs competitors' first"), nil, output.Format(*format))
				}
				st, loadErr := state.Load(".")
				if loadErr != nil {
					return output.PrintCodedError(output.ErrBacklinksFailed, "failed to load state", loadErr, nil, output.Format(*format))
				}
				if st.Labs == nil || len(st.Labs.Competitors) == 0 {
					return output.PrintCodedError(output.ErrBacklinksFailed, "no competitors found in state",
						fmt.Errorf("use --competitors or run 'supah-seo labs competitors' first"), nil, output.Format(*format))
				}
				compList = st.Labs.Competitors
			}

			// API supports up to 20 targets, but we cap at 3 for cost/relevance.
			if len(compList) > 3 {
				compList = compList[:3]
			}

			costAmount := 0.02 + 0.00003*float64(limit)
			estimate, err := cost.BuildEstimate(cost.EstimateInput{
				UnitCostUSD: costAmount,
				Units:       1,
				Basis:       fmt.Sprintf("dataforseo backlinks gap: $0.02 base + $0.00003 × %d rows", limit),
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
					"target":      target,
					"competitors": compList,
					"limit":       limit,
					"status":      "dry_run",
				}, meta, output.Format(*format))
			}

			if approval.RequiresApproval {
				meta["reason"] = approval.Reason
				return output.PrintCodedError(output.ErrApprovalRequired, "cost exceeds approval threshold",
					fmt.Errorf("%s", approval.Reason), meta, output.Format(*format))
			}

			client := dataforseo.New(cfg.DataForSEOLogin, cfg.DataForSEOPassword)

			// Build targets map: competitors keyed "1", "2", "3".
			targets := make(map[string]string)
			for i, c := range compList {
				targets[fmt.Sprintf("%d", i+1)] = c
			}

			task := map[string]any{
				"targets":         targets,
				"exclude_targets": []string{target},
				"limit":           limit,
				"filters":         []any{"is_lost", "=", false},
			}

			raw, err := client.Post("/v3/backlinks/domain_intersection/live", []map[string]any{task})
			if err != nil {
				return output.PrintCodedError(output.ErrBacklinksFailed, "backlinks gap request failed", err, nil, output.Format(*format))
			}

			var envelope dfseEnvelope
			if jsonErr := json.Unmarshal(raw, &envelope); jsonErr != nil {
				return output.PrintCodedError(output.ErrBacklinksFailed, "failed to decode DataForSEO response", jsonErr, nil, output.Format(*format))
			}

			if envelope.StatusCode != 20000 {
				return output.PrintCodedError(output.ErrBacklinksFailed,
					fmt.Sprintf("DataForSEO error %d: %s", envelope.StatusCode, envelope.StatusMessage),
					fmt.Errorf("code %d", envelope.StatusCode), nil, output.Format(*format))
			}

			var results []json.RawMessage
			if len(envelope.Tasks) > 0 {
				results = envelope.Tasks[0].Result
			}

			numCompetitors := len(compList)
			var items []backlinks.BacklinkGap
			if len(results) > 0 {
				type gapItem struct {
					Domain              string `json:"domain"`
					ReferringLinksCount int    `json:"referring_links_count"`
					ReferringLinks1     int    `json:"referring_links_1"`
					ReferringLinks2     int    `json:"referring_links_2"`
					ReferringLinks3     int    `json:"referring_links_3"`
					IsLost              bool   `json:"is_lost"`
				}
				type gapResult struct {
					Items []gapItem `json:"items"`
				}

				var parsed gapResult
				if parseErr := json.Unmarshal(results[0], &parsed); parseErr == nil {
					for _, item := range parsed.Items {
						covered := 0
						counts := []int{item.ReferringLinks1, item.ReferringLinks2, item.ReferringLinks3}
						for i := 0; i < numCompetitors; i++ {
							if counts[i] > 0 {
								covered++
							}
						}
						items = append(items, backlinks.BacklinkGap{
							Domain:             item.Domain,
							TotalLinks:         item.ReferringLinksCount,
							CompetitorsCovered: covered,
						})
					}
				}
			}

			// Sort by total links descending.
			sort.Slice(items, func(i, j int) bool {
				return items[i].TotalLinks > items[j].TotalLinks
			})

			// Persist to state.
			if state.Exists(".") {
				if st, loadErr := state.Load("."); loadErr == nil {
					// Collect top 20 gap domain names.
					topN := 20
					if len(items) < topN {
						topN = len(items)
					}
					gapDomains := make([]string, topN)
					for i := 0; i < topN; i++ {
						gapDomains[i] = items[i].Domain
					}

					if st.Backlinks == nil {
						st.Backlinks = &state.BacklinksData{}
					}
					st.Backlinks.GapDomains = gapDomains
					st.AddHistory("backlinks", fmt.Sprintf("gap analysis: %d domains linking to competitors (%s) but not %s", len(items), strings.Join(compList, ", "), target))
					_ = st.Save(".")
				}
			}

			meta["fetched_at"] = time.Now().Format(time.RFC3339)
			meta["target"] = target
			meta["competitors"] = compList

			return output.PrintSuccess(items, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&target, "target", "", "Your domain to exclude from results (required)")
	cmd.Flags().StringVar(&competitors, "competitors", "", "Comma-separated competitor domains (auto-loaded from state if omitted)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of gap domains to return")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Estimate cost without executing")

	return cmd
}
