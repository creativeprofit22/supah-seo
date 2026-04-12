package audit

import (
	"fmt"

	"github.com/supah-seo/supah-seo/internal/common/urlnorm"
	"github.com/supah-seo/supah-seo/internal/crawl"
)

const (
	maxTitleLength       = 60
	maxDescriptionLength = 160
	maxResponseTimeMs    = 2000
	minWordCount         = 300
)

func checkTitle(page crawl.PageResult) []Issue {
	var issues []Issue
	if page.Title == "" {
		issues = append(issues, Issue{
			Rule:     "title-missing",
			Severity: SeverityError,
			URL:      page.URL,
			Message:  "Page is missing a title tag",
			Why:      "The title tag is the most important on-page SEO element — Google uses it as the search result headline",
			Fix:      "Add a unique, descriptive <title> under 60 characters",
		})
	} else if len(page.Title) > maxTitleLength {
		issues = append(issues, Issue{
			Rule:     "title-too-long",
			Severity: SeverityWarning,
			URL:      page.URL,
			Message:  fmt.Sprintf("Title exceeds %d characters (%d)", maxTitleLength, len(page.Title)),
			Detail:   page.Title,
			Why:      "Google truncates titles over ~60 characters in search results",
			Fix:      "Shorten to under 60 characters, front-load important keywords",
		})
	}
	return issues
}

func checkMetaDescription(page crawl.PageResult) []Issue {
	var issues []Issue
	if page.MetaDescription == "" {
		issues = append(issues, Issue{
			Rule:     "meta-description-missing",
			Severity: SeverityWarning,
			URL:      page.URL,
			Message:  "Page is missing a meta description",
			Why:      "Google often uses the meta description as the search result snippet",
			Fix:      "Add a compelling meta description under 160 characters",
		})
	} else if len(page.MetaDescription) > maxDescriptionLength {
		issues = append(issues, Issue{
			Rule:     "meta-description-too-long",
			Severity: SeverityWarning,
			URL:      page.URL,
			Message:  fmt.Sprintf("Meta description exceeds %d characters (%d)", maxDescriptionLength, len(page.MetaDescription)),
			Detail:   page.MetaDescription,
			Why:      "Google truncates descriptions over ~160 characters — the message gets cut off",
			Fix:      "Shorten to under 160 characters, include a clear call to action",
		})
	}
	return issues
}

func checkH1(page crawl.PageResult) []Issue {
	var issues []Issue
	h1Count := 0
	for _, h := range page.Headings {
		if h.Level == 1 {
			h1Count++
		}
	}
	if h1Count == 0 {
		issues = append(issues, Issue{
			Rule:     "h1-missing",
			Severity: SeverityError,
			URL:      page.URL,
			Message:  "Page is missing an H1 heading",
			Why:      "The H1 tells Google what the page is about — missing it weakens topical relevance",
			Fix:      "Add a single H1 tag with the primary keyword for the page",
		})
	} else if h1Count > 1 {
		issues = append(issues, Issue{
			Rule:     "h1-multiple",
			Severity: SeverityWarning,
			URL:      page.URL,
			Message:  fmt.Sprintf("Page has %d H1 headings (should have exactly 1)", h1Count),
			Why:      "Multiple H1s dilute the page focus and confuse search engines",
			Fix:      "Keep one H1 and convert others to H2 or H3",
		})
	}
	return issues
}

func checkImageAlt(page crawl.PageResult) []Issue {
	var issues []Issue
	for _, img := range page.Images {
		if img.Alt == "" {
			issues = append(issues, Issue{
				Rule:     "img-alt-missing",
				Severity: SeverityWarning,
				URL:      page.URL,
				Message:  "Image is missing alt text",
				Detail:   img.Src,
				Why:      "Alt text is required for accessibility and helps Google understand images",
				Fix:      "Add descriptive alt text that describes the image content",
			})
		}
	}
	return issues
}

