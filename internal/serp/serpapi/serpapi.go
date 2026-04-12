package serpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/supah-seo/supah-seo/internal/common/cost"
	"github.com/supah-seo/supah-seo/internal/serp"
)

const (
	defaultBaseURL    = "https://serpapi.com/search"
	defaultAccountURL = "https://serpapi.com/account.json"
	costPerSearchUSD  = 0.01 // estimated cost per search
	costBasis         = "serpapi: $0.01/search (estimate based on 100 searches/month plan)"
)

// HTTPClient is an interface for HTTP operations (supports testing).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Adapter implements serp.Provider for SerpAPI.
type Adapter struct {
	apiKey     string
	baseURL    string
	accountURL string
	httpClient HTTPClient
}

// Option configures the SerpAPI adapter.
type Option func(*Adapter)

// WithBaseURL overrides the default SerpAPI endpoint (useful for testing).
func WithBaseURL(url string) Option {
	return func(a *Adapter) { a.baseURL = url }
}

// WithAccountURL overrides the default SerpAPI account endpoint (useful for testing).
func WithAccountURL(url string) Option {
	return func(a *Adapter) { a.accountURL = url }
}

// WithHTTPClient overrides the default HTTP client (useful for testing).
func WithHTTPClient(c HTTPClient) Option {
	return func(a *Adapter) { a.httpClient = c }
}

