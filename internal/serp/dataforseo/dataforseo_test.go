package dataforseo

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	dfs "github.com/supah-seo/supah-seo/internal/dataforseo"
	"github.com/supah-seo/supah-seo/internal/serp"
)

func newAnalyzeAdapter(t *testing.T, response any, assertRequest func(t *testing.T, r *http.Request, body []byte)) *Adapter {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if assertRequest != nil {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			assertRequest(t, r, body)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	return New("login", "password", dfs.WithBaseURL(srv.URL), dfs.WithHTTPClient(srv.Client()))
}

func baseAnalyzeResponse(items []map[string]any) map[string]any {
	return map[string]any{
		"status_code":    20000,
		"status_message": "Ok.",
		"tasks": []map[string]any{
			{
				"status_code":    20000,
				"status_message": "Ok.",
				"result": []map[string]any{
					{
						"keyword":          "test query",
						"se_results_count": 1000,
						"items":            items,
					},
				},
			},
		},
	}
}

func featureByType(features []serp.SERPFeature, ft serp.SERPFeatureType) *serp.SERPFeature {
	for i := range features {
		if features[i].Type == ft {
			return &features[i]
		}
	}
	return nil
}

func TestAnalyze_OrganicResults(t *testing.T) {
	respJSON := baseAnalyzeResponse([]map[string]any{
		{"type": "organic", "rank_group": 1, "rank_absolute": 1, "title": "First", "url": "https://one.example.com/a", "description": "First snippet", "domain": "one.example.com"},
		{"type": "organic", "rank_group": 2, "rank_absolute": 2, "title": "Second", "url": "https://two.example.com/b", "description": "Second snippet", "domain": "two.example.com"},
		{"type": "organic", "rank_group": 3, "rank_absolute": 3, "title": "Third", "url": "https://three.example.com/c", "description": "Third snippet", "domain": "three.example.com"},
	})

	a := newAnalyzeAdapter(t, respJSON, nil)
	resp, err := a.Analyze(serp.AnalyzeRequest{Query: "test query"})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	if len(resp.OrganicResults) != 3 {
		t.Fatalf("expected 3 organic results, got %d", len(resp.OrganicResults))
	}

	want := []struct {
		position int
		title    string
		link     string
		domain   string
	}{
		{1, "First", "https://one.example.com/a", "one.example.com"},
		{2, "Second", "https://two.example.com/b", "two.example.com"},
		{3, "Third", "https://three.example.com/c", "three.example.com"},
	}
	for i, w := range want {
		got := resp.OrganicResults[i]
		if got.Position != w.position || got.Title != w.title || got.Link != w.link || got.Domain != w.domain {
			t.Fatalf("organic result %d mismatch: got %+v", i, got)
		}
	}
}

func TestAnalyze_FeaturedSnippet(t *testing.T) {
	respJSON := baseAnalyzeResponse([]map[string]any{
		{
			"type":          "featured_snippet",
			"rank_group":    1,
			"rank_absolute": 1,
			"title":         "Container title",
			"url":           "https://container.example.com",
			"description":   "Container description",
			"domain":        "container.example.com",
			"items": []map[string]any{
				{"type": "featured_snippet_element", "title": "Sub title", "description": "Sub snippet", "url": "https://sub.example.com/snippet"},
			},
		},
	})

	a := newAnalyzeAdapter(t, respJSON, nil)
	resp, err := a.Analyze(serp.AnalyzeRequest{Query: "test query"})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	fs := featureByType(resp.Features, serp.FeatureFeaturedSnippet)
	if fs == nil {
		t.Fatal("expected featured_snippet feature")
	}
	if fs.URL != "https://sub.example.com/snippet" {
		t.Fatalf("expected featured snippet URL from sub-item, got %q", fs.URL)
	}
	if fs.Snippet != "Sub snippet" {
		t.Fatalf("expected featured snippet text from sub-item, got %q", fs.Snippet)
	}
}

func TestAnalyze_PeopleAlsoAsk(t *testing.T) {
	respJSON := baseAnalyzeResponse([]map[string]any{
		{
			"type":          "people_also_ask",
			"rank_group":    3,
			"rank_absolute": 5,
			"items": []map[string]any{
				{"type": "people_also_ask_element", "title": "What is X?", "description": "X is...", "url": "https://example.com/x", "domain": "example.com"},
				{"type": "people_also_ask_element", "title": "How does X work?", "description": "By doing...", "url": "https://example.com/how", "domain": "example.com"},
				{"type": "people_also_ask_element", "title": "Why use X?", "description": "Because...", "url": "https://example.com/why", "domain": "example.com"},
			},
		},
	})

	a := newAnalyzeAdapter(t, respJSON, nil)
	resp, err := a.Analyze(serp.AnalyzeRequest{Query: "test query"})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	if len(resp.RelatedQuestions) != 3 {
		t.Fatalf("expected 3 related questions, got %d", len(resp.RelatedQuestions))
	}

	wantQuestions := []string{"What is X?", "How does X work?", "Why use X?"}
	for i, q := range wantQuestions {
		if resp.RelatedQuestions[i].Question != q {
			t.Fatalf("related question %d: expected %q, got %q", i, q, resp.RelatedQuestions[i].Question)
		}
		if resp.RelatedQuestions[i].Question == "" {
			t.Fatalf("related question %d should not be empty", i)
		}
	}
}

func TestAnalyze_AIOverview(t *testing.T) {
	respJSON := baseAnalyzeResponse([]map[string]any{{"type": "ai_overview", "rank_group": 1, "rank_absolute": 1, "title": "AI"}})
	a := newAnalyzeAdapter(t, respJSON, nil)

	resp, err := a.Analyze(serp.AnalyzeRequest{Query: "test query"})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if !resp.HasAIOverview {
		t.Fatal("expected HasAIOverview=true")
	}
}

func TestAnalyze_NoAIOverview(t *testing.T) {
	respJSON := baseAnalyzeResponse([]map[string]any{{"type": "organic", "rank_group": 1, "rank_absolute": 1, "title": "Result", "url": "https://example.com", "description": "Snippet"}})
	a := newAnalyzeAdapter(t, respJSON, nil)

	resp, err := a.Analyze(serp.AnalyzeRequest{Query: "test query"})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if resp.HasAIOverview {
		t.Fatal("expected HasAIOverview=false")
	}
}

func TestAnalyze_DefaultLocation(t *testing.T) {
	respJSON := baseAnalyzeResponse([]map[string]any{})
	a := newAnalyzeAdapter(t, respJSON, func(t *testing.T, r *http.Request, body []byte) {
		t.Helper()
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != serpEndpoint {
			t.Fatalf("expected path %s, got %s", serpEndpoint, r.URL.Path)
		}

		var tasks []struct {
			LocationCode int    `json:"location_code"`
			LocationName string `json:"location_name"`
		}
		if err := json.Unmarshal(body, &tasks); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		if tasks[0].LocationCode != defaultLocationCode {
			t.Fatalf("expected location_code=%d, got %d", defaultLocationCode, tasks[0].LocationCode)
		}
		if tasks[0].LocationName != "" {
			t.Fatalf("expected empty location_name, got %q", tasks[0].LocationName)
		}
	})

	_, err := a.Analyze(serp.AnalyzeRequest{Query: "test query"})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
}

func TestAnalyze_MixedFeatures(t *testing.T) {
	respJSON := baseAnalyzeResponse([]map[string]any{
		{"type": "organic", "rank_group": 1, "rank_absolute": 1, "title": "Result", "url": "https://example.com", "description": "Snippet", "domain": "example.com"},
		{"type": "featured_snippet", "rank_group": 1, "rank_absolute": 1, "title": "FS", "url": "https://fs.example.com", "description": "FS snippet", "domain": "fs.example.com"},
		{"type": "people_also_ask", "rank_group": 2, "rank_absolute": 3, "items": []map[string]any{{"type": "people_also_ask_element", "title": "Question 1", "description": "Answer 1", "url": "https://example.com/q1"}, {"type": "people_also_ask_element", "title": "Question 2", "description": "Answer 2", "url": "https://example.com/q2"}}},
		{"type": "ai_overview", "rank_group": 1, "rank_absolute": 0, "title": "AI Overview"},
		{"type": "video", "rank_group": 4, "rank_absolute": 9, "title": "Video block"},
	})

	a := newAnalyzeAdapter(t, respJSON, nil)
	resp, err := a.Analyze(serp.AnalyzeRequest{Query: "test query"})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	if len(resp.OrganicResults) != 1 {
		t.Fatalf("expected 1 organic result, got %d", len(resp.OrganicResults))
	}
	if len(resp.RelatedQuestions) != 2 {
		t.Fatalf("expected 2 related questions, got %d", len(resp.RelatedQuestions))
	}
	if !resp.HasAIOverview {
		t.Fatal("expected HasAIOverview=true")
	}

	if len(resp.Features) != 4 {
		t.Fatalf("expected 4 features, got %d: %+v", len(resp.Features), resp.Features)
	}

	expected := map[serp.SERPFeatureType]bool{
		serp.FeatureFeaturedSnippet: true,
		serp.FeaturePeopleAlsoAsk:   true,
		serp.FeatureAIOverview:      true,
		serp.FeatureInlineVideos:    true,
	}
	for _, f := range resp.Features {
		if !expected[f.Type] {
			t.Fatalf("unexpected feature type: %s", f.Type)
		}
		delete(expected, f.Type)
	}
	if len(expected) != 0 {
		t.Fatalf("missing expected features: %+v", expected)
	}
}

func TestParseTaskGetResponse(t *testing.T) {
	raw := []byte(`{
		"status_code": 20000,
		"status_message": "Ok.",
		"tasks": [{
			"status_code": 20000,
			"status_message": "Ok.",
			"result": [{
				"keyword": "batch query",
				"se_results_count": 500,
				"items": [
					{"type": "organic", "rank_group": 1, "rank_absolute": 1, "title": "First", "url": "https://one.example.com/a", "description": "First snippet", "domain": "one.example.com"},
					{"type": "featured_snippet", "rank_group": 1, "rank_absolute": 1, "title": "FS container", "url": "https://container.example.com", "description": "container", "domain": "container.example.com", "items": [{"type": "featured_snippet_element", "title": "FS sub", "description": "FS sub snippet", "url": "https://sub.example.com/fs"}]},
					{"type": "people_also_ask", "rank_group": 3, "rank_absolute": 4, "items": [
						{"type": "people_also_ask_element", "title": "What is X?", "description": "X is...", "url": "https://example.com/x", "domain": "example.com"},
						{"type": "people_also_ask_element", "title": "How does X work?", "description": "It works...", "url": "https://example.com/how", "domain": "example.com"}
					]}
				]
			}]
		}]
	}`)

	resp, err := parseTaskGetResponse(raw, "fallback query")
	if err != nil {
		t.Fatalf("parseTaskGetResponse failed: %v", err)
	}

	if resp.Query != "batch query" {
		t.Fatalf("expected parsed query 'batch query', got %q", resp.Query)
	}
	if len(resp.OrganicResults) != 1 {
		t.Fatalf("expected 1 organic result, got %d", len(resp.OrganicResults))
	}
	if resp.OrganicResults[0].Domain != "one.example.com" {
		t.Fatalf("expected domain one.example.com, got %q", resp.OrganicResults[0].Domain)
	}

	if len(resp.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(resp.Features))
	}
	if featureByType(resp.Features, serp.FeatureFeaturedSnippet) == nil {
		t.Fatal("expected featured snippet feature")
	}
	if featureByType(resp.Features, serp.FeaturePeopleAlsoAsk) == nil {
		t.Fatal("expected people also ask feature")
	}

	if len(resp.RelatedQuestions) != 2 {
		t.Fatalf("expected 2 related questions, got %d", len(resp.RelatedQuestions))
	}
	if resp.RelatedQuestions[0].Question != "What is X?" || resp.RelatedQuestions[1].Question != "How does X work?" {
		t.Fatalf("unexpected related questions: %+v", resp.RelatedQuestions)
	}
}
