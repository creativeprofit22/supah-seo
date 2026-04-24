package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	DirName  = ".supah-seo"
	FileName = "state.json"
)

// GSCRow is a single row of GSC search analytics data.
type GSCRow struct {
	Key         string  `json:"key"`
	Clicks      float64 `json:"clicks"`
	Impressions float64 `json:"impressions"`
	CTR         float64 `json:"ctr"`
	Position    float64 `json:"position"`
}

// GSCData holds the most recent GSC pull saved to state.
type GSCData struct {
	LastPull    string   `json:"last_pull,omitempty"`
	Property    string   `json:"property,omitempty"`
	TopPages    []GSCRow `json:"top_pages,omitempty"`
	TopKeywords []GSCRow `json:"top_keywords,omitempty"`
}

// Finding is a single interpreted audit result.
type Finding struct {
	Rule    string      `json:"rule"`
	URL     string      `json:"url"`
	Value   interface{} `json:"value"`
	Verdict string      `json:"verdict"`
	Why     string      `json:"why"`
	Fix     string      `json:"fix"`
}

// HistoryEntry records an action taken by the agent or user.
type HistoryEntry struct {
	Timestamp string `json:"ts"`
	Action    string `json:"action"`
	Detail    string `json:"detail,omitempty"`
}

// PSIResult holds a single PageSpeed Insights result for one URL.
type PSIResult struct {
	URL              string  `json:"url"`
	PerformanceScore float64 `json:"performance_score"`
	LCP              float64 `json:"lcp_ms"`
	CLS              float64 `json:"cls"`
	Strategy         string  `json:"strategy"`
}

// PSIData holds all PageSpeed Insights results saved to state.
type PSIData struct {
	LastRun string      `json:"last_run,omitempty"`
	Pages   []PSIResult `json:"pages,omitempty"`
}

// SERPFeatureRecord is a SERP feature stored in state.
type SERPFeatureRecord struct {
	Type     string `json:"type"`
	Position int    `json:"position,omitempty"`
	Title    string `json:"title,omitempty"`
	URL      string `json:"url,omitempty"`
	Domain   string `json:"domain,omitempty"`
}

// SERPQueryResult stores SERP data for a single query.
type SERPQueryResult struct {
	Query            string              `json:"query"`
	HasAIOverview    bool                `json:"has_ai_overview"`
	Features         []SERPFeatureRecord `json:"features,omitempty"`
	RelatedQuestions []string            `json:"related_questions,omitempty"`
	TopDomains       []string            `json:"top_domains,omitempty"`
	OurPosition      int                 `json:"our_position"` // -1 if not found in results, 0+ for actual position
}

// SERPData holds all SERP results saved to state.
type SERPData struct {
	LastRun string            `json:"last_run,omitempty"`
	Queries []SERPQueryResult `json:"queries,omitempty"`
}

// LabsKeyword stores keyword intelligence from DataForSEO Labs.
type LabsKeyword struct {
	Keyword      string  `json:"keyword"`
	SearchVolume int     `json:"search_volume"`
	Difficulty   float64 `json:"difficulty"` // 0-100
	CPC          float64 `json:"cpc,omitempty"`
	Intent       string  `json:"intent,omitempty"`   // informational, navigational, commercial, transactional
	Position     int     `json:"position,omitempty"` // current ranking position if from ranked-keywords
}

// LabsData holds Labs intelligence saved to state.
type LabsData struct {
	LastRun     string        `json:"last_run,omitempty"`
	Target      string        `json:"target,omitempty"` // domain that was analyzed
	Keywords    []LabsKeyword `json:"keywords,omitempty"`
	Competitors []string      `json:"competitors,omitempty"` // top competitor domains
}

// BacklinksData holds backlink profile data saved to state.
type BacklinksData struct {
	LastRun               string   `json:"last_run,omitempty"`
	Target                string   `json:"target,omitempty"`
	TotalBacklinks        int64    `json:"total_backlinks"`
	TotalReferringDomains int64    `json:"total_referring_domains"`
	BrokenBacklinks       int64    `json:"broken_backlinks"`
	Rank                  int      `json:"rank"`
	DoFollow              int64    `json:"dofollow"`
	NoFollow              int64    `json:"nofollow"`
	SpamScore             float64  `json:"spam_score"`
	TopReferrers          []string `json:"top_referrers,omitempty"` // top 10 referring domain names
	GapDomains            []string `json:"gap_domains,omitempty"`   // top 20 gap domains from backlinks gap analysis
}

