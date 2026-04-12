# Supah SEO — Master Braindump

## Status
- CLI is built, compiles, unit tests pass
- Tested live against coastalprograms.com — crawl, audit, GSC, Labs all working
- Brainstorming phase — recording everything before building

---

## Thread 0: Who Uses This and What Do They Have?

### User types
1. **Site owner with full access** — has GSC, has the codebase, can make changes. Using the AI agent to find and fix their own SEO issues.
2. **SEO consultant/agency** — has GSC access granted by client, may not have the codebase. Uses Supah SEO to audit and produce a report for the client.
3. **Cold auditor** — no GSC access. Just has a URL. Wants to crawl, audit, and show the site owner what's wrong to win the work.

### What this means for the tool
- Crawl + audit must work standalone with just a URL (user type 3)
- GSC commands are a bonus layer — more data, better opportunities, but not required
- Paid commands (SERP/Labs/AEO/GEO) are the deepest layer — competitive intel, keyword data
- The tool should gracefully handle missing access: "GSC not connected — run supah-seo login to get richer data"
- state.json should reflect what data sources were used — agent knows what it has and what it's missing

### The report use case
- User type 2 and 3 want a PDF they can hand to a client
- "Here's your site, here's your score, here are the top 10 things to fix"
- Simple language, not technical jargon
- This is a future feature but it shapes how we structure findings now

---

## Thread 1: Testing

### Real-world testing (not unit tests)
- Pick one live site, run the full pipeline, see what breaks
- Run against a real GSC account
- Run against real DataForSEO credentials
- Compare SerpAPI vs DataForSEO on the same query

### Unit/integration gaps
- GSC package has zero tests
- SERP adapters have no integration tests
- No contract tests for the JSON envelope
- No tests verifying --dry-run never makes a real HTTP call
- No race condition tests on the crawler (need -race flag)

### The 20-point test plan (see agent-context-braindump.md)
- Functional correctness per command group
- Cost/safety controls (dry-run, approval gate, cache)
- Config & auth lifecycle
- Error codes and structured error envelope
- Network failure resilience
- Output contract validation
- Concurrency safety

---

## Thread 2: Project Memory / Context Layer

### The problem
- Every command is stateless today
- AI agent starts blind every session
- No record of what was found, done, or planned

### The .supah-seo/ folder idea
Drop a .supah-seo/ folder in the user's project (or site working dir):

```
.supah-seo/
  site.json           # site URL, name, date started
  crawl.json          # last crawl snapshot
  audit.json          # last audit + scores
  gsc.json            # last GSC pull
  opportunities.json  # current opportunity list
  plan.json           # prioritised action list
  history.jsonl       # append-only action log
  context.md          # AI-readable summary (read this first)
```

### Key open question
- In-project .supah-seo/ (version controllable) vs centralised ~/.config/supah-seo/projects/?
- Leaning in-project — team can see it, git can track plan/history

### context.md — what the AI reads on startup
Auto-generated. Tells the agent:
- What site, current score
- What we already know (issues found)
- What we've already done
- What the current plan is

### history.jsonl — agent memory
Append-only. Every action logged:
{"ts": "...", "action": "fix_title", "page": "/about", "before": "...", "after": "...", "status": "done"}

---

## Thread 3: New Commands Needed

### supah-seo init
- Creates .supah-seo/site.json
- Stamps site URL, name, started date
- Possibly adds .supah-seo/ to .gitignore (secrets only)

### supah-seo status
- Reads all .supah-seo/ context files
- Returns one JSON blob: current state of the site
- This is what the AI reads to get up to speed instantly

### supah-seo plan generate
- Reads crawl + audit + GSC + opportunities
- Scores and prioritises actions
- Writes .supah-seo/plan.json

### supah-seo plan next
- Returns the single next best action for the agent to take

### supah-seo record
- Appends an entry to history.jsonl
- Called by the agent after it does something

---

## Thread 4: The compare skill

- We have a `compare` skill that searches GitHub for real-world implementations
- Use this before building new patterns to validate approach
- Especially useful for: context file formats, CLI state management, agent-native CLI patterns
- Search examples: how other CLIs store project state, how AI agent tools structure context

### Things to compare
- How do other CLIs handle per-project state? (.terraform, .claude, .cursor etc)
- How do agent-native tools expose capabilities to LLMs?
- JSON schema patterns for CLI output contracts
- How do other tools handle append-only history logs?

---

## Thread 5: How an AI Agent Actually Uses This

### Questions we haven't answered
- How does the agent discover what commands exist? (--help output? a schema file?)
- Does it get a system prompt telling it about Supah SEO?
- Does it re-read context.md every session or cache it?
- Is the agent reading history to understand what it did, or just current state?

### Proposed flow
1. Agent reads .supah-seo/context.md — gets current state in seconds
2. Agent runs supah-seo status — gets full JSON state
3. Agent runs supah-seo plan next — gets the single next action
4. Agent does the action (may involve other tools, not just supah-seo)
5. Agent runs supah-seo record — logs what it did
6. Loop

### The supah-seo schema command idea
- Returns machine-readable description of every command, flag, and output shape
- Agent reads this once, knows how to use the whole CLI
- Like an OpenAPI spec but for CLI

---

## Thread 6: What We're Not Doing Yet
- No LLM embedded in the CLI
- No dashboard or web UI  
- No auto-executing fixes without agent oversight
- No multi-site management (yet)

---

## Thread 7: What the Audit Actually Checks (and Why)

