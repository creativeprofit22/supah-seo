package psi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// HTTPClient abstracts HTTP calls for testing.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client calls the Google PageSpeed Insights API.
type Client struct {
	apiKey      string
	accessToken string
	httpClient  HTTPClient
}

// Result holds the extracted Core Web Vitals from a PSI response.
type Result struct {
	URL              string  `json:"url"`
	PerformanceScore float64 `json:"performance_score"`
	FCP              float64 `json:"fcp_ms"`
	LCP              float64 `json:"lcp_ms"`
	CLS              float64 `json:"cls"`
	TBT              float64 `json:"tbt_ms"`
	SpeedIndex       float64 `json:"speed_index_ms"`
	Strategy         string  `json:"strategy"`
}

// NewClient creates a PSI client. apiKey may be empty for unauthenticated access.
func NewClient(apiKey string, httpClient HTTPClient) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &Client{apiKey: apiKey, httpClient: httpClient}
}

// NewClientWithToken creates a PSI client that authenticates via an OAuth2 Bearer token.
// This allows reuse of an existing Google OAuth token (e.g. from GSC) for PSI requests.
func NewClientWithToken(accessToken string, httpClient HTTPClient) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 120 * time.Second}
	}
	return &Client{accessToken: accessToken, httpClient: httpClient}
}

const baseURL = "https://www.googleapis.com/pagespeedonline/v5/runPagespeed"

// Run fetches PageSpeed Insights for the given URL and strategy (mobile|desktop).
// It retries up to 3 times on transient errors.
func (c *Client) Run(targetURL, strategy string) (*Result, error) {
	if strategy == "" {
		strategy = "mobile"
	}
	if strategy != "mobile" && strategy != "desktop" {
		return nil, fmt.Errorf("invalid strategy %q: must be mobile or desktop", strategy)
	}

	params := url.Values{}
	params.Set("url", targetURL)
	params.Set("strategy", strategy)
	if c.apiKey != "" {
		params.Set("key", c.apiKey)
	}

	reqURL := baseURL + "?" + params.Encode()

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}

		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("building request: %w", err)
		}

		if c.accessToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.accessToken)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("reading response body: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("PSI API returned status %d", resp.StatusCode)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("PSI API returned status %d: %s", resp.StatusCode, string(body))
		}

		return parseResponse(body, targetURL, strategy)
	}

	return nil, fmt.Errorf("PSI request failed after 3 attempts: %w", lastErr)
}

// psiResponse mirrors the subset of the PSI JSON we need.
type psiResponse struct {
	LighthouseResult struct {
		Categories struct {
			Performance struct {
				Score float64 `json:"score"`
			} `json:"performance"`
		} `json:"categories"`
		Audits map[string]struct {
			NumericValue float64 `json:"numericValue"`
		} `json:"audits"`
	} `json:"lighthouseResult"`
}

func parseResponse(body []byte, targetURL, strategy string) (*Result, error) {
	var raw psiResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing PSI response: %w", err)
	}

	audits := raw.LighthouseResult.Audits
	return &Result{
		URL:              targetURL,
		PerformanceScore: raw.LighthouseResult.Categories.Performance.Score * 100,
		FCP:              audits["first-contentful-paint"].NumericValue,
		LCP:              audits["largest-contentful-paint"].NumericValue,
		CLS:              audits["cumulative-layout-shift"].NumericValue,
		TBT:              audits["total-blocking-time"].NumericValue,
		SpeedIndex:       audits["speed-index"].NumericValue,
		Strategy:         strategy,
	}, nil
}
