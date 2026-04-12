package commands

import (
	"context"
	"errors"

	"github.com/supah-seo/supah-seo/internal/audit"
	"github.com/supah-seo/supah-seo/internal/common/config"
	"github.com/supah-seo/supah-seo/internal/crawl"
	"github.com/supah-seo/supah-seo/internal/provider"
	_ "github.com/supah-seo/supah-seo/internal/provider/local"
	"github.com/supah-seo/supah-seo/internal/report"
	"github.com/supah-seo/supah-seo/pkg/output"
	"github.com/spf13/cobra"
)

// NewReportCmd returns the report command group.
func NewReportCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Report generation and listing commands",
	}

	cmd.AddCommand(
		newReportGenerateCmd(format, verbose),
		newReportListCmd(format, verbose),
	)
	return cmd
}

func newReportGenerateCmd(format *string, verbose *bool) *cobra.Command {
	var (
		targetURL string
		depth     int
		maxPages  int
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Crawl, audit, and generate a stored report",
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

			// Crawl
			crawlSvc := crawl.NewService(fetcher)
			crawlResult, err := crawlSvc.Run(cmd.Context(), crawl.Request{
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

			// Audit
			auditSvc := audit.NewService()
			auditResult, err := auditSvc.Run(cmd.Context(), audit.Request{
				CrawlResult: crawlResult,
			})
			if err != nil {
				return output.PrintCodedError(output.ErrAuditFailed, "audit failed", err, nil, output.Format(*format))
			}

			// Report
			reportSvc := report.NewService()
			reportResult, err := reportSvc.Generate(cmd.Context(), report.Request{
				AuditResult: auditResult,
			})
			if err != nil {
				return output.PrintCodedError(output.ErrReportWriteFailed, "report generation failed", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(reportResult, map[string]any{
				"verbose": *verbose,
			}, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&targetURL, "url", "", "Target URL to report on (required)")
	cmd.Flags().IntVar(&depth, "depth", 2, "Maximum crawl depth")
	cmd.Flags().IntVar(&maxPages, "max-pages", 50, "Maximum number of pages to crawl")

	return cmd
}

func newReportListCmd(format *string, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stored reports",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := report.NewService()
			reports, err := svc.List(cmd.Context(), "")
			if err != nil {
				return output.PrintCodedError(output.ErrReportListFailed, "failed to list reports", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(reports, map[string]any{
				"count":   len(reports),
				"verbose": *verbose,
			}, output.Format(*format))
		},
	}
}
