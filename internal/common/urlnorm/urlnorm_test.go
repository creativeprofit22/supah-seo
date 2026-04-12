package urlnorm_test

import (
	"testing"

	"github.com/supah-seo/supah-seo/internal/common/urlnorm"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "www vs non-www match",
			input: "https://www.coastalprograms.com/blog/",
			want:  "https://coastalprograms.com/blog",
		},
		{
			name:  "non-www already matches",
			input: "https://coastalprograms.com/blog",
			want:  "https://coastalprograms.com/blog",
		},
		{
			name:  "trailing slash removed",
			input: "https://example.com/about/",
			want:  "https://example.com/about",
		},
		{
			name:  "root path preserves single slash",
			input: "https://example.com/",
			want:  "https://example.com/",
		},
		{
			name:  "root path without slash",
			input: "https://example.com",
			want:  "https://example.com",
		},
		{
			name:  "fragment removed",
			input: "https://example.com/page#section",
			want:  "https://example.com/page",
		},
		{
			name:  "fragment removed with trailing slash",
			input: "https://example.com/page/#section",
			want:  "https://example.com/page",
		},
		{
			name:  "scheme lowercased",
			input: "HTTPS://Example.COM/path",
			want:  "https://example.com/path",
		},
		{
			name:  "query params sorted alphabetically",
			input: "https://example.com/search?z=last&a=first&m=middle",
			want:  "https://example.com/search?a=first&m=middle&z=last",
		},
		{
			name:  "default http port 80 removed",
			input: "http://example.com:80/page",
			want:  "http://example.com/page",
		},
		{
			name:  "default https port 443 removed",
			input: "https://example.com:443/page",
			want:  "https://example.com/page",
		},
		{
			name:  "non-default port preserved",
			input: "https://example.com:8443/page",
			want:  "https://example.com:8443/page",
		},
		{
			name:  "empty string returns empty",
			input: "",
			want:  "",
		},
		{
			name:  "invalid URL returns empty",
			input: "not a url",
			want:  "",
		},
		{
			name:  "relative URL returns empty",
			input: "/relative/path",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := urlnorm.Normalize(tc.input)
			if got != tc.want {
				t.Errorf("Normalize(%q)\n  got  %q\n  want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestWWWAndNonWWWMatch confirms that www and non-www variants normalize to
// the same string, making them directly comparable.
func TestWWWAndNonWWWMatch(t *testing.T) {
	www := urlnorm.Normalize("https://www.coastalprograms.com/blog/")
	noWWW := urlnorm.Normalize("https://coastalprograms.com/blog")
	if www != noWWW {
		t.Errorf("www and non-www did not match: %q vs %q", www, noWWW)
	}
}