### Current rules (hardcoded today)
| Check | Rule | Why it matters |
|---|---|---|
| Title length | < 60 chars | Google truncates longer titles in search results |
| Meta description | < 160 chars | Google truncates in snippets — wastes click potential |
| H1 present | Exactly one H1 | Tells Google what the page is about |
| Image alt text | All images have alt | Accessibility + Google image search |
| Canonical tag | Present | Prevents duplicate content penalties |
| HTTP status | 200 only | 4xx/5xx pages hurt crawl budget and rankings |

### What we found on coastalprograms.com (first real run)
- Score: 80.3/100
- 34 issues, all warnings
- Pattern: meta descriptions too long on almost EVERY page — homepage, services, blog, products
- Titles also over 60 chars on several key pages
- This is a systemic problem, not isolated — suggests they were written without SEO character limits in mind

### What we noticed about the output
- Score shows as 80.28985507246377 — needs rounding to 1 decimal place
- No `pages_audited` key in the data envelope — inconsistency vs what we expected
- Nothing is saved after the run — terminal output only, then gone

### What's missing from the audit rules today
- Page speed / Core Web Vitals (needs external data)
- Schema markup / structured data present
- Internal linking depth (orphaned pages)
- Duplicate titles or metas across pages
- Missing H2 structure on long pages
- Mobile viewport meta tag
- Open Graph tags (social sharing)

### The output problem
Right now output is JSON to terminal. That's good for machines. But:
- Nothing persists between runs
- No before/after comparison
- No human-readable summary
- No PDF export

### Output format decision
- JSON only. No markdown middleman.
- AI agents read JSON natively — no need for a context.md
- The .supah-seo/ folder stores everything as JSON
- PDF is the only other format, generated on demand for human clients

---

## Thread 8: Real-World Testing Log

### coastalprograms.com — 2026-04-06
- Crawl: ✅ 23 pages found, all 200s, structure mapped correctly
- Audit: ✅ Score 80.3/100, 34 issues found
- Report: not run yet
- GSC: not run yet
- Issues found with the tool itself:
  - Score decimal places need rounding
  - pages_audited key missing from envelope

---

## Thread 9: The Architecture Decision

### Decision: single .supah-seo/state.json
- One file. Agent reads it, knows everything, acts.
- No scanning multiple files, no stitching, no wasted tokens.

### Decision: rules stay in Go
- Compiled. Fast. No file parsing. No extra config.
- Custom thresholds can come later if needed. Not now.

### Decision: findings carry verdicts
- The CLI interprets, not just reports.
- Every finding: value, verdict, why, fix. Agent doesn't guess.

### The flow
1. `supah-seo init --url X` → creates `.supah-seo/state.json`
2. `supah-seo crawl run` → crawls + audits + interprets → updates state.json
3. Agent reads state.json → knows site, score, findings, history
4. Every command reads and updates the same file

### state.json shape
```json
{
  "site": "https://www.coastalprograms.com",
  "initialized": "2026-04-06T10:00:00Z",
  "last_crawl": "2026-04-06T10:05:00Z",
  "score": 80.3,
  "pages_crawled": 23,
  "findings": [
    {
      "rule": "response_time",
      "url": "/",
      "value": 4192,
      "verdict": "bad",
      "why": "Slow pages rank lower and lose visitors",
      "fix": "Optimize server response, enable caching"
    }
  ],
  "history": []
}
```

---

## Thread 10: Fixes Made During This Session

### Score rounding
- File: internal/audit/engine.go — rounded to 1 decimal

### Crawl data extraction expanded
- Files: internal/crawl/types.go, page.go, crawler.go
- Added: meta_robots, viewport, og tags, lang, hreflang, word_count, schema_types, content_type, x_robots_tag, response_time_ms

---

## Build Order
1. ✅ `supah-seo init --url` — creates .supah-seo/state.json
2. ✅ Wire audit to write findings with verdicts into state.json
3. ✅ Expand audit rules (7 new: viewport, OG, response time, word count, schema, meta robots, lang)
4. ✅ Test end-to-end on coastalprograms.com (23 pages, 42 findings, score 75.7)
5. Next: supah-seo status, crawl state-writing, double-error-print bug

## Bugs Found During Testing

### Double error print on command failure
- PrintCodedError prints the JSON envelope, then returns an error
- Cobra catches the error and root.go calls PrintError again
- Result: two JSON blobs printed on error
- Pre-existing bug, not new

### Response time makes scores non-deterministic
- Same pages give different scores on consecutive runs
- Because response_time_ms varies per request
- Score went 80 → 66.7 on same 2 pages
- Possible fix: exclude response_time from score calc, report it separately

### CTR/position decimal precision in GSC output
- CTR comes back as 0.045454545454545456 instead of 0.045 or 4.5%
- Position comes back as 4.681818181818182 instead of 4.7
- Should round: CTR to 4 decimal places, position to 1 decimal

---

## Thread 11: Testing Log

### coastalprograms.com — full results
- 23 pages, score 75.7/100, 42 findings
- 17x title-too-long (systemic)
- 17x meta-description-too-long (systemic)
- 7x slow-response (server performance)
- 1x og-image-missing
- No H1 issues, no viewport issues, lang set, schema present on relevant pages

### Commands tested ✅
- crawl run (live, 23 pages, full extraction)
- audit run (live, 13 rules, verdicts with why/fix)
- init → audit → state.json (full persistence)
- re-audit (state updates, history stacks)
- report generate + list
- config show (secrets redacted)
- auth login/status/logout (GSC OAuth)
- gsc sites list/use
- gsc query pages (real data, 10 pages returned)
- gsc query keywords (real data, 15 keywords)
- gsc opportunities (28 keyword/page pairs)
- labs ranked-keywords --dry-run (cost estimate only)
- labs ranked-keywords (real DataForSEO, "review funnel" pos 11, volume 50)
- version

