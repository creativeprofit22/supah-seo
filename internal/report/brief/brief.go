// Package brief turns PAA questions captured during SERP analysis into
// content briefs an agency can hand to a writer or strategist.
//
// The input is a .supah-seo/state.json. The output is a single markdown
// document with one brief per question, plus an overview section at the top.
package brief

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/supah-seo/supah-seo/internal/state"
)

// Options tune the brief output.
type Options struct {
	ProspectName string
	Industry     string // optional, may shape schema suggestions
	MaxBriefs    int    // cap the number of briefs generated (0 = no cap)
}

// Brief is one generated content brief.
type Brief struct {
	Question        string
	Slug            string
	SuggestedTitle  string
	PrimaryH1       string
	WordCountTarget string
	H2Outline       []string
	RelatedKeywords []RelatedKeyword
	SchemaTypes     []string
	LinkingHints    []string
	MetaDescription string
}

// RelatedKeyword is a keyword the writer should naturally weave into the page.
type RelatedKeyword struct {
	Keyword      string
	Volume       int
	Difficulty   float64
	Relationship string // "target", "related", "long-tail"
}

// Bundle is the full output document.
type Bundle struct {
	ProspectName   string
	GeneratedAt    time.Time
	SourceSite     string
	TotalBriefs    int
	TotalWords     int // estimated across all briefs
	ClusterSummary []ClusterEntry
	Briefs         []Brief
}

// ClusterEntry groups related questions by theme for the overview section.
type ClusterEntry struct {
	Theme     string
	Questions []string
}

// Generate produces a Bundle from the given state.
func Generate(s *state.State, opt Options) Bundle {
	b := Bundle{
		ProspectName: fallback(opt.ProspectName, domainFromURL(s.Site)),
		GeneratedAt:  time.Now(),
		SourceSite:   s.Site,
	}

	questions := collectPAA(s)
	if opt.MaxBriefs > 0 && len(questions) > opt.MaxBriefs {
		questions = questions[:opt.MaxBriefs]
	}

	// Index Labs keywords for relationship discovery.
	var labsKeywords []state.LabsKeyword
	if s.Labs != nil {
		labsKeywords = s.Labs.Keywords
	}

	for _, q := range questions {
		brief := buildBrief(q, labsKeywords, opt)
		b.Briefs = append(b.Briefs, brief)
		b.TotalWords += estimateWords(brief.WordCountTarget)
	}
	b.TotalBriefs = len(b.Briefs)
	b.ClusterSummary = clusterQuestions(questions)
	return b
}

