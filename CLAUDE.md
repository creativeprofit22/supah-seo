# Supah SEO — Agent Notes

## Scope

The CLI has working crawl, audit, report, provider, auth, GSC, PSI, SERP, AEO, GEO, Labs, backlinks, merge, and opportunities services.

### Implemented
- BFS website crawler with depth/page limits and concurrent fetching
- SEO audit engine with rule-based checks and scoring
- JSON report generation and storage
- Provider abstraction with built-in `local` HTTP fetcher
- Full crawl → audit → report pipeline
- Google Search Console integration (sites list, query pages/keywords, opportunity seeds)
- OAuth2 authentication flow for GSC with token persistence and auto-refresh
- PageSpeed Insights integration (Core Web Vitals: LCP, CLS, FCP, TBT, SI)
- PSI auto-persistence to state.json with upsert by URL+strategy
- PSI auth cascade: API key → GSC OAuth token → unauthenticated
- SERP analysis adapters (SerpAPI and DataForSEO)
- SERP feature detection (9 types: Featured Snippets, PAA, AI Overviews, Local Pack, Knowledge Graph, Top Stories, Inline Videos, Inline Shopping, Inline Images)
- SERP batch analysis via DataForSEO Standard queue ($0.0006/keyword, up to 100 keywords)
- SERP data persistence to state.json (features, related questions, top domains, our position)
- AEO and GEO command groups backed by DataForSEO
- Labs command group (`ranked-keywords`, `keywords`, `overview`, `competitors`, `keyword-ideas`, `bulk-difficulty`)
- Labs keyword data persistence to state.json (difficulty, volume, intent, position)
- Labs bulk keyword difficulty with `--from-gsc` flag to auto-load keywords from state
- Labs competitor data persistence to state.json
- Backlinks API integration (`summary`, `list`, `referring-domains`, `competitors`, `gap`)
- Backlinks gap auto-loads competitors from state.json when `--competitors` not provided
- Backlink data persistence to state.json (summary metrics + gap domains)
- Cost-aware execution contracts (`estimated_cost`, `requires_approval`, `cached`, `source`, `fetched_at`)
- `--dry-run` support for all paid workflows
- File-based response caching with TTL
- Approval gate blocking execution when estimated cost exceeds threshold
- Project state management (`init`, `status`, `analyze`)
- Cross-source merge engine with 13 rules:
  - Rules 1–5: crawl + GSC rules (ranking-but-not-clicking, not-indexed, issues-on-high-traffic-page, thin-content-ranking-well, schema-not-showing)
  - Rule 6: PSI + GSC (slow-core-web-vitals)
  - Rules 7–9: SERP-aware (ai-overview-eating-clicks, featured-snippet-opportunity, paa-content-opportunity)
  - Rules 10–11: Labs-aware (easy-win-keyword, informational-content-gap)
  - Rules 12–13: Backlinks-aware (weak-backlink-profile, broken-backlinks-found)
- Priority scoring system (10–100) with automatic sorting by urgency
- Interactive login flow (`supah-seo login`) for GSC OAuth and DataForSEO credentials
- Default locale: Australia (location_code 2036, language `en`) for all DataForSEO calls
- URL normalisation utilities for cross-source data joining
- Opportunity detection merging GSC + optional SERP evidence (legacy, superseded by merge engine)

### Do now
- Keep command architecture stable
- Maintain JSON-first output contract
- Preserve config and output consistency
- Extend audit rules as needed
- Add new providers via the registry pattern
- Add new SERP providers behind the `serp.Provider` interface
- Extend merge rules when new data sources are added

### Do not do without explicit instructions
- Add multiple paid SEO providers at once
- Embed OpenAI/Anthropic inside the CLI by default
- Change the output envelope contract incompatibly
- Restructure the command hierarchy unnecessarily

## Conventions

- Language: Go
- CLI framework: Cobra
- Entry point: `cmd/supah-seo/main.go`
- Root command wiring: `internal/cli/root.go`
- Command files: `internal/cli/commands/*.go`
- Config package: `internal/common/config`
- Cost package: `internal/common/cost`
- Cache package: `internal/common/cache`
- URL normalisation: `internal/common/urlnorm`
- Retry utilities: `internal/common/retry`
- Output package: `pkg/output`
- Provider package: `internal/provider`
- Auth package: `internal/auth`
- GSC package: `internal/gsc`
- PSI package: `internal/psi`
- SERP package: `internal/serp` (adapters: `internal/serp/serpapi`, `internal/serp/dataforseo`)
- DataForSEO shared client: `internal/dataforseo`
- Opportunities package: `internal/opportunities`
- Backlinks package: `internal/backlinks`
- Merge engine: `internal/merge`
- State persistence: `internal/state`
- Domain packages: `internal/crawl`, `internal/audit`, `internal/report`

## Output Contract

Prefer envelope-style structured output:
- `success`
- `data`
- `error`
- `metadata`

Default command output should remain `json` for automation and agent usage.

Paid commands include additional metadata keys:
- `estimated_cost`, `currency`, `requires_approval`, `cached`, `source`, `fetched_at`, `dry_run`

## Configuration

Default config path:
- `~/.config/supah-seo/config.json`

Optional override:
- `SUPAHSEO_CONFIG` (absolute `.json` path)

Supported env overrides:
- `SUPAHSEO_PROVIDER`
- `SUPAHSEO_API_KEY`
- `SUPAHSEO_BASE_URL`
- `SUPAHSEO_ORGANIZATION_ID`
- `SUPAHSEO_SERP_PROVIDER`
- `SUPAHSEO_SERP_API_KEY`
- `SUPAHSEO_DATAFORSEO_LOGIN`
- `SUPAHSEO_DATAFORSEO_PASSWORD`
- `SUPAHSEO_APPROVAL_THRESHOLD_USD`
- `SUPAHSEO_GSC_PROPERTY`
- `SUPAHSEO_GSC_CLIENT_ID`
- `SUPAHSEO_GSC_CLIENT_SECRET`
- `SUPAHSEO_PSI_API_KEY`

> Backlinks API uses the existing DataForSEO credentials (`SUPAHSEO_DATAFORSEO_LOGIN` / `SUPAHSEO_DATAFORSEO_PASSWORD`) — no new env vars needed.

## Validation Commands

Run after changes:

```bash
go test ./...
go vet ./...
```

For full quality gate:

```bash
make fmt
make vet
make test
make lint
```

## Local CLI Install/Update

Use a single global command (`supah-seo`) by installing from source:

```bash
make install
```

If `supah-seo` is not found after install, ensure `~/go/bin` is on PATH and reload shell config.

## Lightweight Release Policy

Default release flow should stay lightweight (avoid costly multi-platform packaging unless explicitly requested):

1. Run fast checks:
   - `go vet ./...`
   - `go test -race ./...`
2. Commit and push to `main`
3. Create/push a semver tag (patch bump)
4. Create a GitHub Release from the tag **without** attached binary assets

Only run `make release` when explicitly asked to produce packaged binaries.

## Commit Message Quality Policy

Every commit should include a clear summary of what changed in the project:

- Subject line: one line, starts with `Add`, `Update`, `Fix`, `Remove`, or `Refactor`
- Body: short bullet points grouped under relevant sections (omit empty sections):
  - `Added:`
  - `Updated:`
  - `Fixed:`
  - `Docs:`
- Bullets must describe concrete changes (file/function/behavior), not generic wording.
