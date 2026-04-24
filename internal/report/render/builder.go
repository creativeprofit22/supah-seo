package render

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/supah-seo/supah-seo/internal/state"
)

// Options carry prospect-specific overrides and agency branding.
type Options struct {
	AgencyName    string
	AgencyLogoB64 string
	CTAURL        string
	CTALabel      string
	ProspectName  string
	Location      string
	CurrentAgency string
	AvgTicket     float64
	CloseRate     float64
	Industry      string // slug: generic, car-detailing, trades, professional-services, restaurants, dental
	GBP           *GBPAudit
	Pricing       []PricingGap
}

// Build transforms the audit state into a view model ready for templating.
func Build(s *state.State, opt Options) ReportView {
	pack := Pack(opt.Industry)

	v := ReportView{
		GeneratedAt:   time.Now(),
		AgencyName:    fallback(opt.AgencyName, "Douro Digital"),
		AgencyLogoB64: opt.AgencyLogoB64,
		CTAURL:        opt.CTAURL,
		CTALabel:      fallback(opt.CTALabel, "Book a strategy call"),
		ProspectName:  fallback(opt.ProspectName, domainFromURL(s.Site)),
		ProspectSite:  s.Site,
		Location:      opt.Location,
		CurrentAgency: opt.CurrentAgency,
		Score:         s.Score,
		PagesCrawled:  s.PagesCrawled,
		TotalIssues:   len(s.Findings),
		Pricing:       opt.Pricing,
		IndustryName:  pack.DisplayName,
		InsightCards:  pack.InsightCards,
		PricingAngle:  pack.PricingAngle,
	}
	if opt.GBP != nil {
		v.GBP = *opt.GBP
	}

	v.Verdict = buildVerdict(s)
	v.MoneyLine = buildMoneyLine(s, opt)
	v.Revenue = buildRevenue(s, opt)
	v.SERPIntel = buildSERPIntel(s)
	v.Backlinks = buildBacklinksIntel(s)
	v.Competitors = buildCompetitors(s)
	v.AllFindings = buildRawFindings(s)
	v.FindingsByURL = buildFindingsByURL(s)
	v.AllPSIPages = buildPSIPages(s)
	v.AllKeywords = buildRankedKeywords(s, pack)
	v.AllSERPQueries = buildSERPQueryViews(s)
	v.KeywordBuckets = buildKeywordBuckets(v.AllKeywords)
	v.Phases = pack.Phases

	findings := buildFindings(s)
	sort.SliceStable(findings, func(i, j int) bool {
		return severityRank(findings[i].Severity) < severityRank(findings[j].Severity)
	})

	top := findings
	if len(top) > 5 {
		top = top[:5]
		v.OtherFindings = findings[5:]
	}
	v.TopFindings = top

	return v
}

// --- raw data surfaces for the agency view ---

func buildRawFindings(s *state.State) []RawFinding {
	out := make([]RawFinding, 0, len(s.Findings))
	for _, f := range s.Findings {
		val := ""
		if v, ok := f.Value.(string); ok {
			val = v
		}
		out = append(out, RawFinding{
			Rule:    f.Rule,
			URL:     f.URL,
			Value:   val,
			Verdict: f.Verdict,
			Why:     f.Why,
			Fix:     f.Fix,
		})
	}
	return out
}

func buildFindingsByURL(s *state.State) []PageFindings {
	groups := map[string]*PageFindings{}
	for _, f := range s.Findings {
		url := f.URL
		if url == "" {
			url = "(site-wide)"
		}
		if _, ok := groups[url]; !ok {
			groups[url] = &PageFindings{URL: url}
		}
		val := ""
		if v, ok := f.Value.(string); ok {
			val = v
		}
		groups[url].Issues = append(groups[url].Issues, RawFinding{
			Rule:    f.Rule,
			URL:     f.URL,
			Value:   val,
			Verdict: f.Verdict,
			Why:     f.Why,
			Fix:     f.Fix,
		})
		switch f.Verdict {
		case "critical", "fail":
			groups[url].Critical++
		case "warning":
			groups[url].High++
		}
	}
	out := make([]PageFindings, 0, len(groups))
	for _, p := range groups {
		out = append(out, *p)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Critical != out[j].Critical {
			return out[i].Critical > out[j].Critical
		}
		if out[i].High != out[j].High {
			return out[i].High > out[j].High
		}
		return out[i].URL < out[j].URL
	})
	return out
}

