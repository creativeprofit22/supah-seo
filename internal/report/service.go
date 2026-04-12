package report

import (
	"context"

	"github.com/supah-seo/supah-seo/internal/audit"
)

// Request defines inputs for report generation.
type Request struct {
	AuditResult audit.Result
	OutputDir   string
}

// Result defines outputs for generated reports.
type Result struct {
	FilePath string         `json:"file_path"`
	Summary  map[string]any `json:"summary"`
}

// ReportMeta holds metadata about a stored report.
type ReportMeta struct {
	FilePath  string  `json:"file_path"`
	TargetURL string  `json:"target_url"`
	Score     float64 `json:"score"`
	CreatedAt string  `json:"created_at"`
}

// Service defines report service behavior.
type Service interface {
	Generate(ctx context.Context, req Request) (Result, error)
	List(ctx context.Context, dir string) ([]ReportMeta, error)
}

// NewService creates a new report service.
func NewService() Service {
	return &generator{}
}