### Phase 2 commands tested ✅ (2026-04-06)
All tested live against coastalprograms.com GSC property:
- gsc query trends --start-date 2026-03-01 --end-date 2026-04-03 (34 daily rows, clicks/impressions/ctr/position per day)
- gsc query devices (3 rows: MOBILE 9 clicks, DESKTOP 5 clicks, TABLET 0 clicks)
- gsc query countries (10+ countries, aus leading with 8 clicks)
- gsc query appearances (0 rows — no search appearances for this property, valid empty response)
- gsc query keywords --page https://coastalprograms.com/blog/how-to-build-a-google-review-funnel60 (7+ keywords for specific page)
- gsc query pages --query "review funnel" (1 page, 94 impressions, position 10.2)
- gsc query pages --type discover (0 rows — no Discover traffic, valid empty response)
- state.json integration: init → gsc query pages → state.json has gsc.top_pages and gsc.last_pull
- status command: sources_used includes "gsc" ✅

### Phase 2 fixes applied
- Added automatic GSC OAuth token refresh (RefreshGSCToken in internal/auth/auth.go)
- gscClient() now auto-refreshes expired tokens using stored refresh_token + client credentials
- No more manual re-authentication needed when tokens expire (1hr Google OAuth tokens)

### Phase 3 — Full merge flow — coastalprograms.com — 2026-04-06

Full end-to-end test of the complete `init → audit → gsc → analyze` merge pipeline:

**Steps run:**
1. `init --url https://www.coastalprograms.com` ✅ — state.json created
2. `audit run --url https://www.coastalprograms.com --depth 2 --max-pages 50` ✅ — 23 pages, 51 findings, score 79.7/100
3. `status` (post-crawl) ✅ — `sources_used: ["crawl"]`, `sources_missing: ["gsc"]`
4. `gsc sites use sc-domain:coastalprograms.com` ✅
5. `gsc query pages --limit 50` ✅ — 19 pages returned (real GSC data)
6. `gsc query keywords --limit 50` ✅ — 28 keywords returned
7. `status` (post-GSC) ✅ — `sources_used: ["crawl", "gsc"]`, `sources_missing: null`
8. `analyze` ✅ — 17 merged findings produced
9. `status` (post-analyze) ✅ — `merged_findings_count: 17`, `top_priority: "issues-on-high-traffic-page"`

**state.json verification:**
- `merged_findings` array: ✅ 17 items with priority scores
- All findings reference `sources: ["crawl", "gsc"]` ✅
- Sorted by priority descending (95, 90, 90, 85, 80, 80…) ✅
- URL normalization: ✅ — crawl had `www.coastalprograms.com`, GSC had `coastalprograms.com` — merger correctly matched via strip-www normalization; all findings use canonical non-www form

**Top 3 merged findings:**
1. `https://coastalprograms.com/` — `issues-on-high-traffic-page` — priority 95 — "Page gets 5 clicks but has 2 crawl issue(s) — fixing these protects existing traffic"
2. `https://coastalprograms.com/blog/manual-work-cost` — `issues-on-high-traffic-page` — priority 90 — "Page gets 1 clicks but has 3 crawl issue(s)"
3. `https://coastalprograms.com/about` — `issues-on-high-traffic-page` — priority 90 — "Page gets 2 clicks but has 3 crawl issue(s)"

**All three are sensible:** the homepage and best-traffic pages get highest priority, and the findings correctly merge crawl issues with real GSC click data to justify the priority score.

**Tests:** `go test ./...` — all pass ✅ | `go vet ./...` — clean ✅

**Phase 3 conclusion:** The full merge pipeline works correctly against live data. URL normalization, priority scoring, source attribution, and state persistence all function as designed.

### Phase 4 — New audit rules + PSI + merged flow — coastalprograms.com — 2026-04-06

**Build & quality gate:**
- `go build -o /tmp/supah-seo-ph4 ./cmd/supah-seo` ✅ — clean build
- `go test ./...` ✅ — all 18 packages pass
- `go vet ./...` ✅ — clean

**New audit rules tested against live data:**
- `duplicate-title` — 4 issues found ✅ (contact, apollo, smokohub, zeus all share "Australia" as title — real bug on the site)
- `schema-detected-BreadcrumbList` — 21 detections ✅ (correctly flags schema types found on pages)
- `og-image-missing` — 1 issue on /products ✅
- `meta-description-too-long` — 17 issues ✅ (systemic, same as before)
- `title-too-long` — 17 issues ✅ (systemic)
- `slow-response` — 10 issues ✅
- `duplicate-description` — 0 issues (descriptions are all unique on this site — correct behavior)
- `orphan-page` — 0 issues (all pages well-linked — correct behavior)
- Total: 70 findings across 23 pages, score 74.3/100

**PSI testing:**
- PSI command structure verified: `psi run --url <url> --strategy mobile|desktop` ✅
- Unit tests pass (3/3): parseResponse, RunSuccess (mock), InvalidStrategy ✅
- Live PSI API: 429 rate-limited (unauthenticated Google PSI API has very low rate limit)
- Requires `SUPAHSEO_PSI_API_KEY` env var or config for reliable live use
- Command correctly returns structured error: `{"code": "PSI_FAILED", "detail": "...status 429"}`
- State integration code reviewed: correctly upserts PSI results keyed by URL+strategy

