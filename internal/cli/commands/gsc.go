package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/supah-seo/supah-seo/internal/auth"
	"github.com/supah-seo/supah-seo/internal/common/config"
	"github.com/supah-seo/supah-seo/internal/gsc"
	"github.com/supah-seo/supah-seo/internal/state"
	"github.com/supah-seo/supah-seo/pkg/output"
)

// NewGSCCmd returns the gsc command group.
func NewGSCCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gsc",
		Short: "Google Search Console commands",
		Long:  `Query Google Search Console data including sites, pages, keywords, and opportunity signals.`,
	}

	cmd.AddCommand(
		newGSCSitesCmd(format, verbose),
		newGSCQueryCmd(format, verbose),
		newGSCOpportunitiesCmd(format, verbose),
	)

	return cmd
}

func newGSCSitesCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sites",
		Short: "Manage GSC site properties",
	}

	cmd.AddCommand(
		newGSCSitesListCmd(format, verbose),
		newGSCSitesUseCmd(format, verbose),
	)

	return cmd
}

func newGSCSitesListCmd(format *string, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List accessible GSC properties",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := gscClient(format)
			if err != nil {
				return err
			}

			sites, err := client.ListSites()
			if err != nil {
				return output.PrintCodedError(output.ErrGSCFailed, "failed to list sites", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(sites, map[string]any{
				"count":   len(sites),
				"source":  "gsc",
				"verbose": *verbose,
			}, output.Format(*format))
		},
	}
}

func newGSCSitesUseCmd(format *string, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "use <site_url>",
		Short: "Set the active GSC property",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
			}

			cfg.GSCProperty = args[0]
			if err := cfg.Save(); err != nil {
				return output.PrintCodedError(output.ErrConfigSaveFailed, "failed to save config", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(map[string]any{
				"gsc_property": args[0],
				"status":       "ok",
			}, map[string]any{
				"verbose": *verbose,
			}, output.Format(*format))
		},
	}
}

func newGSCQueryCmd(format *string, verbose *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query GSC search analytics data",
	}

	cmd.AddCommand(
		newGSCQueryPagesCmd(format, verbose),
		newGSCQueryKeywordsCmd(format, verbose),
		newGSCQueryTrendsCmd(format, verbose),
		newGSCQueryDevicesCmd(format, verbose),
		newGSCQueryCountriesCmd(format, verbose),
		newGSCQueryAppearancesCmd(format, verbose),
	)

	return cmd
}

func newGSCQueryPagesCmd(format *string, verbose *bool) *cobra.Command {
	var startDate, endDate, filterQuery, searchType string
	var rowLimit int

	cmd := &cobra.Command{
		Use:   "pages",
		Short: "Query page-level performance data",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, property, err := gscClientAndProperty(format)
			if err != nil {
				return err
			}

			if startDate == "" {
				startDate = time.Now().AddDate(0, 0, -28).Format("2006-01-02")
			}
			if endDate == "" {
				endDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
			}

			if err := gsc.ValidateSearchType(searchType); err != nil {
				return err
			}

			req := gsc.QueryRequest{
				SiteURL:    property,
				StartDate:  startDate,
				EndDate:    endDate,
				SearchType: searchType,
				RowLimit:   rowLimit,
			}
			if filterQuery != "" {
				req.DimensionFilterGroups = []gsc.DimensionFilterGroup{
					{Filters: []gsc.DimensionFilter{
						{Dimension: "query", Operator: "contains", Expression: filterQuery},
					}},
				}
			}

			resp, err := client.QueryPages(req)
			if err != nil {
				return output.PrintCodedError(output.ErrGSCFailed, "failed to query pages", err, nil, output.Format(*format))
			}

			if state.Exists(".") {
				if s, lerr := state.Load("."); lerr == nil {
					rows := make([]state.GSCRow, 0, len(resp.Rows))
					for _, r := range resp.Rows {
						key := ""
						if len(r.Keys) > 0 {
							key = r.Keys[0]
						}
						rows = append(rows, state.GSCRow{
							Key:         key,
							Clicks:      r.Clicks,
							Impressions: r.Impressions,
							CTR:         r.CTR,
							Position:    r.Position,
						})
					}
					if s.GSC == nil {
						s.GSC = &state.GSCData{}
					}
					s.GSC.TopPages = rows
					s.GSC.LastPull = time.Now().UTC().Format(time.RFC3339)
					s.GSC.Property = property
					s.AddHistory("gsc.query.pages", fmt.Sprintf("saved %d rows for %s", len(rows), property))
					_ = s.Save(".")
				}
			}

			meta := map[string]any{
				"count":      len(resp.Rows),
				"source":     "gsc",
				"start_date": startDate,
				"end_date":   endDate,
				"verbose":    *verbose,
			}
			if filterQuery != "" {
				meta["filter_query"] = filterQuery
			}
			return output.PrintSuccess(resp.Rows, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD, default: 28 days ago)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD, default: yesterday)")
	cmd.Flags().IntVar(&rowLimit, "limit", 100, "Maximum rows to return")
	cmd.Flags().StringVar(&filterQuery, "query", "", "Filter to pages ranking for this keyword (contains match)")
	cmd.Flags().StringVar(&searchType, "type", "web", "Search type (web, image, video, news, discover, googleNews)")

	return cmd
}

