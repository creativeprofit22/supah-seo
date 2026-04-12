package crawl

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/supah-seo/supah-seo/internal/common/retry"
	"github.com/supah-seo/supah-seo/internal/provider"
)

const (
	defaultDepth    = 2
	defaultMaxPages = 50
	concurrency     = 5
)

type crawler struct {
	fetcher provider.Fetcher
}

type crawlItem struct {
	url   string
	depth int
}

func (c *crawler) Run(ctx context.Context, req Request) (Result, error) {
	depth := req.Depth
	if depth <= 0 {
		depth = defaultDepth
	}
	maxPages := req.MaxPages
	if maxPages <= 0 {
		maxPages = defaultMaxPages
	}

	parsedBase, err := url.Parse(req.TargetURL)
	if err != nil {
		return Result{}, err
	}

	result := Result{TargetURL: req.TargetURL}

	var (
		mu      sync.Mutex
		visited = map[string]bool{}
		wg      sync.WaitGroup
		sem     = make(chan struct{}, concurrency)
		queue   = make(chan crawlItem, maxPages*10)
	)

	// Normalize and mark the start URL as visited
	startURL := normalizeURL(parsedBase)
	visited[startURL] = true
	queue <- crawlItem{url: startURL, depth: 0}

	// Track active work to know when to close the queue
	active := 1

	for item := range queue {
		// Abort on context cancellation
		if ctx.Err() != nil {
			mu.Lock()
			active--
			if active == 0 {
				close(queue)
			}
			mu.Unlock()
			continue
		}

		// Check page limit
		mu.Lock()
		pageCount := len(result.Pages)
		mu.Unlock()
		if pageCount >= maxPages {
			mu.Lock()
			active--
			if active == 0 {
				close(queue)
			}
			mu.Unlock()
			continue
		}

		wg.Add(1)
		sem <- struct{}{} // acquire semaphore

		go func(item crawlItem) {
			defer wg.Done()
			defer func() { <-sem }() // release semaphore

			// Abort on context cancellation
			if ctx.Err() != nil {
				mu.Lock()
				active--
				if active == 0 {
					close(queue)
				}
				mu.Unlock()
				return
			}

			// Check limit before fetching
			mu.Lock()
			if len(result.Pages) >= maxPages {
				active--
				if active == 0 {
					close(queue)
				}
				mu.Unlock()
				return
			}
			mu.Unlock()

			fetchStart := time.Now()
			var fetchResult provider.FetchResult
			var lastWas5xx bool
			fetchErr := retry.Do(1, func() error {
				lastWas5xx = false
				var err error
				fetchResult, err = c.fetcher.Fetch(ctx, item.url)
				if err != nil {
					// Wrap timeout errors so the retry layer treats them as retryable.
					var netErr net.Error
					if errors.As(err, &netErr) && netErr.Timeout() {
						return &retry.RetryableError{StatusCode: 503, Err: err}
					}
					if errors.Is(err, context.DeadlineExceeded) {
						return &retry.RetryableError{StatusCode: 503, Err: err}
					}
					return err
				}
				// Treat 5xx responses as retryable, but remember it was a server
				// status (not a network error) so we still record the page after
				// all retries are exhausted.
				if fetchResult.StatusCode >= 500 {
					lastWas5xx = true
					return &retry.RetryableError{
						StatusCode: fetchResult.StatusCode,
						Err:        fmt.Errorf("HTTP %d", fetchResult.StatusCode),
					}
				}
				return nil
			})
			// A persistent 5xx response is still a valid page — clear the error so
			// the page is recorded with its status code as the original code did.
			if fetchErr != nil && lastWas5xx {
				fetchErr = nil
			}
			responseTimeMs := int(time.Since(fetchStart).Milliseconds())

			mu.Lock()
			if fetchErr != nil {
				result.Errors = append(result.Errors, CrawlError{
					URL:     item.url,
					Message: fetchErr.Error(),
				})
				active--
				if active == 0 {
					close(queue)
				}
				mu.Unlock()
				return
			}

			// Double-check limit after fetch (another goroutine may have filled it)
			if len(result.Pages) >= maxPages {
				active--
				if active == 0 {
					close(queue)
				}
				mu.Unlock()
				return
			}

			page := extractPageData(item.url, fetchResult.StatusCode, fetchResult.Body, fetchResult.Headers, responseTimeMs)
			result.Pages = append(result.Pages, page)
			currentCount := len(result.Pages)
			mu.Unlock()

			// Enqueue discovered internal links if within depth limit
			if item.depth < depth && currentCount < maxPages {
				for _, link := range page.Links {
					if !link.Internal {
						continue
					}
					parsed, parseErr := url.Parse(link.Href)
					if parseErr != nil {
						continue
					}
					normalized := normalizeURL(parsed)
					if !isSameDomain(parsed, parsedBase) {
						continue
					}
					// Skip non-HTTP schemes, fragments, etc.
					if parsed.Scheme != "http" && parsed.Scheme != "https" {
						continue
					}

					mu.Lock()
					if !visited[normalized] && len(result.Pages) < maxPages {
						visited[normalized] = true
						mu.Unlock()
						select {
						case queue <- crawlItem{url: normalized, depth: item.depth + 1}:
							mu.Lock()
							active++
							mu.Unlock()
						default:
							// Queue full — drop item, don't increment active
						}
					} else {
						mu.Unlock()
					}
				}
			}

			mu.Lock()
			active--
			if active == 0 {
				close(queue)
			}
			mu.Unlock()
		}(item)
	}

	wg.Wait()

	sort.Slice(result.Pages, func(i, j int) bool {
		return result.Pages[i].URL < result.Pages[j].URL
	})

	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return result, nil
}

func normalizeURL(u *url.URL) string {
	// Strip fragment, ensure trailing slash consistency
	u.Fragment = ""
	path := u.Path
	if path == "" {
		path = "/"
	}
	return u.Scheme + "://" + u.Host + path
	// Note: query params are intentionally stripped for crawl deduplication
}

func isSameDomain(link, base *url.URL) bool {
	return strings.EqualFold(link.Host, base.Host)
}
