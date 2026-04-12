package output

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestPrintSuccessJSONEnvelope(t *testing.T) {
	captured := captureStdout(t, func() {
		err := PrintSuccess(map[string]any{"hello": "world"}, map[string]any{"verbose": true}, FormatJSON)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	var env map[string]any
	if err := json.Unmarshal([]byte(captured), &env); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Verify required envelope keys
	if _, ok := env["success"]; !ok {
		t.Fatal("missing 'success' key in envelope")
	}
	if env["success"] != true {
		t.Fatalf("expected success=true, got %v", env["success"])
	}
	if _, ok := env["data"]; !ok {
		t.Fatal("missing 'data' key in envelope")
	}
	if _, ok := env["metadata"]; !ok {
		t.Fatal("missing 'metadata' key in envelope")
	}

	// Verify data content
	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatal("expected 'data' to be an object")
	}
	if data["hello"] != "world" {
		t.Fatalf("expected data.hello='world', got %v", data["hello"])
	}
}

func TestPrintErrorResponseJSONEnvelope(t *testing.T) {
	captured := captureStdout(t, func() {
		err := PrintErrorResponse("not implemented", nil, map[string]any{"status": "not_implemented"}, FormatJSON)
		if err == nil {
			t.Fatalf("expected non-nil error")
		}
	})

	var env map[string]any
	if err := json.Unmarshal([]byte(captured), &env); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if env["success"] != false {
		t.Fatalf("expected success=false, got %v", env["success"])
	}
	errObj, ok := env["error"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'error' key in envelope")
	}
	if errObj["message"] != "not implemented" {
		t.Fatalf("expected error.message='not implemented', got %v", errObj["message"])
	}
}

func TestPrintCodedErrorJSONEnvelope(t *testing.T) {
	captured := captureStdout(t, func() {
		err := PrintCodedError(ErrCrawlFailed, "crawl failed", nil, nil, FormatJSON)
		if err == nil {
			t.Fatalf("expected non-nil error")
		}
	})

	var env map[string]any
	if err := json.Unmarshal([]byte(captured), &env); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Verify envelope shape
	if env["success"] != false {
		t.Fatalf("expected success=false, got %v", env["success"])
	}

	errObj, ok := env["error"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid 'error' key in envelope")
	}
	if errObj["message"] != "crawl failed" {
		t.Fatalf("expected error.message='crawl failed', got %v", errObj["message"])
	}
	if errObj["code"] != ErrCrawlFailed {
		t.Fatalf("expected error.code='%s', got %v", ErrCrawlFailed, errObj["code"])
	}
}

func TestSuccessEnvelopeOmitsErrorKey(t *testing.T) {
	captured := captureStdout(t, func() {
		err := PrintSuccess("ok", nil, FormatJSON)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	var env map[string]any
	if err := json.Unmarshal([]byte(captured), &env); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if _, ok := env["error"]; ok {
		t.Fatal("success envelope should not contain 'error' key")
	}
}

func TestErrorEnvelopeOmitsDataKey(t *testing.T) {
	captured := captureStdout(t, func() {
		_ = PrintCodedError(ErrInvalidURL, "bad url", nil, nil, FormatJSON)
	})

	var env map[string]any
	if err := json.Unmarshal([]byte(captured), &env); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if _, ok := env["data"]; ok {
		t.Fatal("error envelope should not contain 'data' key")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}

	os.Stdout = w
	defer func() {
		os.Stdout = orig
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("closing writer: %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("reading output: %v", err)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("closing reader: %v", err)
	}

	return buf.String()
}
