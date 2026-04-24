// Package diff compares two .supah-seo/state.json snapshots and produces a
// view model representing what changed between them. Used by
// 'supah-seo report compare' to render a progress-over-time deliverable for
// retainer clients.
package diff

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/supah-seo/supah-seo/internal/state"
)

//go:embed templates/*.tmpl.html
var templateFS embed.FS

// Options configure the compare output.
type Options struct {
	ProspectName   string
	AgencyName     string
	AgencyLogoB64  string
	FromLabel      string // display label for the "from" snapshot
	ToLabel        string // display label for the "to" snapshot
	TopMoversLimit int    // cap the number of keyword rows shown (default 20)
}

// View is the top-level data bound to the compare template.
type View struct {
	GeneratedAt   time.Time
	AgencyName    string
	AgencyLogoB64 string
	ProspectName  string
	Site          string
	FromLabel     string
	ToLabel       string
	FromDate      string
	ToDate        string
	DaysBetween   int

	ScoreFrom  float64
	ScoreTo    float64
	ScoreDelta float64
	ScoreArrow string // up | down | flat

	FindingsFrom     int
	FindingsTo       int
	NewFindings      []state.Finding
	ResolvedFindings []state.Finding
	PersistentCount  int

	KeywordMovers []KeywordMove // improved and declined, sorted by impact
	NewRankings   []KeywordMove // ranked "to" that weren't in "from"
	LostRankings  []KeywordMove // ranked "from" but not "to"

	BacklinksFrom BacklinksSnapshot
	BacklinksTo   BacklinksSnapshot

	PSIMovers []PSIMove
}

// KeywordMove summarises a position change for one keyword.
type KeywordMove struct {
	Keyword      string
	Volume       int
	FromPosition int
	ToPosition   int
	Delta        int    // positive = improved (lower position number)
	Direction    string // up | down | new | lost
	Impact       int    // rough weight used for sorting (volume * abs(delta))
}

// BacklinksSnapshot is a point-in-time subset of authority metrics.
type BacklinksSnapshot struct {
	RefDomains     int64
	TotalBacklinks int64
	SpamScore      float64
}

// PSIMove tracks LCP change for a URL between snapshots.
type PSIMove struct {
	URL       string
	LCPFrom   float64 // seconds
	LCPTo     float64
	Delta     float64 // seconds (positive = slower, negative = faster)
	Direction string  // faster | slower | flat | new | lost
}

// Compute builds a View from two states.
func Compute(from, to *state.State, opt Options) View {
	v := View{
		GeneratedAt:   time.Now(),
		AgencyName:    fallback(opt.AgencyName, "Douro Digital"),
		AgencyLogoB64: opt.AgencyLogoB64,
		ProspectName:  fallback(opt.ProspectName, domainFromURL(to.Site)),
		Site:          to.Site,
		FromLabel:     fallback(opt.FromLabel, "Baseline"),
		ToLabel:       fallback(opt.ToLabel, "Current"),
		ScoreFrom:     from.Score,
		ScoreTo:       to.Score,
	}
	v.ScoreDelta = to.Score - from.Score
	v.ScoreArrow = arrow(v.ScoreDelta, 0.5)

	v.FromDate, v.ToDate, v.DaysBetween = datesBetween(from, to)

	v.FindingsFrom = len(from.Findings)
	v.FindingsTo = len(to.Findings)
	v.NewFindings, v.ResolvedFindings, v.PersistentCount = diffFindings(from.Findings, to.Findings)

	limit := opt.TopMoversLimit
	if limit <= 0 {
		limit = 20
	}
	v.KeywordMovers, v.NewRankings, v.LostRankings = diffKeywords(from, to, limit)

	v.BacklinksFrom = snapshotBacklinks(from)
	v.BacklinksTo = snapshotBacklinks(to)

	v.PSIMovers = diffPSI(from, to, limit)

	return v
}

