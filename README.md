<div align="center">
<pre>
███████╗██╗   ██╗██████╗  █████╗ ██╗  ██╗    ███████╗███████╗ ██████╗
██╔════╝██║   ██║██╔══██╗██╔══██╗██║  ██║    ██╔════╝██╔════╝██╔═══██╗
███████╗██║   ██║██████╔╝███████║███████║    ███████╗█████╗  ██║   ██║
╚════██║██║   ██║██╔═══╝ ██╔══██║██╔══██║    ╚════██║██╔══╝  ██║   ██║
███████║╚██████╔╝██║     ██║  ██║██║  ██║    ███████║███████╗╚██████╔╝
╚══════╝ ╚═════╝ ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═╝    ╚══════╝╚══════╝ ╚═════╝
</pre>

<p align="center">
  <a href="https://go.dev/">
    <img src="https://img.shields.io/badge/go-%3E%3D1.26-00ADD8.svg" alt="Go Version">
  </a>
  <a href="https://github.com/supah-seo/supah-seo/actions">
    <img src="https://github.com/supah-seo/supah-seo/actions/workflows/test.yml/badge.svg" alt="CI">
  </a>
  <a href="https://github.com/supah-seo/supah-seo/blob/main/LICENSE">
    <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License">
  </a>
</p>
</div>

**Open-source SEO CLI tool** — crawl, audit, and optimize websites from the command line. Single Go binary. JSON output on every command.

Supah SEO replaces the need to juggle multiple SEO tools. It crawls your site, runs a technical SEO audit, pulls Google Search Console data, checks PageSpeed (Core Web Vitals), analyzes SERP features (AI Overviews, Featured Snippets, People Also Ask), gets keyword difficulty and search intent, audits your backlink profile, and merges everything into prioritised action items — all from one CLI.

Crawling, auditing, and PageSpeed Insights are free. Paid features (SERP, keyword research, backlinks, AEO/GEO) use DataForSEO with cost estimates and `--dry-run` on every command.

## Install

```bash
git clone https://github.com/supah-seo/supah-seo.git
cd supah-seo
make build
# binary: build/supah-seo
```

Requires Go 1.26+.

### Run without installing globally

```bash
go run ./cmd/supah-seo --help
go run ./cmd/supah-seo login
```

### Install globally (dev)

```bash
make install
export PATH="$HOME/go/bin:$PATH"

# verify
which supah-seo
supah-seo --help
```

> Add `export PATH="$HOME/go/bin:$PATH"` to `~/.zshrc` (or your shell profile) to make this permanent.

## Quick Start

```bash
# 1. Set up credentials
supah-seo login

# 2. Initialize a project
supah-seo init --url https://yoursite.com

# 3. Audit your site (includes automatic PageSpeed check)
supah-seo audit run --url https://yoursite.com --depth 2 --max-pages 50

# 4. Connect Google Search Console
supah-seo auth login gsc
supah-seo gsc sites use https://yoursite.com/
supah-seo gsc query pages
supah-seo gsc query keywords

# 5. Run cross-source analysis
supah-seo analyze

# 6. Check your status
supah-seo status
```

## Commands

### Site crawl and audit

```bash
supah-seo crawl run --url https://example.com --depth 2 --max-pages 50
supah-seo audit run --url https://example.com --depth 2 --max-pages 50
supah-seo audit run --url https://example.com --depth 2 --max-pages 50 --skip-psi
supah-seo report generate --url https://example.com
supah-seo report list
```

`audit run` automatically runs PageSpeed Insights for top pages unless you pass `--skip-psi`.

### Project management

```bash
supah-seo init --url https://example.com
supah-seo status
supah-seo analyze
```

### Google Search Console

```bash
supah-seo gsc sites list
supah-seo gsc sites use https://example.com/

supah-seo gsc query pages --start-date 2026-03-01 --end-date 2026-03-28 --query "brand term" --type web
supah-seo gsc query keywords --limit 50 --page "/pricing" --type web
supah-seo gsc query trends --type web
supah-seo gsc query devices --type web
supah-seo gsc query countries --type web
supah-seo gsc query appearances --type web

supah-seo gsc opportunities
```

### PageSpeed Insights

```bash
supah-seo psi run --url https://example.com --strategy mobile
```

- PSI uses your GSC OAuth token automatically if you've run `supah-seo auth login gsc`. No separate API key needed.
- Optionally set `psi_api_key` for higher rate limits (25,000/day vs OAuth limits).

### SERP

```bash
supah-seo serp analyze --query "seo tools" --dry-run
supah-seo serp analyze --query "seo tools"
supah-seo serp compare --query "seo tools" --query "seo software"
supah-seo serp batch --keywords "keyword1,keyword2,keyword3" --dry-run
```

