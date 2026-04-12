package audit

import (
	"context"
	"math"

	"github.com/supah-seo/supah-seo/internal/crawl"
)

// Severity weights for score deduction per issue.
var severityWeight = map[Severity]float64{
	SeverityError:   5.0,
	SeverityWarning: 2.0,
	SeverityInfo:    0.5,
}

type engine struct{}

func (e *engine) Run(ctx context.Context, req Request) (Result, error) {
	checkers := []func(crawl.PageResult) []Issue{
		checkTitle,
		checkMetaDescription,
		checkH1,
		checkImageAlt,
		checkCanonical,
		checkStatusCode,
		checkViewport,
		checkOpenGraph,
		checkResponseTime,
		checkWordCount,
		checkSchema,
		checkMetaRobots,
		checkLang,
	}

	var allIssues []Issue
	issueCount := map[Severity]int{}

	for _, page := range req.CrawlResult.Pages {
		for _, checker := range checkers {
			issues := checker(page)
			allIssues = append(allIssues, issues...)
			for _, issue := range issues {
				issueCount[issue.Severity]++
			}
		}
	}

	crossPageIssues := checkDuplicateTitles(req.CrawlResult.Pages)
	allIssues = append(allIssues, crossPageIssues...)
	for _, issue := range crossPageIssues {
		issueCount[issue.Severity]++
	}

	dupDescIssues := checkDuplicateDescriptions(req.CrawlResult.Pages)
	allIssues = append(allIssues, dupDescIssues...)
	for _, issue := range dupDescIssues {
		issueCount[issue.Severity]++
	}

	orphanIssues := checkOrphanPages(req.CrawlResult.Pages)
	allIssues = append(allIssues, orphanIssues...)
	for _, issue := range orphanIssues {
		issueCount[issue.Severity]++
	}

	score := computeScore(allIssues, len(req.CrawlResult.Pages))

	return Result{
		TargetURL:  req.CrawlResult.TargetURL,
		Score:      score,
		Issues:     allIssues,
		PageCount:  len(req.CrawlResult.Pages),
		IssueCount: issueCount,
		Pages:      req.CrawlResult.Pages,
	}, nil
}

// volatileRules are excluded from score calculation because they vary between runs.
var volatileRules = map[string]bool{
	"slow-response": true,
}

// computeScore calculates a 0-100 score based on issues found.
// Deductions are weighted by severity and normalized by page count.
// Volatile rules (e.g. response time) are excluded to keep scores deterministic.
func computeScore(issues []Issue, pageCount int) float64 {
	if pageCount == 0 {
		return 100
	}

	totalDeduction := 0.0
	for _, issue := range issues {
		if volatileRules[issue.Rule] {
			continue
		}
		totalDeduction += severityWeight[issue.Severity]
	}

	// Normalize: max possible deduction per page is ~15 points (all checks fail)
	maxDeduction := float64(pageCount) * 15.0
	normalized := (totalDeduction / maxDeduction) * 100.0

	score := 100.0 - normalized
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}
