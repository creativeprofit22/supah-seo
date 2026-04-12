package serp

import "github.com/supah-seo/supah-seo/internal/common/cost"

// AnalyzeRequest defines the input for a SERP analysis.
type AnalyzeRequest struct {
	Query      string `json:"query"`
	Location   string `json:"location,omitempty"`
	Language   string `json:"language,omitempty"`
	NumResults int    `json:"num_results,omitempty"`
}

// OrganicResult represents a single organic search result.
type OrganicResult struct {
	Position int    `json:"position"`
	Title    string `json:"title"`
	Link     string `json:"link"`
	Snippet  string `json:"snippet"`
	Domain   string `json:"domain,omitempty"`
}

// SERPFeatureType classifies a SERP element.
type SERPFeatureType string

const (
	FeatureFeaturedSnippet SERPFeatureType = "featured_snippet"
	FeaturePeopleAlsoAsk   SERPFeatureType = "people_also_ask"
	FeatureLocalPack       SERPFeatureType = "local_pack"
	FeatureKnowledgeGraph  SERPFeatureType = "knowledge_graph"
	FeatureAIOverview      SERPFeatureType = "ai_overview"
	FeatureTopStories      SERPFeatureType = "top_stories"
	FeatureInlineVideos    SERPFeatureType = "inline_videos"
	FeatureInlineShopping  SERPFeatureType = "inline_shopping"
	FeatureInlineImages    SERPFeatureType = "inline_images"
)

// SERPFeature represents a non-organic element detected on a SERP.
type SERPFeature struct {
	Type     SERPFeatureType `json:"type"`
	Position int             `json:"position,omitempty"` // rank position on page, 0 if not applicable
	Title    string          `json:"title,omitempty"`
	URL      string          `json:"url,omitempty"`
	Domain   string          `json:"domain,omitempty"`
	Snippet  string          `json:"snippet,omitempty"`
}

// RelatedQuestion represents a "People Also Ask" question from the SERP.
type RelatedQuestion struct {
	Question string `json:"question"`
	Snippet  string `json:"snippet,omitempty"`
	Title    string `json:"title,omitempty"`
	Link     string `json:"link,omitempty"`
}

// AnalyzeResponse holds the result of a SERP analysis.
type AnalyzeResponse struct {
	Query            string            `json:"query"`
	OrganicResults   []OrganicResult   `json:"organic_results"`
	Features         []SERPFeature     `json:"features,omitempty"`
	RelatedQuestions []RelatedQuestion `json:"related_questions,omitempty"`
	HasAIOverview    bool              `json:"has_ai_overview"`
	TotalResults     int64             `json:"total_results,omitempty"`
	SearchTime       float64           `json:"search_time,omitempty"`
}

// Provider defines the interface for SERP data providers.
type Provider interface {
	// Name returns the provider identifier.
	Name() string
	// Estimate returns a cost estimate for the given request without executing.
	Estimate(req AnalyzeRequest) (cost.Estimate, error)
	// Analyze executes a SERP query and returns results.
	Analyze(req AnalyzeRequest) (*AnalyzeResponse, error)
}
