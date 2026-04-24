package render

import "time"

// ReportView is the top-level data the HTML templates bind to.
type ReportView struct {
	// Meta / branding
	GeneratedAt   time.Time
	AgencyName    string
	AgencyLogoB64 string
	CTAURL        string
	CTALabel      string

	// Prospect identification
	ProspectName  string
	ProspectSite  string
	Location      string
	CurrentAgency string // name of their existing agency if detected

	// Headline numbers
	Score     float64
	Verdict   string
	MoneyLine string

	// Revenue modelling
	Revenue RevenueModel

	// Findings ranked by priority
	TopFindings   []Finding // top 5 for client report
	OtherFindings []Finding // rest of the notable findings
	AgencyNotes   []Finding // findings surfaced in agency report only

	// Context blocks
	GBP         GBPAudit
	Competitors []CompetitorEntry
	SERPIntel   SERPIntel
	Backlinks   BacklinksIntel
	Pricing     []PricingGap

	// Meta counters for the agency view
	PagesCrawled int
	TotalIssues  int

	// Raw data surfaces for the agency report
	AllFindings    []RawFinding
	FindingsByURL  []PageFindings
	AllPSIPages    []PSIPage
	AllKeywords    []RankedKeyword
	AllSERPQueries []SERPQueryView
	KeywordBuckets KeywordBuckets

	// Roadmap / phasing
	Phases []Phase

	// Industry context
	IndustryName string
	InsightCards []InsightCard
	PricingAngle string
}

// RawFinding is a single audit rule result surfaced verbatim in the agency report.
type RawFinding struct {
	Rule    string
	URL     string
	Value   string
	Verdict string
	Why     string
	Fix     string
}

// PageFindings groups all raw findings for a single crawled URL.
type PageFindings struct {
	URL      string
	Issues   []RawFinding
	Critical int
	High     int
}

// PSIPage is a per-URL PageSpeed Insights result formatted for display.
type PSIPage struct {
	URL         string
	Strategy    string
	Performance float64
	LCPSeconds  float64
	LCPVerdict  string // good | needs improvement | poor
	CLS         float64
	FCPSeconds  float64
	TBTSeconds  float64
	Tier        string // "Good", "Needs Work", "Poor"
}

// RankedKeyword is a single ranked keyword shown in the agency table.
type RankedKeyword struct {
	Keyword     string
	Volume      int
	Difficulty  float64
	Position    int
	Intent      string
	CPC         float64
	Opportunity string // narrative: "easy win", "page 2 push", "out of reach"
}

// SERPQueryView renders SERP intel for one money keyword.
type SERPQueryView struct {
	Query            string
	OurPosition      int
	HasAIOverview    bool
	HasLocalPack     bool
	HasPAA           bool
	LocalPackTop3    []string
	RelatedQuestions []string
}

// KeywordBuckets groups ranked keywords by strategic bucket.
type KeywordBuckets struct {
	EasyWins    []RankedKeyword // pos 4-10 with volume > 40
	Page2Push   []RankedKeyword // pos 11-30 with volume > 40
	DeepStretch []RankedKeyword // pos 31+ with volume > 40
	Branded     []RankedKeyword // navigational / brand
}

// Phase is one block in the 30/60/90 day roadmap.
type Phase struct {
	Window       string // "Days 1-30"
	Theme        string
	Focus        string
	Deliverables []string
	KPIs         []string
}

// Finding is a single pain point rendered in a report section.
// Fields are designed so the same struct feeds both client and agency templates,
// with agency-only detail held on the Technical block.
type Finding struct {
	ID        string
	Severity  string // critical | high | medium | low
	Headline  string // plain English title shown to the client
	What      string // short plain English explanation
	Why       string // business consequence
	Dream     string // what fixed looks like (optional)
	Effort    string // "a few hours" / "weekend project" / "ongoing"
	Impact    string // dollar or bookings framing
	TechName  string // credibility-signal vocabulary ("Core Web Vitals", "structured data")
	Technical Technical
}

// Technical carries the agency-only under-the-hood detail.
type Technical struct {
	URLs      []string
	Metrics   []Metric
	FixNotes  string // short bullet-style direction (only shown in agency template)
	Priority  int    // 10..100
	EffortHrs int    // rough
}

type Metric struct {
	Label string
	Value string
}

// GBPAudit collects Google Business Profile findings.
// Populated optionally via CLI flags or manual entry.
type GBPAudit struct {
	Enabled       bool
	ProfileURL    string
	Name          string
	ReviewCount   int
	Rating        float64
	PhotoCount    int
	LastPostDays  int
	AttributesSet int
	AttributesMax int
	Issues        []string
}

type CompetitorEntry struct {
	Domain         string
	Name           string
	AppearsInQuery []string // queries where they rank in local pack or top 3
}

type SERPIntel struct {
	QueriesAnalyzed   int
	NotInLocalPack    int      // queries where our_position == -1 and local_pack features exist
	AIOverviewQueries []string // money queries with AI Overview
	PAAQuestions      []string // unique list across all analysed queries
	TopMoneyMisses    []KeywordMiss
}

type KeywordMiss struct {
	Keyword       string
	Volume        int
	OurPosition   int
	TopCompetitor string
}

type BacklinksIntel struct {
	RefDomains     int64
	TotalBacklinks int64
	SpamScore      float64
	GapDomains     []string
}

type PricingGap struct {
	Service       string
	TheirPrice    string
	MarketPrice   string
	MonthlyMisses string // narrative like "$600 per job below market"
}

type RevenueModel struct {
	AvgTicket         float64
	CloseRate         float64
	MonthlyLowEnd     float64
	MonthlyHighEnd    float64
	AnnualLowEnd      float64
	AnnualHighEnd     float64
	PremiumUpsideNote string // narrative on premium pricing upside
}
