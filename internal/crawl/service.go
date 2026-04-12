package crawl

import (
	"context"

	"github.com/supah-seo/supah-seo/internal/provider"
)

// Service defines crawl service behavior.
type Service interface {
	Run(ctx context.Context, req Request) (Result, error)
}

// NewService creates a crawl service backed by the given fetcher.
func NewService(fetcher provider.Fetcher) Service {
	return &crawler{fetcher: fetcher}
}
