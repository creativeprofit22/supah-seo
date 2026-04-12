# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [0.5.0] - 2026-04-05

### Added
- **Labs command group** (`labs`) for DataForSEO Labs intelligence:
  - `labs ranked-keywords` — keywords a domain/URL ranks for
  - `labs keywords` — keyword ideas relevant to a domain
  - `labs overview` — domain ranking distribution and estimated traffic
  - `labs competitors` — competing domains by ranking overlap
  - `labs keyword-ideas` — keyword ideas from a seed keyword

### Improved
- **Interactive login** (`supah-seo login`) rewritten with Charm Huh forms — selector flow, masked secret inputs, Esc-to-back navigation, and setup summary on exit

## [0.1.0] - 2026-04-05

### Added
- **Website crawler**: BFS crawler with depth limit, max-pages cap, same-domain scoping, and concurrent fetching. HTML parsing extracts title, meta description, canonical, headings, links, and images.
- **SEO audit engine**: Rule-based checker covering title, meta description, H1, image alt text, canonical tag, and HTTP status codes. Produces per-page issues with severity levels and a 0–100 aggregate score.
- **Report generator**: JSON audit reports stored to `~/.config/supah-seo/reports/` with metadata listing.
- **Provider abstraction**: `Fetcher` interface with registry pattern. Built-in `local` provider using `net/http` with configurable timeout and User-Agent.
- **Google Search Console integration**: `gsc sites list`, `gsc sites use`, `gsc query pages`, `gsc query keywords`, `gsc opportunities` commands for real search performance data.
- **OAuth2 authentication**: `auth login gsc`, `auth status`, `auth logout gsc` with local callback server and file-based token persistence.
- **SerpAPI SERP analysis**: `serp analyze` and `serp compare` commands with `--dry-run` support and cost estimation.
- **DataForSEO SERP adapter**: Implements `serp.Provider` against DataForSEO's organic search endpoint. Selected via `serp_provider = "dataforseo"` config key.
- **AEO command group** (Answer Engine Optimization):
  - `aeo responses` — query AI models (ChatGPT, Claude, Gemini, Perplexity) and view the response. Supports `--model`, `--dry-run`, cost estimation, and approval gate.
  - `aeo keywords` — retrieve AI search volume data for a keyword from DataForSEO.
- **GEO command group** (Generative Engine Optimization):
  - `geo mentions` — track how often a domain or brand appears in AI-generated responses for a keyword. Supports `--domain`, `--platform`, `--dry-run`, cost estimation, and approval gate.
  - `geo top-pages` — show which pages are most cited by AI engines for a keyword.
- **Opportunity detection**: `opportunities` command merges GSC seeds with optional SERP enrichment, classifying by type, confidence, impact, and effort.
- **Interactive login**: `supah-seo login` guides setup for Google Search Console (OAuth), DataForSEO (login + password), and SerpAPI (API key) in a single flow. `supah-seo logout` clears all stored credentials.
- **Cost-aware execution**: Estimation, approval gates, `--dry-run` support, and file-based response caching with TTL for all paid API commands.
- **Structured output**: JSON envelope with `success`, `data`, `error`, `metadata` fields. Machine-readable error codes for programmatic classification.
- **Config management**: JSON config at `~/.config/supah-seo/config.json` with environment variable overrides and secret redaction.
- **Build tooling**: Makefile for build/test/lint/release workflows. Cross-compilation script for macOS, Linux, and Windows.
