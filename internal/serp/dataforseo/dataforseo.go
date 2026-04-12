package dataforseo

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/supah-seo/supah-seo/internal/common/cost"
	dfs "github.com/supah-seo/supah-seo/internal/dataforseo"
	"github.com/supah-seo/supah-seo/internal/serp"
)

const (
	serpEndpoint    = "/v3/serp/google/organic/live/advanced"
	costPerQueryUSD = 0.002 // live mode
	costBasis       = "dataforseo: $0.002/query (live/advanced, default AU/en)"
)

// Adapter implements serp.Provider using the DataForSEO SERP API.
type Adapter struct {
	client *dfs.Client
}

// New creates a DataForSEO SERP adapter.
func New(login, password string, opts ...dfs.Option) *Adapter {
	return &Adapter{client: dfs.New(login, password, opts...)}
}

// Name returns the provider identifier.
func (a *Adapter) Name() string { return "dataforseo" }

// Estimate returns a cost estimate without executing the search.
func (a *Adapter) Estimate(req serp.AnalyzeRequest) (cost.Estimate, error) {
	return cost.BuildEstimate(cost.EstimateInput{
		UnitCostUSD: costPerQueryUSD,
		Units:       1,
		Basis:       costBasis,
	})
}

// defaultLocationCode is the DataForSEO location code for Australia.
// DataForSEO requires at least one of location_code/location_name to be set.
const defaultLocationCode = 2036

// defaultLanguageCode is the DataForSEO language code for English.
// DataForSEO requires at least one of language_code/language_name to be set.
const defaultLanguageCode = "en"

// Analyze executes a SERP query against the DataForSEO live endpoint.
func (a *Adapter) Analyze(req serp.AnalyzeRequest) (*serp.AnalyzeResponse, error) {
	task := map[string]any{
		"keyword": req.Query,
	}
	if req.Location != "" {
		task["location_name"] = req.Location
	} else {
		task["location_code"] = defaultLocationCode // Australia
	}
	if req.Language != "" {
		task["language_code"] = req.Language
	} else {
		task["language_code"] = defaultLanguageCode
	}
	if req.NumResults > 0 {
		task["depth"] = req.NumResults
	}

	raw, err := a.client.Post(serpEndpoint, []map[string]any{task})
	if err != nil {
		return nil, fmt.Errorf("dataforseo serp request: %w", err)
	}

	var envelope struct {
		StatusCode    int    `json:"status_code"`
		StatusMessage string `json:"status_message"`
		Tasks         []struct {
			StatusCode    int    `json:"status_code"`
			StatusMessage string `json:"status_message"`
			Result        []struct {
				Keyword    string `json:"keyword"`
				TotalCount int64  `json:"se_results_count"`
				Items      []struct {
					Type         string          `json:"type"`
					RankGroup    int             `json:"rank_group"`
					RankAbsolute int             `json:"rank_absolute"`
					Title        string          `json:"title"`
					URL          string          `json:"url"`
					Description  string          `json:"description"`
					Domain       string          `json:"domain"`
					SubItems     json.RawMessage `json:"items,omitempty"`
				} `json:"items"`
			} `json:"result"`
		} `json:"tasks"`
	}

	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decoding dataforseo response: %w", err)
	}

	if envelope.StatusCode != 20000 {
		return nil, fmt.Errorf("dataforseo error %d: %s", envelope.StatusCode, envelope.StatusMessage)
	}
	if len(envelope.Tasks) == 0 {
		return nil, fmt.Errorf("dataforseo returned no tasks")
	}
	task0 := envelope.Tasks[0]
	if task0.StatusCode != 20000 {
		return nil, fmt.Errorf("dataforseo task error %d: %s", task0.StatusCode, task0.StatusMessage)
	}
	if len(task0.Result) == 0 {
		return &serp.AnalyzeResponse{Query: req.Query}, nil
	}

	result := task0.Result[0]
	organic := make([]serp.OrganicResult, 0)
	features := make([]serp.SERPFeature, 0)
	relatedQuestions := make([]serp.RelatedQuestion, 0)
	hasAIOverview := false

	for _, item := range result.Items {
		if item.Type == "organic" {
			parsed, _ := url.Parse(item.URL)
			domain := ""
			if parsed != nil {
				domain = parsed.Hostname()
			}
			organic = append(organic, serp.OrganicResult{
				Position: item.RankGroup,
				Title:    item.Title,
				Link:     item.URL,
				Snippet:  item.Description,
				Domain:   domain,
			})
			continue
		}

		if item.Type == "ai_overview" {
			hasAIOverview = true
		}

		if item.Type == "people_also_ask" && len(item.SubItems) > 0 {
			var subItems []struct {
				Type        string `json:"type"`
				Title       string `json:"title"`
				Description string `json:"description"`
				URL         string `json:"url"`
				Domain      string `json:"domain"`
			}
			if err := json.Unmarshal(item.SubItems, &subItems); err == nil {
				for _, sub := range subItems {
					if sub.Title != "" {
						relatedQuestions = append(relatedQuestions, serp.RelatedQuestion{
							Question: sub.Title,
							Snippet:  sub.Description,
							Title:    sub.Title,
							Link:     sub.URL,
						})
					}
				}
			}
		}

		ft, ok := mapDFSTypeToFeature(item.Type)
		if !ok {
			continue
		}

		// For featured_snippet, prefer richer data from the first sub-item when available.
		featureURL := item.URL
		featureSnippet := item.Description
		featureTitle := item.Title
		if item.Type == "featured_snippet" && len(item.SubItems) > 0 {
			var subItems []struct {
				Type        string `json:"type"`
				Title       string `json:"title"`
				Description string `json:"description"`
				URL         string `json:"url"`
			}
			if err := json.Unmarshal(item.SubItems, &subItems); err == nil && len(subItems) > 0 {
				sub := subItems[0]
				if sub.URL != "" {
					featureURL = sub.URL
				}
				if sub.Description != "" {
					featureSnippet = sub.Description
				}
				if sub.Title != "" {
					featureTitle = sub.Title
				}
			}
		}

		features = append(features, serp.SERPFeature{
			Type:     ft,
			Position: item.RankAbsolute,
			Title:    featureTitle,
			URL:      featureURL,
			Domain:   item.Domain,
			Snippet:  featureSnippet,
		})
	}

	return &serp.AnalyzeResponse{
		Query:            req.Query,
		OrganicResults:   organic,
		Features:         features,
		RelatedQuestions: relatedQuestions,
		HasAIOverview:    hasAIOverview,
		TotalResults:     result.TotalCount,
	}, nil
}

