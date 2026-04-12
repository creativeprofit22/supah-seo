package opportunities

import (
	"fmt"

	"github.com/supah-seo/supah-seo/internal/gsc"
	"github.com/supah-seo/supah-seo/internal/serp"
)

// Type classifies an opportunity.
type Type string

const (
	TypePage    Type = "page"
	TypeKeyword Type = "keyword"
	TypeAnswer  Type = "answer"
)

// Opportunity represents an actionable SEO improvement signal.
type Opportunity struct {
	Type           Type     `json:"type"`
	Target         string   `json:"target"`
	Evidence       []string `json:"evidence"`
	Confidence     float64  `json:"confidence"`
	ImpactEstimate string   `json:"impact_estimate"`
	EffortEstimate string   `json:"effort_estimate"`
	Sources        []string `json:"sources"`
	EstimatedCost  float64  `json:"estimated_cost"`
}

// LabsKeywordInfo holds keyword intelligence for opportunity scoring.
type LabsKeywordInfo struct {
	SearchVolume int
	Difficulty   float64
	Intent       string
}

// MergeInput holds all data sources for opportunity detection.
type MergeInput struct {
	GSCSeeds     []gsc.OpportunitySeed
	SERPResults  map[string]*serp.AnalyzeResponse
	LabsKeywords map[string]LabsKeywordInfo // keyword -> difficulty/volume/intent
}

// Merge combines GSC seed data with optional SERP evidence into ranked opportunities.
func Merge(input MergeInput) []Opportunity {
	var opps []Opportunity

	for _, seed := range input.GSCSeeds {
		var labsInfo *LabsKeywordInfo
		if input.LabsKeywords != nil {
			if info, ok := input.LabsKeywords[seed.Query]; ok {
				labsInfo = &info
			}
		}
		opp := fromGSCSeed(seed, labsInfo)

		// Enrich with SERP data if available
		if serpResp, ok := input.SERPResults[seed.Query]; ok && serpResp != nil {
			enrichWithSERP(&opp, seed, serpResp)
		}

		opps = append(opps, opp)
	}

	return opps
}

func fromGSCSeed(seed gsc.OpportunitySeed, labsInfo *LabsKeywordInfo) Opportunity {
	opp := Opportunity{
		Type:    TypeKeyword,
		Target:  seed.Query,
		Sources: []string{"gsc"},
	}

	// Position-based opportunity classification
	if seed.Position <= 10 && seed.CTR < 0.03 {
		opp.Evidence = append(opp.Evidence, "low CTR despite first-page ranking")
		opp.ImpactEstimate = "high"
		opp.EffortEstimate = "low"
		opp.Confidence = 0.8
	} else if seed.Position > 10 && seed.Position <= 20 {
		opp.Evidence = append(opp.Evidence, "ranking on page 2, close to first page")
		opp.ImpactEstimate = "medium"
		opp.EffortEstimate = "medium"
		opp.Confidence = 0.6
	} else if seed.Impressions > 100 && seed.Position > 20 {
		opp.Evidence = append(opp.Evidence, "high impressions with poor ranking")
		opp.ImpactEstimate = "medium"
		opp.EffortEstimate = "high"
		opp.Confidence = 0.5
	} else {
		opp.Evidence = append(opp.Evidence, "underperforming query")
		opp.ImpactEstimate = "low"
		opp.EffortEstimate = "medium"
		opp.Confidence = 0.4
	}

	// Page-level opportunity if specific page is involved
	if seed.Page != "" {
		opp.Type = TypePage
		opp.Target = seed.Page
		opp.Evidence = append(opp.Evidence, "query: "+seed.Query)
	}

	// Enrich with Labs keyword intelligence
	if labsInfo != nil {
		opp.Sources = append(opp.Sources, "labs")
		opp.Evidence = append(opp.Evidence, fmt.Sprintf("keyword difficulty: %.0f/100", labsInfo.Difficulty))
		opp.Evidence = append(opp.Evidence, fmt.Sprintf("monthly search volume: %d", labsInfo.SearchVolume))

		if labsInfo.Difficulty < 30 && seed.Position > 5 {
			opp.ImpactEstimate = "high"
			opp.Confidence += 0.15
		}

		if labsInfo.Difficulty > 70 {
			opp.ImpactEstimate = "low"
			opp.Evidence = append(opp.Evidence, "highly competitive keyword")
		}

		if labsInfo.Intent != "" {
			opp.Evidence = append(opp.Evidence, fmt.Sprintf("search intent: %s", labsInfo.Intent))
		}

		// Cap confidence at 1.0
		if opp.Confidence > 1.0 {
			opp.Confidence = 1.0
		}
	}

	return opp
}

func enrichWithSERP(opp *Opportunity, seed gsc.OpportunitySeed, serpResp *serp.AnalyzeResponse) {
	opp.Sources = append(opp.Sources, "serp")
	opp.EstimatedCost = 0.002 // approximate per-query SERP cost

	// Check if the seed page appears in current SERP results
	pageFound := false
	for _, result := range serpResp.OrganicResults {
		if result.Link == seed.Page {
			pageFound = true
			if result.Position < int(seed.Position) {
				opp.Evidence = append(opp.Evidence, "SERP position improved since GSC data")
				opp.Confidence += 0.1
			} else if result.Position > int(seed.Position) {
				opp.Evidence = append(opp.Evidence, "SERP position declined since GSC data")
				opp.Confidence += 0.05
			}
			break
		}
	}

	if !pageFound && len(serpResp.OrganicResults) > 0 {
		opp.Evidence = append(opp.Evidence, "page not found in current SERP top results")
	}

	// AI Overview detection
	if serpResp.HasAIOverview {
		opp.Type = TypeAnswer
		opp.Evidence = append(opp.Evidence, "AI Overview present — clicks may be suppressed")
	}

	// Featured Snippet opportunity
	for _, feat := range serpResp.Features {
		if feat.Type == serp.FeatureFeaturedSnippet {
			// If site ranks 2-10, there's an opportunity to capture position 0
			if int(seed.Position) >= 2 && int(seed.Position) <= 10 {
				opp.Evidence = append(opp.Evidence, "Featured Snippet opportunity — you could capture position 0")
			}
			break
		}
	}

	// People Also Ask detection
	if len(serpResp.RelatedQuestions) > 0 {
		opp.Evidence = append(opp.Evidence, fmt.Sprintf("%d People Also Ask questions detected", len(serpResp.RelatedQuestions)))
	}

	// Cap confidence at 1.0
	if opp.Confidence > 1.0 {
		opp.Confidence = 1.0
	}
}