func checkCanonical(page crawl.PageResult) []Issue {
	var issues []Issue
	if page.Canonical == "" {
		issues = append(issues, Issue{
			Rule:     "canonical-missing",
			Severity: SeverityInfo,
			URL:      page.URL,
			Message:  "Page is missing a canonical tag",
			Why:      "Without a canonical, Google may treat URL variants as duplicate content",
			Fix:      "Add <link rel=\"canonical\" href=\"...\"> pointing to the preferred URL",
		})
	}
	return issues
}

func checkStatusCode(page crawl.PageResult) []Issue {
	var issues []Issue
	if page.StatusCode >= 400 {
		severity := SeverityWarning
		if page.StatusCode >= 500 {
			severity = SeverityError
		}
		issues = append(issues, Issue{
			Rule:     "broken-page",
			Severity: severity,
			URL:      page.URL,
			Message:  fmt.Sprintf("Page returned HTTP %d", page.StatusCode),
		})
	}
	return issues
}

func checkViewport(page crawl.PageResult) []Issue {
	if page.Viewport == "" {
		return []Issue{{
			Rule:     "viewport-missing",
			Severity: SeverityError,
			URL:      page.URL,
			Message:  "Page is missing a viewport meta tag",
			Why:      "Without a viewport tag, mobile devices render desktop layout — Google penalises this",
			Fix:      "Add <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">",
		}}
	}
	return nil
}

func checkOpenGraph(page crawl.PageResult) []Issue {
	var issues []Issue
	if page.OGTitle == "" {
		issues = append(issues, Issue{
			Rule:     "og-title-missing",
			Severity: SeverityWarning,
			URL:      page.URL,
			Message:  "Page is missing og:title tag",
			Why:      "Social shares will use a generic or wrong title without og:title",
			Fix:      "Add <meta property=\"og:title\" content=\"Your Page Title\">",
		})
	}
	if page.OGDescription == "" {
		issues = append(issues, Issue{
			Rule:     "og-description-missing",
			Severity: SeverityWarning,
			URL:      page.URL,
			Message:  "Page is missing og:description tag",
			Why:      "Social shares won't show a description preview",
			Fix:      "Add <meta property=\"og:description\" content=\"Short description\">",
		})
	}
	if page.OGImage == "" {
		issues = append(issues, Issue{
			Rule:     "og-image-missing",
			Severity: SeverityWarning,
			URL:      page.URL,
			Message:  "Page is missing og:image tag",
			Why:      "Social shares appear without a preview image — lower click rates",
			Fix:      "Add <meta property=\"og:image\" content=\"https://...\">",
		})
	}
	return issues
}

func checkResponseTime(page crawl.PageResult) []Issue {
	if page.ResponseTime > maxResponseTimeMs {
		return []Issue{{
			Rule:     "slow-response",
			Severity: SeverityWarning,
			URL:      page.URL,
			Message:  fmt.Sprintf("Response time %dms exceeds %dms target", page.ResponseTime, maxResponseTimeMs),
			Why:      "Slow pages rank lower and lose visitors to bounce",
			Fix:      "Check server response time, enable caching, compress assets",
		}}
	}
	return nil
}

func checkWordCount(page crawl.PageResult) []Issue {
	if page.WordCount < minWordCount && page.WordCount > 0 {
		return []Issue{{
			Rule:     "thin-content",
			Severity: SeverityWarning,
			URL:      page.URL,
			Message:  fmt.Sprintf("Page has only %d words (target: %d+)", page.WordCount, minWordCount),
			Why:      "Thin content pages struggle to rank — Google prefers comprehensive pages",
			Fix:      "Add more relevant content, answer related questions, expand on the topic",
		}}
	}
	return nil
}