**Full merged flow (without PSI — rate-limited):**
1. `init --url` ✅ → state.json created
2. `audit run --depth 2 --max-pages 50` ✅ → 23 pages, 70 findings, score 74.3
3. `analyze` ✅ → 0 merged findings (no GSC/PSI data available — correct behavior)
4. `status` ✅ → `sources_used: ["crawl"]`, `sources_missing: ["gsc", "psi"]`
5. State.json keys verified: site, initialized, last_crawl, score, pages_crawled, findings (70), merged_findings, last_analysis, history

**Final test run:** `go test ./...` ✅ — all pass

**Phase 4 conclusion:** All new audit rules work correctly against live data. Duplicate-title detection caught a real issue (4 pages sharing generic "Australia" title). PSI code is correct but requires API key for live use (free tier rate-limits aggressively). Merge pipeline handles graceful degradation when data sources are missing.

### Commands not yet tested
- serp analyze/compare
- aeo responses/keywords
- geo mentions/top-pages
- labs keywords/overview/competitors/keyword-ideas
- opportunities --with-serp
- approval gate (set threshold, exceed it)
- cache (second call returns cached)
- provider list/use

---

## Thread 12: GSC Data — Are We Getting Everything?

### What we pull now
- Pages: clicks, impressions, CTR, position (aggregated)
- Keywords: clicks, impressions, CTR, position (aggregated)
- Opportunities: query+page pairs filtered by position > 3 or CTR < 3%

### What GSC Search Analytics API actually gives us (from Google docs)
Metrics: clicks, impressions, CTR, position
Dimensions: query, page, date, device, country, searchAppearance
Search types: web, image, video, news, discover, googleNews
Filters: contains, equals, notContains, notEquals on any dimension
Pagination: rowLimit (max 25000), startRow for paging, up to 50k rows/day/site/type
Data freshness: last 3 days are incomplete, use endDate 3+ days ago
Retention: 16 months of history

### What we use now
- page dimension only (gsc query pages)
- query dimension only (gsc query keywords)
- query + page dimensions (gsc opportunities)
- No date dimension — can't see trends
- No device dimension — can't see mobile vs desktop
- No country dimension — can't see geographic performance
- No searchAppearance — can't see rich results
- No search type selection — always defaults to "web"
- No filters — can't drill into a specific page or keyword
- No pagination — limited to rowLimit, never pages through

### What we should add (priority order)

#### 1. gsc query trends (date dimension)
- Dimensions: ["date", "query"] or ["date", "page"]
- Shows traffic going up or down over time
- Agent needs this to know: is the SEO work having impact?
- Could feed into state.json as a "trend" metric per page/keyword

#### 2. gsc query devices (device dimension)
- Dimensions: ["device"] or ["device", "page"]
- Shows mobile vs desktop split
- Agent needs this because: mobile and desktop rankings differ, Google uses mobile-first indexing
- Common finding: page ranks well on desktop but terribly on mobile

#### 3. gsc query countries (country dimension)
- Dimensions: ["country"] or ["country", "query"]
- Shows where traffic comes from
- Important for: businesses targeting specific regions

#### 4. Filters on existing commands
- --page flag on gsc query keywords → show keywords for a specific page
- --query flag on gsc query pages → show pages ranking for a specific keyword
- These exist in the API (dimensionFilterGroups) but we don't expose them
- Extremely useful for the agent to drill down after seeing opportunities

#### 5. Pagination for large sites
- Current: single request, get back whatever fits in rowLimit
- Should: page through with startRow to get ALL data
- Airbyte/Mage connectors do this — they page in 10k-25k chunks
- Matters for sites with hundreds/thousands of pages

#### 6. Search type selection
- Current: always "web"
- API supports: web, image, video, news, discover, googleNews
- Should: add --type flag, default "web"
- "discover" is increasingly important — Google Discover drives traffic

#### 7. searchAppearance dimension
- Shows: rich results, AMP, FAQ snippets, etc
- Tells the agent: which pages have enhanced SERP features
- Cross-reference with schema data from crawl — do pages with schema actually get rich results?

### What the opportunity filter misses
- Current filter: position > 3 OR CTR < 3%
- Misses: high-impression keywords with 0 clicks regardless of position
- Misses: queries where position changed recently (was 5, now 15) — needs date dimension
- No volume data from GSC alone — need SERP/Labs to know if a keyword is worth chasing
- No device split — a keyword might rank well on mobile but not desktop

### What real-world tools do with GSC data (from GitHub research)
- Airbyte connector: partitions by ALL 6 search types, pages in 25k chunks, uses P3D date step
- Mage integration: pulls performance_report_date and performance_report_custom as separate streams
- Google's own sample code: demonstrates searchAppearance as a two-step process (list types first, then filter by type)
- Best practice from Incremys: "build a daily aggregated baseline, then run deeper dives only on high-stakes segments"

---

## Thread 13: Ideas from Scrapling (D4Vinci/Scrapling)

Scrapling is a Python scraping framework with 34.8k stars. Different language, different purpose, but several ideas worth stealing.

### What they do that we should think about

#### 1. Adaptive element tracking
- Their parser "learns" from website changes and auto-relocates elements when pages update
- For us: if we re-crawl a site and the HTML structure changed, we should detect that — "this page's schema was removed" or "this page's H1 moved"
- Ties into our re-audit diffing idea

#### 2. MCP Server built in
- They ship an MCP server so AI agents (Claude, Cursor) can use Scrapling directly as a tool
- Their MCP server pre-processes content before passing to the AI to reduce token usage
- **This is the answer to "how does an AI agent use Supah SEO"**
- Instead of the agent running CLI commands, it connects via MCP and gets structured responses
- We should consider: `supah-seo mcp` command that starts an MCP server

