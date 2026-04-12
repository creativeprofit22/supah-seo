package report

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/supah-seo/supah-seo/internal/audit"
	"github.com/supah-seo/supah-seo/internal/crawl"
)

func TestGenerateCreatesReportFile(t *testing.T) {
	dir := t.TempDir()
	svc := NewService()

	result, err := svc.Generate(context.Background(), Request{
		AuditResult: audit.Result{
			TargetURL: "https://example.com",
			Score:     85.5,
			PageCount: 3,
			IssueCount: map[audit.Severity]int{
				audit.SeverityWarning: 2,
				audit.SeverityInfo:    1,
			},
			Issues: []audit.Issue{
				{Rule: "title-missing", Severity: audit.SeverityError, URL: "https://example.com"},
			},
			Pages: []crawl.PageResult{
				{URL: "https://example.com", StatusCode: 200, Title: "Example"},
			},
		},
		OutputDir: dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FilePath == "" {
		t.Fatal("expected non-empty file path")
	}

	if _, err := os.Stat(result.FilePath); os.IsNotExist(err) {
		t.Fatalf("report file not created: %s", result.FilePath)
	}

	if result.Summary["target_url"] != "https://example.com" {
		t.Errorf("unexpected target_url in summary: %v", result.Summary["target_url"])
	}
	if result.Summary["score"] != 85.5 {
		t.Errorf("unexpected score in summary: %v", result.Summary["score"])
	}
}

func TestListReturnsStoredReports(t *testing.T) {
	dir := t.TempDir()
	svc := NewService()

	// Generate two reports
	_, err := svc.Generate(context.Background(), Request{
		AuditResult: audit.Result{
			TargetURL:  "https://example.com",
			Score:      90,
			PageCount:  1,
			IssueCount: map[audit.Severity]int{},
		},
		OutputDir: dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Write a second report with different name
	secondFile := filepath.Join(dir, "2025-01-02T10-00-00.json")
	if err := os.WriteFile(secondFile, []byte(`{"generated_at":"2025-01-02T10:00:00Z","target_url":"https://other.com","score":75,"page_count":2,"issue_count":{},"issues":[],"pages":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	reports, err := svc.List(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(reports) < 2 {
		t.Fatalf("expected at least 2 reports, got %d", len(reports))
	}
}

func TestListEmptyDir(t *testing.T) {
	dir := t.TempDir()
	svc := NewService()

	reports, err := svc.List(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 0 {
		t.Errorf("expected 0 reports, got %d", len(reports))
	}
}

func TestListNonExistentDir(t *testing.T) {
	svc := NewService()
	reports, err := svc.List(context.Background(), "/tmp/supah-seo-test-nonexistent-dir-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reports) != 0 {
		t.Errorf("expected 0 reports for nonexistent dir, got %d", len(reports))
	}
}
