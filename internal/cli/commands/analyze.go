package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/supah-seo/supah-seo/internal/merge"
	"github.com/supah-seo/supah-seo/internal/state"
	"github.com/supah-seo/supah-seo/pkg/output"
	"github.com/spf13/cobra"
)

// NewAnalyzeCmd returns the analyze command that runs cross-source merge
// and writes results to state.json.
func NewAnalyzeCmd(format *string, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "analyze",
		Short: "Run cross-source analysis and merge findings into state",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !state.Exists(".") {
				return output.PrintCodedError(
					"NO_PROJECT",
					"No project initialized — run supah-seo init --url <site>",
					nil, nil,
					output.Format(*format),
				)
			}

			st, err := state.Load(".")
			if err != nil {
				return output.PrintCodedError("STATE_LOAD_FAILED", "failed to load state", err, nil, output.Format(*format))
			}

			if st.LastCrawl == "" {
				return output.PrintCodedError(
					"NO_CRAWL_DATA",
					"Run supah-seo audit run first",
					nil, nil,
					output.Format(*format),
				)
			}

			findings := merge.Run(st)
			if findings == nil {
				findings = []merge.MergedFinding{}
			}

			// Marshal merged findings into state as raw JSON.
			raw, err := json.Marshal(findings)
			if err != nil {
				return output.PrintCodedError("MARSHAL_FAILED", "failed to marshal merged findings", err, nil, output.Format(*format))
			}
			st.MergedFindings = raw
			st.LastAnalysis = time.Now().UTC().Format(time.RFC3339)

			// Build history detail.
			detail := fmt.Sprintf("analyze: %d merged findings", len(findings))
			if len(findings) > 0 {
				detail += fmt.Sprintf(", top priority: %s", findings[0].Rule)
			}
			st.AddHistory("analyze", detail)

			if err := st.Save("."); err != nil {
				return output.PrintCodedError("STATE_SAVE_FAILED", "failed to save state", err, nil, output.Format(*format))
			}

			// Build top 5 summary.
			type findingSummary struct {
				Rule     string `json:"rule"`
				URL      string `json:"url"`
				Priority string `json:"priority"`
				Why      string `json:"why"`
			}
			top := make([]findingSummary, 0, 5)
			for i := 0; i < len(findings) && i < 5; i++ {
				top = append(top, findingSummary{
					Rule:     findings[i].Rule,
					URL:      findings[i].URL,
					Priority: findings[i].Priority,
					Why:      findings[i].Why,
				})
			}

			gscAvailable := st.GSC != nil && st.GSC.LastPull != ""
			serpAvailable := st.SERP != nil && st.SERP.LastRun != ""
			labsAvailable := st.Labs != nil && st.Labs.LastRun != ""
			backlinksAvailable := st.Backlinks != nil && st.Backlinks.LastRun != ""

			data := map[string]any{
				"merged_findings_count": len(findings),
				"top_findings":          top,
				"gsc_available":         gscAvailable,
				"serp_available":        serpAvailable,
				"labs_available":        labsAvailable,
				"backlinks_available":   backlinksAvailable,
			}
			if !gscAvailable {
				data["gsc_note"] = "GSC data not available — connect GSC for deeper analysis"
			}
			if !serpAvailable {
				data["serp_note"] = "SERP data not available — run 'supah-seo serp analyze' on key queries for SERP feature detection"
			}
			if !labsAvailable {
				data["labs_note"] = "Labs data not available — run 'supah-seo labs ranked-keywords' for keyword difficulty scoring"
			}
			if !backlinksAvailable {
				data["backlinks_note"] = "Backlinks data not available — run 'supah-seo backlinks summary' for link profile analysis"
			}

			metadata := map[string]any{
				"generated_at": time.Now().UTC().Format(time.RFC3339),
			}
			return output.PrintSuccess(data, metadata, output.Format(*format))
		},
	}
}
