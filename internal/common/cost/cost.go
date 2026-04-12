package cost

import "fmt"

// CurrencyUSD is the default billing currency for provider estimates.
const CurrencyUSD = "USD"

// Estimate represents a computed execution cost estimate.
type Estimate struct {
	Amount   float64 `json:"estimated_cost"`
	Currency string  `json:"currency"`
	Basis    string  `json:"basis,omitempty"`
}

// ApprovalDecision represents whether execution can proceed.
type ApprovalDecision struct {
	RequiresApproval bool   `json:"requires_approval"`
	Reason           string `json:"reason,omitempty"`
}

// EstimateInput describes a request used to compute cost.
type EstimateInput struct {
	UnitCostUSD float64
	Units       int
	Basis       string
}

// BuildEstimate computes a deterministic estimate from unit pricing.
func BuildEstimate(input EstimateInput) (Estimate, error) {
	if input.UnitCostUSD < 0 {
		return Estimate{}, fmt.Errorf("unit cost cannot be negative")
	}
	if input.Units < 0 {
		return Estimate{}, fmt.Errorf("units cannot be negative")
	}

	return Estimate{
		Amount:   input.UnitCostUSD * float64(input.Units),
		Currency: CurrencyUSD,
		Basis:    input.Basis,
	}, nil
}

// EvaluateApproval checks whether an estimate exceeds a configured threshold.
// A threshold <= 0 means no approval gate is configured.
func EvaluateApproval(estimate Estimate, thresholdUSD float64) ApprovalDecision {
	if thresholdUSD <= 0 {
		return ApprovalDecision{RequiresApproval: false}
	}

	if estimate.Amount > thresholdUSD {
		return ApprovalDecision{
			RequiresApproval: true,
			Reason:           fmt.Sprintf("estimated cost %.4f exceeds threshold %.4f", estimate.Amount, thresholdUSD),
		}
	}

	return ApprovalDecision{RequiresApproval: false}
}