func newGSCQueryKeywordsCmd(format *string, verbose *bool) *cobra.Command {
	var startDate, endDate, filterPage, searchType string
	var rowLimit int

	cmd := &cobra.Command{
		Use:   "keywords",
		Short: "Query keyword-level performance data",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, property, err := gscClientAndProperty(format)
			if err != nil {
				return err
			}

			if startDate == "" {
				startDate = time.Now().AddDate(0, 0, -28).Format("2006-01-02")
			}
			if endDate == "" {
				endDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
			}

			if err := gsc.ValidateSearchType(searchType); err != nil {
				return err
			}

			req := gsc.QueryRequest{
				SiteURL:    property,
				StartDate:  startDate,
				EndDate:    endDate,
				SearchType: searchType,
				RowLimit:   rowLimit,
			}
			if filterPage != "" {
				req.DimensionFilterGroups = []gsc.DimensionFilterGroup{
					{Filters: []gsc.DimensionFilter{
						{Dimension: "page", Operator: "contains", Expression: filterPage},
					}},
				}
			}

			resp, err := client.QueryKeywords(req)
			if err != nil {
				return output.PrintCodedError(output.ErrGSCFailed, "failed to query keywords", err, nil, output.Format(*format))
			}

			if state.Exists(".") {
				if s, lerr := state.Load("."); lerr == nil {
					rows := make([]state.GSCRow, 0, len(resp.Rows))
					for _, r := range resp.Rows {
						key := ""
						if len(r.Keys) > 0 {
							key = r.Keys[0]
						}
						rows = append(rows, state.GSCRow{
							Key:         key,
							Clicks:      r.Clicks,
							Impressions: r.Impressions,
							CTR:         r.CTR,
							Position:    r.Position,
						})
					}
					if s.GSC == nil {
						s.GSC = &state.GSCData{}
					}
					s.GSC.TopKeywords = rows
					s.GSC.LastPull = time.Now().UTC().Format(time.RFC3339)
					s.GSC.Property = property
					s.AddHistory("gsc.query.keywords", fmt.Sprintf("saved %d rows for %s", len(rows), property))
					_ = s.Save(".")
				}
			}

			meta := map[string]any{
				"count":      len(resp.Rows),
				"source":     "gsc",
				"start_date": startDate,
				"end_date":   endDate,
				"verbose":    *verbose,
			}
			if filterPage != "" {
				meta["filter_page"] = filterPage
			}
			return output.PrintSuccess(resp.Rows, meta, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD, default: 28 days ago)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD, default: yesterday)")
	cmd.Flags().IntVar(&rowLimit, "limit", 100, "Maximum rows to return")
	cmd.Flags().StringVar(&filterPage, "page", "", "Filter to keywords for this page URL (contains match)")
	cmd.Flags().StringVar(&searchType, "type", "web", "Search type (web, image, video, news, discover, googleNews)")

	return cmd
}

