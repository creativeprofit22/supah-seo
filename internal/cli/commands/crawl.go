package commands

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/supah-seo/supah-seo/internal/common/config"
	"github.com/supah-seo/supah-seo/internal/crawl"
	"github.com/supah-seo/supah-seo/internal/provider"
	_ "github.com/supah-seo/supah-seo/internal/provider/local"
	"github.com/supah-seo/supah-seo/internal/state"
	"github.com/supah-seo/supah-seo/pkg/output"
)

// NewCrawlCmd returns the crawl command group.
func NewCrawlCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "crawl",
		Short: "Website crawling commands",
	}

	cmd.AddCommand(newCrawlRunCmd(format, verbose))
	return cmd
}

func newCrawlRunCmd(format *string, verbose *bool) *cobra.Command {
	var (
		targetURL string
		depth     int
		maxPages  int
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Crawl a website and extract page data",
		RunE: func(cmd *cobra.Command, args []string) error {
			if targetURL == "" {
				return output.PrintCodedError(output.ErrInvalidURL, "missing required --url flag", nil, nil, output.Format(*format))
			}

			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			fetcher, err := provider.NewFetcher(cfg.ActiveProvider)
			if err != nil {
				return output.PrintCodedError(output.ErrProviderNotFound, "failed to create provider", err, nil, output.Format(*format))
			}

			svc := crawl.NewService(fetcher)
			result, err := svc.Run(cmd.Context(), crawl.Request{
				TargetURL: targetURL,
				Depth:     depth,
				MaxPages:  maxPages,
			})
			if err != nil {
				code := output.ErrCrawlFailed
				if errors.Is(err, context.DeadlineExceeded) {
					code = output.ErrFetchTimeout
				} else if errors.Is(err, context.Canceled) {
					code = output.ErrCancelled
				}
				return output.PrintCodedError(code, "crawl failed", err, nil, output.Format(*format))
			}

			if state.Exists(".") {
				if st, err := state.Load("."); err == nil {
					st.LastCrawl = time.Now().UTC().Format(time.RFC3339)
					st.PagesCrawled = len(result.Pages)
					st.AddHistory("crawl", fmt.Sprintf("pages=%d errors=%d", len(result.Pages), len(result.Errors)))
					_ = st.Save(".")
				}
			}

			return output.PrintSuccess(result, map[string]any{
				"pages_crawled": len(result.Pages),
				"errors":        len(result.Errors),
				"verbose":       *verbose,
			}, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&targetURL, "url", "", "Target URL to crawl (required)")
	cmd.Flags().IntVar(&depth, "depth", 2, "Maximum crawl depth")
	cmd.Flags().IntVar(&maxPages, "max-pages", 50, "Maximum number of pages to crawl")

	return cmd
}
