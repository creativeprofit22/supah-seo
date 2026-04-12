package serpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/supah-seo/supah-seo/internal/serp"
)

func TestEstimate(t *testing.T) {
	a := New("test-key")
	est, err := a.Estimate(serp.AnalyzeRequest{Query: "test"})
	if err != nil {
		t.Fatalf("estimate failed: %v", err)
	}
	if est.Amount != 0.01 {
		t.Fatalf("expected 0.01, got %v", est.Amount)
	}
	if est.Currency != "USD" {
		t.Fatalf("expected USD, got %s", est.Currency)
	}
}

func TestAnalyze(t *testing.T) {
	mockResp := map[string]any{
		"organic_results": []map[string]any{
			{
				"position": 1,
				"title":    "Test Result",
				"link":     "https://example.com/page",
				"snippet":  "A test snippet",
			},
			{
				"position": 2,
				"title":    "Another Result",
				"link":     "https://other.com/page",
				"snippet":  "Another snippet",
			},
		},
		"search_information": map[string]any{
			"total_results":        "12345",
			"time_taken_displayed": 0.42,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "test-key" {
			t.Error("expected api_key=test-key")
		}
		if r.URL.Query().Get("q") != "seo tools" {
			t.Error("expected q=seo tools")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResp)
	}))
	defer srv.Close()

	a := New("test-key", WithBaseURL(srv.URL))
	resp, err := a.Analyze(serp.AnalyzeRequest{Query: "seo tools"})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	if resp.Query != "seo tools" {
		t.Fatalf("expected query 'seo tools', got %q", resp.Query)
	}
	if len(resp.OrganicResults) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.OrganicResults))
	}
	if resp.OrganicResults[0].Domain != "example.com" {
		t.Fatalf("expected domain 'example.com', got %q", resp.OrganicResults[0].Domain)
	}
	if resp.TotalResults != 12345 {
		t.Fatalf("expected 12345 total results, got %d", resp.TotalResults)
	}
}

