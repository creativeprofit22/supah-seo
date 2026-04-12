package dataforseo

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.dataforseo.com"

// maxResponseSize caps the bytes read from any DataForSEO response to prevent
// OOM on malformed or malicious payloads. 50 MB is generous enough for large
// Labs/SERP/Backlinks result sets.
const maxResponseSize = 50 * 1024 * 1024 // 50 MB

// HTTPClient is an interface for HTTP operations (supports testing).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client is a shared DataForSEO API client using HTTP Basic Auth.
type Client struct {
	login      string
	password   string
	baseURL    string
	httpClient HTTPClient
}

// Option configures the DataForSEO client.
type Option func(*Client)

// WithBaseURL overrides the default DataForSEO base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithHTTPClient overrides the default HTTP client (useful for testing).
func WithHTTPClient(hc HTTPClient) Option {
	return func(c *Client) { c.httpClient = hc }
}

// New creates a DataForSEO client with Basic Auth credentials.
func New(login, password string, opts ...Option) *Client {
	c := &Client{
		login:      login,
		password:   password,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Post sends a POST request to the given endpoint path (e.g. "/v3/serp/google/organic/live/regular")
// with the provided body serialized as JSON. Returns the raw response bytes.
func (c *Client) Post(endpoint string, body any) ([]byte, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request body: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+basicAuth(c.login, c.password))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dataforseo returned status %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

// Get sends a GET request to the given endpoint path (e.g. "/v3/serp/google/organic/tasks_ready")
// and returns the raw response bytes.
func (c *Client) Get(endpoint string) ([]byte, error) {
	req, err := http.NewRequest("GET", c.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(c.login, c.password))
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dataforseo returned status %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

// verifyTask represents a single task entry in the DataForSEO response.
type verifyTask struct {
	StatusCode    int    `json:"status_code"`
	StatusMessage string `json:"status_message"`
}

// verifyResponse is the minimal envelope returned by /v3/appendix/user_data.
type verifyResponse struct {
	StatusCode    int          `json:"status_code"`
	StatusMessage string       `json:"status_message"`
	Tasks         []verifyTask `json:"tasks"`
}

// VerifyCredentials checks the client's login/password against the lightweight
// /v3/appendix/user_data endpoint. Credentials are considered valid only when
// the HTTP status is 200, the envelope status_code is 20000, and the first
// task's status_code is also 20000.
func (c *Client) VerifyCredentials() error {
	req, err := http.NewRequest("GET", c.baseURL+"/v3/appendix/user_data", nil)
	if err != nil {
		return fmt.Errorf("creating verify request: %w", err)
	}

	req.Header.Set("Authorization", "Basic "+basicAuth(c.login, c.password))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("transport error verifying credentials: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return fmt.Errorf("reading verify response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("credential verification failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var envelope verifyResponse
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("malformed verify response: %w", err)
	}

	if envelope.StatusCode != 20000 {
		return fmt.Errorf("credential verification rejected: API status %d – %s", envelope.StatusCode, envelope.StatusMessage)
	}

	if len(envelope.Tasks) == 0 {
		return fmt.Errorf("credential verification failed: response contains no tasks")
	}

	task := envelope.Tasks[0]
	if task.StatusCode != 20000 {
		return fmt.Errorf("credential verification task failed: task status %d – %s", task.StatusCode, task.StatusMessage)
	}

	return nil
}

func basicAuth(login, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(login + ":" + password))
}
