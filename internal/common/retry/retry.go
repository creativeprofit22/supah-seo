package retry

import (
	"errors"
	"time"
)

// RetryableError wraps an error with an HTTP status code so callers can signal
// that a failure is eligible for retry.
type RetryableError struct {
	StatusCode int
	Err        error
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// retryableStatus returns true for HTTP status codes that warrant a retry.
func retryableStatus(code int) bool {
	switch code {
	case 429, 500, 502, 503:
		return true
	}
	return false
}

// backoffDurations defines the sleep durations between successive attempts.
// Index 0 is the pause before attempt 2, index 1 before attempt 3, etc.
var backoffDurations = []time.Duration{
	1 * time.Second,
	3 * time.Second,
}

// Do calls fn up to maxRetries+1 times. It retries only when fn returns a
// *RetryableError whose StatusCode is one of 429, 500, 502, or 503.
// Any other error is returned immediately without further attempts.
// If all attempts fail the last error is returned.
func Do(maxRetries int, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		var re *RetryableError
		if !errors.As(lastErr, &re) || !retryableStatus(re.StatusCode) {
			// Non-retryable — surface immediately.
			return lastErr
		}

		if attempt < maxRetries {
			pause := backoffDuration(attempt)
			time.Sleep(pause)
		}
	}

	return lastErr
}

// backoffDuration returns the sleep duration for the given attempt index
// (0-based). Falls back to the last defined duration for any index beyond the
// table.
func backoffDuration(attempt int) time.Duration {
	if attempt < len(backoffDurations) {
		return backoffDurations[attempt]
	}
	return backoffDurations[len(backoffDurations)-1]
}
