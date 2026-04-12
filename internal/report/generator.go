package report

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const defaultReportsDir = ".config/supah-seo/reports"

type generator struct{}

type storedReport struct {
	GeneratedAt string         `json:"generated_at"`
	TargetURL   string         `json:"target_url"`
	Score       float64        `json:"score"`
	PageCount   int            `json:"page_count"`
	IssueCount  map[string]int `json:"issue_count"`
	Issues      any            `json:"issues"`
	Pages       any            `json:"pages"`
}

func (g *generator) Generate(ctx context.Context, req Request) (Result, error) {
	dir := req.OutputDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Result{}, fmt.Errorf("getting home dir: %w", err)
		}
		dir = filepath.Join(home, defaultReportsDir)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{}, fmt.Errorf("creating reports dir: %w", err)
	}

	now := time.Now()
	filename := fmt.Sprintf("%s.json", now.Format("2006-01-02T15-04-05"))
	filePath := filepath.Join(dir, filename)

	// Convert severity keys to strings for JSON
	issueCount := map[string]int{}
	for k, v := range req.AuditResult.IssueCount {
		issueCount[string(k)] = v
	}

	report := storedReport{
		GeneratedAt: now.Format(time.RFC3339),
		TargetURL:   req.AuditResult.TargetURL,
		Score:       req.AuditResult.Score,
		PageCount:   req.AuditResult.PageCount,
		IssueCount:  issueCount,
		Issues:      req.AuditResult.Issues,
		Pages:       req.AuditResult.Pages,
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("encoding report: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return Result{}, fmt.Errorf("writing report: %w", err)
	}

	summary := map[string]any{
		"target_url":  req.AuditResult.TargetURL,
		"score":       req.AuditResult.Score,
		"page_count":  req.AuditResult.PageCount,
		"issue_count": issueCount,
	}

	return Result{
		FilePath: filePath,
		Summary:  summary,
	}, nil
}

func (g *generator) List(ctx context.Context, dir string) ([]ReportMeta, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("getting home dir: %w", err)
		}
		dir = filepath.Join(home, defaultReportsDir)
	}

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []ReportMeta{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading reports dir: %w", err)
	}

	var reports []ReportMeta
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var stored storedReport
		if err := json.Unmarshal(data, &stored); err != nil {
			continue
		}

		reports = append(reports, ReportMeta{
			FilePath:  filePath,
			TargetURL: stored.TargetURL,
			Score:     stored.Score,
			CreatedAt: stored.GeneratedAt,
		})
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].CreatedAt > reports[j].CreatedAt
	})

	return reports, nil
}
