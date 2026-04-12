// Package urlnorm provides URL normalization for comparing crawl and GSC URLs.
package urlnorm

import (
	"net/url"
	"sort"
	"strings"
)

// Normalize returns a canonical form of rawURL suitable for comparison across
// crawl results and Google Search Console data.
//
// Rules applied:
//   - Scheme and host are lowercased
//   - www. prefix is stripped from the host
//   - Default ports are removed (80 for http, 443 for https)
//   - Fragment (#...) is removed
//   - Trailing slash is removed, except on the root path "/"
//   - Query parameters are sorted alphabetically
//
// Returns empty string if rawURL is empty or cannot be parsed.
func Normalize(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}

	// Lowercase scheme and host.
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)

	// Remove www. prefix.
	u.Host = strings.TrimPrefix(u.Host, "www.")

	// Remove default ports.
	host := u.Hostname()
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		u.Host = host
	}

	// Remove fragment.
	u.Fragment = ""

	// Sort query parameters alphabetically.
	if u.RawQuery != "" {
		params := u.Query()
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			vals := params[k]
			sort.Strings(vals)
			for _, v := range vals {
				parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
			}
		}
		u.RawQuery = strings.Join(parts, "&")
	}

	// Remove trailing slash, but preserve the root path "/".
	if len(u.Path) > 1 {
		u.Path = strings.TrimRight(u.Path, "/")
	}

	return u.String()
}
