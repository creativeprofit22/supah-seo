# Supah SEO — Architecture

## Overview

Supah SEO is a Go + Cobra single-binary command-line tool for SEO, AEO, and GEO operations. It crawls websites, runs rule-based SEO audits, integrates Google Search Console, fetches SERP data with feature detection (AI Overviews, Featured Snippets, PAA), runs DataForSEO Labs keyword intelligence, performs backlink analysis, measures Core Web Vitals via PageSpeed Insights, and merges all signals through a 13-rule merge engine to produce prioritised, actionable findings. All paid API calls are cost-estimated, approval-gated, and cacheable. Default locale is Australia (location_code 2036, language `en`).

## Data Flow

```
Crawl → Audit (+ auto PSI) → GSC → SERP (features) → Labs (difficulty/intent) → Backlinks → Analyze (merge) → Prioritised Findings
```

- `supah-seo init --url <site>` creates `.supah-seo/state.json`
- `supah-seo crawl run` + `supah-seo audit run` populate crawl findings
- `supah-seo psi run` fetches Core Web Vitals from Google PageSpeed Insights
- `supah-seo gsc query pages/keywords` pulls Google Search Console data
- `supah-seo serp analyze/batch` fetches SERP features and rankings
- `supah-seo labs ranked-keywords/bulk-difficulty` enriches with keyword difficulty and intent
- `supah-seo backlinks summary/gap` adds link profile data
- `supah-seo analyze` runs the merge engine across all sources → writes `merged_findings` to state
- `supah-seo status` shows a summary of what data is present and what's missing

## Data Tiers

| Tier | Sources | Cost | Notes |
|------|---------|------|-------|
| **1** | Crawl + Audit + PSI | Free | PSI uses Google's free API (optional API key for higher quota) |
| **2** | GSC | Free | Requires OAuth2 setup (`supah-seo auth login gsc`) |
| **3** | SERP, Labs, AEO, GEO | Paid | DataForSEO — SERP Live $0.002/query, Batch $0.0006/query, Labs $0.01/task |
| **4** | Backlinks | Paid | DataForSEO Backlinks API — requires $100/month minimum commitment |

## Command Hierarchy

### Project commands
- `init` — create `.supah-seo/state.json` for a site (`--url`)
- `status` — show project state summary (sources present/missing, findings count)
- `analyze` — run cross-source merge engine, write `merged_findings` to state

### Core pipeline
- `crawl run` — BFS website crawler
- `audit run` — SEO audit with rule-based checks and scoring
- `report generate` — full crawl → audit → JSON report pipeline
- `report list` — list stored reports

### PageSpeed Insights
- `psi run` — fetch Core Web Vitals for a URL (`--url`, `--strategy mobile|desktop`)

### Google Search Console
- `gsc sites list` — list accessible GSC properties
- `gsc sites use` — set active property
- `gsc query pages` — top pages by search performance
- `gsc query keywords` — top keywords by search performance
- `gsc opportunities` — filtered query+page pairs for opportunity detection

### SERP analysis (paid, `--dry-run`)
- `serp analyze` — single query SERP analysis with feature detection
- `serp compare` — compare SERP results across multiple queries
- `serp batch` — batch SERP analysis via DataForSEO Standard queue ($0.0006/keyword, up to 100 keywords)

### Labs intelligence (paid, `--dry-run`)
- `labs ranked-keywords` — keywords a domain ranks for (with difficulty, volume, intent, position)
- `labs keywords` — keyword ideas relevant to a domain
- `labs overview` — domain ranking overview
- `labs competitors` — competing domains by ranking overlap
- `labs keyword-ideas` — keyword ideas from a seed keyword
- `labs bulk-difficulty` — bulk keyword difficulty scores (`--keywords` or `--from-gsc`)

### Backlinks (paid, `--dry-run`)
- `backlinks summary` — backlink profile overview (total backlinks, referring domains, spam score, rank)
- `backlinks list` — individual backlinks with source URLs and anchor text (`--dofollow-only`)
- `backlinks referring-domains` — domains linking to the target
- `backlinks competitors` — domains sharing backlink sources
- `backlinks gap` — domains linking to competitors but not the target (auto-loads competitors from state)

