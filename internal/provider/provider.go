package provider

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
)

// FetchResult holds the response from a Fetch call.
type FetchResult struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// Fetcher abstracts HTTP fetching so the crawl service is decoupled from net/http.
type Fetcher interface {
	Fetch(ctx context.Context, url string) (FetchResult, error)
}

// Factory creates a Fetcher instance.
type Factory func() (Fetcher, error)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register adds a named provider factory to the registry.
func Register(name string, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = factory
}

// NewFetcher creates a Fetcher by looking up the provider name in the registry.
func NewFetcher(providerName string) (Fetcher, error) {
	mu.RLock()
	factory, ok := registry[providerName]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", providerName)
	}
	return factory()
}

// Available returns the names of all registered providers.
func Available() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
