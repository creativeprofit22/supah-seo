package crawl

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

// extractPageData parses HTML and extracts SEO-relevant data from a page.
func extractPageData(pageURL string, statusCode int, body []byte, headers http.Header, responseTimeMs int) PageResult {
	result := PageResult{
		URL:          pageURL,
		StatusCode:   statusCode,
		ResponseTime: responseTimeMs,
	}

	// Extract HTTP header data
	if headers != nil {
		result.ContentType = headers.Get("Content-Type")
		result.XRobotsTag = headers.Get("X-Robots-Tag")
	}

	parsedBase, _ := url.Parse(pageURL)

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return result
	}

	var bodyText strings.Builder
	var inBody bool

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "html":
				for _, a := range n.Attr {
					if a.Key == "lang" {
						result.Lang = a.Val
					}
				}
			case "body":
				inBody = true
			case "title":
				if text := textContent(n); text != "" {
					result.Title = text
				}
			case "meta":
				handleMeta(n, &result)
			case "link":
				handleLink(n, &result)
			case "h1", "h2", "h3", "h4", "h5", "h6":
				level := int(n.Data[1] - '0')
				result.Headings = append(result.Headings, Heading{
					Level: level,
					Text:  textContent(n),
				})
			case "a":
				handleAnchor(n, parsedBase, &result)
			case "img":
				handleImg(n, &result)
			case "script":
				handleScript(n, &result)
			}
		}
		// Collect body text for word count
		if inBody && n.Type == html.TextNode {
			bodyText.WriteString(n.Data)
			bodyText.WriteString(" ")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && n.Data == "body" {
			inBody = false
		}
	}
	walk(doc)

	result.WordCount = countWords(bodyText.String())

	return result
}

func handleMeta(n *html.Node, result *PageResult) {
	var name, property, content string
	for _, a := range n.Attr {
		switch strings.ToLower(a.Key) {
		case "name":
			name = strings.ToLower(a.Val)
		case "property":
			property = strings.ToLower(a.Val)
		case "content":
			content = a.Val
		}
	}
	switch name {
	case "description":
		result.MetaDescription = content
	case "robots":
		result.MetaRobots = content
	case "viewport":
		result.Viewport = content
	}
	switch property {
	case "og:title":
		result.OGTitle = content
	case "og:description":
		result.OGDescription = content
	case "og:image":
		result.OGImage = content
	case "og:type":
		result.OGType = content
	}
}

func handleLink(n *html.Node, result *PageResult) {
	var rel, href, hreflang string
	for _, a := range n.Attr {
		switch strings.ToLower(a.Key) {
		case "rel":
			rel = strings.ToLower(a.Val)
		case "href":
			href = a.Val
		case "hreflang":
			hreflang = a.Val
		}
	}
	if rel == "canonical" && href != "" {
		result.Canonical = href
	}
	if rel == "alternate" && hreflang != "" {
		result.Hreflang = append(result.Hreflang, hreflang)
	}
}

func handleAnchor(n *html.Node, base *url.URL, result *PageResult) {
	var href string
	for _, a := range n.Attr {
		if a.Key == "href" {
			href = a.Val
			break
		}
	}
	if href == "" {
		return
	}

	parsed, err := url.Parse(href)
	if err != nil {
		return
	}

	resolved := base.ResolveReference(parsed)
	internal := resolved.Host == base.Host

	result.Links = append(result.Links, Link{
		Href:     resolved.String(),
		Text:     textContent(n),
		Internal: internal,
	})
}

func handleImg(n *html.Node, result *PageResult) {
	var src, alt string
	for _, a := range n.Attr {
		switch a.Key {
		case "src":
			src = a.Val
		case "alt":
			alt = a.Val
		}
	}
	result.Images = append(result.Images, Image{
		Src: src,
		Alt: alt,
	})
}

// handleScript extracts JSON-LD schema types from script tags.
func handleScript(n *html.Node, result *PageResult) {
	var isJSONLD bool
	for _, a := range n.Attr {
		if a.Key == "type" && strings.ToLower(a.Val) == "application/ld+json" {
			isJSONLD = true
		}
	}
	if !isJSONLD {
		return
	}
	raw := textContent(n)
	if raw == "" {
		return
	}
	// Try to extract @type from the JSON-LD
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		if t, ok := obj["@type"].(string); ok {
			result.SchemaTypes = append(result.SchemaTypes, t)
		}
	}
	// Also try array form
	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &arr); err == nil {
		for _, item := range arr {
			if t, ok := item["@type"].(string); ok {
				result.SchemaTypes = append(result.SchemaTypes, t)
			}
		}
	}
}

// countWords counts words in a text string.
func countWords(s string) int {
	count := 0
	inWord := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if !inWord {
				count++
				inWord = true
			}
		} else {
			inWord = false
		}
	}
	return count
}

// textContent returns the concatenated text content of a node and its children.
func textContent(n *html.Node) string {
	var sb strings.Builder
	var collect func(*html.Node)
	collect = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(n)
	return strings.TrimSpace(sb.String())
}