### AEO / GEO (paid, `--dry-run`)
- `aeo responses` — AI answer engine responses for a query
- `aeo keywords` — AEO keyword analysis
- `geo mentions` — generative engine mention tracking
- `geo top-pages` — top pages in generative results

### Authentication & config
- `login` — interactive credential setup (GSC OAuth, DataForSEO)
- `logout` — clear stored credentials
- `auth login` / `auth status` / `auth logout` — service-level authentication
- `config show` / `config get` / `config set` / `config path` — config management
- `provider list` / `provider use` — HTTP fetcher provider management
- `version` — build/runtime metadata

### Opportunities (legacy)
- `opportunities` — merged opportunity detection from GSC + optional SERP enrichment

### Global flags
- `--output, -o` (`json`, `text`, `table`) default `json`
- `--verbose, -v` boolean flag

## Package Responsibilities

### `cmd/supah-seo/main.go`
Entrypoint that calls `internal/cli.Execute(version)`. Version is injected via ldflags.

### `internal/cli`
- `root.go`: root Cobra command, global flags, registration of all 18 top-level commands.
- `commands/*.go`: command constructors that wire services, cost gates, and output.

### `internal/crawl`
BFS website crawler with depth limit, max-pages cap, same-domain scoping, and concurrent workers. Extracts title, meta description, canonical, headings, links, images, word count, and structured data.

### `internal/audit`
Rule-based SEO audit engine. Checks title, meta description, H1, image alt, canonical, status code, word count, and noindex directives. Computes a 0–100 score across all crawled pages.

### `internal/report`
JSON report generation and storage. Writes reports to `~/.config/supah-seo/reports/` and lists stored report metadata.

### `internal/psi`
Google PageSpeed Insights client. Fetches Core Web Vitals (LCP, CLS, FCP, TBT, SI) for a URL. Supports API key auth, OAuth token auth (reusing GSC token), or unauthenticated mode.

### `internal/state`
Project state persistence. Manages `.supah-seo/state.json` containing crawl findings, GSC data, PSI results, SERP queries with features, Labs keywords with difficulty/intent, backlink profile data, merged findings, and action history.

### `internal/merge`
Cross-source merge engine with 13 rules. Compares crawl findings, GSC data, PSI results, SERP features, Labs keywords, and backlink data to produce prioritised `MergedFinding` objects sorted by urgency score.

### `internal/serp`
SERP provider abstraction. Defines `Provider` interface (`Name`, `Estimate`, `Analyze`), request/response types, and 9 SERP feature type constants (Featured Snippet, PAA, Local Pack, Knowledge Graph, AI Overview, Top Stories, Inline Videos, Inline Shopping, Inline Images).

### `internal/serp/dataforseo`
DataForSEO SERP adapter. Implements `serp.Provider` for Live endpoint ($0.002/query) and adds `AnalyzeBatch` for Standard queue ($0.0006/query). Defaults to Australia (location_code 2036) and English.

### `internal/serp/serpapi`
SerpAPI adapter. Implements `serp.Provider` at $0.01/search. Fallback when DataForSEO credentials are not configured.

### `internal/backlinks`
Backlink domain types: `Summary`, `Backlink`, `ReferringDomain`, `CompetitorBacklinks`, `BacklinkGap`. Command implementations live in `internal/cli/commands/backlinks.go` and call the DataForSEO Backlinks API directly.

### `internal/opportunities`
Legacy opportunity detection. Merges GSC seeds with optional SERP evidence into scored opportunities. Largely superseded by the `merge` engine + `analyze` command for cross-source analysis.

### `internal/gsc`
Google Search Console client. Authenticated API client for listing sites, querying pages/keywords, and generating opportunity seeds.

### `internal/auth`
OAuth token store. Persists tokens to `~/.config/supah-seo/auth/<service>.json` with expiry checking and refresh support.

### `internal/provider`
Provider abstraction for HTTP fetching. Registry pattern with built-in `local` fetcher using `net/http`.

### `internal/dataforseo`
Shared DataForSEO HTTP client with Basic Auth. Reused by SERP, Labs, AEO, GEO, and Backlinks commands.

### `internal/common/config`
Config management with path resolution, env override hooks, `Load`/`Save`, and secret redaction.

### `internal/common/cost`
Cost estimation (`BuildEstimate`) and approval gates (`EvaluateApproval`). Used by all paid commands.

