package retry

import (
	"errors"
	"testing"
	"time"
)

// replace backoffDurations with zero durations during tests so they run fast.
func init() {
	backoffDurations = []time.Duration{0, 0}
}

func TestDo_SucceedsFirstTry(t *testing.T) {
	calls := 0
	err := Do(2, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDo_SucceedsOnRetry(t *testing.T) {
	calls := 0
	err := Do(2, func() error {
		calls++
		if calls < 2 {
			return &RetryableError{StatusCode: 503, Err: errors.New("service unavailable")}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestDo_FailsAfterMaxRetries(t *testing.T) {
	calls := 0
	retryableErr := &RetryableError{StatusCode: 429, Err: errors.New("rate limited")}

	err := Do(2, func() error {
		calls++
		return retryableErr
	})

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	// Should have made 3 total attempts (1 initial + 2 retries).
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
	var re *RetryableError
	if !errors.As(err, &re) || re.StatusCode != 429 {
		t.Fatalf("expected RetryableError with 429, got %v", err)
	}
}

func TestDo_NonRetryableStopsImmediately(t *testing.T) {
	calls := 0
	plainErr := errors.New("not found")

	err := Do(2, func() error {
		calls++
		return plainErr
	})

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retries), got %d", calls)
	}
	if !errors.Is(err, plainErr) {
		t.Fatalf("expected plainErr, got %v", err)
	}
}

func TestDo_NonRetryableStatusCodeStopsImmediately(t *testing.T) {
	calls := 0
	notFoundErr := &RetryableError{StatusCode: 404, Err: errors.New("not found")}

	err := Do(2, func() error {
		calls++
		return notFoundErr
	})

	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retries for 404), got %d", calls)
	}
}

func TestDo_AllRetryableStatusCodes(t *testing.T) {
	codes := []int{429, 500, 502, 503}
	for _, code := range codes {
		code := code
		t.Run(http429name(code), func(t *testing.T) {
			calls := 0
			err := Do(2, func() error {
				calls++
				return &RetryableError{StatusCode: code, Err: errors.New("error")}
			})
			if err == nil {
				t.Fatal("expected error")
			}
			if calls != 3 {
				t.Fatalf("code %d: expected 3 calls, got %d", code, calls)
			}
		})
	}
}

func http429name(code int) string {
	switch code {
	case 429:
		return "429_TooManyRequests"
	case 500:
		return "500_InternalServerError"
	case 502:
		return "502_BadGateway"
	case 503:
		return "503_ServiceUnavailable"
	}
	return "unknown"
}