#### 3. Agent Skill file
- They package their docs as an "agent skill" — a markdown file an AI reads to understand the tool
- Works with Claude Code, OpenClaw, and other agentic tools
- Aligns with the AgentSkill specification
- "It encapsulates almost all of the documentation website's content in Markdown"
- **We could ship a supah-seo agent skill** — one file explaining every command, what data it returns, recommended workflow
- Simpler than building `supah-seo schema` command — just a well-structured markdown file

#### 4. CLI usage without code
- They let you scrape from the terminal without writing Python
- We already do this — but their framing is good: "use it directly from the Terminal"

#### 5. Auto schema detection
- On their roadmap: "auto-detect schemas in pages and manipulate them"
- We already extract JSON-LD schema types — but we could go deeper (validate schema, check for errors)

#### 6. Page analyzer
- On their roadmap: "analyzer ability that tries to learn about the page through meta-elements and return what it learned"
- This is exactly what our audit does — but they frame it as "learning about the page" not "checking rules"
- Different mental model: discovery vs compliance

### What this means for Supah SEO

Two big takeaways:

**A. MCP server is probably the right distribution mechanism for AI agents.**
- CLI commands work but are clunky — the agent has to parse JSON from stdout
- MCP gives structured tool calls with typed inputs and outputs
- Every AI coding tool already supports MCP
- This could be the thing that makes Supah SEO actually useful to agents vs just a CLI

**B. An agent skill file is the cheapest, fastest way to teach AI agents about Supah SEO.**
- One markdown file, structured well, shipped with the repo
- Agent reads it at session start, knows every command and the recommended workflow
- Way faster to build than a `supah-seo schema` command
- Can be installed via clawhub or just read from the repo

### Priority call
- Agent skill file: easy, do it soon
- MCP server: NO. CLI is the point. MCP means tool calls, means context overhead, means slower. We're a CLI that returns JSON. That's the product.
- Adaptive tracking: nice-to-have, not now

### Ideas evaluated from Scrapling that don't apply
- Pause/resume: our crawls take seconds, not hours. Noise.
- Streaming output: commands return fast. Not needed.
- Quiet mode, command families, real-time run stats: noise. We return JSON.
- Quick extracts (single field from single page): convenience, not essential now.

### Ideas that ARE useful
- **Retry logic — free calls only**: GSC rate-limited? Retry with backoff. Page timeout on crawl? Retry once. But NEVER retry paid API calls (DataForSEO, SerpAPI). The user already paid for the attempt. Report the error, let them decide. We don't silently spend their money.
- **Cache visibility**: `supah-seo cache list` / `supah-seo cache clear`. Agent should know what's cached and how old it is. Minor but practical.

---

## Thread 14: Rich Results, Enhancements, and Other Search Engines

### Google Search Console — Enhancements / Search Appearance
GSC reports these search appearance types:
- Breadcrumbs
- FAQ rich results
- Review snippets (star ratings in SERPs)
- How-to results
- Sitelinks
- Video results
- Product listings
- Events
- AMP results

**Why this matters:**
- We already extract JSON-LD schema from the crawl (what the page HAS)
- GSC searchAppearance tells us what Google is actually SHOWING
- The connection: crawl = what you implemented, GSC = whether it's working
- Example: page has FAQ schema but GSC shows no FAQ rich result → schema might be invalid or Google isn't picking it up

**What we should do:**
- Add `gsc query appearances` command — pulls searchAppearance dimension
- Cross-reference with crawl schema data
- Finding: "Page has FAQPage schema but Google isn't showing FAQ rich results"
- Finding: "Page has no schema but could qualify for [X] based on content"

### Schema types we already detect vs what we should look for
Currently detect: whatever @type is in JSON-LD (e.g. Organization, WebSite, FAQPage, Review)

Should specifically look for and flag:
- FAQPage — enables FAQ dropdowns in SERPs (big CTR boost)
- HowTo — enables step-by-step rich results
- Product — enables price/availability/rating in SERPs
- LocalBusiness — enables map pack results
- Article / BlogPosting — enables article-specific features
- BreadcrumbList — enables breadcrumb display in SERPs
- VideoObject — enables video thumbnails in SERPs
- Review / AggregateRating — enables star ratings

### Does this cross SEO/AEO/GEO?
- Schema/rich results = **SEO** — directly affects CTR in traditional search
- FAQ schema = **AEO crossover** — Google pulls FAQ content into AI Overviews and featured snippets
- Review data = **SEO + AEO** — trust signals feed into both traditional and AI search
- Structured data generally = **GEO** — AI engines cite structured, well-organized content more

### Bing Webmaster Tools
- Bing has an API, similar data to GSC: clicks, impressions, rankings
- Bing = ~3-8% of search depending on market
- BUT: Bing powers Microsoft Copilot, DuckDuckGo, Yahoo Search
- For AEO specifically, Bing data could matter more than the traffic share suggests
- Bing Webmaster API: https://www.bing.com/webmasters/apidocs

**Decision: note it, don't build it yet.**
- Small traffic share, extra OAuth flow, extra complexity
- Revisit when AEO becomes a bigger focus
- If we add it: same tiered approach — optional layer, graceful when missing

### Bing Places
- Similar to Google Business Profile
- Local business listings on Bing Maps
- Only relevant for local businesses
- Not in scope for Supah SEO currently — we're focused on search/content, not local listings

