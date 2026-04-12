package cost

import "testing"

func TestBuildEstimate(t *testing.T) {
	est, err := BuildEstimate(EstimateInput{
		UnitCostUSD: 0.01,
		Units:       5,
		Basis:       "test basis",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if est.Amount != 0.05 {
		t.Fatalf("expected 0.05, got %v", est.Amount)
	}
	if est.Currency != CurrencyUSD {
		t.Fatalf("expected USD, got %s", est.Currency)
	}
	if est.Basis != "test basis" {
		t.Fatalf("expected 'test basis', got %q", est.Basis)
	}
}

func TestBuildEstimateNegativeUnit(t *testing.T) {
	_, err := BuildEstimate(EstimateInput{UnitCostUSD: -1, Units: 1})
	if err == nil {
		t.Fatal("expected error for negative unit cost")
	}
}

func TestBuildEstimateNegativeUnits(t *testing.T) {
	_, err := BuildEstimate(EstimateInput{UnitCostUSD: 1, Units: -1})
	if err == nil {
		t.Fatal("expected error for negative units")
	}
}

func TestBuildEstimateZero(t *testing.T) {
	est, err := BuildEstimate(EstimateInput{UnitCostUSD: 0, Units: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if est.Amount != 0 {
		t.Fatalf("expected 0, got %v", est.Amount)
	}
}

func TestEvaluateApprovalBelow(t *testing.T) {
	est := Estimate{Amount: 0.05, Currency: CurrencyUSD}
	decision := EvaluateApproval(est, 1.0)
	if decision.RequiresApproval {
		t.Fatal("expected no approval required")
	}
}

func TestEvaluateApprovalAbove(t *testing.T) {
	est := Estimate{Amount: 5.0, Currency: CurrencyUSD}
	decision := EvaluateApproval(est, 1.0)
	if !decision.RequiresApproval {
		t.Fatal("expected approval required")
	}
	if decision.Reason == "" {
		t.Fatal("expected reason to be set")
	}
}

func TestEvaluateApprovalNoThreshold(t *testing.T) {
	est := Estimate{Amount: 100.0, Currency: CurrencyUSD}
	decision := EvaluateApproval(est, 0)
	if decision.RequiresApproval {
		t.Fatal("expected no approval required when threshold is 0")
	}
}

func TestEvaluateApprovalExact(t *testing.T) {
	est := Estimate{Amount: 1.0, Currency: CurrencyUSD}
	decision := EvaluateApproval(est, 1.0)
	if decision.RequiresApproval {
		t.Fatal("expected no approval required when at exact threshold")
	}
}
