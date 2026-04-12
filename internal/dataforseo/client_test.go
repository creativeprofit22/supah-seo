package dataforseo

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// mockHTTP implements HTTPClient for testing.
type mockHTTP struct {
	resp *http.Response
	err  error
}

func (m *mockHTTP) Do(_ *http.Request) (*http.Response, error) {
	return m.resp, m.err
}

func jsonBody(s string) io.ReadCloser {
	return io.NopCloser(bytes.NewBufferString(s))
}

func TestVerifyCredentials_Success(t *testing.T) {
	c := New("user", "pass", WithHTTPClient(&mockHTTP{
		resp: &http.Response{
			StatusCode: 200,
			Body:       jsonBody(`{"status_code":20000,"status_message":"Ok.","tasks":[{"status_code":20000,"status_message":"Ok."}]}`),
		},
	}))

	if err := c.VerifyCredentials(); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestVerifyCredentials_TransportError(t *testing.T) {
	c := New("user", "pass", WithHTTPClient(&mockHTTP{
		err: errors.New("connection refused"),
	}))

	err := c.VerifyCredentials()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "transport error") {
		t.Fatalf("expected transport error, got: %v", err)
	}
}

func TestVerifyCredentials_Non200(t *testing.T) {
	c := New("user", "pass", WithHTTPClient(&mockHTTP{
		resp: &http.Response{
			StatusCode: 401,
			Body:       jsonBody(`Unauthorized`),
		},
	}))

	err := c.VerifyCredentials()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("expected HTTP 401 in error, got: %v", err)
	}
}

func TestVerifyCredentials_MalformedJSON(t *testing.T) {
	c := New("user", "pass", WithHTTPClient(&mockHTTP{
		resp: &http.Response{
			StatusCode: 200,
			Body:       jsonBody(`not json`),
		},
	}))

	err := c.VerifyCredentials()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("expected malformed error, got: %v", err)
	}
}

func TestVerifyCredentials_Non20000Status(t *testing.T) {
	c := New("user", "pass", WithHTTPClient(&mockHTTP{
		resp: &http.Response{
			StatusCode: 200,
			Body:       jsonBody(`{"status_code":40100,"status_message":"Authorization failed."}`),
		},
	}))

	err := c.VerifyCredentials()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "40100") {
		t.Fatalf("expected API status 40100 in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Authorization failed") {
		t.Fatalf("expected message in error, got: %v", err)
	}
}

func TestVerifyCredentials_TaskFailure(t *testing.T) {
	c := New("user", "pass", WithHTTPClient(&mockHTTP{
		resp: &http.Response{
			StatusCode: 200,
			Body:       jsonBody(`{"status_code":20000,"status_message":"Ok.","tasks":[{"status_code":40501,"status_message":"Insufficient credits."}]}`),
		},
	}))

	err := c.VerifyCredentials()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "task status 40501") {
		t.Fatalf("expected task status 40501 in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Insufficient credits") {
		t.Fatalf("expected task message in error, got: %v", err)
	}
}

func TestVerifyCredentials_MissingTasks(t *testing.T) {
	c := New("user", "pass", WithHTTPClient(&mockHTTP{
		resp: &http.Response{
			StatusCode: 200,
			Body:       jsonBody(`{"status_code":20000,"status_message":"Ok.","tasks":[]}`),
		},
	}))

	err := c.VerifyCredentials()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no tasks") {
		t.Fatalf("expected 'no tasks' in error, got: %v", err)
	}
}

func TestVerifyCredentials_MissingTasksField(t *testing.T) {
	c := New("user", "pass", WithHTTPClient(&mockHTTP{
		resp: &http.Response{
			StatusCode: 200,
			Body:       jsonBody(`{"status_code":20000,"status_message":"Ok."}`),
		},
	}))

	err := c.VerifyCredentials()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no tasks") {
		t.Fatalf("expected 'no tasks' in error, got: %v", err)
	}
}

func TestVerifyCredentials_AuthHeader(t *testing.T) {
	var captured *http.Request
	mock := &capturingHTTP{
		resp: &http.Response{
			StatusCode: 200,
			Body:       jsonBody(`{"status_code":20000,"status_message":"Ok.","tasks":[{"status_code":20000,"status_message":"Ok."}]}`),
		},
	}
	mock.capture = &captured

	c := New("mylogin", "mypass", WithHTTPClient(mock))
	if err := c.VerifyCredentials(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if captured == nil {
		t.Fatal("request was not captured")
	}
	auth := captured.Header.Get("Authorization")
	expected := "Basic " + basicAuth("mylogin", "mypass")
	if auth != expected {
		t.Fatalf("expected auth header %q, got %q", expected, auth)
	}
	if captured.Method != "GET" {
		t.Fatalf("expected GET method, got %s", captured.Method)
	}
	if !strings.HasSuffix(captured.URL.Path, "/v3/appendix/user_data") {
		t.Fatalf("expected /v3/appendix/user_data path, got %s", captured.URL.Path)
	}
}

// capturingHTTP records the request for assertion.
type capturingHTTP struct {
	resp    *http.Response
	capture **http.Request
}

func (m *capturingHTTP) Do(req *http.Request) (*http.Response, error) {
	*m.capture = req
	return m.resp, nil
}
