package scanner

import (
	"regexp"
	"strings"
	"unicode"
)

// EpisodeOptions is the folder context ParseEpisode needs to resolve
// ambiguous names.
type EpisodeOptions struct {
	// Anime enables fansub conventions: "[Group] Show - 01", OVA/SP specials,
	// NCOP/NCED extras and absolute numbering.
	Anime bool
	// SeasonFolder is set when the file lives inside a season folder; bare
	// numbers ("02 - Title.mkv") are then accepted and Season is the default.
	SeasonFolder bool
	Season       int
}

// Episode is the result of ParseEpisode. Episode == 0 means no episode number
// was found (the file is an extra).
type Episode struct {
	Show       string // series title in front of the marker (may be empty)
	Year       int    // year found in the series title part
	Season     int
	Episode    int
	EpisodeEnd int    // > Episode for multi-episode files
	Title      string // episode title after the marker
	AirDate    string // YYYY-MM-DD for date-based episodes

	HasSeason bool // the season number came from the file name itself
	Absolute  bool // numbered without a season marker (anime absolute / bare)
	Special   bool // OVA / OAD / SP: season 0
	Extra     bool // creditless OP/ED, PV, CM, menus, recaps (x.5), unnumbered specials

	Release Release // release info (quality, ids, group) from the name
}

