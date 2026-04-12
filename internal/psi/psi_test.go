package psi

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

type mockHTTPClient struct {
	resp *http.Response
	err  error
}

func (m *mockHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	return m.resp, m.err
}

func TestParseResponse(t *testing.T) {
	body := []byte(`{
		"lighthouseResult": {
			"categories": {
				"performance": {"score": 0.85}
			},
			"audits": {
				"first-contentful-paint":    {"numericValue": 1200},
				"largest-contentful-paint":   {"numericValue": 2500},
				"cumulative-layout-shift":    {"numericValue": 0.05},
				"total-blocking-time":        {"numericValue": 150},
				"speed-index":               {"numericValue": 3000}
			}
		}
	}`)

	result, err := parseResponse(body, "https://example.com", "mobile")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.PerformanceScore != 85 {
		t.Errorf("expected score 85, got %v", result.PerformanceScore)
	}
	if result.FCP != 1200 {
		t.Errorf("expected FCP 1200, got %v", result.FCP)
	}
	if result.LCP != 2500 {
		t.Errorf("expected LCP 2500, got %v", result.LCP)
	}
	if result.CLS != 0.05 {
		t.Errorf("expected CLS 0.05, got %v", result.CLS)
	}
	if result.TBT != 150 {
		t.Errorf("expected TBT 150, got %v", result.TBT)
	}
	if result.SpeedIndex != 3000 {
		t.Errorf("expected SpeedIndex 3000, got %v", result.SpeedIndex)
	}
	if result.Strategy != "mobile" {
		t.Errorf("expected strategy mobile, got %v", result.Strategy)
	}
}

func TestRunSuccess(t *testing.T) {
	body := `{
		"lighthouseResult": {
			"categories": {"performance": {"score": 0.9}},
			"audits": {
				"first-contentful-paint":  {"numericValue": 1000},
				"largest-contentful-paint": {"numericValue": 2000},
				"cumulative-layout-shift":  {"numericValue": 0.01},
				"total-blocking-time":      {"numericValue": 100},
				"speed-index":             {"numericValue": 2500}
			}
		}
	}`

	client := NewClient("", &mockHTTPClient{
		resp: &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(body)),
		},
	})

	result, err := client.Run("https://example.com", "desktop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PerformanceScore != 90 {
		t.Errorf("expected score 90, got %v", result.PerformanceScore)
	}
	if result.Strategy != "desktop" {
		t.Errorf("expected strategy desktop, got %v", result.Strategy)
	}
}

func TestRunInvalidStrategy(t *testing.T) {
	client := NewClient("", nil)
	_, err := client.Run("https://example.com", "tablet")
	if err == nil {
		t.Fatal("expected error for invalid strategy")
	}
}
