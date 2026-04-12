package state

import (
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir, "https://example.com")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if s.Site != "https://example.com" {
		t.Errorf("site = %q, want %q", s.Site, "https://example.com")
	}
	if s.Initialized == "" {
		t.Error("Initialized should be set")
	}
	if s.Findings == nil || len(s.Findings) != 0 {
		t.Errorf("Findings should be an empty slice, got %v", s.Findings)
	}
	if s.History == nil || len(s.History) != 0 {
		t.Errorf("History should be an empty slice, got %v", s.History)
	}
}

func TestInitAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	if _, err := Init(dir, "https://example.com"); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	_, err := Init(dir, "https://example.com")
	if err == nil {
		t.Fatal("expected error on second Init, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error %q should contain %q", err.Error(), "already exists")
	}
}

func TestLoadAndSave(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir, "https://example.com")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	s.Findings = append(s.Findings, Finding{
		Rule:    "missing-title",
		URL:     "https://example.com/page",
		Verdict: "fail",
		Why:     "No title tag found",
		Fix:     "Add a <title> element",
	})
	if err := s.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(loaded.Findings))
	}
	f := loaded.Findings[0]
	if f.Rule != "missing-title" {
		t.Errorf("Rule = %q, want %q", f.Rule, "missing-title")
	}
	if f.URL != "https://example.com/page" {
		t.Errorf("URL = %q, want %q", f.URL, "https://example.com/page")
	}
}

func TestUpdateAudit(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir, "https://example.com")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	findings := []Finding{
		{Rule: "missing-title", URL: "https://example.com/a", Verdict: "fail"},
		{Rule: "slow-page", URL: "https://example.com/b", Verdict: "warn"},
	}
	s.UpdateAudit(80.3, 10, findings)

	if s.Score != 80.3 {
		t.Errorf("Score = %.1f, want 80.3", s.Score)
	}
	if s.PagesCrawled != 10 {
		t.Errorf("PagesCrawled = %d, want 10", s.PagesCrawled)
	}
	if len(s.Findings) != 2 {
		t.Errorf("Findings len = %d, want 2", len(s.Findings))
	}
	if s.LastCrawl == "" {
		t.Error("LastCrawl should be set after UpdateAudit")
	}
}

func TestAddHistory(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir, "https://example.com")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	s.AddHistory("crawl", "crawled 10 pages")
	s.AddHistory("audit", "scored 80.3")

	if len(s.History) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(s.History))
	}
	if s.History[0].Action != "crawl" {
		t.Errorf("History[0].Action = %q, want %q", s.History[0].Action, "crawl")
	}
	if s.History[1].Action != "audit" {
		t.Errorf("History[1].Action = %q, want %q", s.History[1].Action, "audit")
	}
	if s.History[0].Timestamp == "" {
		t.Error("History[0].Timestamp should be non-empty")
	}
	if s.History[1].Timestamp == "" {
		t.Error("History[1].Timestamp should be non-empty")
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	if Exists(dir) {
		t.Error("Exists should return false for empty dir")
	}
	if _, err := Init(dir, "https://example.com"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !Exists(dir) {
		t.Error("Exists should return true after Init")
	}
}

func TestPath(t *testing.T) {
	got := Path(".")
	want := ".supah-seo/state.json"
	if got != want {
		t.Errorf("Path(\".\") = %q, want %q", got, want)
	}
}

func TestLoadMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error loading from empty dir, got nil")
	}
}

func TestSourcesIncludesPSI(t *testing.T) {
	s := &State{
		LastCrawl: "2025-01-01T00:00:00Z",
		GSC: &GSCData{
			LastPull: "2025-01-01T00:00:00Z",
		},
		PSI: &PSIData{
			LastRun: "2025-01-01T00:00:00Z",
		},
	}

	used, missing := s.Sources()

	containsAll := func(slice []string, items ...string) bool {
		set := make(map[string]bool, len(slice))
		for _, v := range slice {
			set[v] = true
		}
		for _, item := range items {
			if !set[item] {
				return false
			}
		}
		return true
	}

	if !containsAll(used, "crawl", "gsc", "psi") {
		t.Errorf("Sources().used = %v, want crawl, gsc, psi", used)
	}
	for _, m := range missing {
		if m == "psi" {
			t.Errorf("psi should not appear in missing when PSI data is present")
		}
	}
}

func TestSourcesMissingPSI(t *testing.T) {
	s := &State{
		LastCrawl: "2025-01-01T00:00:00Z",
		GSC: &GSCData{
			LastPull: "2025-01-01T00:00:00Z",
		},
		// PSI intentionally nil
	}

	_, missing := s.Sources()

	found := false
	for _, m := range missing {
		if m == "psi" {
			found = true
			break
		}
	}
	if !found {
		t.Error("psi should appear in missing when PSI data is absent")
	}
}
