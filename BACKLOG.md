# Backlog

Features we agreed are worth building but deliberately parked. Not included in CLAUDE.md on purpose so Claude won't surface them in future sessions unless you bring them up.

When you're ready to pick one of these up, tell Claude: "implement the [name] from BACKLOG.md" and it'll find this file.

---

## 1. Google Business Profile audit

**What it is.** A section in the audit that scores a prospect's GBP vs their local pack competitors. Covers: claim status, verification, review count, star rating, review recency, photo count, primary category alignment, hours completeness, NAP consistency. Output feeds into both the client and agency reports as a dedicated section.

**Why it's parked.** Needs an external data source and we haven't picked one yet.

**When to build it.** Once we have a prospect where GBP is obviously the bottleneck, or once the first batch of client reports feels incomplete without it.

**Recommended data source.** Scraper.Tech Google Maps API (scraper.tech/en/google-maps-unofficial-api/).
- 1,000 free requests per month, paid tiers start at $3/month for 30K.
- Returns `is_claimed`, `verified`, rating, review count, categories, hours, photos array.
- The claim status field is the killer finding and most other providers don't expose it.

**Alternative.** Google Places API (New) direct. Uses your existing Google Cloud billing. 10K free events/month on Essentials SKU. Missing explicit claim status but has `businessStatus`. Standard endpoint: `POST https://places.googleapis.com/v1/places:searchText` with `X-Goog-Api-Key` + `X-Goog-FieldMask` headers.

**Rough scope.** 3 to 4 hours:
- New package `internal/gbp` with the API client
- `supah-seo gbp audit --query "<business name> <city>"` command that fetches the profile, scores it, persists findings to state.json under a `gbp` block
- Extend the render view model to include `GBPAudit` and surface it in both client and agency templates
- Config field for `scrapertech_api_key`

**Suggested audit scoring rules.**

| Check | Severity if fails |
|---|---|
| Profile not claimed | Critical |
| Profile not verified | High |
| Review count under 50% of local pack median | High |
| Star rating under 4.3 | High |
| Latest review over 90 days old | Medium |
| Photo count under 20 | Medium |
| Primary category mismatched to search intent | High |
| Hours missing for any day | Low |
| Website link missing | Medium |

---

## 2. Daily spend cap

**What it is.** A persistent tracker that counts USD spent on paid API calls per day and refuses (or degrades) further calls once a configured daily limit is hit.

**Why it's parked.** Only meaningful when automated runs happen without you watching. Right now you're at the keyboard for every audit and you see the cost in real time. Building this now would be defensive code for a problem you don't have.

**When to build it.** The moment you set up any of these:
- A cron job that re-audits retainer clients weekly
- Unattended batch runs on large prospect CSVs
- An internal dashboard that lets a teammate trigger audits
- A Zapier or similar trigger that fires audits on events

**Recommended behaviour when the cap trips.** Skip paid calls but let free ones (crawl, audit, PSI) continue. Audit completes thinner rather than failing entirely. Log a clear message in the report metadata so downstream tooling can see what was skipped. Alternative is a hard stop, but that breaks batch pipelines mid-run which is worse for an automation use case.

**Rough scope.** 2 hours:
- New file `.supah-seo/spend-YYYY-MM-DD.json` per day tracking running total
- Wrap `dataforseo.Client` calls with a middleware that reads the file, checks against config `daily_spend_limit_usd`, returns an early "over budget" error when exceeded
- Add `supah-seo config set daily_spend_limit_usd <float>` support
- Add `supah-seo spend status` command to see today's running total
- Decision: roll over at UTC midnight or local midnight. UTC is simpler and what most APIs bill on.

---

## Notes

- Both are additive. Neither blocks anything currently working.
- Add items here when we agree something is worth doing later rather than now.
- Remove items here when they ship, or when we decide they're no longer worth doing.
