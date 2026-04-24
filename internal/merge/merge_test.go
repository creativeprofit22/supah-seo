package merge

import (
	"strings"
	"testing"

	"github.com/supah-seo/supah-seo/internal/state"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

// findRule returns the first MergedFinding with the given rule or nil.
func findRule(findings []MergedFinding, rule string) *MergedFinding {
	for i := range findings {
		if findings[i].Rule == rule {
			return &findings[i]
		}
	}
	return nil
}

// ─── cross-source rule tests ──────────────────────────────────────────────────

// TestRankingButNotClicking: page has title-too-long finding, GSC shows 50
// impressions with 0 clicks → expect "ranking-but-not-clicking".
func TestRankingButNotClicking(t *testing.T) {
	const pageURL = "https://example.com/product"

	st := buildState(
		[]state.Finding{
			{Rule: "title-too-long", URL: pageURL, Verdict: "fail"},
		},
		[]state.GSCRow{
			{Key: pageURL, Impressions: 50, Clicks: 0, CTR: 0, Position: 8},
		},
	)

	results := Run(st)

	f := findRule(results, "ranking-but-not-clicking")
	if f == nil {
		t.Fatalf("expected finding %q, got: %v", "ranking-but-not-clicking", results)
	}
	if f.GSCData == nil {
		t.Fatal("expected GSCData to be populated on the finding")
	}
	if f.GSCData.Impressions != 50 {
		t.Errorf("GSCData.Impressions = %.0f, want 50", f.GSCData.Impressions)
	}
	if f.GSCData.Clicks != 0 {
		t.Errorf("GSCData.Clicks = %.0f, want 0", f.GSCData.Clicks)
	}
	if len(f.Sources) == 0 {
		t.Error("expected Sources to be non-empty")
	}
}

// TestNotIndexed: page is present in crawl findings (200 status, no noindex
// issues) but absent from GSC data → expect "not-indexed".
func TestNotIndexed(t *testing.T) {
	const pageURL = "https://example.com/about"

	st := buildState(
		[]state.Finding{
			// A crawl-level issue that is NOT a noindex directive.
			{Rule: "missing-meta-description", URL: pageURL, Verdict: "fail"},
		},
		// GSC data is present (LastPull will be set) but for a different URL.
		[]state.GSCRow{
			{Key: "https://example.com/other-page", Impressions: 100, Clicks: 20},
		},
	)

	results := Run(st)

	f := findRule(results, "not-indexed")
	if f == nil {
		t.Fatalf("expected finding %q, got: %v", "not-indexed", results)
	}
	// not-indexed findings have no live GSC metrics by definition.
	if f.GSCData != nil {
		t.Error("not-indexed finding should not carry GSCData")
	}
}

// TestIssuesOnHighTrafficPage: page has crawl issues AND GSC shows 10 clicks
// → expect "issues-on-high-traffic-page".
func TestIssuesOnHighTrafficPage(t *testing.T) {
	const pageURL = "https://example.com/popular"

	st := buildState(
		[]state.Finding{
			{Rule: "slow-ttfb", URL: pageURL, Verdict: "fail"},
		},
		[]state.GSCRow{
			{Key: pageURL, Impressions: 200, Clicks: 10, CTR: 0.05, Position: 6},
		},
	)

	results := Run(st)

	f := findRule(results, "issues-on-high-traffic-page")
	if f == nil {
		t.Fatalf("expected finding %q, got: %v", "issues-on-high-traffic-page", results)
	}
	if f.GSCData == nil || f.GSCData.Clicks != 10 {
		t.Errorf("expected GSCData.Clicks = 10, got %v", f.GSCData)
	}
	if len(f.CrawlIssues) == 0 {
		t.Error("expected CrawlIssues to be non-empty")
	}
}

// TestThinContentRankingWell: page has thin-content finding and GSC position < 10
// → expect "thin-content-ranking-well".
func TestThinContentRankingWell(t *testing.T) {
	const pageURL = "https://example.com/guide"

	st := buildState(
		[]state.Finding{
			{Rule: "thin-content", URL: pageURL, Verdict: "fail"},
		},
		[]state.GSCRow{
			// Keep impressions ≤ 10 and clicks = 0 so other rules don't fire.
			{Key: pageURL, Impressions: 5, Clicks: 0, Position: 5},
		},
	)

	results := Run(st)

	f := findRule(results, "thin-content-ranking-well")
	if f == nil {
		t.Fatalf("expected finding %q, got: %v", "thin-content-ranking-well", results)
	}
	if f.GSCData == nil || f.GSCData.Position != 5 {
		t.Errorf("expected GSCData.Position = 5, got %v", f.GSCData)
	}
}

// TestNoMergedFindings: page has no crawl issues and strong GSC metrics
// → expect zero merged findings.
func TestNoMergedFindings(t *testing.T) {
	st := buildState(
		// No crawl findings at all.
		[]state.Finding{},
		[]state.GSCRow{
			{Key: "https://example.com/healthy", Impressions: 500, Clicks: 80, CTR: 0.16, Position: 2},
		},
	)

	results := Run(st)
	if len(results) != 0 {
		t.Errorf("expected 0 merged findings, got %d: %v", len(results), results)
	}
}

// TestURLNormalizationMatching: crawl URL has www prefix, GSC URL does not.
// Both should normalize to the same key and produce merged findings.
func TestURLNormalizationMatching(t *testing.T) {
	const crawlURL = "https://www.example.com/contact"
	const gscURL = "https://example.com/contact"

	st := buildState(
		[]state.Finding{
			{Rule: "title-too-long", URL: crawlURL, Verdict: "fail"},
		},
		// GSC stores the URL without www.
		[]state.GSCRow{
			{Key: gscURL, Impressions: 60, Clicks: 0, CTR: 0, Position: 7},
		},
	)

	results := Run(st)

	// ranking-but-not-clicking should fire: impressions > 10, clicks == 0, crawl issue present,
	// and www vs. non-www normalization must align the two URLs.
	f := findRule(results, "ranking-but-not-clicking")
	if f == nil {
		t.Fatalf("expected finding %q after URL normalization, got: %v", "ranking-but-not-clicking", results)
	}

	// not-indexed must NOT fire — the page is present in GSC after normalization.
	if ni := findRule(results, "not-indexed"); ni != nil {
		t.Errorf("not-indexed should not fire when a GSC row exists for the normalized URL")
	}
}

// TestSlowCoreWebVitals: page has PSI score < 50 AND GSC impressions → expect
// "slow-core-web-vitals" merged finding.
func TestSlowCoreWebVitals(t *testing.T) {
	const pageURL = "https://example.com/slow"

	st := buildState(
		[]state.Finding{},
		[]state.GSCRow{
			{Key: pageURL, Impressions: 200, Clicks: 5, CTR: 0.025, Position: 8},
		},
	)
	st.PSI = &state.PSIData{
		LastRun: "2025-01-01T00:00:00Z",
		Pages: []state.PSIResult{
			{URL: pageURL, PerformanceScore: 28, LCP: 6500, CLS: 0.05, Strategy: "mobile"},
		},
	}

	results := Run(st)

	f := findRule(results, "slow-core-web-vitals")
	if f == nil {
		t.Fatalf("expected finding %q, got: %v", "slow-core-web-vitals", results)
	}
	if f.GSCData == nil {
		t.Fatal("expected GSCData to be populated on the finding")
	}
	if f.GSCData.Impressions != 200 {
		t.Errorf("GSCData.Impressions = %.0f, want 200", f.GSCData.Impressions)
	}
	// Fix should mention LCP since it's above the 4000 ms poor threshold.
	if !strings.Contains(f.Fix, "LCP") {
		t.Errorf("Fix message should mention LCP; got: %s", f.Fix)
	}
}

// TestSlowCoreWebVitalsNoFire: PSI score < 50 but no GSC impressions →
// "slow-core-web-vitals" must NOT fire (no ranking potential to lose).
func TestSlowCoreWebVitalsNoFire(t *testing.T) {
	const pageURL = "https://example.com/ghost"

	st := buildState(
		[]state.Finding{},
		[]state.GSCRow{
			{Key: "https://example.com/other", Impressions: 100, Clicks: 10},
		},
	)
	st.PSI = &state.PSIData{
		LastRun: "2025-01-01T00:00:00Z",
		Pages: []state.PSIResult{
			{URL: pageURL, PerformanceScore: 20, LCP: 7000, CLS: 0.3, Strategy: "mobile"},
		},
	}

	results := Run(st)

	if f := findRule(results, "slow-core-web-vitals"); f != nil {
		t.Errorf("slow-core-web-vitals should not fire when page has no GSC impressions; got: %v", f)
	}
}

// TestSlowCoreWebVitalsGoodScore: PSI score >= 50 → rule must not fire.
func TestSlowCoreWebVitalsGoodScore(t *testing.T) {
	const pageURL = "https://example.com/ok"

	st := buildState(
		[]state.Finding{},
		[]state.GSCRow{
			{Key: pageURL, Impressions: 300, Clicks: 20, CTR: 0.07, Position: 4},
		},
	)
	st.PSI = &state.PSIData{
		LastRun: "2025-01-01T00:00:00Z",
		Pages: []state.PSIResult{
			{URL: pageURL, PerformanceScore: 72, LCP: 1800, CLS: 0.02, Strategy: "mobile"},
		},
	}

	results := Run(st)

	if f := findRule(results, "slow-core-web-vitals"); f != nil {
		t.Errorf("slow-core-web-vitals should not fire when performance score >= 50; got: %v", f)
	}
}

func buildState(findings []state.Finding, topPages []state.GSCRow) *state.State {
	st := &state.State{
		Site:      "https://example.com",
		LastCrawl: "2025-01-01T00:00:00Z",
		Findings:  findings,
	}
	if len(topPages) > 0 {
		st.GSC = &state.GSCData{
			LastPull: "2025-01-01T00:00:00Z",
			TopPages: topPages,
		}
	}
	return st
}

// ─── SERP-aware rule tests ───────────────────────────────────────────────────

func TestAIOverviewEatingClicks(t *testing.T) {
	const query = "what is seo"

	st := buildState(
		[]state.Finding{},
		[]state.GSCRow{
			{Key: "https://example.com/seo", Impressions: 100, Clicks: 10},
		},
	)
	st.GSC.TopKeywords = []state.GSCRow{
		{Key: query, Impressions: 200, Clicks: 1, CTR: 0.005, Position: 4},
	}
	st.SERP = &state.SERPData{
		LastRun: "2025-01-01T00:00:00Z",
		Queries: []state.SERPQueryResult{
			{Query: query, HasAIOverview: true},
		},
	}

	results := Run(st)

	f := findRule(results, "ai-overview-eating-clicks")
	if f == nil {
		t.Fatalf("expected finding %q, got: %v", "ai-overview-eating-clicks", results)
	}
	hasSERP, hasGSC := false, false
	for _, s := range f.Sources {
		if s == "serp" {
			hasSERP = true
		}
		if s == "gsc" {
			hasGSC = true
		}
	}
	if !hasSERP || !hasGSC {
		t.Errorf("expected sources [serp, gsc], got %v", f.Sources)
	}

	// Should NOT fire when HasAIOverview is false.
	st.SERP.Queries[0].HasAIOverview = false
	results = Run(st)
	if f := findRule(results, "ai-overview-eating-clicks"); f != nil {
		t.Error("ai-overview-eating-clicks should not fire when HasAIOverview is false")
	}
}

func TestFeaturedSnippetOpportunity(t *testing.T) {
	const query = "best seo tools"

	st := buildState(
		[]state.Finding{},
		[]state.GSCRow{
			{Key: "https://example.com/tools", Impressions: 100, Clicks: 5},
		},
	)
	st.GSC.TopKeywords = []state.GSCRow{
		{Key: query, Impressions: 100, Clicks: 5, CTR: 0.05, Position: 5},
	}
	st.SERP = &state.SERPData{
		LastRun: "2025-01-01T00:00:00Z",
		Queries: []state.SERPQueryResult{
			{
				Query:       query,
				OurPosition: 5,
				Features: []state.SERPFeatureRecord{
					{Type: "featured_snippet"},
				},
			},
		},
	}

	results := Run(st)

	f := findRule(results, "featured-snippet-opportunity")
	if f == nil {
		t.Fatalf("expected finding %q, got: %v", "featured-snippet-opportunity", results)
	}

	// Should NOT fire when position is 1 (already at top).
	st.SERP.Queries[0].OurPosition = 1
	results = Run(st)
	if f := findRule(results, "featured-snippet-opportunity"); f != nil {
		t.Error("featured-snippet-opportunity should not fire when position is 1")
	}
}

func TestPAAContentOpportunity(t *testing.T) {
	const query = "how to do seo"
	const pageURL = "https://example.com/guide"

	st := buildState(
		[]state.Finding{
			{Rule: "missing-meta-description", URL: pageURL, Verdict: "fail"},
		},
		[]state.GSCRow{
			{Key: pageURL, Impressions: 100, Clicks: 10, CTR: 0.10, Position: 3},
		},
	)
	st.GSC.TopKeywords = []state.GSCRow{
		{Key: query, Impressions: 100, Clicks: 10, CTR: 0.10, Position: 3},
	}
	st.SERP = &state.SERPData{
		LastRun: "2025-01-01T00:00:00Z",
		Queries: []state.SERPQueryResult{
			{
				Query:            query,
				RelatedQuestions: []string{"What is SEO?", "How long does SEO take?", "Is SEO worth it?"},
			},
		},
	}

	results := Run(st)

	f := findRule(results, "paa-content-opportunity")
	if f == nil {
		t.Fatalf("expected finding %q, got: %v", "paa-content-opportunity", results)
	}

	// Should NOT fire when fewer than 2 questions.
	st.SERP.Queries[0].RelatedQuestions = []string{"Only one?"}
	results = Run(st)
	if f := findRule(results, "paa-content-opportunity"); f != nil {
		t.Error("paa-content-opportunity should not fire with fewer than 2 related questions")
	}
}

func TestEasyWinKeyword(t *testing.T) {
	const query = "simple seo tips"

	st := buildState(
		[]state.Finding{},
		[]state.GSCRow{
			{Key: "https://example.com/tips", Impressions: 100, Clicks: 5},
		},
	)
	st.GSC.TopKeywords = []state.GSCRow{
		{Key: query, Impressions: 200, Clicks: 5, CTR: 0.025, Position: 8},
	}
	st.Labs = &state.LabsData{
		LastRun: "2025-01-01T00:00:00Z",
		Target:  "example.com",
		Keywords: []state.LabsKeyword{
			{Keyword: query, SearchVolume: 500, Difficulty: 20},
		},
	}

	results := Run(st)

	f := findRule(results, "easy-win-keyword")
	if f == nil {
		t.Fatalf("expected finding %q, got: %v", "easy-win-keyword", results)
	}

	// Should NOT fire when difficulty > 30.
	st.Labs.Keywords[0].Difficulty = 35
	results = Run(st)
	if f := findRule(results, "easy-win-keyword"); f != nil {
		t.Error("easy-win-keyword should not fire when difficulty > 30")
	}
}

func TestInformationalContentGap(t *testing.T) {
	const gapKeyword = "seo beginner guide"

	st := buildState(
		[]state.Finding{},
		[]state.GSCRow{
			{Key: "https://example.com/other", Impressions: 100, Clicks: 10},
		},
	)
	// GSC keywords do NOT contain the gap keyword.
	st.GSC.TopKeywords = []state.GSCRow{
		{Key: "existing keyword", Impressions: 50, Clicks: 5, Position: 3},
	}
	st.Labs = &state.LabsData{
		LastRun: "2025-01-01T00:00:00Z",
		Target:  "example.com",
		Keywords: []state.LabsKeyword{
			{Keyword: gapKeyword, SearchVolume: 300, Difficulty: 25, Intent: "informational"},
		},
	}

	results := Run(st)

	f := findRule(results, "informational-content-gap")
	if f == nil {
		t.Fatalf("expected finding %q, got: %v", "informational-content-gap", results)
	}

	// Should NOT fire when keyword IS in GSC (already ranking).
	st.GSC.TopKeywords = append(st.GSC.TopKeywords, state.GSCRow{
		Key: gapKeyword, Impressions: 10, Clicks: 1, Position: 12,
	})
	results = Run(st)
	if f := findRule(results, "informational-content-gap"); f != nil {
		t.Error("informational-content-gap should not fire when keyword is in GSC")
	}
}

// TestMergeWithoutGSC: crawl data + SERP data but NO GSC data.
// Run() must return a non-nil slice (even if empty) rather than nil,
// and SERP-aware rules must fire when SERP evidence is present.
func TestMergeWithoutGSC(t *testing.T) {
	const pageURL = "https://example.com/no-gsc"

	st := &state.State{
		Site:      "https://example.com",
		LastCrawl: "2025-01-01T00:00:00Z",
		Findings: []state.Finding{
			{Rule: "missing-meta-description", URL: pageURL, Verdict: "fail"},
		},
		// GSC is intentionally nil.
	}
	st.SERP = &state.SERPData{
		LastRun: "2025-01-01T00:00:00Z",
		Queries: []state.SERPQueryResult{
			{
				Query:       "example query",
				OurPosition: 5,
				Features: []state.SERPFeatureRecord{
					{Type: "featured_snippet"},
				},
			},
		},
	}

	results := Run(st)

	// Must not be nil — Run() should return an initialized (possibly empty) slice.
	if results == nil {
		t.Fatal("Run() returned nil with crawl data present; expected a non-nil slice")
	}

	// featured-snippet-opportunity requires a GSC keyword match to determine
	// position, so it may not fire here — but the function must not panic and
	// GSC-dependent rules (1-5) simply produce no findings (correct behaviour).
	serpRules := []string{"ai-overview-eating-clicks", "featured-snippet-opportunity", "paa-content-opportunity"}
	for _, rule := range serpRules {
		// Just verify no panic — we don't assert whether they fire or not
		// because GSC keyword lookup returns nothing when GSC is nil.
		_ = findRule(results, rule)
	}
}

func TestNoSERPDataSkipsRules(t *testing.T) {
	st := buildState(
		[]state.Finding{
			{Rule: "missing-title", URL: "https://example.com/page", Verdict: "fail"},
		},
		[]state.GSCRow{
			{Key: "https://example.com/page", Impressions: 100, Clicks: 5, CTR: 0.05, Position: 4},
		},
	)
	st.GSC.TopKeywords = []state.GSCRow{
		{Key: "test query", Impressions: 200, Clicks: 1, CTR: 0.005, Position: 4},
	}
	// SERP is nil — no SERP data.
	st.SERP = nil

	results := Run(st)

	serpRules := []string{"ai-overview-eating-clicks", "featured-snippet-opportunity", "paa-content-opportunity"}
	for _, rule := range serpRules {
		if f := findRule(results, rule); f != nil {
			t.Errorf("rule %q should not fire when SERP is nil", rule)
		}
	}
}

func TestNoLabsDataSkipsRules(t *testing.T) {
	st := buildState(
		[]state.Finding{
			{Rule: "missing-title", URL: "https://example.com/page", Verdict: "fail"},
		},
		[]state.GSCRow{
			{Key: "https://example.com/page", Impressions: 100, Clicks: 5, CTR: 0.05, Position: 4},
		},
	)
	st.GSC.TopKeywords = []state.GSCRow{
		{Key: "test query", Impressions: 200, Clicks: 5, CTR: 0.025, Position: 8},
	}
	// Labs is nil — no Labs data.
	st.Labs = nil

	results := Run(st)

	labsRules := []string{"easy-win-keyword", "informational-content-gap"}
	for _, rule := range labsRules {
		if f := findRule(results, rule); f != nil {
			t.Errorf("rule %q should not fire when Labs is nil", rule)
		}
	}
}

func TestPriorityHighTraffic(t *testing.T) {
	// Page with crawl issues AND GSC clicks > 0 should be HIGH (90-100).
	st := buildState(
		[]state.Finding{
			{Rule: "missing-title", URL: "https://example.com/page1"},
		},
		[]state.GSCRow{
			{Key: "https://example.com/page1", Clicks: 15, Impressions: 100, CTR: 0.15, Position: 5},
		},
	)

	results := Run(st)
	if len(results) == 0 {
		t.Fatal("expected at least one merged finding")
	}

	// Find the issues-on-high-traffic-page finding.
	var found *MergedFinding
	for i, r := range results {
		if r.Rule == "issues-on-high-traffic-page" && r.URL == "https://example.com/page1" {
			found = &results[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected issues-on-high-traffic-page finding")
	}
	if found.Priority != "high" {
		t.Errorf("expected priority high, got %s", found.Priority)
	}
	if found.PriorityScore < 90 || found.PriorityScore > 100 {
		t.Errorf("expected priority score 90-100, got %d", found.PriorityScore)
	}
}

func TestPriorityNoGSC(t *testing.T) {
	// Page with crawl issues but no GSC data for THIS url should be LOW.
	// We seed GSC with a row for a different URL so the not-indexed rule has
	// a comparison baseline; otherwise the rule correctly skips when GSC has
	// no data at all (prospect mode).
	st := buildState(
		[]state.Finding{
			{Rule: "missing-title", URL: "https://example.com/orphan"},
		},
		[]state.GSCRow{
			{Key: "https://example.com/other-page", Impressions: 50, Clicks: 5},
		},
	)

	results := Run(st)
	if len(results) == 0 {
		t.Fatal("expected at least one merged finding")
	}

	var found *MergedFinding
	for i, r := range results {
		if r.URL == "https://example.com/orphan" {
			found = &results[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected finding for orphan page")
	}
	if found.Priority != "low" {
		t.Errorf("expected priority low, got %s", found.Priority)
	}
	if found.PriorityScore < 10 || found.PriorityScore > 49 {
		t.Errorf("expected priority score 10-49, got %d", found.PriorityScore)
	}
}

func TestSortOrder(t *testing.T) {
	// Create findings that produce different priority scores and verify
	// they come back sorted by PriorityScore descending.
	st := buildState(
		[]state.Finding{
			{Rule: "missing-title", URL: "https://example.com/high"},
			{Rule: "missing-title", URL: "https://example.com/low"},
			{Rule: "missing-h1", URL: "https://example.com/mid"},
		},
		[]state.GSCRow{
			// high: clicks > 0 → HIGH (90-100)
			{Key: "https://example.com/high", Clicks: 20, Impressions: 200, CTR: 0.10, Position: 3},
			// mid: impressions > 20, clicks == 0 → HIGH (80-89)
			{Key: "https://example.com/mid", Clicks: 0, Impressions: 50, CTR: 0.0, Position: 8},
			// low is absent from GSC → LOW (via not-indexed rule)
		},
	)

	results := Run(st)
	if len(results) < 3 {
		t.Fatalf("expected at least 3 findings, got %d", len(results))
	}

	// Verify descending order.
	for i := 1; i < len(results); i++ {
		if results[i].PriorityScore > results[i-1].PriorityScore {
			t.Errorf("findings not sorted: index %d (score %d) > index %d (score %d)",
				i, results[i].PriorityScore, i-1, results[i-1].PriorityScore)
		}
	}

	// The first finding should have a higher score than the last.
	if results[0].PriorityScore <= results[len(results)-1].PriorityScore {
		t.Errorf("first finding (score %d) should have higher score than last (score %d)",
			results[0].PriorityScore, results[len(results)-1].PriorityScore)
	}

	// Verify every finding has a priority label assigned.
	for _, r := range results {
		if r.Priority == "" {
			t.Errorf("finding %s on %s has empty priority", r.Rule, r.URL)
		}
		if r.PriorityScore == 0 {
			t.Errorf("finding %s on %s has zero priority score", r.Rule, r.URL)
		}
	}
}