func newGSCQueryTrendsCmd(format *string, verbose *bool) *cobra.Command {
	var startDate, endDate, searchType string
	var rowLimit int

	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Query date-grouped traffic trends",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, property, err := gscClientAndProperty(format)
			if err != nil {
				return err
			}

			if startDate == "" {
				startDate = time.Now().AddDate(0, 0, -28).Format("2006-01-02")
			}
			if endDate == "" {
				endDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
			}

			if err := gsc.ValidateSearchType(searchType); err != nil {
				return err
			}

			resp, err := client.QueryTrends(gsc.QueryRequest{
				SiteURL:    property,
				StartDate:  startDate,
				EndDate:    endDate,
				SearchType: searchType,
				RowLimit:   rowLimit,
			})
			if err != nil {
				return output.PrintCodedError(output.ErrGSCFailed, "failed to query trends", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(resp.Rows, map[string]any{
				"count":      len(resp.Rows),
				"source":     "gsc",
				"start_date": startDate,
				"end_date":   endDate,
				"verbose":    *verbose,
			}, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD, default: 28 days ago)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD, default: yesterday)")
	cmd.Flags().IntVar(&rowLimit, "limit", 100, "Maximum rows to return")
	cmd.Flags().StringVar(&searchType, "type", "web", "Search type (web, image, video, news, discover, googleNews)")

	return cmd
}

func newGSCQueryDevicesCmd(format *string, verbose *bool) *cobra.Command {
	var startDate, endDate, searchType string
	var rowLimit int

	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Query device-level performance data (MOBILE, DESKTOP, TABLET)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, property, err := gscClientAndProperty(format)
			if err != nil {
				return err
			}

			if startDate == "" {
				startDate = time.Now().AddDate(0, 0, -28).Format("2006-01-02")
			}
			if endDate == "" {
				endDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
			}

			if err := gsc.ValidateSearchType(searchType); err != nil {
				return err
			}

			resp, err := client.QueryDevices(gsc.QueryRequest{
				SiteURL:    property,
				StartDate:  startDate,
				EndDate:    endDate,
				SearchType: searchType,
				RowLimit:   rowLimit,
			})
			if err != nil {
				return output.PrintCodedError(output.ErrGSCFailed, "failed to query devices", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(resp.Rows, map[string]any{
				"count":      len(resp.Rows),
				"source":     "gsc",
				"start_date": startDate,
				"end_date":   endDate,
				"verbose":    *verbose,
			}, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD, default: 28 days ago)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD, default: yesterday)")
	cmd.Flags().IntVar(&rowLimit, "limit", 10, "Maximum rows to return")
	cmd.Flags().StringVar(&searchType, "type", "web", "Search type (web, image, video, news, discover, googleNews)")

	return cmd
}

func newGSCQueryCountriesCmd(format *string, verbose *bool) *cobra.Command {
	var startDate, endDate, searchType string
	var rowLimit int

	cmd := &cobra.Command{
		Use:   "countries",
		Short: "Query country-level performance data",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, property, err := gscClientAndProperty(format)
			if err != nil {
				return err
			}

			if startDate == "" {
				startDate = time.Now().AddDate(0, 0, -28).Format("2006-01-02")
			}
			if endDate == "" {
				endDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
			}

			if err := gsc.ValidateSearchType(searchType); err != nil {
				return err
			}

			resp, err := client.QueryCountries(gsc.QueryRequest{
				SiteURL:    property,
				StartDate:  startDate,
				EndDate:    endDate,
				SearchType: searchType,
				RowLimit:   rowLimit,
			})
			if err != nil {
				return output.PrintCodedError(output.ErrGSCFailed, "failed to query countries", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(resp.Rows, map[string]any{
				"count":      len(resp.Rows),
				"source":     "gsc",
				"start_date": startDate,
				"end_date":   endDate,
				"verbose":    *verbose,
			}, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD, default: 28 days ago)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD, default: yesterday)")
	cmd.Flags().IntVar(&rowLimit, "limit", 100, "Maximum rows to return")
	cmd.Flags().StringVar(&searchType, "type", "web", "Search type (web, image, video, news, discover, googleNews)")

	return cmd
}

func newGSCQueryAppearancesCmd(format *string, verbose *bool) *cobra.Command {
	var startDate, endDate, searchType string
	var rowLimit int

	cmd := &cobra.Command{
		Use:   "appearances",
		Short: "Query search appearance types (rich results, FAQ, breadcrumbs, etc.)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, property, err := gscClientAndProperty(format)
			if err != nil {
				return err
			}

			if startDate == "" {
				startDate = time.Now().AddDate(0, 0, -28).Format("2006-01-02")
			}
			if endDate == "" {
				endDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
			}

			if err := gsc.ValidateSearchType(searchType); err != nil {
				return err
			}

			resp, err := client.QueryAppearances(gsc.QueryRequest{
				SiteURL:    property,
				StartDate:  startDate,
				EndDate:    endDate,
				SearchType: searchType,
				RowLimit:   rowLimit,
			})
			if err != nil {
				return output.PrintCodedError(output.ErrGSCFailed, "failed to query appearances", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(resp.Rows, map[string]any{
				"count":      len(resp.Rows),
				"source":     "gsc",
				"start_date": startDate,
				"end_date":   endDate,
				"verbose":    *verbose,
			}, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD, default: 28 days ago)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD, default: yesterday)")
	cmd.Flags().IntVar(&rowLimit, "limit", 100, "Maximum rows to return")
	cmd.Flags().StringVar(&searchType, "type", "web", "Search type (web, image, video, news, discover, googleNews)")

	return cmd
}

func newGSCOpportunitiesCmd(format *string, verbose *bool) *cobra.Command {
	var startDate, endDate, searchType string
	var rowLimit int

	cmd := &cobra.Command{
		Use:   "opportunities",
		Short: "Find SEO opportunity signals from GSC data",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, property, err := gscClientAndProperty(format)
			if err != nil {
				return err
			}

			if startDate == "" {
				startDate = time.Now().AddDate(0, 0, -28).Format("2006-01-02")
			}
			if endDate == "" {
				endDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
			}

			if err := gsc.ValidateSearchType(searchType); err != nil {
				return err
			}

			seeds, err := client.QueryOpportunities(property, startDate, endDate, rowLimit, searchType)
			if err != nil {
				return output.PrintCodedError(output.ErrGSCFailed, "failed to query opportunities", err, nil, output.Format(*format))
			}

			return output.PrintSuccess(seeds, map[string]any{
				"count":      len(seeds),
				"source":     "gsc",
				"start_date": startDate,
				"end_date":   endDate,
				"verbose":    *verbose,
			}, output.Format(*format))
		},
	}

	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD, default: 28 days ago)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD, default: yesterday)")
	cmd.Flags().IntVar(&rowLimit, "limit", 1000, "Maximum rows to return")
	cmd.Flags().StringVar(&searchType, "type", "web", "Search type (web, image, video, news, discover, googleNews)")

	return cmd
}

// gscClient creates an authenticated GSC client.
func gscClient(format *string) (*gsc.Client, error) {
	store := auth.NewFileTokenStore()

	st, err := store.Status("gsc")
	if err != nil {
		return nil, output.PrintCodedError(output.ErrAuthFailed, "failed to check auth status", err, nil, output.Format(*format))
	}

	// If token exists but is expired, attempt automatic refresh.
	if !st.Authenticated {
		token, loadErr := store.Load("gsc")
		if loadErr != nil || token.RefreshToken == "" {
			return nil, output.PrintCodedError(output.ErrAuthRequired, "not authenticated with GSC",
				fmt.Errorf("run 'supah-seo login' first (token may be missing or expired)"), nil, output.Format(*format))
		}

		cfg, cfgErr := config.Load()
		if cfgErr != nil || cfg.GSCClientID == "" || cfg.GSCClientSecret == "" {
			return nil, output.PrintCodedError(output.ErrAuthRequired, "not authenticated with GSC",
				fmt.Errorf("run 'supah-seo login' first (cannot refresh — missing client credentials)"), nil, output.Format(*format))
		}

		refreshed, refreshErr := store.RefreshGSCToken(cfg.GSCClientID, cfg.GSCClientSecret)
		if refreshErr != nil {
			return nil, output.PrintCodedError(output.ErrAuthFailed, "failed to refresh GSC token",
				fmt.Errorf("re-authenticate with 'supah-seo login': %w", refreshErr), nil, output.Format(*format))
		}

		return gsc.NewClient(refreshed.AccessToken), nil
	}

	token, err := store.Load("gsc")
	if err != nil {
		return nil, output.PrintCodedError(output.ErrAuthRequired, "not authenticated with GSC", fmt.Errorf("run 'supah-seo login' first"), nil, output.Format(*format))
	}

	return gsc.NewClient(token.AccessToken), nil
}

// gscClientAndProperty creates an authenticated GSC client and resolves the active property.
func gscClientAndProperty(format *string) (*gsc.Client, string, error) {
	client, err := gscClient(format)
	if err != nil {
		return nil, "", err
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, "", output.PrintCodedError(output.ErrConfigLoadFailed, "failed to load config", err, nil, output.Format(*format))
	}

	if cfg.GSCProperty == "" {
		return nil, "", output.PrintCodedError(output.ErrGSCFailed, "no GSC property configured",
			fmt.Errorf("run 'supah-seo gsc sites use <url>' or set gsc_property in config"), nil, output.Format(*format))
	}

	return client, cfg.GSCProperty, nil
}
