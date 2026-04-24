package commands

import (
	"encoding/json"
	"time"

	"github.com/spf13/cobra"
	"github.com/supah-seo/supah-seo/internal/merge"
	"github.com/supah-seo/supah-seo/internal/state"
	"github.com/supah-seo/supah-seo/pkg/output"
)

// NewStatusCmd returns the status command.
func NewStatusCmd(format *string, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current project state",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !state.Exists(".") {
				return output.PrintCodedError(
					"NO_PROJECT",
					"No project initialized — run supah-seo init --url <site>",
					nil, nil,
					output.Format(*format),
				)
			}

			s, err := state.Load(".")
			if err != nil {
				return output.PrintCodedError("STATE_LOAD_FAILED", "failed to load state", err, nil, output.Format(*format))
			}

			used, missing := s.Sources()

			// Parse merged findings for count and top priority.
			var mergedCount int
			var topPriority string
			if len(s.MergedFindings) > 0 {
				var merged []merge.MergedFinding
				if err := json.Unmarshal(s.MergedFindings, &merged); err == nil {
					mergedCount = len(merged)
					if len(merged) > 0 {
						topPriority = merged[0].Rule
					}
				}
			}

			data := map[string]any{
				"site":                  s.Site,
				"initialized":           s.Initialized,
				"last_crawl":            s.LastCrawl,
				"score":                 s.Score,
				"pages_crawled":         s.PagesCrawled,
				"findings_count":        len(s.Findings),
				"merged_findings_count": mergedCount,
				"top_priority":          topPriority,
				"last_analysis":         s.LastAnalysis,
				"history_count":         len(s.History),
				"sources_used":          used,
				"sources_missing":       missing,
			}
			if s.SERP != nil {
				data["serp_queries_stored"] = len(s.SERP.Queries)
			}
			if s.Labs != nil {
				data["labs_keywords_stored"] = len(s.Labs.Keywords)
				data["labs_competitors_stored"] = len(s.Labs.Competitors)
			}
			if s.Backlinks != nil {
				data["backlinks_total"] = s.Backlinks.TotalBacklinks
				data["referring_domains_total"] = s.Backlinks.TotalReferringDomains
			}
			metadata := map[string]any{
				"generated_at": time.Now().UTC().Format(time.RFC3339),
			}
			return output.PrintSuccess(data, metadata, output.Format(*format))
		},
	}
}