var (
	reSxE         = regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])(s(\d{1,4})[ .\-]?e(\d{1,4}))`)
	reMultiE      = regexp.MustCompile(`(?i)^(?:[ .\-]?e|-e?)(\d{1,4})`)
	reNxN         = regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])((\d{1,2})x(\d{2,3}))(?:$|[^\p{L}\p{N}])`)
	reMultiX      = regexp.MustCompile(`(?i)^(?:-\d{1,2}x|-)(\d{2,3})`)
	reSeasonEp    = regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])((?:season|series)[ .\-]?(\d{1,4})[ .\-,]*(?:episode|ep)[ .\-]?(\d{1,4}))`)
	reDate        = regexp.MustCompile(`(?:^|[^\p{N}])(((?:19|20)\d\d)[ .\-](\d\d)[ .\-](\d\d))(?:$|[^\p{N}])`)
	reNCStrong    = tok(`nc[ .\-]?(?:op|ed)\d{0,2}[a-z]?|creditless(?:[ .\-](?:opening|ending|op|ed))?|clean[ .\-](?:opening|ending)`)
	reNCWeak      = regexp.MustCompile(`(?i)(?:^|\s[-–]\s*)((?:op|ed|pv|cm|preview|menu|trailer|teaser|promo|spot|opening|ending)(?:\s*\d{1,2}[a-z]?)?)(?:$|[^\p{L}\p{N}])`)
	reSpecial     = regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])((?:ova|oad|sp|specials?|omake)(?:[ .\-]*(\d{1,3}))?)(?:$|[^\p{L}\p{N}])`)
	reEpWord      = regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])((?:episode[ .\-]?|ep[ .\-]?|e)(\d{1,4})(?:v\d{1,2})?)(?:$|[^\p{L}\p{N}])`)
	reDash        = regexp.MustCompile(`\s[-–]\s*((\d{1,4})(?:v\d{1,2})?(\.\d)?)(?:$|[\s\[(\-_~])`)
	reLead        = regexp.MustCompile(`^\s*((\d{1,4})(?:v\d{1,2})?)(?:$|\s*[\-–.:]\s*|\s)`)
	reTrailGroups = regexp.MustCompile(`(?:\s*[\[(][^\[\]()]*[\])])+\s*$`)
	reTrail       = regexp.MustCompile(`(?:^|\s)((\d{1,4})(?:v\d{1,2})?)\s*$`)
	reShowSeason  = regexp.MustCompile(`(?i)(?:^|\s)(?:(\d{1,2})(?:st|nd|rd|th)\s+season|season\s*(\d{1,2})|s(\d{1,2}))\s*$`)
	reShowSpecial = regexp.MustCompile(`(?i)\s+(?:ova|oad|sp|specials?)$`)
)

func multiEp(n string, end, first int, re *regexp.Regexp) (int, int) {
	last, epEnd := first, 0
	for {
		m := re.FindStringSubmatchIndex(n[end:])
		if m == nil {
			break
		}
		after := end + m[3]
		if after < len(n) {
			c := rune(n[after])
			if (unicode.IsLetter(c) || unicode.IsDigit(c)) && c != 'e' && c != 'E' && c != 'v' {
				break
			}
		}
		v := atoi(n[end+m[2] : end+m[3]])
		if v <= last || v-first > 100 {
			break
		}
		last, epEnd, end = v, v, after
	}
	return epEnd, end
}

func isYearNumber(s string) bool {
	if len(s) != 4 {
		return false
	}
	y := atoi(s)
	return y >= 1900 && y <= maxYear
}

// ParseEpisode parses an episode file name (without extension).
//
// Recognised markers, in priority order: S01E02 (with multi-episode
// S01E02E03 / S01E02-E03 / S01E02-03), 1x02, "Season 1 Episode 2", dates
// (2024.03.15), then (anime) OVA/SP specials, "Episode 2" / "Ep02" / "E02",
// and — only for anime or inside a season folder — " - 02", leading "02 - "
// and (anime with a [Group] tag) a trailing bare number.
//
// Date-based episodes get Season = year and Episode = month*100 + day
// (2024-03-15 -> season 2024, episode 315), which is stable and sorts in air
// order; AirDate carries the full date.
func ParseEpisode(name string, opt EpisodeOptions) Episode {
	p := prepare(name, false)
	n := p.n
	e := Episode{}
	e.Release.IDs, e.Release.Group, e.Release.Edition = p.ids, p.group, p.edition
	start, end := -1, -1
	fallback := opt.Anime || opt.SeasonFolder

	if m := reSxE.FindStringSubmatchIndex(n); m != nil {
		start, end = m[2], m[3]
		e.Season, e.HasSeason = atoi(n[m[4]:m[5]]), true
		e.Episode = atoi(n[m[6]:m[7]])
		e.EpisodeEnd, end = multiEp(n, end, e.Episode, reMultiE)
	} else if m := reNxN.FindStringSubmatchIndex(n); m != nil {
		start, end = m[2], m[3]
		e.Season, e.HasSeason = atoi(n[m[4]:m[5]]), true
		e.Episode = atoi(n[m[6]:m[7]])
		e.EpisodeEnd, end = multiEp(n, end, e.Episode, reMultiX)
	} else if m := reSeasonEp.FindStringSubmatchIndex(n); m != nil {
		start, end = m[2], m[3]
		e.Season, e.HasSeason = atoi(n[m[4]:m[5]]), true
		e.Episode = atoi(n[m[6]:m[7]])
	} else if m := matchDate(n); m != nil {
		start, end = m[2], m[3]
		y, mo, d := n[m[4]:m[5]], n[m[6]:m[7]], n[m[8]:m[9]]
		e.Season, e.HasSeason = atoi(y), true
		e.Episode = atoi(mo)*100 + atoi(d)
		e.AirDate = y + "-" + mo + "-" + d
	}

	if start < 0 {
		if opt.Anime {
			if i := firstTok(reNCStrong, n); i >= 0 {
				return extraEpisode(e, n, i, p.dotted)
			}
			if m := reNCWeak.FindStringSubmatchIndex(n); m != nil {
				return extraEpisode(e, n, m[2], p.dotted)
			}
		}
		specStart, specNumbered := -1, false
		if opt.Anime {
			if m := reSpecial.FindStringSubmatchIndex(n); m != nil {
				specStart = m[2]
				if m[4] >= 0 {
					start, end = m[2], m[5]
					e.Episode, e.Special, specNumbered = atoi(n[m[4]:m[5]]), true, true
				}
			}
		}
		if start < 0 {
			if m := reEpWord.FindStringSubmatchIndex(n); m != nil {
				start, end = m[2], m[3]
				e.Episode = atoi(n[m[4]:m[5]])
			}
		}
		if start < 0 && fallback {
			for _, m := range reDash.FindAllStringSubmatchIndex(n, -1) {
				num := n[m[4]:m[5]]
				if isYearNumber(num) {
					continue
				}
				if m[6] >= 0 { // "12.5": recap / half episode
					return extraEpisode(e, n, m[0], p.dotted)
				}
				start, end = m[2], m[3]
				e.Episode = atoi(num)
				break
			}
		}
		if start < 0 && fallback {
			if m := reLead.FindStringSubmatchIndex(n); m != nil && !isYearNumber(n[m[4]:m[5]]) {
				start, end = m[2], m[3]
				v := atoi(n[m[4]:m[5]])
				// "101 - Pilot" inside "Season 1" means S01E01.
				if opt.SeasonFolder && opt.Season > 0 && v >= 100 && v/100 == opt.Season && v%100 > 0 {
					v %= 100
				}
				e.Episode = v
			}
		}
		if start < 0 && opt.Anime && p.group != "" {
			stripped := reTrailGroups.ReplaceAllString(n, "")
			if m := reTrail.FindStringSubmatchIndex(stripped); m != nil && !isYearNumber(stripped[m[4]:m[5]]) && hasAlnum(stripped[:m[2]]) {
				start, end = m[2], m[3]
				e.Episode = atoi(stripped[m[4]:m[5]])
			}
		}
		if start < 0 {
			if specStart >= 0 {
				e.Special = true
				return extraEpisode(e, n, specStart, p.dotted)
			}
			e.Show, e.Year, _, _ = splitShow(n, p.dotted)
			e.Release.Title, e.Release.Year = e.Show, e.Year
			parseQuality(n, &e.Release.Resolution, &e.Release.Source, &e.Release.VideoCodec, &e.Release.AudioCodec, &e.Release.HDR)
			return e
		}
		if specStart >= 0 && !specNumbered && (specStart < start || (specStart > 0 && strings.ContainsRune("[(", rune(n[specStart-1])))) {
			e.Special = true
		}
		e.Absolute = !e.Special
	}

	var animeSeason int
	var showSpecial bool
	e.Show, e.Year, animeSeason, showSpecial = splitShow(n[:start], p.dotted)
	if showSpecial && opt.Anime {
		e.Special, e.Absolute = true, false
	}
	switch {
	case e.HasSeason:
	case e.Special:
		e.Season = 0
	case opt.SeasonFolder:
		e.Season = opt.Season
	case animeSeason > 0:
		e.Season = animeSeason
	default:
		e.Season = 1
	}
	if e.EpisodeEnd <= e.Episode {
		e.EpisodeEnd = 0
	}
	rem := n[end:]
	e.Title = episodeTitle(rem, p.dotted)
	e.Release.Title, e.Release.Year = e.Show, e.Year
	parseQuality(rem, &e.Release.Resolution, &e.Release.Source, &e.Release.VideoCodec, &e.Release.AudioCodec, &e.Release.HDR)
	if e.Release.Edition == "" {
		e.Release.Edition = findEdition(rem[cutAtQuality(rem, p.dotted):])
	}
	return e
}

func extraEpisode(e Episode, n string, showEnd int, dotted bool) Episode {
	e.Extra = true
	e.Episode, e.EpisodeEnd, e.Season = 0, 0, 0
	e.Show, e.Year, _, _ = splitShow(n[:showEnd], dotted)
	e.Release.Title, e.Release.Year = e.Show, e.Year
	parseQuality(n[showEnd:], &e.Release.Resolution, &e.Release.Source, &e.Release.VideoCodec, &e.Release.AudioCodec, &e.Release.HDR)
	return e
}

func matchDate(n string) []int {
	for _, m := range reDate.FindAllStringSubmatchIndex(n, -1) {
		mo, d := atoi(n[m[6]:m[7]]), atoi(n[m[8]:m[9]])
		if mo >= 1 && mo <= 12 && d >= 1 && d <= 31 && atoi(n[m[4]:m[5]]) <= maxYear {
			return m
		}
	}
	return nil
}

// splitShow cleans the series-title part of an episode name and pulls out a
// trailing year, an anime-style season ("2nd Season", "S2") and an OVA/SP
// marker.
func splitShow(s string, dotted bool) (title string, year, season int, special bool) {
	t := cleanTitle(s, dotted)
	if m := reShowSpecial.FindStringIndex(t); m != nil && hasAlnum(t[:m[0]]) {
		special = true
		t = cleanTitle(t[:m[0]], false)
	}
	if m := reShowSeason.FindStringSubmatchIndex(t); m != nil && hasAlnum(t[:m[0]]) {
		for g := 2; g <= 6; g += 2 {
			if m[g] >= 0 {
				season = atoi(t[m[g]:m[g+1]])
			}
		}
		t = cleanTitle(t[:m[0]], false)
	}
	if ms := allTok(yearRe, t); len(ms) > 0 {
		m := ms[len(ms)-1]
		y := atoi(t[m[2]:m[3]])
		if strings.Trim(t[m[3]:], " )]") == "" && hasAlnum(t[:m[2]]) && y <= maxYear {
			year = y
			t = cleanTitle(t[:m[2]], false)
		}
	}
	return t, year, season, special
}

// cutAtQuality returns where release info starts in an episode-title tail.
func cutAtQuality(s string, dotted bool) int {
	cut := len(s)
	if i := firstTok(strongRe, s); i >= 0 && i < cut {
		cut = i
	}
	if dotted {
		if i := firstTok(sceneTagRe, s); i >= 0 && i < cut {
			cut = i
		}
	}
	if i := strings.IndexAny(s, "[{"); i >= 0 && i < cut {
		cut = i
	}
	if open := strings.LastIndex(s[:cut], "("); open >= 0 && !strings.Contains(s[open:cut], ")") {
		cut = open
	}
	return cut
}

func episodeTitle(rem string, dotted bool) string {
	rem = strings.TrimLeft(rem, " -–—._:~")
	raw := rem[:cutAtQuality(rem, dotted)]
	if !dotted && isDotted(strings.TrimSpace(raw)) {
		raw = normSeps(raw, true) // "Show - S01E01 - Title.Part.1"
		dotted = true
	}
	t := cleanTitle(raw, dotted)
	if !hasAlnum(t) {
		return ""
	}
	t, _ = stripTrailingEdition(t)
	return t
}

var (
	reSeasonDir  = regexp.MustCompile(`(?i)^(?:season|series|saison|staffel|temporada|stagione|seizoen|sezon)[ ._\-]*(\d{1,4})(?:$|[^\p{N}])`)
	reSDir       = regexp.MustCompile(`(?i)^s(\d{1,4})(?:$|[ ._\-\[(])`)
	reSeasonAny  = regexp.MustCompile(`(?i)(?:^|[ ._\-])(?:s(\d{1,2})|season[ ._\-]?(\d{1,2})|(\d{1,2})(?:st|nd|rd|th)[ ._\-]season)(?:$|[ ._\-\[(])`)
	reSeasonZero = regexp.MustCompile(`(?i)^(?:specials?|ovas?|oads?|sps?|season[ ._\-]*0+|s0+)$`)
)

// ParseSeasonFolder recognises season folders: "Season 1", "Season 01",
// "S01", "Specials" / "Season 0" (season 0), "Staffel 2", and season-pack
// names such as "Show.Name.S02.1080p.WEB-DL" or "Show 2nd Season".
func ParseSeasonFolder(name string) (int, bool) {
	s := strings.TrimSpace(name)
	if reSeasonZero.MatchString(s) {
		return 0, true
	}
	if m := reSeasonDir.FindStringSubmatch(s); m != nil {
		return atoi(m[1]), true
	}
	if m := reSDir.FindStringSubmatch(s); m != nil && !reSxE.MatchString(s) {
		return atoi(m[1]), true
	}
	if reSxE.MatchString(s) {
		return 0, false
	}
	if m := reSeasonAny.FindStringSubmatch(s); m != nil {
		for _, g := range m[1:] {
			if g != "" {
				return atoi(g), true
			}
		}
	}
	return 0, false
}

func squash(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}

var extrasDirs = map[string]bool{
	"extras": true, "extra": true, "featurettes": true, "featurette": true,
	"behindthescenes": true, "deletedscenes": true, "deleted": true,
	"interviews": true, "interview": true, "scenes": true, "shorts": true,
	"short": true, "trailers": true, "trailer": true, "samples": true,
	"sample": true, "other": true, "others": true, "bonus": true,
	"bonusfeatures": true, "bonusmaterial": true, "specialfeatures": true,
	"makingof": true, "clips": true, "nc": true, "ncop": true, "nced": true,
	"ncopnced": true, "creditless": true, "menus": true, "menu": true,
	"scans": true, "cm": true, "cms": true, "pv": true, "pvs": true,
	"promos": true, "promo": true, "openings": true, "endings": true,
	"teasers": true, "previews": true,
}

// isExtrasDir reports whether a folder holds extras. Movies also treat
// "Specials" as extras; for shows it is season 0.
func isExtrasDir(name string, movies bool) bool {
	k := squash(name)
	if extrasDirs[k] {
		return true
	}
	return movies && (k == "specials" || k == "special")
}

func isSubsDir(name string) bool {
	switch squash(name) {
	case "subs", "sub", "subtitles", "subtitle", "subtitulos", "untertitel":
		return true
	}
	return false
}

var reDiscDir = regexp.MustCompile(`(?i)^(?:cd|disc|disk|dvd|part|pt)[ ._\-]?(\d{1,3})$`)

// partDirNumber returns N for "CD1", "Disc 2", "Part 3" folders.
func partDirNumber(name string) int {
	if m := reDiscDir.FindStringSubmatch(strings.TrimSpace(name)); m != nil {
		return atoi(m[1])
	}
	return 0
}

var (
	reExtraSuffix = regexp.MustCompile(`(?i)-(trailer|teaser|behindthescenes|deleted|deletedscene|featurette|interview|scene|short|other|extra|clip)$`)
	reSample      = tok(`sample`)
)

func isExtraName(stem string) bool { return reExtraSuffix.MatchString(stem) }
func isSampleName(stem string) bool {
	return reSample.MatchString(strings.ReplaceAll(stem, "_", " "))
}