### Labs

```bash
supah-seo labs ranked-keywords --target example.com --dry-run
supah-seo labs keywords --target example.com --limit 50
supah-seo labs overview --target example.com
supah-seo labs competitors --target example.com --limit 20
supah-seo labs keyword-ideas --keyword "seo tools" --limit 50
supah-seo labs search-intent --keywords "kw1,kw2,kw3"
supah-seo labs bulk-difficulty --from-gsc --dry-run
```

### Backlinks

```bash
supah-seo backlinks summary --target example.com --dry-run
supah-seo backlinks list --target example.com --limit 50 --dofollow-only
supah-seo backlinks referring-domains --target example.com
supah-seo backlinks competitors --target example.com
supah-seo backlinks gap --target example.com
```

### AEO (Answer Engine Optimization)

```bash
# Query an AI model directly
supah-seo aeo responses --prompt "What is Supah SEO?" --model chatgpt --dry-run
supah-seo aeo responses --prompt "What is Supah SEO?" --model claude

# AI search volume for a keyword
supah-seo aeo keywords --keyword "seo tools" --location "United States"
```

Supported models: `chatgpt`, `claude`, `gemini`, `perplexity`

### GEO (Generative Engine Optimization)

```bash
# How often a domain appears in AI responses for a keyword
supah-seo geo mentions --keyword "seo tools" --domain example.com --dry-run
supah-seo geo mentions --keyword "seo tools" --domain example.com

# Which pages are most cited by AI engines
supah-seo geo top-pages --keyword "seo tools"
```

### Opportunities

```bash
supah-seo opportunities
supah-seo opportunities --with-serp --serp-queries 10 --dry-run
supah-seo opportunities --with-serp --serp-queries 10
```

### Auth / Config

```bash
supah-seo auth login gsc
supah-seo auth status
supah-seo auth logout gsc

supah-seo config show
supah-seo config get serp_provider
supah-seo config set serp_provider dataforseo
supah-seo config set psi_api_key YOUR_KEY
supah-seo config path
```

## Environment variables

| Variable | Purpose |
|---|---|
| `SUPAHSEO_CONFIG` | Override config file path |
| `SUPAHSEO_PROVIDER` | Override active provider |
| `SUPAHSEO_API_KEY` | Provider API key override |
| `SUPAHSEO_BASE_URL` | Provider base URL override |
| `SUPAHSEO_ORGANIZATION_ID` | Provider organization/account override |
| `SUPAHSEO_SERP_PROVIDER` | `dataforseo` or `serpapi` |
| `SUPAHSEO_SERP_API_KEY` | SerpAPI key |
| `SUPAHSEO_DATAFORSEO_LOGIN` | DataForSEO login (email) |
| `SUPAHSEO_DATAFORSEO_PASSWORD` | DataForSEO API password |
| `SUPAHSEO_APPROVAL_THRESHOLD_USD` | Cost approval threshold |
| `SUPAHSEO_GSC_PROPERTY` | Active GSC property |
| `SUPAHSEO_GSC_CLIENT_ID` | GSC OAuth client ID |
| `SUPAHSEO_GSC_CLIENT_SECRET` | GSC OAuth client secret |
| `SUPAHSEO_PSI_API_KEY` | Google PageSpeed Insights API key (optional, for higher rate limits) |

## Data Tiers

| Tier | Cost | What you get |
|------|------|-------------|
| Free | $0 | Crawl, audit, PageSpeed Insights, project state |
| Free + OAuth | $0 | Google Search Console data |
| Paid | ~$0.001-0.02/call | SERP features, Labs keywords, AEO/GEO |
| Paid + commitment | $100/mo deposit | Backlinks API |

## Development

```bash
make fmt && make vet && make test && make lint
make precommit
```

See [`ARCHITECTURE.md`](./ARCHITECTURE.md) for package structure and data flow.

---

## Why Supah SEO?

- **One tool, not five** — crawl + audit + GSC + SERP + backlinks in a single binary
- **Free where it counts** — crawling, auditing, PageSpeed, and GSC cost nothing
- **Cost-aware** — every paid call shows a cost estimate before executing, with `--dry-run` support
- **Agent-native** — JSON output on every command, designed for AI agents and automation
- **Cross-source analysis** — finds issues that only appear when you compare crawl data against GSC, SERP features, keyword difficulty, and backlinks together
- **70% cheaper SERP** — batch mode uses DataForSEO Standard Queue at $0.0006/query vs $0.002 live

---

**SEO + AEO + GEO + Backlinks. Open source. Single Go binary.**