func buildPSIPages(s *state.State) []PSIPage {
	if s.PSI == nil {
		return nil
	}
	out := make([]PSIPage, 0, len(s.PSI.Pages))
	for _, p := range s.PSI.Pages {
		lcpSec := p.LCP / 1000.0
		verdict := "good"
		tier := "Good"
		switch {
		case lcpSec > 4.0:
			verdict = "poor"
			tier = "Poor"
		case lcpSec > 2.5:
			verdict = "needs improvement"
			tier = "Needs Work"
		}
		out = append(out, PSIPage{
			URL:         p.URL,
			Strategy:    p.Strategy,
			Performance: p.PerformanceScore,
			LCPSeconds:  lcpSec,
			LCPVerdict:  verdict,
			CLS:         p.CLS,
			Tier:        tier,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].LCPSeconds > out[j].LCPSeconds
	})
	return out
}

func buildRankedKeywords(s *state.State, pack IndustryPack) []RankedKeyword {
	if s.Labs == nil {
		return nil
	}
	out := make([]RankedKeyword, 0, len(s.Labs.Keywords))
	for _, k := range s.Labs.Keywords {
		opp := "monitor"
		switch {
		case k.Intent == "navigational" || isBrandedKeyword(k.Keyword, pack):
			opp = "branded"
		case k.Position >= 4 && k.Position <= 10 && k.SearchVolume > 40:
			opp = "easy win (push to top 3)"
		case k.Position >= 11 && k.Position <= 30 && k.SearchVolume > 40:
			opp = "page 2 push"
		case k.Position > 30 && k.SearchVolume > 40:
			opp = "long-term target"
		case k.Position >= 1 && k.Position <= 3:
			opp = "defend"
		}
		out = append(out, RankedKeyword{
			Keyword:     k.Keyword,
			Volume:      k.SearchVolume,
			Difficulty:  k.Difficulty,
			Position:    k.Position,
			Intent:      k.Intent,
			CPC:         k.CPC,
			Opportunity: opp,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Volume != out[j].Volume {
			return out[i].Volume > out[j].Volume
		}
		return out[i].Position < out[j].Position
	})
	return out
}

func buildSERPQueryViews(s *state.State) []SERPQueryView {
	if s.SERP == nil {
		return nil
	}
	out := make([]SERPQueryView, 0, len(s.SERP.Queries))
	for _, q := range s.SERP.Queries {
		v := SERPQueryView{
			Query:            q.Query,
			OurPosition:      q.OurPosition,
			HasAIOverview:    q.HasAIOverview,
			RelatedQuestions: q.RelatedQuestions,
		}
		for _, f := range q.Features {
			switch f.Type {
			case "ai_overview":
				v.HasAIOverview = true
			case "local_pack":
				v.HasLocalPack = true
				if len(v.LocalPackTop3) < 3 {
					label := f.Title
					if label == "" {
						label = f.Domain
					}
					v.LocalPackTop3 = append(v.LocalPackTop3, label)
				}
			case "people_also_ask":
				v.HasPAA = true
			}
		}
		out = append(out, v)
	}
	return out
}

func buildKeywordBuckets(all []RankedKeyword) KeywordBuckets {
	var b KeywordBuckets
	for _, k := range all {
		switch k.Opportunity {
		case "branded":
			b.Branded = append(b.Branded, k)
		case "easy win (push to top 3)":
			b.EasyWins = append(b.EasyWins, k)
		case "page 2 push":
			b.Page2Push = append(b.Page2Push, k)
		case "long-term target":
			b.DeepStretch = append(b.DeepStretch, k)
		}
	}
	return b
}

func isBrandedKeyword(kw string, pack IndustryPack) bool {
	l := strings.ToLower(kw)
	for _, hint := range pack.BrandedHints {
		if hint == "" {
			continue
		}
		if strings.Contains(l, hint) {
			return true
		}
	}
	return false
}

// --- finding construction ---

func buildFindings(s *state.State) []Finding {
	var out []Finding

	// Finding: Catastrophic mobile page speed
	if s.PSI != nil {
		worstLCP := 0.0
		worstURL := ""
		var slowPages []string
		for _, p := range s.PSI.Pages {
			if p.Strategy != "mobile" {
				continue
			}
			if p.LCP > worstLCP {
				worstLCP = p.LCP
				worstURL = p.URL
			}
			if p.LCP > 4000 {
				slowPages = append(slowPages, p.URL)
			}
		}
		if worstLCP > 4000 {
			seconds := worstLCP / 1000.0
			f := Finding{
				ID:       "slow-mobile-lcp",
				Severity: severityFor(seconds, 10, 6, 4),
				Headline: fmt.Sprintf("Your site takes %.1f seconds to load on a phone", seconds),
				TechName: "Core Web Vitals (Largest Contentful Paint)",
				What:     fmt.Sprintf("On mobile, your homepage takes around %.1f seconds before the main content appears. Google considers anything over 4 seconds poor performance, and studies put mobile user drop-off at roughly half by the 3 second mark.", seconds),
				Why:      fmt.Sprintf("Every second past that window is a visitor gone before they saw a service, a price, or a booking button. Across %d slow pages tested, this is running site-wide, not just on one bad page.", len(slowPages)),
				Dream:    "Fixed properly, your booking page opens in under 2 seconds on a phone. The drop-off that's happening today stops, and the traffic you already get starts actually landing.",
				Effort:   "Technical work, a few days",
				Impact:   "Immediate lift in time-on-site and enquiries",
				Technical: Technical{
					URLs:      slowPages,
					Metrics:   []Metric{{Label: "Worst mobile LCP", Value: fmt.Sprintf("%.1fs (%s)", seconds, worstURL)}},
					FixNotes:  "Image optimisation, lazy-loading, render-blocking resources, oversized hero assets, WordPress bloat audit.",
					Priority:  95,
					EffortHrs: 16,
				},
			}
			out = append(out, f)
		}
	}

	// Finding: Invisible in the local pack
	if s.SERP != nil {
		var missed []KeywordMiss
		aiQueries := []string{}
		for _, q := range s.SERP.Queries {
			hasLocal := false
			var firstLocal string
			for _, f := range q.Features {
				if f.Type == "local_pack" && firstLocal == "" {
					firstLocal = f.Domain
					hasLocal = true
				}
				if f.Type == "ai_overview" {
					aiQueries = append(aiQueries, q.Query)
				}
			}
			if hasLocal && q.OurPosition <= 0 {
				missed = append(missed, KeywordMiss{
					Keyword:       q.Query,
					OurPosition:   q.OurPosition,
					TopCompetitor: firstLocal,
				})
			}
		}
		if len(missed) > 0 {
			volumeTotal := 0
			if s.Labs != nil {
				for _, kw := range s.Labs.Keywords {
					for _, m := range missed {
						if strings.EqualFold(kw.Keyword, m.Keyword) {
							volumeTotal += kw.SearchVolume
						}
					}
				}
			}
			urls := make([]string, 0, len(missed))
			for _, m := range missed {
				urls = append(urls, m.Keyword)
			}
			volumeLine := ""
			if volumeTotal > 0 {
				volumeLine = fmt.Sprintf(" These searches combine to roughly %d per month in your area.", volumeTotal)
			}
			f := Finding{
				ID:       "local-pack-missing",
				Severity: "critical",
				Headline: fmt.Sprintf("You're not in the map pack for %d of your highest-value local searches", len(missed)),
				TechName: "Local Pack (Google Maps 3-Pack)",
				What:     fmt.Sprintf("When someone in your service area searches for a car detailer, Google shows three businesses in a map block at the top. That block takes roughly two thirds of all clicks on local searches. You are not appearing in it for any of the %d money keywords we tested.%s", len(missed), volumeLine),
				Why:      "Every one of those searches is someone looking to book. They land on a competitor's listing with photos, reviews, and an 'Open now' badge, and you never get seen. The businesses showing up aren't necessarily better, they're just set up properly.",
				Dream:    "In the map pack, your listing shows alongside the top three in your area. Booking enquiries stop going to the same few competitors every time.",
				Effort:   "Local positioning work, ongoing",
				Impact:   "Largest single driver of local bookings",
				Technical: Technical{
					URLs:      urls,
					Metrics:   []Metric{{Label: "Queries with local pack where we miss", Value: fmt.Sprintf("%d / %d analysed", len(missed), len(s.SERP.Queries))}},
					FixNotes:  "Google Business Profile optimisation, NAP consistency across citations, localised service pages per suburb, review velocity, category alignment, geo-tagged photography.",
					Priority:  98,
					EffortHrs: 40,
				},
			}
			out = append(out, f)
		}
		if len(aiQueries) > 0 {
			f := Finding{
				ID:       "ai-overview-capture",
				Severity: "high",
				Headline: "Google is answering your customers' questions using a competitor's content",
				TechName: "AI Overview (Search Generative Experience)",
				What:     fmt.Sprintf("For %d of your money queries, Google now writes a direct answer at the top of search results using whichever site it trusts most on the topic. That source is not you.", len(aiQueries)),
				Why:      "The click is gone before the page even loads. Customers read Google's summary and either call the business named in it or move on. You don't get counted in either outcome.",
				Dream:    "Your own service pages become the source Google quotes. Your name shows up inside the AI summary instead of a competitor's.",
				Effort:   "Content structure work, ongoing",
				Impact:   "Queries that are already high intent",
				Technical: Technical{
					URLs:      aiQueries,
					FixNotes:  "Structured content, clear question-answer blocks, schema markup (Service, LocalBusiness, FAQPage), entity associations, topical depth across the service cluster.",
					Priority:  85,
					EffortHrs: 30,
				},
			}
			out = append(out, f)
		}
	}

	// Finding: Near-zero backlinks
	if s.Backlinks != nil && s.Backlinks.TotalReferringDomains < 5 {
		f := Finding{
			ID:       "no-authority",
			Severity: "critical",
			Headline: "Google has no reason to trust your site over your competitors'",
			TechName: "Referring domains / off-page authority",
			What:     fmt.Sprintf("Other websites linking to yours is the single strongest signal Google uses to measure reputation. You have %d referring domains. The businesses ranking above you are in the 30-plus range.", s.Backlinks.TotalReferringDomains),
			Why:      "Without authority signals, even a perfectly optimised site sits on page two. This is the main reason your existing rankings are stuck where they are. Every other fix in this report compounds with a proper authority base.",
			Dream:    "Mentions on local Philly business directories, motoring blogs, and supplier pages. Google starts treating your site as a credible local option instead of a new face.",
			Effort:   "Ongoing campaign, months not weeks",
			Impact:   "Unlocks the ceiling on every other ranking win",
			Technical: Technical{
				Metrics: []Metric{
					{Label: "Referring domains", Value: fmt.Sprintf("%d", s.Backlinks.TotalReferringDomains)},
					{Label: "Total backlinks", Value: fmt.Sprintf("%d", s.Backlinks.TotalBacklinks)},
				},
				FixNotes:  "Digital PR targeting local publications in the service area, niche industry blog outreach, supplier and brand co-marketing, sector-specific directory placement. Deliverable list detailed in the 90-day roadmap section.",
				Priority:  92,
				EffortHrs: 80,
			},
		}
		out = append(out, f)
	}

	// Finding: Technical hygiene issues
	if len(s.Findings) > 0 {
		missingH1 := 0
		missingTitle := 0
		missingMeta := 0
		missingSchema := 0
		missingAlt := 0
		brokenPage := 0
		multiH1 := 0
		for _, f := range s.Findings {
			switch f.Rule {
			case "h1-missing":
				missingH1++
			case "h1-multiple":
				multiH1++
			case "title-missing":
				missingTitle++
			case "meta-description-missing":
				missingMeta++
			case "schema-missing":
				missingSchema++
			case "img-alt-missing":
				missingAlt++
			case "broken-page":
				brokenPage++
			}
		}
		if missingH1+missingTitle+missingMeta+missingSchema+missingAlt+brokenPage+multiH1 > 0 {
			parts := []string{}
			if missingTitle > 0 {
				parts = append(parts, fmt.Sprintf("%d pages missing a title tag", missingTitle))
			}
			if missingH1 > 0 {
				parts = append(parts, fmt.Sprintf("%d pages missing an H1 heading", missingH1))
			}
			if multiH1 > 0 {
				parts = append(parts, fmt.Sprintf("%d pages with multiple H1s competing for focus", multiH1))
			}
			if missingMeta > 0 {
				parts = append(parts, fmt.Sprintf("%d pages without a meta description", missingMeta))
			}
			if missingSchema > 0 {
				parts = append(parts, fmt.Sprintf("%d pages missing structured data markup", missingSchema))
			}
			if missingAlt > 0 {
				parts = append(parts, fmt.Sprintf("%d images with no alt text", missingAlt))
			}
			if brokenPage > 0 {
				parts = append(parts, fmt.Sprintf("%d broken page on the live site", brokenPage))
			}

			f := Finding{
				ID:       "technical-hygiene",
				Severity: "high",
				Headline: "Your site is missing the basic labels Google reads",
				TechName: "On-page SEO, structured data, semantic HTML",
				What:     "A working website and a Google-friendly website aren't the same thing. Yours loads fine for humans, but Google reads a different version. Right now that version is mostly blank. Specifically: " + joinWithAnd(parts) + ".",
				Why:      "Google ranks what it can read. Missing titles mean Google picks its own, usually from random text on the page. Missing schema means no stars, no prices, no 'Open now' in search results. Missing alt text on your images means they contribute nothing to image search and hurt accessibility too.",
				Dream:    "The same pages, still working for visitors, now also working for Google. Every page tells search engines exactly what it offers and where.",
				Effort:   "One-off technical pass, a week",
				Impact:   "Baseline every other ranking effort depends on",
				Technical: Technical{
					Metrics: []Metric{
						{Label: "Missing title tags", Value: fmt.Sprintf("%d", missingTitle)},
						{Label: "Multiple H1 tags", Value: fmt.Sprintf("%d", multiH1)},
						{Label: "Missing H1", Value: fmt.Sprintf("%d", missingH1)},
						{Label: "Missing meta description", Value: fmt.Sprintf("%d", missingMeta)},
						{Label: "Missing schema", Value: fmt.Sprintf("%d", missingSchema)},
						{Label: "Images without alt text", Value: fmt.Sprintf("%d", missingAlt)},
						{Label: "Broken pages", Value: fmt.Sprintf("%d", brokenPage)},
					},
					FixNotes:  "Template-level title/meta/H1 templating, LocalBusiness and Service schema, sector-specific schema types, Product/Service schema where applicable, review schema tied to Google profile, alt text pass across the image library.",
					Priority:  78,
					EffortHrs: 24,
				},
			}
			out = append(out, f)
		}
	}

	return out
}

// --- helpers ---

func buildVerdict(s *state.State) string {
	switch {
	case s.Score < 40:
		return "There's a lot of money being left on the table here"
	case s.Score < 60:
		return "Solid bones, big gaps to close"
	case s.Score < 80:
		return "Close to competitive, a few clear fixes away"
	default:
		return "Fundamentals are in place, sharpening from here"
	}
}

func buildMoneyLine(s *state.State, opt Options) string {
	if s.Backlinks != nil && s.Backlinks.TotalReferringDomains == 0 {
		return "Your site is invisible to Google in the ways that matter for bookings. Fixable, but nothing about it is going to fix itself."
	}
	return "Your current setup is leaving recurring monthly bookings on the table, and the gap widens every month this isn't addressed."
}

func buildRevenue(s *state.State, opt Options) RevenueModel {
	avg := opt.AvgTicket
	if avg <= 0 {
		avg = 180
	}
	close := opt.CloseRate
	if close <= 0 {
		close = 0.25
	}
	// Estimate from SERP + Labs: for each money keyword where we're not in top 10,
	// assume realistic recovery of CTR at position 3 (roughly 10%) over current volume.
	var monthlyClicksLow, monthlyClicksHigh float64
	if s.Labs != nil {
		for _, kw := range s.Labs.Keywords {
			if kw.Position <= 0 || kw.Position > 30 {
				continue
			}
			if kw.SearchVolume < 20 {
				continue
			}
			// Low end: move from current pos to top 5. Volume * 0.08
			// High end: move to top 3. Volume * 0.15
			monthlyClicksLow += float64(kw.SearchVolume) * 0.05
			monthlyClicksHigh += float64(kw.SearchVolume) * 0.12
		}
	}
	enquiriesLow := monthlyClicksLow * 0.35
	enquiriesHigh := monthlyClicksHigh * 0.45
	bookingsLow := enquiriesLow * close
	bookingsHigh := enquiriesHigh * close

	return RevenueModel{
		AvgTicket:      avg,
		CloseRate:      close,
		MonthlyLowEnd:  math.Round(bookingsLow * avg),
		MonthlyHighEnd: math.Round(bookingsHigh * avg),
		AnnualLowEnd:   math.Round(bookingsLow * avg * 12),
		AnnualHighEnd:  math.Round(bookingsHigh * avg * 12),
	}
}

func buildSERPIntel(s *state.State) SERPIntel {
	if s.SERP == nil {
		return SERPIntel{}
	}
	out := SERPIntel{QueriesAnalyzed: len(s.SERP.Queries)}
	paaSet := map[string]struct{}{}
	for _, q := range s.SERP.Queries {
		hasLocal := false
		for _, f := range q.Features {
			if f.Type == "local_pack" {
				hasLocal = true
			}
			if f.Type == "ai_overview" {
				out.AIOverviewQueries = append(out.AIOverviewQueries, q.Query)
			}
		}
		if hasLocal && q.OurPosition <= 0 {
			out.NotInLocalPack++
		}
		for _, p := range q.RelatedQuestions {
			paaSet[p] = struct{}{}
		}
		if q.OurPosition <= 0 && hasLocal {
			topComp := ""
			for _, f := range q.Features {
				if f.Type == "local_pack" {
					topComp = f.Domain
					break
				}
			}
			out.TopMoneyMisses = append(out.TopMoneyMisses, KeywordMiss{
				Keyword:       q.Query,
				OurPosition:   q.OurPosition,
				TopCompetitor: topComp,
			})
		}
	}
	for q := range paaSet {
		out.PAAQuestions = append(out.PAAQuestions, q)
	}
	sort.Strings(out.PAAQuestions)

	// Attach volumes where known
	if s.Labs != nil {
		vols := map[string]int{}
		for _, kw := range s.Labs.Keywords {
			vols[strings.ToLower(kw.Keyword)] = kw.SearchVolume
		}
		for i := range out.TopMoneyMisses {
			out.TopMoneyMisses[i].Volume = vols[strings.ToLower(out.TopMoneyMisses[i].Keyword)]
		}
	}
	sort.SliceStable(out.TopMoneyMisses, func(i, j int) bool {
		return out.TopMoneyMisses[i].Volume > out.TopMoneyMisses[j].Volume
	})
	return out
}

func buildBacklinksIntel(s *state.State) BacklinksIntel {
	if s.Backlinks == nil {
		return BacklinksIntel{}
	}
	return BacklinksIntel{
		RefDomains:     s.Backlinks.TotalReferringDomains,
		TotalBacklinks: s.Backlinks.TotalBacklinks,
		SpamScore:      s.Backlinks.SpamScore,
		GapDomains:     s.Backlinks.GapDomains,
	}
}

func buildCompetitors(s *state.State) []CompetitorEntry {
	// Prefer SERP-derived local competitors over Labs competitors (which are polluted by directories).
	if s.SERP == nil {
		return nil
	}
	seen := map[string]*CompetitorEntry{}
	skip := map[string]bool{
		"yelp.com": true, "www.yelp.com": true,
		"facebook.com": true, "www.facebook.com": true,
		"instagram.com": true, "www.instagram.com": true,
		"reddit.com": true, "www.reddit.com": true,
		"mapquest.com": true, "www.mapquest.com": true,
		"google.com": true, "www.google.com": true,
		"youtube.com": true, "www.youtube.com": true,
	}
	for _, q := range s.SERP.Queries {
		for _, f := range q.Features {
			if f.Type != "local_pack" {
				continue
			}
			d := strings.TrimPrefix(f.Domain, "www.")
			if skip[d] || skip[f.Domain] {
				continue
			}
			if _, ok := seen[d]; !ok {
				seen[d] = &CompetitorEntry{Domain: d, Name: f.Title}
			}
			seen[d].AppearsInQuery = append(seen[d].AppearsInQuery, q.Query)
		}
	}
	out := make([]CompetitorEntry, 0, len(seen))
	for _, c := range seen {
		out = append(out, *c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return len(out[i].AppearsInQuery) > len(out[j].AppearsInQuery)
	})
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}

func severityFor(v, critical, high, medium float64) string {
	switch {
	case v >= critical:
		return "critical"
	case v >= high:
		return "high"
	case v >= medium:
		return "medium"
	default:
		return "low"
	}
}

func severityRank(s string) int {
	switch s {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
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

func joinWithAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
	}
}