// State is the single project file the AI reads and writes.
type State struct {
	Site           string          `json:"site"`
	Initialized    string          `json:"initialized"`
	LastCrawl      string          `json:"last_crawl,omitempty"`
	Score          float64         `json:"score,omitempty"`
	PagesCrawled   int             `json:"pages_crawled,omitempty"`
	Findings       []Finding       `json:"findings,omitempty"`
	MergedFindings json.RawMessage `json:"merged_findings,omitempty"`
	LastAnalysis   string          `json:"last_analysis,omitempty"`
	GSC            *GSCData        `json:"gsc,omitempty"`
	PSI            *PSIData        `json:"psi,omitempty"`
	SERP           *SERPData       `json:"serp,omitempty"`
	Labs           *LabsData       `json:"labs,omitempty"`
	Backlinks      *BacklinksData  `json:"backlinks,omitempty"`
	History        []HistoryEntry  `json:"history,omitempty"`
}

// Path returns the state.json path relative to a working directory.
func Path(dir string) string {
	return filepath.Join(dir, DirName, FileName)
}

// Init creates a new .supah-seo/state.json for a site.
func Init(dir string, siteURL string) (*State, error) {
	supahSeoDir := filepath.Join(dir, DirName)
	if err := os.MkdirAll(supahSeoDir, 0755); err != nil {
		return nil, fmt.Errorf("create .supah-seo dir: %w", err)
	}

	path := filepath.Join(supahSeoDir, FileName)
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("state.json already exists, run supah-seo status to view")
	}

	s := &State{
		Site:        siteURL,
		Initialized: time.Now().UTC().Format(time.RFC3339),
		Findings:    []Finding{},
		History:     []HistoryEntry{},
	}

	if err := s.Save(dir); err != nil {
		return nil, err
	}
	return s, nil
}

// Load reads state.json from disk.
func Load(dir string) (*State, error) {
	data, err := os.ReadFile(Path(dir))
	if err != nil {
		return nil, fmt.Errorf("read state.json: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state.json: %w", err)
	}
	return &s, nil
}

// Save writes state.json to disk.
func (s *State) Save(dir string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	return os.WriteFile(Path(dir), data, 0644)
}

// Exists returns true if .supah-seo/state.json exists in the given directory.
func Exists(dir string) bool {
	_, err := os.Stat(Path(dir))
	return err == nil
}

// UpdateAudit replaces findings and score from an audit run.
func (s *State) UpdateAudit(score float64, pagesCrawled int, findings []Finding) {
	s.LastCrawl = time.Now().UTC().Format(time.RFC3339)
	s.Score = score
	s.PagesCrawled = pagesCrawled
	s.Findings = findings
}

// Sources returns which data sources have contributed to this state.
// used contains sources with data present; missing contains sources not yet populated.
func (s *State) Sources() (used []string, missing []string) {
	if s.LastCrawl != "" {
		used = append(used, "crawl")
	}

	if s.GSC != nil && s.GSC.LastPull != "" {
		used = append(used, "gsc")
	} else {
		missing = append(missing, "gsc")
	}

	if s.PSI != nil && s.PSI.LastRun != "" {
		used = append(used, "psi")
	} else {
		missing = append(missing, "psi")
	}

	if s.SERP != nil && s.SERP.LastRun != "" {
		used = append(used, "serp")
	} else {
		missing = append(missing, "serp")
	}

	if s.Labs != nil && s.Labs.LastRun != "" {
		used = append(used, "labs")
	} else {
		missing = append(missing, "labs")
	}

	if s.Backlinks != nil && s.Backlinks.LastRun != "" {
		used = append(used, "backlinks")
	} else {
		missing = append(missing, "backlinks")
	}

	return used, missing
}

// UpsertPSI replaces the PSI data in state.
func (s *State) UpsertPSI(data PSIData) {
	data.LastRun = time.Now().UTC().Format(time.RFC3339)
	s.PSI = &data
}

const maxHistoryEntries = 200

// AddHistory appends an entry to the history log, keeping at most the last
// maxHistoryEntries entries to prevent unbounded state growth.
func (s *State) AddHistory(action, detail string) {
	s.History = append(s.History, HistoryEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Action:    action,
		Detail:    detail,
	})
	if len(s.History) > maxHistoryEntries {
		s.History = s.History[len(s.History)-maxHistoryEntries:]
	}
}
