package render

import "strings"

// IndustryPack holds the copy and configuration that varies by the prospect's vertical.
// A pack drives: the insight cards in the client report, the roadmap deliverables,
// and the keyword classifier (what counts as branded for this business).
type IndustryPack struct {
	Slug         string
	DisplayName  string
	InsightCards []InsightCard
	Phases       []Phase
	BrandedHints []string // lowercase substrings that flag a keyword as navigational/branded
	PricingAngle string   // one-line framing for the pricing section intro
}

// InsightCard is one of the "Why this matters in your industry specifically" tiles
// rendered in the client report.
type InsightCard struct {
	Title string
	Body  string
}

// Pack returns the IndustryPack for the given slug.
// Falls back to the generic pack if unknown or empty.
func Pack(slug string) IndustryPack {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return packs["generic"]
	}
	if p, ok := packs[slug]; ok {
		return p
	}
	return packs["generic"]
}

// AvailablePacks returns the list of supported industry slugs.
func AvailablePacks() []string {
	out := make([]string, 0, len(packs))
	for k := range packs {
		out = append(out, k)
	}
	return out
}

var packs = map[string]IndustryPack{
	"generic": {
		Slug:        "generic",
		DisplayName: "General local service",
		InsightCards: []InsightCard{
			{
				Title: "Most of your customers are searching on a phone",
				Body:  "Local service searches are overwhelmingly mobile. If your site takes more than a few seconds to load on a phone, most of the traffic you do earn bounces before they've seen what you offer.",
			},
			{
				Title: "The map pack is the whole game for local services",
				Body:  "Google shows three map listings at the top of local searches. Research consistently puts those three businesses as the destination for roughly two thirds of the clicks. Organic listings below them fight for the rest. If you're not in the map pack, you're competing for scraps.",
			},
			{
				Title: "Reviews and photos carry more weight than copy",
				Body:  "For local services, customers make decisions on trust signals before they read a single line of your website. Review count, star rating, and recent photos matter more than polished marketing copy. Your digital presence needs to look busy and alive.",
			},
			{
				Title: "Demand has seasonal and weekly rhythm",
				Body:  "Local services see predictable peaks. The businesses that dominate each peak started preparing months earlier. Every quiet month is a chance to put work in that pays off when demand returns.",
			},
		},
		Phases: defaultPhases(
			"Digital PR push targeting local publications",
			"Supplier and brand partnership outreach for backlinks",
			"Niche directory placement",
		),
	},
	"car-detailing": {
		Slug:        "car-detailing",
		DisplayName: "Car detailing",
		InsightCards: []InsightCard{
			{
				Title: "Most of your customers are searching on a phone",
				Body:  `"Mobile car wash near me", "detailing [suburb]", "ceramic coating [city]". These are phone searches done in the moment by people actively looking to book. If the site takes six seconds to load on a phone, you've lost the half of them that would have converted.`,
			},
			{
				Title: "The map pack is the whole game for local services",
				Body:  "Google shows three map listings at the top of local searches. Research consistently puts those three businesses as the destination for roughly two thirds of the clicks. Organic listings below them compete for the remaining third. If you're not in the map pack, you're fighting over the scraps.",
			},
			{
				Title: "Before-and-after photos are a wasted asset right now",
				Body:  "Detailing is a visual service. Your photos should be doing heavy lifting in search, on your Google Business Profile, and on social. Right now the images on your site aren't labelled in a way Google understands, which means they don't appear in image search and contribute nothing to your rankings.",
			},
			{
				Title: "Seasonal demand swings hard in this business",
				Body:  "Spring and early summer are your peak months. Winter has its own market for interior-focused work and paint protection. The businesses that dominate each season started preparing 60 to 90 days earlier. Leaving this unfixed going into peak season leaves the biggest months of the year on the table.",
			},
		},
		BrandedHints: []string{"express auto", "express detail", "xpress", "express car"},
		PricingAngle: "Premium detailers in your area charge meaningfully more than you for the same services. The gap isn't about the quality of the work. It's about positioning.",
		Phases: defaultPhases(
			"Digital PR push targeting local metro publications",
			"Supplier and brand partnership outreach for backlinks (ceramic coating product brands, detailing equipment suppliers)",
			"Automotive niche directory placement",
		),
	},
	"trades": {
		Slug:        "trades",
		DisplayName: "Trades (plumber, electrician, HVAC)",
		InsightCards: []InsightCard{
			{
				Title: "Emergency searches happen on a phone",
				Body:  `"Plumber near me", "emergency electrician [suburb]", "AC repair 24 hour". Most of your highest-value calls come from someone whose pipe just burst. A slow-loading site at that moment is losing you a job worth hundreds of dollars in minutes.`,
			},
			{
				Title: "Map pack presence is the single biggest lever",
				Body:  "Emergency-style local searches skew even harder to the map pack than most verticals. Being in the top three on Google Maps is the difference between getting the call and watching a competitor take it.",
			},
			{
				Title: "Reviews compound faster than in most industries",
				Body:  "Trades customers rely on reviews more heavily than almost any other local service. Volume matters, but recency matters more. A business with 200 reviews where the latest is 8 months old loses to a business with 40 reviews from last week.",
			},
			{
				Title: "Your service area map is a competitive asset",
				Body:  "Most trades businesses list a single location. The ones winning are building individual service-area pages for every suburb or postcode they cover. That alone can double local visibility without touching anything else.",
			},
		},
		Phases: defaultPhases(
			"Digital PR push targeting home-services publications",
			"Trade supplier and manufacturer partnership outreach for backlinks",
			"Industry directory placement (HomeAdvisor, Angi, Yelp trade pages)",
		),
	},
	"professional-services": {
		Slug:        "professional-services",
		DisplayName: "Professional services (legal, accounting, consulting)",
		InsightCards: []InsightCard{
			{
				Title: "Searchers are researching, not reacting",
				Body:  "Unlike emergency trades, professional service searches tend to be deliberate. Prospects compare three to five options before they book a consultation. Your site needs to win the comparison step, which means depth of content and social proof matter more than speed alone.",
			},
			{
				Title: "Authority signals carry the most weight",
				Body:  "Google weighs credentials, backlinks from authoritative sources, and publication mentions more heavily in professional services than in most verticals. Thin site, zero mentions elsewhere on the web, and you're essentially invisible for the competitive queries.",
			},
			{
				Title: "Reviews and case studies both matter",
				Body:  "Unlike restaurants or trades where reviews alone carry the weight, professional services prospects want to see case studies, example outcomes, and how you've handled situations like theirs. Reviews get them in the door. Case studies close them.",
			},
			{
				Title: "Local plus specialty is the positioning sweet spot",
				Body:  `Ranking for "lawyer [city]" is brutal. Ranking for "employment lawyer [suburb]" is achievable. The winning firms specialise by service and by geography simultaneously, and their site architecture reflects that.`,
			},
		},
		Phases: defaultPhases(
			"Digital PR push targeting industry publications and association outlets",
			"Speaking engagement and guest post outreach for authority backlinks",
			"Specialty directory placement (bar associations, industry-specific platforms)",
		),
	},
	"restaurants": {
		Slug:        "restaurants",
		DisplayName: "Restaurants and hospitality",
		InsightCards: []InsightCard{
			{
				Title: "Phone-first, photo-first, decision in seconds",
				Body:  "Nobody reads your menu description. They look at photos, scan reviews, check hours, and either book or move on. Your site and your Google Business Profile both need to win that 15-second scan.",
			},
			{
				Title: "The map pack takes an even bigger share of clicks here",
				Body:  "Food searches skew heavily to the map. Over 70 percent of clicks on restaurant queries land on the three map pack listings. Organic results below rarely get a look. Being in the three-pack is table stakes, not an extra.",
			},
			{
				Title: "Review velocity beats review total",
				Body:  "Google weighs recent reviews more than old ones in the restaurant space. A place with 80 reviews in the last 6 months outranks a place with 400 reviews from three years ago. Active review generation is a weekly activity, not a one-time push.",
			},
			{
				Title: "Menu schema and booking integration are quiet wins",
				Body:  "Restaurants that implement Menu and Reservation schema get rich result treatment in search. Their listings show prices, photos, and booking buttons inline. Competitors that haven't done this show up as plain links, and it shows in conversion.",
			},
		},
		Phases: defaultPhases(
			"Local food publication and blog outreach",
			"Reservation platform optimisation and cross-promotion",
			"Event collaboration and neighbourhood directory placement",
		),
	},
	"dental": {
		Slug:        "dental",
		DisplayName: "Dental practice",
		InsightCards: []InsightCard{
			{
				Title: "Patients choose a dentist like they choose a car mechanic",
				Body:  "On trust, reviews, and proximity. Not on marketing copy. Your Google Business Profile and review count decide most of the outcome before someone ever lands on your website.",
			},
			{
				Title: "Insurance queries are the highest intent searches",
				Body:  `"Dentists that accept [insurance] near me" is the query that converts. If your site doesn't clearly list the insurance networks you accept, you're invisible for it even if the patient is ready to book right now.`,
			},
			{
				Title: "Specialty pages beat the generic service page",
				Body:  "A single page listing all your services gets outranked by individual pages for veneers, implants, Invisalign, emergency care. Each of those is a specific search with different intent, and the practices winning are mapping them to dedicated pages.",
			},
			{
				Title: "E-E-A-T matters more than in most verticals",
				Body:  "Google treats dental (like other medical) content as YMYL, which means expertise and authority signals carry more weight. Author bios on content, clear credentials on practitioner pages, and mentions on health authority sites all move rankings in ways they don't for other local services.",
			},
		},
		Phases: defaultPhases(
			"Local health publication and parenting blog outreach",
			"Insurance platform optimisation and directory placement",
			"Professional association and dental school alumni network outreach",
		),
	},
}