### `internal/common/cache`
File-based response caching to `~/.config/supah-seo/cache/<provider>/<hash>.json` with TTL-based expiry.

### `internal/common/urlnorm`
URL normalisation utilities. Strips trailing slashes, lowercases scheme/host, removes fragments. Used by the merge engine to join data across sources.

### `internal/common/retry`
HTTP retry utilities with backoff for transient failures.

### `pkg/output`
Shared envelope renderer. JSON-first output with optional text/table modes. Machine-readable error codes for programmatic classification.

## State Model

The `.supah-seo/state.json` file is the single source of truth for a project. It accumulates data from multiple sources:

```json
{
  "site": "https://example.com",
  "initialized": "2025-01-01T00:00:00Z",
  "last_crawl": "...",
  "score": 72.5,
  "pages_crawled": 15,
  "findings": [
    { "rule": "missing-meta-description", "url": "...", "value": "...", "verdict": "...", "why": "...", "fix": "..." }
  ],
  "gsc": {
    "last_pull": "...",
    "property": "sc-domain:example.com",
    "top_pages": [{ "key": "...", "clicks": 10, "impressions": 200, "ctr": 0.05, "position": 8.3 }],
    "top_keywords": [{ "key": "...", "clicks": 5, "impressions": 100, "ctr": 0.05, "position": 12.1 }]
  },
  "psi": {
    "last_run": "...",
    "pages": [{ "url": "...", "performance_score": 45, "lcp_ms": 5200, "cls": 0.32, "strategy": "mobile" }]
  },
  "serp": {
    "last_run": "...",
    "queries": [{
      "query": "example keyword",
      "has_ai_overview": true,
      "features": [{ "type": "featured_snippet", "position": 1, "title": "...", "url": "...", "domain": "..." }],
      "related_questions": ["What is...?", "How to...?"],
      "top_domains": ["competitor1.com", "competitor2.com"],
      "our_position": 5
    }]
  },
  "labs": {
    "last_run": "...",
    "target": "example.com",
    "keywords": [{ "keyword": "...", "search_volume": 1200, "difficulty": 23, "cpc": 1.50, "intent": "informational", "position": 8 }],
    "competitors": ["competitor1.com", "competitor2.com"]
  },
  "backlinks": {
    "last_run": "...",
    "target": "example.com",
    "total_backlinks": 1500,
    "total_referring_domains": 120,
    "broken_backlinks": 15,
    "rank": 45,
    "dofollow": 1200,
    "nofollow": 300,
    "spam_score": 5.2,
    "top_referrers": ["referrer1.com", "referrer2.com"],
    "gap_domains": ["gapsite1.com", "gapsite2.com"]
  },
  "merged_findings": [...],
  "last_analysis": "...",
  "history": [{ "ts": "...", "action": "crawl", "detail": "..." }]
}
```

## Merge Engine Rules

The merge engine (`internal/merge`) runs 13 rules across all available data sources. Rules only fire when their required data sources are present.

| # | Rule | Sources Required | Verdict | Description |
|---|------|-----------------|---------|-------------|
| 1 | `ranking-but-not-clicking` | crawl + GSC | high | Page has impressions but 0 clicks and crawl issues |
| 2 | `not-indexed` | crawl + GSC | medium | Page found in crawl but absent from GSC (may not be indexed) |
| 3 | `issues-on-high-traffic-page` | crawl + GSC | high | Page with organic clicks also has crawl issues |
| 4 | `thin-content-ranking-well` | crawl + GSC | medium | Page under 300 words ranks in top 10 — content expansion opportunity |
| 5 | `schema-not-showing` | crawl + GSC | low | Page has schema markup but CTR is below 5% — rich results may not appear |
| 6 | `slow-core-web-vitals` | PSI + GSC | high | Performance score < 50 on a page with GSC impressions |
| 7 | `ai-overview-eating-clicks` | SERP + GSC | medium | Query has AI Overview and CTR is below 3% |
| 8 | `featured-snippet-opportunity` | SERP + GSC | medium | Query has Featured Snippet and site ranks positions 2–10 |
| 9 | `paa-content-opportunity` | SERP + crawl | low | Query has 2+ PAA questions not addressed by the page |
| 10 | `easy-win-keyword` | Labs + GSC | high | Keyword with difficulty < 30, volume > 30, ranking beyond position 5 |
| 11 | `informational-content-gap` | Labs | medium | Informational keyword with volume > 50, difficulty < 40, not ranking |
| 12 | `weak-backlink-profile` | Backlinks + Labs | high | Fewer than 10 referring domains while targeting keywords with difficulty > 20 |
| 13 | `broken-backlinks-found` | Backlinks | medium | Broken backlinks wasting link equity |

