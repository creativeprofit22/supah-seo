package local

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/supah-seo/supah-seo/internal/provider"
)

const (
	defaultTimeout   = 30 * time.Second
	defaultUserAgent = "Supah SEOCrawler/0.2"
	maxBodySize      = 10 * 1024 * 1024 // 10 MB
)

// Fetcher implements provider.Fetcher using net/http.
type Fetcher struct {
	client    *http.Client
	userAgent string
}

// Option configures the local Fetcher.
type Option func(*Fetcher)

// WithUserAgent sets the User-Agent header.
func WithUserAgent(ua string) Option {
	return func(f *Fetcher) {
		f.userAgent = ua
	}
}

// New creates a local Fetcher with the given options.
func New(opts ...Option) *Fetcher {
	f := &Fetcher{
		client: &http.Client{
			Timeout: defaultTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		userAgent: defaultUserAgent,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Fetch performs an HTTP GET and returns the result.
func (f *Fetcher) Fetch(ctx context.Context, url string) (provider.FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return provider.FetchResult{}, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", f.userAgent)

	resp, err := f.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return provider.FetchResult{}, fmt.Errorf("fetch timeout for %s: %w", url, context.DeadlineExceeded)
		}
		if ctx.Err() == context.Canceled {
			return provider.FetchResult{}, fmt.Errorf("fetch cancelled for %s: %w", url, context.Canceled)
		}
		return provider.FetchResult{}, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return provider.FetchResult{}, fmt.Errorf("reading body: %w", err)
	}

	return provider.FetchResult{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    resp.Header,
	}, nil
}

func init() {
	provider.Register("local", func() (provider.Fetcher, error) {
		return New(), nil
	})
}
