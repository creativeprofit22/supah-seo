package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout via an os.Pipe so we can capture output
// from the output package, which writes directly to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	old := os.Stdout
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	_ = r.Close()

	return buf.String()
}

// runCmd creates a fresh root command, sets args, captures stdout, and returns
// the raw output string.
func runCmd(t *testing.T, args ...string) string {
	t.Helper()

	return captureStdout(t, func() {
		root := newRootCmd("test-version")
		root.SetArgs(args)
		// Discard cobra's stderr so test output stays clean.
		root.SetErr(io.Discard)
		// We intentionally ignore the returned error; envelope_test only
		// cares about the JSON written to stdout.
		_ = root.Execute()
	})
}

// assertEnvelope parses raw as JSON and verifies the standard envelope contract.
func assertEnvelope(t *testing.T, label, raw string) {
	t.Helper()

	raw = strings.TrimSpace(raw)
	if raw == "" {
		t.Fatalf("[%s] stdout is empty — expected JSON envelope", label)
	}

	var env map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("[%s] stdout is not valid JSON: %v\nraw: %s", label, err, raw)
	}

	successRaw, ok := env["success"]
	if !ok {
		t.Fatalf("[%s] envelope missing 'success' key", label)
	}

	var success bool
	if err := json.Unmarshal(successRaw, &success); err != nil {
		t.Fatalf("[%s] 'success' is not a bool: %v", label, err)
	}

	if success {
		if _, ok := env["data"]; !ok {
			t.Errorf("[%s] success=true but 'data' key is absent", label)
		}
	} else {
		errRaw, ok := env["error"]
		if !ok {
			t.Fatalf("[%s] success=false but 'error' key is absent", label)
		}

		var errPayload map[string]json.RawMessage
		if err := json.Unmarshal(errRaw, &errPayload); err != nil {
			t.Fatalf("[%s] 'error' is not a JSON object: %v", label, err)
		}

		if _, ok := errPayload["message"]; !ok {
			t.Errorf("[%s] error object missing 'message' key", label)
		}
	}
}

// TestEnvelopeOutputContract verifies that every tested command writes valid
// JSON to stdout and respects the success/data/error envelope contract.
func TestEnvelopeOutputContract(t *testing.T) {
	tests := []struct {
		label       string
		args        []string
		wantSuccess bool
	}{
		{
			label:       "version",
			args:        []string{"version"},
			wantSuccess: true,
		},
		{
			label:       "config show",
			args:        []string{"config", "show"},
			wantSuccess: true,
		},
		{
			label:       "config path",
			args:        []string{"config", "path"},
			wantSuccess: true,
		},
		{
			label:       "provider list",
			args:        []string{"provider", "list"},
			wantSuccess: true,
		},
		{
			label:       "audit run (no --url)",
			args:        []string{"audit", "run"},
			wantSuccess: false,
		},
		{
			label:       "crawl run (no --url)",
			args:        []string{"crawl", "run"},
			wantSuccess: false,
		},
		{
			label:       "init (no --url)",
			args:        []string{"init"},
			wantSuccess: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.label, func(t *testing.T) {
			raw := runCmd(t, tc.args...)
			assertEnvelope(t, tc.label, raw)

			// Also verify the success value matches expectation.
			var env map[string]json.RawMessage
			_ = json.Unmarshal([]byte(strings.TrimSpace(raw)), &env)
			var success bool
			_ = json.Unmarshal(env["success"], &success)
			if success != tc.wantSuccess {
				t.Errorf("[%s] expected success=%v, got success=%v\nraw: %s",
					tc.label, tc.wantSuccess, success, raw)
			}
		})
	}
}