All findings are scored numerically (priority_score 10–100) and sorted by urgency. Priority labels: `high` (70–100), `medium` (40–69), `low` (10–39).

## SERP Feature Detection

The SERP adapter detects 9 feature types from DataForSEO results:

| Feature Type | Constant |
|-------------|----------|
| Featured Snippet | `featured_snippet` |
| People Also Ask | `people_also_ask` |
| Local Pack | `local_pack` |
| Knowledge Graph | `knowledge_graph` |
| AI Overview | `ai_overview` |
| Top Stories | `top_stories` |
| Inline Videos | `inline_videos` |
| Inline Shopping | `inline_shopping` |
| Inline Images | `inline_images` |

## Cost-Aware Execution Model

### Pricing Reference

| Command | Method | Cost |
|---------|--------|------|
| `serp analyze` | Live/Advanced | $0.002/query |
| `serp batch` | Standard queue | $0.0006/query |
| `serp compare` | Live × N queries | $0.002 × N |
| `labs ranked-keywords` | Labs API | $0.01/task |
| `labs keywords` | Labs API | $0.01/task |
| `labs overview` | Labs API | $0.01/task |
| `labs competitors` | Labs API | $0.01/task |
| `labs keyword-ideas` | Labs API | $0.01/task |
| `labs bulk-difficulty` | Labs API | $0.001/task (up to 1000 keywords) |
| `backlinks summary` | Backlinks API | $0.02/task |
| `backlinks list` | Backlinks API | $0.02 + $0.00003/row |
| `backlinks referring-domains` | Backlinks API | $0.02 + $0.00003/row |
| `backlinks competitors` | Backlinks API | $0.02 + $0.00003/row |
| `backlinks gap` | Backlinks API | $0.02 + $0.00003/row |

### Cost metadata
Paid commands expose machine-readable metadata:
- `estimated_cost` — computed cost estimate before execution
- `currency` — always `USD`
- `requires_approval` — true when estimate exceeds `approval_threshold_usd`
- `cached` — whether the response came from cache
- `source` — which provider produced the data
- `fetched_at` — RFC3339 timestamp of data retrieval
- `dry_run` — true when `--dry-run` flag was used

### Approval gate
When `approval_threshold_usd` is set (> 0), any paid command whose estimated cost exceeds the threshold returns `APPROVAL_REQUIRED` without executing.

### Caching
Paid responses are cached to `~/.config/supah-seo/cache/` with configurable TTL. Cache hits are reflected in output metadata and avoid repeat charges.

## Config Model

Keys:
- `active_provider` — HTTP fetcher provider (default: `local`)
- `api_key` (redacted on read/show)
- `base_url`
- `organization_id`
- `serp_provider` — SERP data provider (default: `serpapi`, auto-fallback to `dataforseo` when credentials present)
- `serp_api_key` (redacted)
- `dataforseo_login` (redacted)
- `dataforseo_password` (redacted)
- `approval_threshold_usd` — cost gate threshold; 0 means no gate
- `gsc_property` — active GSC property URL
- `gsc_client_id` (redacted)
- `gsc_client_secret` (redacted)
- `psi_api_key` — Google PageSpeed Insights API key (redacted)

Default file: `~/.config/supah-seo/config.json`
Override: `SUPAHSEO_CONFIG` (must be absolute `.json` path)

## Location Defaults

All DataForSEO API calls default to:
- **Location**: Australia (location_code `2036`)
- **Language**: English (`en`)

Labs and SERP commands accept `--location` and `--language` flags to override. Supported locations for Labs: Australia, United States, United Kingdom, Canada, New Zealand.

## Build and Release

- `Makefile` for build/test/lint/release workflows
- `scripts/release.sh` cross-compiles macOS, Linux, and Windows artifacts
- `make install` builds and installs `supah-seo` to `~/go/bin`
