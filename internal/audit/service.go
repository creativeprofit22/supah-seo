package audit

import "context"

// Service defines audit service behavior.
type Service interface {
	Run(ctx context.Context, req Request) (Result, error)
}

// NewService creates a new audit service.
func NewService() Service {
	return &engine{}
}