const (
	taskPostEndpoint   = "/v3/serp/google/organic/task_post"
	tasksReadyEndpoint = "/v3/serp/google/organic/tasks_ready"
	taskGetEndpoint    = "/v3/serp/google/organic/task_get/advanced/"
	batchCostPerQuery  = 0.0006 // standard queue pricing
)

// BatchEstimate returns a cost estimate for a batch of keywords using the Standard method.
func (a *Adapter) BatchEstimate(count int) (cost.Estimate, error) {
	return cost.BuildEstimate(cost.EstimateInput{
		UnitCostUSD: batchCostPerQuery,
		Units:       count,
		Basis:       fmt.Sprintf("dataforseo: %d queries @ $0.0006/query (standard queue)", count),
	})
}

// AnalyzeBatch submits multiple keywords via the Standard (POST-GET) method.
// This is cheaper ($0.0006/keyword vs $0.002 live) and supports up to 100 keywords per call.
// It posts all tasks, polls for completion (up to 60 seconds), and returns results.
func (a *Adapter) AnalyzeBatch(requests []serp.AnalyzeRequest) ([]*serp.AnalyzeResponse, error) {
	// Build task objects.
	tasks := make([]map[string]any, len(requests))
	for i, req := range requests {
		task := map[string]any{
			"keyword": req.Query,
		}
		if req.Location != "" {
			task["location_name"] = req.Location
		} else {
			task["location_code"] = defaultLocationCode // Australia
		}
		if req.Language != "" {
			task["language_code"] = req.Language
		} else {
			task["language_code"] = defaultLanguageCode
		}
		if req.NumResults > 0 {
			task["depth"] = req.NumResults
		}
		tasks[i] = task
	}

	// POST all tasks.
	raw, err := a.client.Post(taskPostEndpoint, tasks)
	if err != nil {
		return nil, fmt.Errorf("dataforseo batch task_post: %w", err)
	}

	var postEnvelope struct {
		StatusCode    int    `json:"status_code"`
		StatusMessage string `json:"status_message"`
		Tasks         []struct {
			ID            string `json:"id"`
			StatusCode    int    `json:"status_code"`
			StatusMessage string `json:"status_message"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(raw, &postEnvelope); err != nil {
		return nil, fmt.Errorf("decoding task_post response: %w", err)
	}
	if postEnvelope.StatusCode != 20000 {
		return nil, fmt.Errorf("dataforseo task_post error %d: %s", postEnvelope.StatusCode, postEnvelope.StatusMessage)
	}

	// Collect submitted task IDs, mapping id → original request.
	submittedIDs := make(map[string]serp.AnalyzeRequest, len(postEnvelope.Tasks))
	for i, t := range postEnvelope.Tasks {
		if t.StatusCode == 20100 && t.ID != "" && i < len(requests) {
			submittedIDs[t.ID] = requests[i]
		}
	}
	if len(submittedIDs) == 0 {
		return nil, fmt.Errorf("dataforseo task_post: no tasks were accepted")
	}

	// Poll tasks_ready until all done or timeout.
	deadline := time.Now().Add(60 * time.Second)
	readyResults := make(map[string]*serp.AnalyzeResponse)

	for time.Now().Before(deadline) && len(readyResults) < len(submittedIDs) {
		time.Sleep(5 * time.Second)

		readyRaw, err := a.client.Get(tasksReadyEndpoint)
		if err != nil {
			return nil, fmt.Errorf("dataforseo tasks_ready: %w", err)
		}

		var readyEnvelope struct {
			StatusCode    int    `json:"status_code"`
			StatusMessage string `json:"status_message"`
			Tasks         []struct {
				StatusCode int `json:"status_code"`
				Result     []struct {
					ID string `json:"id"`
				} `json:"result"`
			} `json:"tasks"`
		}
		if err := json.Unmarshal(readyRaw, &readyEnvelope); err != nil {
			return nil, fmt.Errorf("decoding tasks_ready response: %w", err)
		}

		for _, rt := range readyEnvelope.Tasks {
			for _, r := range rt.Result {
				if _, ok := submittedIDs[r.ID]; !ok {
					continue // not our task
				}
				if _, done := readyResults[r.ID]; done {
					continue // already fetched
				}

				// Fetch the task result.
				getRaw, err := a.client.Get(taskGetEndpoint + r.ID)
				if err != nil {
					return nil, fmt.Errorf("dataforseo task_get %s: %w", r.ID, err)
				}

				resp, err := parseTaskGetResponse(getRaw, submittedIDs[r.ID].Query)
				if err != nil {
					return nil, fmt.Errorf("parsing task_get response for %s: %w", r.ID, err)
				}
				readyResults[r.ID] = resp
			}
		}
	}

	// Check for timed-out tasks.
	var timedOut []string
	for id, origReq := range submittedIDs {
		if _, ok := readyResults[id]; !ok {
			timedOut = append(timedOut, origReq.Query)
		}
	}

	// Assemble results in original request order.
	responses := make([]*serp.AnalyzeResponse, len(requests))
	for i, req := range requests {
		for id, origReq := range submittedIDs {
			if origReq.Query == req.Query {
				if resp, ok := readyResults[id]; ok {
					responses[i] = resp
				}
				break
			}
		}
		if responses[i] == nil {
			responses[i] = &serp.AnalyzeResponse{Query: req.Query}
		}
	}

	if len(timedOut) > 0 {
		return responses, fmt.Errorf("%d of %d batch tasks timed out after 60s (queries: %s) — partial results returned",
			len(timedOut), len(submittedIDs), strings.Join(timedOut, ", "))
	}

	return responses, nil
}

// parseTaskGetResponse parses the JSON returned by task_get/advanced into an AnalyzeResponse.
func parseTaskGetResponse(raw []byte, query string) (*serp.AnalyzeResponse, error) {
	var envelope struct {
		StatusCode    int    `json:"status_code"`
		StatusMessage string `json:"status_message"`
		Tasks         []struct {
			StatusCode    int    `json:"status_code"`
			StatusMessage string `json:"status_message"`
			Result        []struct {
				Keyword    string `json:"keyword"`
				TotalCount int64  `json:"se_results_count"`
				Items      []struct {
					Type         string          `json:"type"`
					RankGroup    int             `json:"rank_group"`
					RankAbsolute int             `json:"rank_absolute"`
					Title        string          `json:"title"`
					URL          string          `json:"url"`
					Description  string          `json:"description"`
					Domain       string          `json:"domain"`
					SubItems     json.RawMessage `json:"items,omitempty"`
				} `json:"items"`
			} `json:"result"`
		} `json:"tasks"`
	}

	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decoding task_get response: %w", err)
	}
	if envelope.StatusCode != 20000 {
		return nil, fmt.Errorf("dataforseo error %d: %s", envelope.StatusCode, envelope.StatusMessage)
	}
	if len(envelope.Tasks) == 0 {
		return &serp.AnalyzeResponse{Query: query}, nil
	}

	task0 := envelope.Tasks[0]
	if task0.StatusCode != 20000 {
		return nil, fmt.Errorf("dataforseo task error %d: %s", task0.StatusCode, task0.StatusMessage)
	}
	if len(task0.Result) == 0 {
		return &serp.AnalyzeResponse{Query: query}, nil
	}

	result := task0.Result[0]
	resolvedQuery := result.Keyword
	if resolvedQuery == "" {
		resolvedQuery = query
	}

	organic := make([]serp.OrganicResult, 0)
	features := make([]serp.SERPFeature, 0)
	relatedQuestions := make([]serp.RelatedQuestion, 0)
	hasAIOverview := false

	for _, item := range result.Items {
		if item.Type == "organic" {
			parsed, _ := url.Parse(item.URL)
			domain := ""
			if parsed != nil {
				domain = parsed.Hostname()
			}
			organic = append(organic, serp.OrganicResult{
				Position: item.RankGroup,
				Title:    item.Title,
				Link:     item.URL,
				Snippet:  item.Description,
				Domain:   domain,
			})
			continue
		}

		if item.Type == "ai_overview" {
			hasAIOverview = true
		}

		if item.Type == "people_also_ask" && len(item.SubItems) > 0 {
			var subItems []struct {
				Type        string `json:"type"`
				Title       string `json:"title"`
				Description string `json:"description"`
				URL         string `json:"url"`
				Domain      string `json:"domain"`
			}
			if err := json.Unmarshal(item.SubItems, &subItems); err == nil {
				for _, sub := range subItems {
					if sub.Title != "" {
						relatedQuestions = append(relatedQuestions, serp.RelatedQuestion{
							Question: sub.Title,
							Snippet:  sub.Description,
							Title:    sub.Title,
							Link:     sub.URL,
						})
					}
				}
			}
		}

		ft, ok := mapDFSTypeToFeature(item.Type)
		if !ok {
			continue
		}

		featureURL := item.URL
		featureSnippet := item.Description
		featureTitle := item.Title
		if item.Type == "featured_snippet" && len(item.SubItems) > 0 {
			var subItems []struct {
				Type        string `json:"type"`
				Title       string `json:"title"`
				Description string `json:"description"`
				URL         string `json:"url"`
			}
			if err := json.Unmarshal(item.SubItems, &subItems); err == nil && len(subItems) > 0 {
				sub := subItems[0]
				if sub.URL != "" {
					featureURL = sub.URL
				}
				if sub.Description != "" {
					featureSnippet = sub.Description
				}
				if sub.Title != "" {
					featureTitle = sub.Title
				}
			}
		}

		features = append(features, serp.SERPFeature{
			Type:     ft,
			Position: item.RankAbsolute,
			Title:    featureTitle,
			URL:      featureURL,
			Domain:   item.Domain,
			Snippet:  featureSnippet,
		})
	}

	return &serp.AnalyzeResponse{
		Query:            resolvedQuery,
		OrganicResults:   organic,
		Features:         features,
		RelatedQuestions: relatedQuestions,
		HasAIOverview:    hasAIOverview,
		TotalResults:     result.TotalCount,
	}, nil
}

// mapDFSTypeToFeature maps a DataForSEO item type string to a serp.SERPFeatureType.
// Returns false for unrecognized types.
func mapDFSTypeToFeature(dfsType string) (serp.SERPFeatureType, bool) {
	switch dfsType {
	case "featured_snippet":
		return serp.FeatureFeaturedSnippet, true
	case "people_also_ask":
		return serp.FeaturePeopleAlsoAsk, true
	case "local_pack":
		return serp.FeatureLocalPack, true
	case "knowledge_graph":
		return serp.FeatureKnowledgeGraph, true
	case "ai_overview":
		return serp.FeatureAIOverview, true
	case "top_stories":
		return serp.FeatureTopStories, true
	case "video":
		return serp.FeatureInlineVideos, true
	case "shopping":
		return serp.FeatureInlineShopping, true
	case "images":
		return serp.FeatureInlineImages, true
	default:
		return "", false
	}
}
