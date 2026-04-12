package audit

import "github.com/supah-seo/supah-seo/internal/crawl"

// Severity represents the severity level of an audit issue.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Issue represents a single SEO problem found on a page.
type Issue struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	URL      string   `json:"url"`
	Message  string   `json:"message"`
	Detail   string   `json:"detail,omitempty"`
	Why      string   `json:"why,omitempty"`
	Fix      string   `json:"fix,omitempty"`
}

// Request defines inputs for an audit operation.
type Request struct {
	CrawlResult crawl.Result
}

// Result defines outputs for an audit operation.
type Result struct {
	TargetURL  string             `json:"target_url"`
	Score      float64            `json:"score"`
	Issues     []Issue            `json:"issues"`
	PageCount  int                `json:"page_count"`
	IssueCount map[Severity]int   `json:"issue_count"`
	Pages      []crawl.PageResult `json:"pages"`
}
