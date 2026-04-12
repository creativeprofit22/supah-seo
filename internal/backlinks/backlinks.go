package backlinks

// Summary holds the backlink profile overview for a domain.
type Summary struct {
	Target                   string  `json:"target"`
	TotalBacklinks           int64   `json:"total_backlinks"`
	TotalReferringDomains    int64   `json:"total_referring_domains"`
	TotalReferringPages      int64   `json:"total_referring_pages"`
	BrokenBacklinks          int64   `json:"broken_backlinks"`
	ReferringDomainsNofollow int64   `json:"referring_domains_nofollow"`
	BacklinksSpamScore       float64 `json:"backlinks_spam_score"`
	Rank                     int     `json:"rank"` // DataForSEO domain rank
	// Link type breakdown
	DoFollowLinks int64 `json:"dofollow"`
	NoFollowLinks int64 `json:"nofollow"`
}

// Backlink represents a single inbound link.
type Backlink struct {
	DomainFrom     string `json:"domain_from"`
	URLFrom        string `json:"url_from"`
	URLTo          string `json:"url_to"`
	Anchor         string `json:"anchor"`
	IsDoFollow     bool   `json:"is_dofollow"`
	PageFromRank   int    `json:"page_from_rank"`
	DomainFromRank int    `json:"domain_from_rank"`
	IsNew          bool   `json:"is_new"`
	IsLost         bool   `json:"is_lost"`
	FirstSeen      string `json:"first_seen,omitempty"`
	LastSeen       string `json:"last_seen,omitempty"`
}

// ReferringDomain represents a domain that links to the target.
type ReferringDomain struct {
	Domain        string `json:"domain"`
	Rank          int    `json:"rank"`
	Backlinks     int64  `json:"backlinks"`
	DoFollowLinks int64  `json:"dofollow"`
	FirstSeen     string `json:"first_seen,omitempty"`
}

// CompetitorBacklinks represents a competitor found via backlink overlap.
type CompetitorBacklinks struct {
	Domain                 string `json:"domain"`
	CommonReferringDomains int64  `json:"common_referring_domains"`
	Rank                   int    `json:"rank"`
}

// BacklinkGap represents a domain that links to competitors but not to you.
type BacklinkGap struct {
	Domain             string `json:"domain"`
	TotalLinks         int    `json:"total_links"`
	CompetitorsCovered int    `json:"competitors_covered"`
}