func checkSchema(page crawl.PageResult) []Issue {
	if len(page.SchemaTypes) == 0 {
		return []Issue{{
			Rule:     "schema-missing",
			Severity: SeverityInfo,
			URL:      page.URL,
			Message:  "Page has no structured data (JSON-LD)",
			Why:      "Structured data enables rich results in Google — stars, FAQs, breadcrumbs",
			Fix:      "Add JSON-LD structured data relevant to the page type",
		}}
	}

	var issues []Issue
	schemaSet := make(map[string]bool, len(page.SchemaTypes))
	for _, st := range page.SchemaTypes {
		schemaSet[st] = true
	}

	// FAQPage: should have H2 headings for question/answer structure.
	if schemaSet["FAQPage"] {
		h2Count := 0
		for _, h := range page.Headings {
			if h.Level == 2 {
				h2Count++
			}
		}
		if h2Count == 0 {
			issues = append(issues, Issue{
				Rule:     "schema-faq-no-questions",
				Severity: SeverityWarning,
				URL:      page.URL,
				Message:  "FAQPage schema found but page has no H2 headings",
				Why:      "FAQPage schema works best when the page has clear question/answer pairs in headings",
				Fix:      "Structure FAQ content with H2 headings as questions",
			})
		}
	}

	// Article/BlogPosting: should have substantial content.
	if schemaSet["Article"] || schemaSet["BlogPosting"] {
		if page.WordCount < minWordCount {
			issues = append(issues, Issue{
				Rule:     "schema-article-thin",
				Severity: SeverityWarning,
				URL:      page.URL,
				Message:  fmt.Sprintf("Article schema on page with only %d words (target: %d+)", page.WordCount, minWordCount),
				Why:      "Article schema on thin content pages may not get rich results",
				Fix:      "Expand the article content to at least 300 words",
			})
		}
	}

	// Informational detections for LocalBusiness, Product, BreadcrumbList.
	for _, st := range []string{"LocalBusiness", "Product", "BreadcrumbList"} {
		if schemaSet[st] {
			issues = append(issues, Issue{
				Rule:     "schema-detected-" + st,
				Severity: SeverityInfo,
				URL:      page.URL,
				Message:  fmt.Sprintf("%s schema detected", st),
			})
		}
	}

	// Article schema on homepage is unusual — usually should be Organization/WebSite.
	if isHomepage(page.URL) && (schemaSet["Article"] || schemaSet["BlogPosting"]) {
		issues = append(issues, Issue{
			Rule:     "schema-article-on-homepage",
			Severity: SeverityWarning,
			URL:      page.URL,
			Message:  "Article schema found on the homepage",
			Why:      "Homepages typically use Organization or WebSite schema — Article may confuse Google",
			Fix:      "Replace Article schema with Organization or WebSite schema on the homepage",
		})
	}

	// Conflicting schemas: Article + Product.
	if (schemaSet["Article"] || schemaSet["BlogPosting"]) && schemaSet["Product"] {
		issues = append(issues, Issue{
			Rule:     "schema-conflict-article-product",
			Severity: SeverityWarning,
			URL:      page.URL,
			Message:  "Page has both Article and Product schema",
			Why:      "Conflicting schema types may confuse Google about the page purpose",
			Fix:      "Use a single primary schema type that best represents the page content",
		})
	}

	return issues
}

// isHomepage checks whether a URL is a site root (e.g. https://example.com or https://example.com/).
func isHomepage(rawURL string) bool {
	// Strip scheme.
	u := rawURL
	for _, prefix := range []string{"https://", "http://"} {
		if len(u) > len(prefix) && u[:len(prefix)] == prefix {
			u = u[len(prefix):]
			break
		}
	}
	// After removing scheme, a homepage has no path or just "/".
	for i := 0; i < len(u); i++ {
		if u[i] == '/' {
			return u[i:] == "/" || u[i:] == ""
		}
		if u[i] == '?' || u[i] == '#' {
			return true
		}
	}
	return true // no slash at all, e.g. "example.com"
}

func checkMetaRobots(page crawl.PageResult) []Issue {
	if page.MetaRobots != "" {
		lower := page.MetaRobots
		if contains(lower, "noindex") {
			return []Issue{{
				Rule:     "noindex-detected",
				Severity: SeverityError,
				URL:      page.URL,
				Message:  "Page has noindex directive — Google will not index this page",
				Why:      "This page will not appear in search results",
				Fix:      "Remove noindex if this page should be searchable",
			}}
		}
	}
	if page.XRobotsTag != "" && contains(page.XRobotsTag, "noindex") {
		return []Issue{{
			Rule:     "x-robots-noindex",
			Severity: SeverityError,
			URL:      page.URL,
			Message:  "X-Robots-Tag header contains noindex",
			Why:      "This page will not appear in search results",
			Fix:      "Remove the X-Robots-Tag noindex header on the server",
		}}
	}
	return nil
}