// New creates a SerpAPI adapter.
func New(apiKey string, opts ...Option) *Adapter {
	a := &Adapter{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		accountURL: defaultAccountURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Name returns the provider identifier.
func (a *Adapter) Name() string { return "serpapi" }

// Estimate returns a cost estimate without executing the search.
func (a *Adapter) Estimate(req serp.AnalyzeRequest) (cost.Estimate, error) {
	units := 1
	if req.NumResults > 100 {
		units = (req.NumResults + 99) / 100
	}
	return cost.BuildEstimate(cost.EstimateInput{
		UnitCostUSD: costPerSearchUSD,
		Units:       units,
		Basis:       costBasis,
	})
}

// Analyze executes a SERP query against SerpAPI.
func (a *Adapter) Analyze(req serp.AnalyzeRequest) (*serp.AnalyzeResponse, error) {
	params := url.Values{
		"api_key": {a.apiKey},
		"q":       {req.Query},
		"engine":  {"google"},
	}
	if req.Location != "" {
		params.Set("location", req.Location)
	} else {
		params.Set("location", "Perth,Western Australia,Australia")
	}
	if req.Language != "" {
		params.Set("hl", req.Language)
	} else {
		params.Set("hl", "en")
	}
	params.Set("gl", "au") // default to Australian Google results
	if req.NumResults > 0 {
		params.Set("num", strconv.Itoa(req.NumResults))
	}

	httpReq, err := http.NewRequest("GET", a.baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating serpapi request: %w", err)
	}

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("serpapi request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("serpapi returned status %d", resp.StatusCode)
	}

	// Decode into a generic map so we can inspect top-level keys for SERP features.
	var topLevel map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&topLevel); err != nil {
		return nil, fmt.Errorf("decoding serpapi response: %w", err)
	}

	// Parse organic results.
	var organicRaw []struct {
		Position int    `json:"position"`
		Title    string `json:"title"`
		Link     string `json:"link"`
		Snippet  string `json:"snippet"`
	}
	if raw, ok := topLevel["organic_results"]; ok {
		_ = json.Unmarshal(raw, &organicRaw)
	}

	results := make([]serp.OrganicResult, 0, len(organicRaw))
	for _, r := range organicRaw {
		parsed, _ := url.Parse(r.Link)
		domain := ""
		if parsed != nil {
			domain = parsed.Hostname()
		}
		results = append(results, serp.OrganicResult{
			Position: r.Position,
			Title:    r.Title,
			Link:     r.Link,
			Snippet:  r.Snippet,
			Domain:   domain,
		})
	}

	// Parse search information.
	var searchInfo struct {
		TotalResults     string  `json:"total_results"`
		TimeTakenDisplay float64 `json:"time_taken_displayed"`
	}
	if raw, ok := topLevel["search_information"]; ok {
		_ = json.Unmarshal(raw, &searchInfo)
	}
	totalResults, _ := strconv.ParseInt(searchInfo.TotalResults, 10, 64)

	// Parse SERP features.
	var features []serp.SERPFeature
	var relatedQuestions []serp.RelatedQuestion
	hasAIOverview := false

	// answer_box → featured snippet
	if raw, ok := topLevel["answer_box"]; ok {
		var ab struct {
			Type    string `json:"type"`
			Title   string `json:"title"`
			Answer  string `json:"answer"`
			Snippet string `json:"snippet"`
			Link    string `json:"link"`
		}
		if json.Unmarshal(raw, &ab) == nil {
			snippet := ab.Snippet
			if snippet == "" {
				snippet = ab.Answer
			}
			features = append(features, serp.SERPFeature{
				Type:    serp.FeatureFeaturedSnippet,
				Title:   ab.Title,
				URL:     ab.Link,
				Snippet: snippet,
			})
		}
	}

	// related_questions → people also ask
	if raw, ok := topLevel["related_questions"]; ok {
		var rqs []struct {
			Question string `json:"question"`
			Snippet  string `json:"snippet"`
			Title    string `json:"title"`
			Link     string `json:"link"`
		}
		if json.Unmarshal(raw, &rqs) == nil && len(rqs) > 0 {
			features = append(features, serp.SERPFeature{
				Type: serp.FeaturePeopleAlsoAsk,
			})
			for _, rq := range rqs {
				relatedQuestions = append(relatedQuestions, serp.RelatedQuestion{
					Question: rq.Question,
					Snippet:  rq.Snippet,
					Title:    rq.Title,
					Link:     rq.Link,
				})
			}
		}
	}

	// local_results → local pack
	if _, ok := topLevel["local_results"]; ok {
		features = append(features, serp.SERPFeature{
			Type: serp.FeatureLocalPack,
		})
	}

	// knowledge_graph
	if raw, ok := topLevel["knowledge_graph"]; ok {
		var kg struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Source      string `json:"source"`
		}
		if json.Unmarshal(raw, &kg) == nil {
			features = append(features, serp.SERPFeature{
				Type:    serp.FeatureKnowledgeGraph,
				Title:   kg.Title,
				Snippet: kg.Description,
			})
		}
	}

	// ai_overview
	if _, ok := topLevel["ai_overview"]; ok {
		hasAIOverview = true
		features = append(features, serp.SERPFeature{
			Type: serp.FeatureAIOverview,
		})
	}

	// top_stories
	if _, ok := topLevel["top_stories"]; ok {
		features = append(features, serp.SERPFeature{
			Type: serp.FeatureTopStories,
		})
	}

	// inline_videos
	if _, ok := topLevel["inline_videos"]; ok {
		features = append(features, serp.SERPFeature{
			Type: serp.FeatureInlineVideos,
		})
	}

	// inline_images
	if _, ok := topLevel["inline_images"]; ok {
		features = append(features, serp.SERPFeature{
			Type: serp.FeatureInlineImages,
		})
	}

	// shopping_results
	if _, ok := topLevel["shopping_results"]; ok {
		features = append(features, serp.SERPFeature{
			Type: serp.FeatureInlineShopping,
		})
	}

	return &serp.AnalyzeResponse{
		Query:            req.Query,
		OrganicResults:   results,
		Features:         features,
		RelatedQuestions: relatedQuestions,
		HasAIOverview:    hasAIOverview,
		TotalResults:     totalResults,
		SearchTime:       searchInfo.TimeTakenDisplay,
	}, nil
}

// VerifyKey checks whether the configured API key is valid by calling the
// SerpAPI account endpoint and inspecting the response.
func (a *Adapter) VerifyKey() error {
	params := url.Values{"api_key": {a.apiKey}}
	httpReq, err := http.NewRequest("GET", a.accountURL+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("creating serpapi account request: %w", err)
	}

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("serpapi account request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("serpapi: invalid API key")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("serpapi account endpoint returned status %d", resp.StatusCode)
	}

	var acct struct {
		AccountEmail string `json:"account_email"`
		PlanName     string `json:"plan_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&acct); err != nil {
		return fmt.Errorf("serpapi: malformed account response: %w", err)
	}

	if acct.AccountEmail == "" {
		return fmt.Errorf("serpapi: invalid API key (no account email in response)")
	}

	return nil
}