### What other enhancements/data sources exist?
- Google Business Profile API — reviews, photos, posts, Q&A (local SEO)
- PageSpeed Insights API — Core Web Vitals, performance scores (free, needs API key)
- Google Knowledge Graph API — entity recognition
- Schema.org validator — validate structured data correctness

**Most useful to add next: PageSpeed Insights.**
- Free API, just needs a key
- Returns: performance score, LCP, CLS, FID/INP, specific opportunities
- Directly actionable findings
- Pairs with our response_time_ms data — we measure server time, PSI measures user experience

---

## Thread 15: The Core Idea — Compare Crawl vs GSC, Find the Gaps

### This is what makes the tool actually useful

Two sources of truth:
- **Crawl** = what the website actually looks like (what you control)
- **GSC** = what Google actually does with your site (reality)

The value is in the **mismatch between the two.**

### Example findings that only exist when you compare both

#### Schema implemented but Google isn't using it
- Crawl: page has FAQPage schema ✅
- GSC: no FAQ rich result appearing ❌
- Finding: "Schema may be invalid or Google is choosing not to display it"
- Fix: validate schema at schema.org validator, check for errors

#### Page ranks but nobody clicks
- Crawl: page has good H1, title, meta, content ✅
- GSC: 500 impressions, 0 clicks, position 8 ❌
- Finding: "Page is ranking but the title/description aren't compelling enough to click"
- Fix: rewrite title and meta description to be more specific and include a call to action

#### Page exists but Google doesn't know about it
- Crawl: page is healthy, returns 200, has content ✅
- GSC: zero impressions ❌
- Finding: "Google may not be indexing this page"
- Fix: check robots.txt, check meta robots, check canonical, check internal links pointing to it

#### Page has issues AND is underperforming
- Crawl: title too long, no OG image ❌
- GSC: "review funnel" keyword, 59 impressions, position 9, 0 clicks ❌
- Finding: "This page is almost on page 1 for a valuable keyword but has basic SEO issues holding it back"
- Fix: fix the title, add OG image, this could break into top 5

#### Page has thin content but ranks anyway
- Crawl: word count 150, below threshold ❌
- GSC: decent impressions, position 3 ✅
- Finding: "This page ranks well despite thin content — expanding it could lock in the position"
- Fix: add more relevant content to defend and improve the ranking

### How this works technically

Right now crawl and GSC are separate commands with separate outputs.
To merge them we need:

1. **URL matching** — crawl URLs and GSC URLs won't always match exactly
   - Crawl: https://www.coastalprograms.com/blog/review-funnel
   - GSC: https://coastalprograms.com/blog/review-funnel (no www)
   - Need URL normalization to match them

2. **A merge command or automatic merge in state.json**
   - After crawl + GSC data both exist in state.json
   - Run comparison logic: for each page, what does the crawl say vs what does GSC say?
   - Produce merged findings that reference both sources

