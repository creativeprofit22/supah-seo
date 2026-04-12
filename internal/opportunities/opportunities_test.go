package opportunities

import (
	"testing"

	"github.com/supah-seo/supah-seo/internal/gsc"
	"github.com/supah-seo/supah-seo/internal/serp"
)

func TestMergeGSCOnly(t *testing.T) {
	input := MergeInput{
		GSCSeeds: []gsc.OpportunitySeed{
			{
				Query:       "seo tool",
				Page:        "https://example.com/seo",
				Clicks:      10,
				Impressions: 500,
				CTR:         0.02,
				Position:    8,
			},
		},
	}

	opps := Merge(input)
	if len(opps) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(opps))
	}

	opp := opps[0]
	if opp.Type != TypePage {
		t.Fatalf("expected type 'page', got %q", opp.Type)
	}
	if opp.Target != "https://example.com/seo" {
		t.Fatalf("expected target 'https://example.com/seo', got %q", opp.Target)
	}
	if len(opp.Sources) != 1 || opp.Sources[0] != "gsc" {
		t.Fatalf("expected sources [gsc], got %v", opp.Sources)
	}
	if opp.Confidence == 0 {
		t.Fatal("expected non-zero confidence")
	}
}

func TestMergeWithSERP(t *testing.T) {
	input := MergeInput{
		GSCSeeds: []gsc.OpportunitySeed{
			{
				Query:       "seo tool",
				Page:        "https://example.com/seo",
				Clicks:      10,
				Impressions: 500,
				CTR:         0.02,
				Position:    8,
			},
		},
		SERPResults: map[string]*serp.AnalyzeResponse{
			"seo tool": {
				Query: "seo tool",
				OrganicResults: []serp.OrganicResult{
					{Position: 1, Title: "Top Result", Link: "https://competitor.com", Domain: "competitor.com"},
					{Position: 5, Title: "Our Page", Link: "https://example.com/seo", Domain: "example.com"},
				},
			},
		},
	}

	opps := Merge(input)
	if len(opps) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(opps))
	}

	opp := opps[0]
	if len(opp.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %v", opp.Sources)
	}

	hasSERP := false
	for _, s := range opp.Sources {
		if s == "serp" {
			hasSERP = true
		}
	}
	if !hasSERP {
		t.Fatal("expected serp in sources")
	}
}

func TestMergeEmpty(t *testing.T) {
	opps := Merge(MergeInput{})
	if opps != nil {
		t.Fatalf("expected nil for empty input, got %v", opps)
	}
}

func TestMergeLowCTRFirstPage(t *testing.T) {
	input := MergeInput{
		GSCSeeds: []gsc.OpportunitySeed{
			{
				Query:       "best tool",
				Page:        "https://example.com/best",
				Clicks:      5,
				Impressions: 1000,
				CTR:         0.005,
				Position:    5,
			},
		},
	}

	opps := Merge(input)
	if len(opps) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(opps))
	}

	if opps[0].ImpactEstimate != "high" {
		t.Fatalf("expected high impact for low CTR first-page, got %q", opps[0].ImpactEstimate)
	}
	if opps[0].EffortEstimate != "low" {
		t.Fatalf("expected low effort for first-page CTR fix, got %q", opps[0].EffortEstimate)
	}
}

func TestLabsEnrichment(t *testing.T) {
	input := MergeInput{
		GSCSeeds: []gsc.OpportunitySeed{
			{
				Query:       "seo tips",
				Page:        "https://example.com/tips",
				Clicks:      5,
				Impressions: 200,
				CTR:         0.025,
				Position:    8,
			},
		},
		LabsKeywords: map[string]LabsKeywordInfo{
			"seo tips": {SearchVolume: 1200, Difficulty: 25, Intent: "informational"},
		},
	}

	opps := Merge(input)
	if len(opps) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(opps))
	}

	opp := opps[0]

	// Check labs is in sources.
	hasLabs := false
	for _, s := range opp.Sources {
		if s == "labs" {
			hasLabs = true
		}
	}
	if !hasLabs {
		t.Fatalf("expected labs in sources, got %v", opp.Sources)
	}

	// Check evidence mentions difficulty and volume.
	hasDifficulty, hasVolume := false, false
	for _, e := range opp.Evidence {
		if contains(e, "difficulty") {
			hasDifficulty = true
		}
		if contains(e, "volume") {
			hasVolume = true
		}
	}
	if !hasDifficulty {
		t.Error("expected evidence to mention keyword difficulty")
	}
	if !hasVolume {
		t.Error("expected evidence to mention search volume")
	}
}

func TestSERPFeatureEnrichment(t *testing.T) {
	input := MergeInput{
		GSCSeeds: []gsc.OpportunitySeed{
			{
				Query:       "ai search",
				Page:        "https://example.com/ai",
				Clicks:      10,
				Impressions: 500,
				CTR:         0.02,
				Position:    5,
			},
		},
		SERPResults: map[string]*serp.AnalyzeResponse{
			"ai search": {
				Query:         "ai search",
				HasAIOverview: true,
				OrganicResults: []serp.OrganicResult{
					{Position: 5, Title: "Our Page", Link: "https://example.com/ai", Domain: "example.com"},
				},
			},
		},
	}

	opps := Merge(input)
	if len(opps) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(opps))
	}

	opp := opps[0]
	if opp.Type != TypeAnswer {
		t.Errorf("expected type %q, got %q", TypeAnswer, opp.Type)
	}

	hasAIEvidence := false
	for _, e := range opp.Evidence {
		if contains(e, "AI Overview") {
			hasAIEvidence = true
		}
	}
	if !hasAIEvidence {
		t.Error("expected evidence to mention AI Overview")
	}
}

func TestFeaturedSnippetEnrichment(t *testing.T) {
	input := MergeInput{
		GSCSeeds: []gsc.OpportunitySeed{
			{
				Query:       "best practices",
				Page:        "https://example.com/best",
				Clicks:      8,
				Impressions: 300,
				CTR:         0.027,
				Position:    4,
			},
		},
		SERPResults: map[string]*serp.AnalyzeResponse{
			"best practices": {
				Query: "best practices",
				Features: []serp.SERPFeature{
					{Type: serp.FeatureFeaturedSnippet, Position: 0, Title: "Snippet"},
				},
				OrganicResults: []serp.OrganicResult{
					{Position: 4, Title: "Our Page", Link: "https://example.com/best", Domain: "example.com"},
				},
			},
		},
	}

	opps := Merge(input)
	if len(opps) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(opps))
	}

	hasSnippetEvidence := false
	for _, e := range opps[0].Evidence {
		if contains(e, "Featured Snippet") {
			hasSnippetEvidence = true
		}
	}
	if !hasSnippetEvidence {
		t.Error("expected evidence to mention Featured Snippet")
	}
}

// contains checks if s contains substr (case-sensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestMergePage2Ranking(t *testing.T) {
	input := MergeInput{
		GSCSeeds: []gsc.OpportunitySeed{
			{
				Query:       "near miss",
				Page:        "https://example.com/near",
				Clicks:      2,
				Impressions: 100,
				CTR:         0.02,
				Position:    15,
			},
		},
	}

	opps := Merge(input)
	if len(opps) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(opps))
	}
	if opps[0].ImpactEstimate != "medium" {
		t.Fatalf("expected medium impact for page-2, got %q", opps[0].ImpactEstimate)
	}
}