func TestAnalyzeSERPFeatures(t *testing.T) {
	mockResp := map[string]any{
		"organic_results": []map[string]any{
			{"position": 1, "title": "Result 1", "link": "https://example.com", "snippet": "Snippet 1"},
		},
		"search_information": map[string]any{
			"total_results":        "100",
			"time_taken_displayed": 0.5,
		},
		"answer_box": map[string]any{
			"type":    "organic_result",
			"title":   "Featured Title",
			"snippet": "Featured snippet text",
			"link":    "https://featured.com/page",
		},
		"related_questions": []map[string]any{
			{"question": "What is SEO?", "snippet": "SEO is...", "title": "SEO Guide", "link": "https://seo.com"},
			{"question": "How does SEO work?", "snippet": "It works by...", "title": "SEO 101", "link": "https://seo101.com"},
		},
		"local_results": map[string]any{
			"places": []map[string]any{
				{"title": "Local Business", "address": "123 Main St"},
			},
		},
		"knowledge_graph": map[string]any{
			"title":       "Knowledge Title",
			"description": "Knowledge description",
			"source":      "Wikipedia",
		},
		"ai_overview":      map[string]any{"text": "AI generated overview"},
		"top_stories":      []map[string]any{{"title": "Story"}},
		"inline_videos":    []map[string]any{{"title": "Video"}},
		"inline_images":    []map[string]any{{"thumbnail": "img.jpg"}},
		"shopping_results": []map[string]any{{"title": "Product", "price": "$10"}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResp)
	}))
	defer srv.Close()

	a := New("test-key", WithBaseURL(srv.URL))
	resp, err := a.Analyze(serp.AnalyzeRequest{Query: "test"})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}

	// Verify organic results still work.
	if len(resp.OrganicResults) != 1 {
		t.Fatalf("expected 1 organic result, got %d", len(resp.OrganicResults))
	}

	// Verify HasAIOverview.
	if !resp.HasAIOverview {
		t.Fatal("expected HasAIOverview to be true")
	}

	// Verify related questions.
	if len(resp.RelatedQuestions) != 2 {
		t.Fatalf("expected 2 related questions, got %d", len(resp.RelatedQuestions))
	}
	if resp.RelatedQuestions[0].Question != "What is SEO?" {
		t.Fatalf("expected first question 'What is SEO?', got %q", resp.RelatedQuestions[0].Question)
	}

	// Verify features — expect 9 total:
	// featured_snippet, people_also_ask, local_pack, knowledge_graph, ai_overview,
	// top_stories, inline_videos, inline_images, inline_shopping
	expectedFeatures := map[serp.SERPFeatureType]bool{
		serp.FeatureFeaturedSnippet: true,
		serp.FeaturePeopleAlsoAsk:   true,
		serp.FeatureLocalPack:       true,
		serp.FeatureKnowledgeGraph:  true,
		serp.FeatureAIOverview:      true,
		serp.FeatureTopStories:      true,
		serp.FeatureInlineVideos:    true,
		serp.FeatureInlineImages:    true,
		serp.FeatureInlineShopping:  true,
	}
	if len(resp.Features) != len(expectedFeatures) {
		t.Fatalf("expected %d features, got %d: %+v", len(expectedFeatures), len(resp.Features), resp.Features)
	}
	for _, f := range resp.Features {
		if !expectedFeatures[f.Type] {
			t.Errorf("unexpected feature type: %s", f.Type)
		}
		delete(expectedFeatures, f.Type)
	}
	if len(expectedFeatures) > 0 {
		t.Errorf("missing features: %+v", expectedFeatures)
	}

	// Verify featured snippet details.
	fs := resp.Features[0]
	if fs.Type != serp.FeatureFeaturedSnippet {
		t.Fatalf("expected first feature to be featured_snippet, got %s", fs.Type)
	}
	if fs.Title != "Featured Title" {
		t.Fatalf("expected featured snippet title 'Featured Title', got %q", fs.Title)
	}
	if fs.URL != "https://featured.com/page" {
		t.Fatalf("expected featured snippet URL, got %q", fs.URL)
	}
	if fs.Snippet != "Featured snippet text" {
		t.Fatalf("expected featured snippet text, got %q", fs.Snippet)
	}

	// Verify knowledge graph details.
	var kgFeature *serp.SERPFeature
	for i := range resp.Features {
		if resp.Features[i].Type == serp.FeatureKnowledgeGraph {
			kgFeature = &resp.Features[i]
			break
		}
	}
	if kgFeature == nil {
		t.Fatal("knowledge_graph feature not found")
	}
	if kgFeature.Title != "Knowledge Title" {
		t.Fatalf("expected KG title 'Knowledge Title', got %q", kgFeature.Title)
	}
	if kgFeature.Snippet != "Knowledge description" {
		t.Fatalf("expected KG snippet 'Knowledge description', got %q", kgFeature.Snippet)
	}
}

func TestAnalyzeNoFeatures(t *testing.T) {
	mockResp := map[string]any{
		"organic_results": []map[string]any{
			{"position": 1, "title": "Only Result", "link": "https://example.com", "snippet": "Only snippet"},
		},
		"search_information": map[string]any{
			"total_results": "1", "time_taken_displayed": 0.1,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResp)
	}))
	defer srv.Close()

	a := New("test-key", WithBaseURL(srv.URL))
	resp, err := a.Analyze(serp.AnalyzeRequest{Query: "plain"})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if len(resp.Features) != 0 {
		t.Fatalf("expected 0 features, got %d", len(resp.Features))
	}
	if resp.HasAIOverview {
		t.Fatal("expected HasAIOverview to be false")
	}
	if len(resp.RelatedQuestions) != 0 {
		t.Fatalf("expected 0 related questions, got %d", len(resp.RelatedQuestions))
	}
}

func TestAnalyzeDefaultLocationAndLocale(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("location"); got != "Perth,Western Australia,Australia" {
			t.Errorf("expected default location 'Perth,Western Australia,Australia', got %q", got)
		}
		if got := q.Get("hl"); got != "en" {
			t.Errorf("expected default hl 'en', got %q", got)
		}
		if got := q.Get("gl"); got != "au" {
			t.Errorf("expected gl 'au', got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"organic_results":    []map[string]any{},
			"search_information": map[string]any{"total_results": "0", "time_taken_displayed": 0.1},
		})
	}))
	defer srv.Close()

	a := New("test-key", WithBaseURL(srv.URL))
	// No Location or Language set — should use defaults.
	_, err := a.Analyze(serp.AnalyzeRequest{Query: "default locale test"})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
}

func TestAnalyzeCustomLocationOverridesDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("location"); got != "Sydney,New South Wales,Australia" {
			t.Errorf("expected custom location, got %q", got)
		}
		if got := q.Get("hl"); got != "en-AU" {
			t.Errorf("expected custom hl 'en-AU', got %q", got)
		}
		if got := q.Get("gl"); got != "au" {
			t.Errorf("expected gl 'au', got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"organic_results":    []map[string]any{},
			"search_information": map[string]any{"total_results": "0", "time_taken_displayed": 0.1},
		})
	}))
	defer srv.Close()

	a := New("test-key", WithBaseURL(srv.URL))
	_, err := a.Analyze(serp.AnalyzeRequest{
		Query:    "custom locale test",
		Location: "Sydney,New South Wales,Australia",
		Language: "en-AU",
	})
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
}

func TestAnalyzeServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := New("test-key", WithBaseURL(srv.URL))
	_, err := a.Analyze(serp.AnalyzeRequest{Query: "fail"})
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestDryRunNoNetworkCall(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer srv.Close()

	a := New("test-key", WithBaseURL(srv.URL))

	// Estimate does not make any HTTP calls
	_, err := a.Estimate(serp.AnalyzeRequest{Query: "dry run test"})
	if err != nil {
		t.Fatalf("estimate failed: %v", err)
	}

	if callCount != 0 {
		t.Fatalf("expected 0 HTTP calls during estimate, got %d", callCount)
	}
}

func TestName(t *testing.T) {
	a := New("key")
	if a.Name() != "serpapi" {
		t.Fatalf("expected 'serpapi', got %q", a.Name())
	}
}

func TestVerifyKeySuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "valid-key" {
			t.Errorf("expected api_key=valid-key, got %q", r.URL.Query().Get("api_key"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account_email":      "user@example.com",
			"plan_name":          "Free",
			"searches_per_month": 100,
			"this_month_usage":   5,
		})
	}))
	defer srv.Close()

	a := New("valid-key", WithAccountURL(srv.URL))
	if err := a.VerifyKey(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestVerifyKeyInvalidKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := New("bad-key", WithAccountURL(srv.URL))
	err := a.VerifyKey()
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
	if got := err.Error(); got != "serpapi: invalid API key" {
		t.Fatalf("unexpected error message: %s", got)
	}
}

func TestVerifyKeyForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	a := New("bad-key", WithAccountURL(srv.URL))
	err := a.VerifyKey()
	if err == nil {
		t.Fatal("expected error for forbidden key")
	}
	if got := err.Error(); got != "serpapi: invalid API key" {
		t.Fatalf("unexpected error message: %s", got)
	}
}

func TestVerifyKeyServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := New("key", WithAccountURL(srv.URL))
	err := a.VerifyKey()
	if err == nil {
		t.Fatal("expected error for server error")
	}
	expected := "serpapi account endpoint returned status 500"
	if got := err.Error(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestVerifyKeyMalformedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	a := New("key", WithAccountURL(srv.URL))
	err := a.VerifyKey()
	if err == nil {
		t.Fatal("expected error for malformed body")
	}
	if got := err.Error(); len(got) == 0 {
		t.Fatal("expected non-empty error message")
	}
}

func TestVerifyKeyMissingEmail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"plan_name": "Free",
		})
	}))
	defer srv.Close()

	a := New("key", WithAccountURL(srv.URL))
	err := a.VerifyKey()
	if err == nil {
		t.Fatal("expected error when account_email is missing")
	}
	expected := "serpapi: invalid API key (no account email in response)"
	if got := err.Error(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestVerifyKeyTransportError(t *testing.T) {
	a := New("key", WithAccountURL("http://127.0.0.1:1"))
	err := a.VerifyKey()
	if err == nil {
		t.Fatal("expected error for transport failure")
	}
}