// defaultPhases returns the standard 90-day roadmap with the off-page deliverables
// swapped in from the industry pack. The first two phases are identical across
// industries because the technical and on-page work is universal.
func defaultPhases(offPage1, offPage2, offPage3 string) []Phase {
	return []Phase{
		{
			Window: "Days 1-30",
			Theme:  "Stop the bleeding",
			Focus:  "Technical foundation and Core Web Vitals",
			Deliverables: []string{
				"Full on-page audit with page-by-page fix list applied",
				"Core Web Vitals remediation pass on mobile (image optimisation, render-blocking resource audit, CDN review)",
				"Missing meta, title, H1, canonical tags templated site-wide",
				"LocalBusiness and Service schema implementation",
				"Alt text pass across image library",
				"Broken URL fixes and redirect map",
			},
			KPIs: []string{
				"Mobile LCP under 4s on all priority pages",
				"Zero missing title/meta/H1 across indexed pages",
				"Schema validation clean in Rich Results Test",
			},
		},
		{
			Window: "Days 31-60",
			Theme:  "Visibility",
			Focus:  "Local SEO and content alignment to ranking opportunities",
			Deliverables: []string{
				"Google Business Profile optimisation sprint (photos, categories, attributes, posting cadence)",
				"Location-specific service pages for priority suburbs",
				"Content refresh on top money pages targeting page-2 keywords",
				"FAQ cluster answering PAA questions captured in this audit",
				"NAP consistency audit across citations and directories",
			},
			KPIs: []string{
				"Appearing in local pack for at least 3 money queries",
				"Top 10 ranking on at least 5 page-2 keywords",
				"PAA snippet captured on at least 2 queries",
			},
		},
		{
			Window: "Days 61-90",
			Theme:  "Authority",
			Focus:  "Off-page signals and competitive positioning",
			Deliverables: []string{
				offPage1,
				offPage2,
				offPage3,
				"Review velocity program with post-service prompts",
				"Content assets designed to earn links (original data, local guides)",
			},
			KPIs: []string{
				"10+ new referring domains from non-directory sources",
				"Entering top 3 on at least 2 high-value local queries",
				"Organic enquiries trending upward month over month",
			},
		},
	}
}