func checkLang(page crawl.PageResult) []Issue {
	if page.Lang == "" {
		return []Issue{{
			Rule:     "lang-missing",
			Severity: SeverityWarning,
			URL:      page.URL,
			Message:  "HTML lang attribute is missing",
			Why:      "Helps search engines understand the page language for correct regional results",
			Fix:      "Add lang attribute to <html> tag, e.g. <html lang=\"en\">",
		}}
	}
	return nil
}

func checkDuplicateTitles(pages []crawl.PageResult) []Issue {
	titleURLs := make(map[string][]string)
	for _, page := range pages {
		if page.Title == "" {
			continue
		}
		titleURLs[page.Title] = append(titleURLs[page.Title], page.URL)
	}

	var issues []Issue
	for title, urls := range titleURLs {
		if len(urls) < 2 {
			continue
		}
		for _, u := range urls {
			issues = append(issues, Issue{
				Rule:     "duplicate-title",
				Severity: SeverityWarning,
				URL:      u,
				Message:  fmt.Sprintf("Title is duplicated across %d pages", len(urls)),
				Detail:   title,
				Why:      "Duplicate titles confuse Google about which page to rank — they compete against each other",
				Fix:      "Write a unique, specific title for each page",
			})
		}
	}
	return issues
}

func checkDuplicateDescriptions(pages []crawl.PageResult) []Issue {
	descURLs := make(map[string][]string)
	for _, page := range pages {
		if page.MetaDescription == "" {
			continue
		}
		descURLs[page.MetaDescription] = append(descURLs[page.MetaDescription], page.URL)
	}

	var issues []Issue
	for desc, urls := range descURLs {
		if len(urls) < 2 {
			continue
		}
		detail := desc
		if len(detail) > 100 {
			detail = detail[:100]
		}
		for _, u := range urls {
			issues = append(issues, Issue{
				Rule:     "duplicate-description",
				Severity: SeverityWarning,
				URL:      u,
				Message:  fmt.Sprintf("Meta description is duplicated across %d pages", len(urls)),
				Detail:   detail,
				Why:      "Duplicate descriptions waste the chance to write compelling, page-specific snippets for search results",
				Fix:      "Write a unique meta description for each page that describes its specific content",
			})
		}
	}
	return issues
}

func checkOrphanPages(pages []crawl.PageResult) []Issue {
	if len(pages) == 0 {
		return nil
	}

	// Build set of all crawled page URLs (normalized).
	crawledSet := make(map[string]string, len(pages)) // normalized -> original URL
	for _, page := range pages {
		norm := urlnorm.Normalize(page.URL)
		if norm != "" {
			crawledSet[norm] = page.URL
		}
	}

	// Determine the homepage: normalize the first page URL.
	homepageNorm := urlnorm.Normalize(pages[0].URL)

	// Build set of all internal link targets across all pages (normalized).
	linkedSet := make(map[string]struct{})
	for _, page := range pages {
		for _, link := range page.Links {
			if !link.Internal {
				continue
			}
			norm := urlnorm.Normalize(link.Href)
			if norm != "" {
				linkedSet[norm] = struct{}{}
			}
		}
	}

	var issues []Issue
	for norm, originalURL := range crawledSet {
		// Skip the homepage — it's the entry point and doesn't need an inbound link.
		if norm == homepageNorm {
			continue
		}
		if _, linked := linkedSet[norm]; !linked {
			issues = append(issues, Issue{
				Rule:     "orphan-page",
				Severity: SeverityWarning,
				URL:      originalURL,
				Message:  "No other page links to this page",
				Why:      "Orphan pages are hard for Google to discover and may not get indexed or ranked",
				Fix:      "Add internal links from related pages to this page",
			})
		}
	}
	return issues
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