// Markdown renders the bundle as a single markdown document.
func (b Bundle) Markdown() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# Content briefs: %s\n\n", b.ProspectName)
	fmt.Fprintf(&sb, "_Generated %s from `%s`_\n\n", b.GeneratedAt.Format("2 January 2006"), b.SourceSite)

	sb.WriteString("## Overview\n\n")
	fmt.Fprintf(&sb, "- **Briefs:** %d\n", b.TotalBriefs)
	fmt.Fprintf(&sb, "- **Estimated total words to produce:** ~%s\n", humanInt(b.TotalWords))
	sb.WriteString("- **Source:** People Also Ask questions captured during the SERP analysis pass\n\n")

	if len(b.ClusterSummary) > 0 {
		sb.WriteString("### Topic clusters\n\n")
		for _, c := range b.ClusterSummary {
			fmt.Fprintf(&sb, "**%s** (%d questions)\n", c.Theme, len(c.Questions))
			for _, q := range c.Questions {
				fmt.Fprintf(&sb, "- %s\n", q)
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("---\n\n## Briefs\n\n")

	for i, brief := range b.Briefs {
		fmt.Fprintf(&sb, "### %d. %s\n\n", i+1, brief.Question)
		fmt.Fprintf(&sb, "**Suggested title tag:** %s\n\n", brief.SuggestedTitle)
		fmt.Fprintf(&sb, "**Suggested URL slug:** `/%s/`\n\n", brief.Slug)
		fmt.Fprintf(&sb, "**Primary H1:** %s\n\n", brief.PrimaryH1)
		fmt.Fprintf(&sb, "**Target word count:** %s\n\n", brief.WordCountTarget)
		fmt.Fprintf(&sb, "**Meta description hint:** %s\n\n", brief.MetaDescription)

		sb.WriteString("**H2 outline:**\n\n")
		for _, h2 := range brief.H2Outline {
			fmt.Fprintf(&sb, "- %s\n", h2)
		}
		sb.WriteString("\n")

		if len(brief.RelatedKeywords) > 0 {
			sb.WriteString("**Keywords to weave in:**\n\n")
			for _, k := range brief.RelatedKeywords {
				if k.Volume > 0 {
					fmt.Fprintf(&sb, "- `%s` — %d searches/mo (%s)\n", k.Keyword, k.Volume, k.Relationship)
				} else {
					fmt.Fprintf(&sb, "- `%s` (%s)\n", k.Keyword, k.Relationship)
				}
			}
			sb.WriteString("\n")
		}

		if len(brief.SchemaTypes) > 0 {
			sb.WriteString("**Schema types to include:**\n\n")
			for _, t := range brief.SchemaTypes {
				fmt.Fprintf(&sb, "- `%s`\n", t)
			}
			sb.WriteString("\n")
		}

		if len(brief.LinkingHints) > 0 {
			sb.WriteString("**Internal linking:**\n\n")
			for _, l := range brief.LinkingHints {
				fmt.Fprintf(&sb, "- %s\n", l)
			}
			sb.WriteString("\n")
		}

		sb.WriteString("---\n\n")
	}

	return sb.String()
}

// --- internals ---

func collectPAA(s *state.State) []string {
	if s.SERP == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, q := range s.SERP.Queries {
		for _, pq := range q.RelatedQuestions {
			p := strings.TrimSpace(pq)
			if p == "" || seen[strings.ToLower(p)] {
				continue
			}
			seen[strings.ToLower(p)] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func buildBrief(question string, labs []state.LabsKeyword, opt Options) Brief {
	slug := slugify(stripQuestionMarks(question))
	h1 := question
	titleTag := truncate(question, 58)
	if !strings.Contains(strings.ToLower(titleTag), strings.ToLower(opt.ProspectName)) && opt.ProspectName != "" {
		suffix := " | " + opt.ProspectName
		if len(titleTag)+len(suffix) <= 60 {
			titleTag = titleTag + suffix
		}
	}

	wordTarget := "900 to 1,400 words"
	if isShortAnswer(question) {
		wordTarget = "600 to 900 words"
	}

	outline := defaultOutline(question)
	schemas := []string{"FAQPage (question plus direct answer)", "Article (BlogPosting) for the body"}
	// Industry-aware schema hints.
	switch opt.Industry {
	case "car-detailing", "trades":
		schemas = append(schemas, "Service (the service offered as the natural next step)")
	case "dental", "professional-services":
		schemas = append(schemas, "MedicalBusiness or ProfessionalService", "Person (author with credentials)")
	case "restaurants":
		schemas = append(schemas, "Restaurant", "Menu (if the answer touches menu items)")
	}

	related := findRelatedKeywords(question, labs)

	meta := buildMetaDescription(question)

	links := []string{
		"Link in from: your homepage or the closest service page",
		"Link out to: booking page or contact form (primary CTA)",
		"Link out to: the service page most relevant to the question",
	}

	return Brief{
		Question:        question,
		Slug:            slug,
		SuggestedTitle:  titleTag,
		PrimaryH1:       h1,
		WordCountTarget: wordTarget,
		H2Outline:       outline,
		RelatedKeywords: related,
		SchemaTypes:     schemas,
		LinkingHints:    links,
		MetaDescription: meta,
	}
}

func defaultOutline(question string) []string {
	base := []string{
		"Direct answer in 40 to 60 words (the snippet target)",
		"The longer explanation with context",
		"What affects the answer (variables, edge cases)",
		"Common related questions answered inline",
		"Our take / what we recommend",
		"Next step or call to action",
	}
	ql := strings.ToLower(question)
	switch {
	case strings.Contains(ql, "how much") || strings.Contains(ql, "cost") || strings.Contains(ql, "price"):
		base = []string{
			"Direct answer with a price range (the snippet target)",
			"What drives the price up or down",
			"What a typical job looks like at each price point",
			"How to tell which tier you need",
			"Our pricing and how to book",
		}
	case strings.Contains(ql, "how to") || strings.Contains(ql, "how do"):
		base = []string{
			"Short overview (the snippet target)",
			"Step-by-step walkthrough",
			"Common mistakes to avoid",
			"When to call a professional",
			"Book us if you'd rather not DIY",
		}
	case strings.Contains(ql, "worth"):
		base = []string{
			"Straight answer: yes or no, and in what circumstances",
			"The case for",
			"The case against",
			"Who benefits most",
			"How to decide for your situation",
		}
	case strings.Contains(ql, "what is") || strings.Contains(ql, "what's"):
		base = []string{
			"Plain definition (the snippet target)",
			"Why it exists / what problem it solves",
			"How it compares to alternatives",
			"When it applies to your situation",
			"Next step if you need it",
		}
	}
	return base
}

func findRelatedKeywords(question string, labs []state.LabsKeyword) []RelatedKeyword {
	ql := strings.ToLower(question)
	tokens := tokenize(ql)
	stop := stopwords()
	meaningful := map[string]bool{}
	for _, t := range tokens {
		if stop[t] || len(t) < 4 {
			continue
		}
		meaningful[t] = true
	}

	var out []RelatedKeyword
	for _, k := range labs {
		kl := strings.ToLower(k.Keyword)
		if kl == ql {
			continue
		}
		matches := 0
		for _, t := range tokenize(kl) {
			if meaningful[t] {
				matches++
			}
		}
		if matches == 0 {
			continue
		}
		rel := "related"
		if matches >= 2 {
			rel = "strongly related"
		}
		out = append(out, RelatedKeyword{
			Keyword:      k.Keyword,
			Volume:       k.SearchVolume,
			Difficulty:   k.Difficulty,
			Relationship: rel,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Volume > out[j].Volume
	})
	if len(out) > 6 {
		out = out[:6]
	}
	return out
}

func buildMetaDescription(question string) string {
	ql := strings.ToLower(question)
	switch {
	case strings.Contains(ql, "how much") || strings.Contains(ql, "cost"):
		return "Clear pricing and what drives it. Answers " + stripQuestionMarks(question) + " in plain language, with examples."
	case strings.Contains(ql, "worth"):
		return "The honest breakdown on " + stripQuestionMarks(question) + " — who benefits, who doesn't, and how to decide."
	case strings.Contains(ql, "how to") || strings.Contains(ql, "how do"):
		return "Step-by-step on " + stripQuestionMarks(question) + " plus the mistakes that cost people time and money."
	default:
		return "Plain-English answer to " + stripQuestionMarks(question) + " with practical context."
	}
}

func clusterQuestions(questions []string) []ClusterEntry {
	// Very lightweight clustering by keyword overlap.
	// We bucket by the first meaningful word (after stopwords) as a proxy theme.
	stop := stopwords()
	buckets := map[string][]string{}
	for _, q := range questions {
		tokens := tokenize(strings.ToLower(q))
		theme := "general"
		for _, t := range tokens {
			if stop[t] || len(t) < 4 {
				continue
			}
			theme = t
			break
		}
		buckets[theme] = append(buckets[theme], q)
	}
	out := make([]ClusterEntry, 0, len(buckets))
	for theme, qs := range buckets {
		out = append(out, ClusterEntry{Theme: strings.Title(theme), Questions: qs})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].Questions) != len(out[j].Questions) {
			return len(out[i].Questions) > len(out[j].Questions)
		}
		return out[i].Theme < out[j].Theme
	})
	return out
}

// --- utilities ---

func tokenize(s string) []string {
	var out []string
	var cur strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			cur.WriteRune(r)
		default:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func stopwords() map[string]bool {
	return map[string]bool{
		"the": true, "a": true, "an": true, "to": true, "of": true, "and": true,
		"in": true, "on": true, "for": true, "at": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"do": true, "does": true, "did": true, "have": true, "has": true, "had": true,
		"this": true, "that": true, "these": true, "those": true,
		"it": true, "its": true, "your": true, "my": true,
		"how": true, "what": true, "why": true, "when": true, "where": true, "which": true,
		"can": true, "should": true, "would": true, "could": true, "will": true,
		"much": true, "many": true, "some": true, "any": true,
		"best": true, "good": true, "bad": true,
	}
}

func isShortAnswer(question string) bool {
	ql := strings.ToLower(question)
	return strings.HasPrefix(ql, "how much") ||
		strings.HasPrefix(ql, "what is") ||
		strings.HasPrefix(ql, "what's") ||
		strings.HasPrefix(ql, "is ") ||
		strings.HasPrefix(ql, "are ")
}

func stripQuestionMarks(s string) string {
	return strings.TrimRight(strings.TrimSpace(s), "?!.")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := s[:n]
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return cut
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prev := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prev = false
		case r == ' ' || r == '-' || r == '_':
			if !prev && b.Len() > 0 {
				b.WriteByte('-')
				prev = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if len(out) > 60 {
		out = out[:60]
		out = strings.TrimRight(out, "-")
	}
	if out == "" {
		return "brief"
	}
	return out
}

func estimateWords(target string) int {
	// Parse "900 to 1,400 words" and return the midpoint.
	t := strings.ToLower(target)
	parts := strings.Fields(t)
	var low, high int
	fmt.Sscanf(strings.ReplaceAll(parts[0], ",", ""), "%d", &low)
	for i, p := range parts {
		if p == "to" && i+1 < len(parts) {
			fmt.Sscanf(strings.ReplaceAll(parts[i+1], ",", ""), "%d", &high)
			break
		}
	}
	if high > 0 {
		return (low + high) / 2
	}
	return low
}

func humanInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

func fallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func domainFromURL(u string) string {
	s := strings.TrimPrefix(u, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "www.")
	if i := strings.Index(s, "/"); i > 0 {
		s = s[:i]
	}
	return s
}