3. **Priority scoring based on both sources**
   - Page with crawl issues + high GSC impressions = HIGH priority (fix this, it's already getting traffic)
   - Page with crawl issues + zero GSC impressions = LOWER priority (fix it but it's not costing you yet)
   - Page with no crawl issues + low GSC CTR = MEDIUM priority (content/copy problem, not technical)

### This is the product
Not "here's your crawl data" and "here's your GSC data" separately.
It's: **"here's what's wrong and here's the evidence from both sides, prioritized by impact."**

The agent reads state.json and gets findings like:
```json
{
  "rule": "ranking-but-not-clicking",
  "url": "/blog/review-funnel",
  "sources": ["crawl", "gsc"],
  "crawl_issues": ["title-too-long"],
  "gsc_data": {"impressions": 59, "clicks": 0, "position": 9.4},
  "verdict": "high-priority",
  "why": "This page gets 59 impressions for 'review funnel' but zero clicks — the title is too long and probably getting truncated",
  "fix": "Shorten title to under 60 chars, make it more specific to 'review funnel'"
}
```

That's what no other tool gives you in one place.

---

## Phase Plan (Final)

### Phase 1: Stabilise ✅ TASKS CREATED
- Fix bugs (crawl ordering, double error print, score rounding — DONE in session)
- Test all untested commands
- Retry logic for free API calls
- supah-seo status command
- Wire crawl to save to state.json
- State package unit tests
- JSON envelope contract tests
- Agent skill file

### Phase 2: Get Everything from GSC ✅ TASKS CREATED
- gsc query trends (date dimension)
- gsc query devices (device dimension)
- gsc query countries
- gsc query appearances (rich results)
- --page and --query filters on existing commands
- --type flag for search type
- Pagination for large sites
- Wire GSC data into state.json
- Update agent skill file

### Phase 3: The Merge — Crawl vs GSC ✅ TASKS CREATED
- URL normalization utility
- Cross-source merge logic (5 rules)
- Merge tests
- Priority scoring
- supah-seo analyze command
- Update status with merged findings
- Update agent skill file
- Live test full flow

### Phase 4: Deeper Audit + PageSpeed ✅ TASKS CREATED
- Duplicate title detection (cross-page)
- Duplicate meta description detection (cross-page)
- Orphan page detection
- Schema validation (quality, not just presence)
- Tests for new rules
- PageSpeed Insights integration (free API)
- Wire PSI into state.json and merge
- Update agent skill file
- Live test

### Phase 5: PDF Reports (PARKED)
- Not creating tasks yet
- Different kind of work (design/layout)
- Doesn't make the tool smarter
- Build when the core product is solid

### What's NOT in any phase (deliberately)
- MCP server — NO, CLI is the product
- Bing Webmaster Tools — parked, revisit later
- Retry on paid API calls — decided against, correct
- LLM embedded in CLI — no
- Dashboard / web UI — no
- Cache visibility (supah-seo cache list/clear) — minor, add anytime

---

## Phase 5: SERP Feature Detection + Labs State Persistence

### What was built
- SERP feature types: Featured Snippet, PAA, AI Overview, Local Pack, Knowledge Graph, Top Stories, Inline Videos/Images/Shopping
- DataForSEO SERP adapter switched from Regular to Advanced endpoint
- Both DataForSEO and SerpAPI adapters parse SERP features
- PAA nested sub-item parsing (DataForSEO returns questions as children of the PAA container)
- SERP data persists to state.json with features, related questions, top domains, our position
- Labs ranked-keywords saves to state.json (keyword, difficulty, volume, intent, position)
- Labs competitors saves to state.json
- 5 new merge engine rules: ai-overview-eating-clicks, featured-snippet-opportunity, paa-content-opportunity, easy-win-keyword, informational-content-gap
- Opportunities engine enriched with Labs difficulty/volume/intent + SERP features
- Batch SERP via DataForSEO Standard Queue (100 keywords per call, $0.0006 vs $0.002)
- Bulk keyword difficulty command (all GSC keywords in one call)
- Auto PSI on audit (top 5 pages, mobile strategy)
- Labs search-intent command (up to 1,000 keywords per call)
- Default location changed to Australia (location_code 2036) across all DataForSEO calls

### Bugs found and fixed
- PAA questions were empty — DataForSEO nests them as sub-items, parser wasn't recursing
- our_position=0 was ambiguous — changed to -1 for "not found"
- Merge engine required GSC — blocked SERP/Labs rules from firing without GSC
- DataForSEO SERP failed without --location flag — now defaults to Australia
- SerpAPI had no default location — now defaults to Perth, Western Australia
- Opportunities source label hardcoded as "serpapi" — changed to generic "serp"
- Opportunities cost hardcoded as $0.01 — updated to $0.002

### Live testing results (coastalprograms.com)
- 23 pages crawled, score 74.3
- 19 GSC pages, 27 keywords
- 5 SERP queries: AI Overview in 4/5, PAA in 5/5, Featured Snippet in 1/5
- 1 Labs keyword (review funnel, volume 50, difficulty 0, intent commercial)
- 31 merged findings total, 1 of 5 new rules fired (featured-snippet-opportunity)
- Other new rules didn't fire due to small site + threshold calibration — thresholds lowered

### Cost optimisation findings
- SERP Live: $0.002/query → Standard Queue: $0.0006/query (70% savings)
- Standard Queue supports 100 keywords per POST call
- Labs Search Intent: up to 1,000 keywords for $0.001 + $0.0001/keyword
- Bulk Keyword Difficulty: multiple keywords per call
- PSI: completely free (25,000/day with API key)

### Wiring trace findings
- Merge engine returned nil without GSC — blocked all SERP/Labs rules (FIXED)
- DataForSEO SERP adapter had zero tests (task created)
- Competitor data saved to state but never consumed by any rule (noted, future feature)

## Phase 6: Backlinks Integration

### What was built
- Backlinks package with types (Summary, Backlink, ReferringDomain, CompetitorBacklinks)
- CLI commands: backlinks summary, list, referring-domains, competitors, gap
- Gap analysis: finds domains linking to competitors but not you (domain_intersection endpoint)
- State persistence for backlink data
- 2 new merge rules: weak-backlink-profile, broken-backlinks-found
- Status/analyze commands updated for backlinks data source

### Pricing
- Requires $100/month DataForSEO Backlinks API commitment (funds go to account balance)
- $0.02 per request + $0.00003 per row
- Max 1,000 rows per request

## Phase Plan (Updated)

### Phase 1-4: ✅ Complete (prior sessions)
### Phase 5: ✅ Complete (this session — SERP features, Labs state, merge rules, batch, fixes)
### Phase 6: ✅ Complete (this session — backlinks)
### Phase 7: Action plan synthesis (NOT STARTED)
- Take all merged findings and produce a prioritised "do these 5 things" action plan
- Simple language, not technical jargon
- This is the product — everything else is data collection

### Phase 8: PDF reports (PARKED)
### Phase 9: Historical tracking (PARKED)

---

## Retest Results (after fixes)

### coastalprograms.com — retest 2026-04-07
All 4 fixes verified against live data:
- Fix 1 (default location): ✅ serp analyze works without --location flag, defaults to AU
- Fix 2 (PAA questions): ✅ 4 questions extracted per query ("What is a review funnel?", etc.)
- Fix 3 (position sentinel): ✅ state.json shows -1 for not-found, 3 for actual rank
- Fix 4 (lowered thresholds): ✅ 3 new rules fired (ai-overview-eating-clicks, paa-content-opportunity, easy-win-keyword)

34 merged findings total (up from 31 pre-fix):
- ranking-but-not-clicking: 7
- not-indexed: 7
- issues-on-high-traffic-page: 3
- schema-not-showing: 13
- ai-overview-eating-clicks: 1 (NEW)
- paa-content-opportunity: 2 (NEW)
- easy-win-keyword: 1 (NEW)
- featured-snippet-opportunity: 0 (fired in first test but not retest — SERP data changed)
- informational-content-gap: 0 (only 1 Labs keyword with commercial intent — no candidates)

### Additional fixes applied after retest
- PSI auto-run during audit was only using API key path, not GSC OAuth token (FIXED)
- PSI auth order: GSC OAuth token → API key → unauthenticated (matches standalone psi run command)
- Login command correctly labels Google OAuth as "GSC + PSI"

---

## Thread 16: Cost Per Website Analysis (FUTURE)

### The question
What does it actually cost to run the full Supah SEO pipeline on one website? If the answer is $0.20 and SEO agencies charge $200-400/month for the same data, that's the selling point for the LinkedIn post and the product positioning.

### What we need to calculate
Run the full pipeline on a real site and track every API call:

**Free tier (always $0):**
- Crawl + audit: free (our crawler)
- PSI: free (Google API, uses GSC OAuth)
- GSC: free (Google API, OAuth)

**Paid tier (DataForSEO):**
- SERP batch (top 20 GSC keywords): 20 × $0.0006 = $0.012
- Labs ranked-keywords: ~$0.01
- Labs bulk-difficulty (all GSC keywords): ~$0.001
- Labs competitors: ~$0.01
- Labs search-intent: ~$0.001
- Backlinks summary: $0.02
- Backlinks gap: ~$0.02

**Estimated total for a full analysis: ~$0.07-0.15 per site**

### What to compare against
- Ahrefs Lite: $129/month
- Semrush Pro: $139/month
- Screaming Frog: $259/year
- Typical SEO agency audit: $500-2,000 one-off

### How to verify
- Run the full pipeline against coastalprograms.com
- Check DataForSEO dashboard for actual spend
- Log the exact cost in TEST-RESULTS.json
- Build a `supah-seo cost-summary` command that reads state.json history and totals up all paid API costs

### When to do this
- After backlinks is live-tested
- Before the LinkedIn post
- This becomes part of the README and marketing: "Full SEO audit for under $0.20"

---

## Thread 17: The `supah-seo run` Command — The Product

### The problem
Right now it takes 8-10 separate commands in the right order to do a full analysis. The user needs to know what to run, what flags to pass, what order matters, which keywords to feed into SERP batch. That's an expert workflow, not a product.

### What it should be
One command:
```bash
supah-seo run
```

Or from a fresh directory with a website project:
```bash
supah-seo run --url https://mysite.com
```

It does everything automatically:

1. **Init** — create `.supah-seo/state.json` if it doesn't exist (use `--url` or detect from existing state)
2. **Crawl + Audit** — crawl the site, run all audit rules, auto-PSI on top pages
3. **GSC** — if authenticated, pull pages + keywords. If not, skip and note it
4. **Labs** — if DataForSEO creds exist:
   - `ranked-keywords` for the domain
   - `bulk-difficulty` on GSC keywords (if GSC data exists)
   - `competitors` for the domain
5. **SERP batch** — take top 10-20 GSC keywords (by impressions), run batch SERP to get features
6. **Backlinks** — if Backlinks API access exists, run `summary` + `gap` (using Labs competitors)
7. **Analyze** — run the merge engine across all available data
8. **Output** — produce the action plan

### How it handles missing credentials
The command must degrade gracefully:
- No GSC? Skip GSC, note "Connect GSC for deeper analysis"
- No DataForSEO? Skip all paid calls, give free-only analysis
- No Backlinks API? Skip backlinks, note "Add Backlinks API for link profile data"
- Each skipped source reduces the analysis but doesn't break it

### Cost control
Before running any paid calls:
- Show estimated total cost: "This analysis will cost approximately $0.08"
- If `--dry-run`, show the breakdown and stop
- If `approval_threshold_usd` is set and exceeded, stop and ask
- Always show what was free vs what cost money in the final output

### The output
Not 34 raw findings. A summary:
```
Site: coastalprograms.com
Score: 74/100
Pages crawled: 23
Data sources: crawl, gsc, psi, serp, labs

Top 5 Actions:
1. [HIGH] Fix title and meta description on /services — 69 impressions, 0 clicks (ranking-but-not-clicking)
2. [HIGH] Your backlink profile is weak (2 referring domains) — start link building
3. [HIGH] Fix crawl issues on homepage — it gets 5 clicks and has 2 issues
4. [MEDIUM] "google review funnel" has an AI Overview eating clicks — optimize for citation
5. [MEDIUM] Add FAQ content answering: "What is a review funnel?", "How do you create a review funnel?"

Cost: $0.08 (crawl/audit/PSI/GSC free, SERP $0.012, Labs $0.022, Backlinks $0.04)
```

### The user's mental model
The user is sitting in their website's working directory. They type `supah-seo run`. They wait 30-60 seconds. They get a clear list of what to fix. They fix it. They run `supah-seo run` again later to check progress.

That's the product. Everything we've built so far is the engine behind this one command.

### What needs to happen
1. Build `supah-seo run` command that orchestrates the pipeline
2. Build the action plan synthesiser (groups findings, deduplicates, ranks, outputs plain English)
3. Add `--dry-run` for cost preview
4. Add `--free-only` flag to skip all paid calls
5. Track total cost in state.json history
6. Smart re-run: if crawl data is < 24h old, skip re-crawl. If GSC data is < 1h old, skip re-pull.

### Phase plan
This is Phase 7. It's the most important phase left. Everything before it was building the engine. This is building the car.

### Phase 7 sub-phases
- 7a: Build `supah-seo run` orchestrator (chains existing commands, handles missing creds)
- 7b: Build action plan synthesiser (groups + ranks + plain English output)
- 7c: Smart caching (skip recent data, re-use state)
- 7d: Cost tracking + summary
- 7e: Live test full flow end-to-end
- 7f: Verify cost against DataForSEO dashboard