// LoadSnapshot reads a state file from disk.
func LoadSnapshot(path string) (*state.State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s state.State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Render produces the HTML diff report.
func Render(v View) ([]byte, error) {
	tpl, err := template.New("compare.tmpl.html").Funcs(template.FuncMap{
		"upper": strings.ToUpper,
		"date":  func(t time.Time) string { return t.Format("2 January 2006") },
		"abs": func(i int) int {
			if i < 0 {
				return -i
			}
			return i
		},
		"absf": func(f float64) float64 {
			if f < 0 {
				return -f
			}
			return f
		},
		"safeURL": func(s string) template.URL { return template.URL(s) },
	}).ParseFS(templateFS, "templates/compare.tmpl.html")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// --- internals ---

func diffFindings(from, to []state.Finding) (added, resolved []state.Finding, persistent int) {
	key := func(f state.Finding) string {
		return f.Rule + "|" + f.URL
	}
	fromSet := map[string]state.Finding{}
	for _, f := range from {
		fromSet[key(f)] = f
	}
	toSet := map[string]state.Finding{}
	for _, f := range to {
		toSet[key(f)] = f
	}
	for k, f := range toSet {
		if _, ok := fromSet[k]; !ok {
			added = append(added, f)
		} else {
			persistent++
		}
	}
	for k, f := range fromSet {
		if _, ok := toSet[k]; !ok {
			resolved = append(resolved, f)
		}
	}
	sort.SliceStable(added, func(i, j int) bool { return added[i].Rule < added[j].Rule })
	sort.SliceStable(resolved, func(i, j int) bool { return resolved[i].Rule < resolved[j].Rule })
	return
}

func diffKeywords(from, to *state.State, limit int) (movers, newRank, lostRank []KeywordMove) {
	fromKW := map[string]state.LabsKeyword{}
	if from.Labs != nil {
		for _, k := range from.Labs.Keywords {
			fromKW[strings.ToLower(k.Keyword)] = k
		}
	}
	toKW := map[string]state.LabsKeyword{}
	if to.Labs != nil {
		for _, k := range to.Labs.Keywords {
			toKW[strings.ToLower(k.Keyword)] = k
		}
	}

	for k, cur := range toKW {
		prev, ok := fromKW[k]
		if !ok {
			newRank = append(newRank, KeywordMove{
				Keyword:    cur.Keyword,
				Volume:     cur.SearchVolume,
				ToPosition: cur.Position,
				Direction:  "new",
				Impact:     cur.SearchVolume,
			})
			continue
		}
		if cur.Position == prev.Position {
			continue
		}
		delta := prev.Position - cur.Position // positive = improved
		direction := "down"
		if delta > 0 {
			direction = "up"
		}
		abs := delta
		if abs < 0 {
			abs = -abs
		}
		movers = append(movers, KeywordMove{
			Keyword:      cur.Keyword,
			Volume:       cur.SearchVolume,
			FromPosition: prev.Position,
			ToPosition:   cur.Position,
			Delta:        delta,
			Direction:    direction,
			Impact:       cur.SearchVolume * abs,
		})
	}
	for k, prev := range fromKW {
		if _, ok := toKW[k]; ok {
			continue
		}
		lostRank = append(lostRank, KeywordMove{
			Keyword:      prev.Keyword,
			Volume:       prev.SearchVolume,
			FromPosition: prev.Position,
			Direction:    "lost",
			Impact:       prev.SearchVolume,
		})
	}

	sort.SliceStable(movers, func(i, j int) bool { return movers[i].Impact > movers[j].Impact })
	sort.SliceStable(newRank, func(i, j int) bool { return newRank[i].Volume > newRank[j].Volume })
	sort.SliceStable(lostRank, func(i, j int) bool { return lostRank[i].Volume > lostRank[j].Volume })

	if len(movers) > limit {
		movers = movers[:limit]
	}
	if len(newRank) > limit {
		newRank = newRank[:limit]
	}
	if len(lostRank) > limit {
		lostRank = lostRank[:limit]
	}
	return
}

func diffPSI(from, to *state.State, limit int) []PSIMove {
	fromPSI := map[string]state.PSIResult{}
	if from.PSI != nil {
		for _, p := range from.PSI.Pages {
			fromPSI[p.URL+"|"+p.Strategy] = p
		}
	}
	toPSI := map[string]state.PSIResult{}
	if to.PSI != nil {
		for _, p := range to.PSI.Pages {
			toPSI[p.URL+"|"+p.Strategy] = p
		}
	}

	var out []PSIMove
	for k, cur := range toPSI {
		prev, ok := fromPSI[k]
		if !ok {
			out = append(out, PSIMove{
				URL:       cur.URL,
				LCPTo:     cur.LCP / 1000.0,
				Direction: "new",
			})
			continue
		}
		fromSec := prev.LCP / 1000.0
		toSec := cur.LCP / 1000.0
		delta := toSec - fromSec
		dir := "flat"
		switch {
		case delta < -0.2:
			dir = "faster"
		case delta > 0.2:
			dir = "slower"
		}
		out = append(out, PSIMove{
			URL:       cur.URL,
			LCPFrom:   fromSec,
			LCPTo:     toSec,
			Delta:     delta,
			Direction: dir,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		// Largest absolute delta first (biggest improvements and regressions).
		ai := out[i].Delta
		if ai < 0 {
			ai = -ai
		}
		aj := out[j].Delta
		if aj < 0 {
			aj = -aj
		}
		return ai > aj
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func snapshotBacklinks(s *state.State) BacklinksSnapshot {
	if s.Backlinks == nil {
		return BacklinksSnapshot{}
	}
	return BacklinksSnapshot{
		RefDomains:     s.Backlinks.TotalReferringDomains,
		TotalBacklinks: s.Backlinks.TotalBacklinks,
		SpamScore:      s.Backlinks.SpamScore,
	}
}

func datesBetween(from, to *state.State) (fromDate, toDate string, days int) {
	fromT, _ := time.Parse(time.RFC3339, firstNonEmpty(from.LastCrawl, from.LastAnalysis, from.Initialized))
	toT, _ := time.Parse(time.RFC3339, firstNonEmpty(to.LastCrawl, to.LastAnalysis, to.Initialized))
	if !fromT.IsZero() {
		fromDate = fromT.Format("2 January 2006")
	}
	if !toT.IsZero() {
		toDate = toT.Format("2 January 2006")
	}
	if !fromT.IsZero() && !toT.IsZero() {
		days = int(toT.Sub(fromT).Hours() / 24)
	}
	return
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func arrow(delta, tolerance float64) string {
	switch {
	case delta > tolerance:
		return "up"
	case delta < -tolerance:
		return "down"
	default:
		return "flat"
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

// Force fmt import (kept for symmetry with other packages; safe to remove later).
var _ = fmt.Sprintf
