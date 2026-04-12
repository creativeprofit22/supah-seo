package crawl

// Request defines inputs for a crawl operation.
type Request struct {
	TargetURL string
	Depth     int
	MaxPages  int
	UserAgent string
}

// Result defines outputs for a crawl operation.
type Result struct {
	TargetURL string       `json:"target_url"`
	Pages     []PageResult `json:"pages"`
	Errors    []CrawlError `json:"errors,omitempty"`
}

// PageResult holds extracted data from a single crawled page.
type PageResult struct {
	URL             string    `json:"url"`
	StatusCode      int       `json:"status_code"`
	Title           string    `json:"title"`
	MetaDescription string    `json:"meta_description"`
	Canonical       string    `json:"canonical"`
	Headings        []Heading `json:"headings,omitempty"`
	Links           []Link    `json:"links,omitempty"`
	Images          []Image   `json:"images,omitempty"`

	// Meta directives
	MetaRobots string `json:"meta_robots,omitempty"`
	Viewport   string `json:"viewport,omitempty"`

	// Open Graph
	OGTitle       string `json:"og_title,omitempty"`
	OGDescription string `json:"og_description,omitempty"`
	OGImage       string `json:"og_image,omitempty"`
	OGType        string `json:"og_type,omitempty"`

	// Language
	Lang     string   `json:"lang,omitempty"`
	Hreflang []string `json:"hreflang,omitempty"`

	// Content
	WordCount   int      `json:"word_count"`
	SchemaTypes []string `json:"schema_types,omitempty"`

	// HTTP response data
	ContentType  string `json:"content_type,omitempty"`
	XRobotsTag   string `json:"x_robots_tag,omitempty"`
	ResponseTime int    `json:"response_time_ms,omitempty"`
}

// Heading represents an HTML heading element.
type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
}

// Link represents an anchor element found on a page.
type Link struct {
	Href     string `json:"href"`
	Text     string `json:"text"`
	Internal bool   `json:"internal"`
}

// Image represents an img element found on a page.
type Image struct {
	Src string `json:"src"`
	Alt string `json:"alt"`
}

// CrawlError records an error encountered while crawling a specific URL.
type CrawlError struct {
	URL     string `json:"url"`
	Message string `json:"message"`
}
