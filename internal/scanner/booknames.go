package scanner

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ---- subtitle languages ----

var langCodes = func() map[string]string {
	m := map[string]string{}
	add := func(code string, aliases ...string) {
		m[code] = code
		for _, a := range aliases {
			m[a] = code
		}
	}
	add("en", "eng", "english")
	add("es", "spa", "spanish", "español", "espanol", "castellano", "latino")
	add("fr", "fre", "fra", "french", "français", "francais")
	add("de", "ger", "deu", "german", "deutsch")
	add("it", "ita", "italian", "italiano")
	add("pt", "por", "portuguese", "português", "portugues")
	add("nl", "dut", "nld", "dutch", "nederlands")
	add("sv", "swe", "swedish", "svenska")
	add("no", "nor", "nob", "nno", "norwegian", "norsk")
	add("da", "dan", "danish", "dansk")
	add("fi", "fin", "finnish", "suomi")
	add("pl", "pol", "polish", "polski")
	add("ru", "rus", "russian")
	add("ja", "jpn", "japanese", "jp")
	add("zh", "chi", "zho", "chinese", "chs", "cht")
	add("ko", "kor", "korean")
	add("ar", "ara", "arabic")
	add("he", "heb", "hebrew")
	add("hi", "hin", "hindi")
	add("tr", "tur", "turkish")
	add("el", "gre", "ell", "greek")
	add("cs", "cze", "ces", "czech")
	add("hu", "hun", "hungarian")
	add("ro", "rum", "ron", "romanian")
	add("th", "tha", "thai")
	add("vi", "vie", "vietnamese")
	add("id", "ind", "indonesian")
	add("ms", "may", "msa", "malay")
	add("uk", "ukr", "ukrainian")
	add("bg", "bul", "bulgarian")
	add("hr", "hrv", "croatian")
	add("sr", "srp", "serbian")
	add("sk", "slo", "slk", "slovak")
	add("sl", "slv", "slovenian")
	add("et", "est", "estonian")
	add("lv", "lav", "latvian")
	add("lt", "lit", "lithuanian")
	add("fa", "per", "fas", "persian", "farsi")
	add("is", "ice", "isl", "icelandic")
	add("ca", "cat", "catalan")
	add("eu", "baq", "eus", "basque")
	add("gl", "glg", "galician")
	return m
}()

var subFlags = map[string]bool{
	"forced": true, "sdh": true, "hi": true, "cc": true, "default": true,
	"full": true, "signs": true, "songs": true, "foreign": true,
	"commentary": true, "dialogue": true,
}

var reLangRegion = regexp.MustCompile(`(?i)^([a-z]{2,3})[-_]([a-z]{2}|[a-z]{4})$`)

// ParseSubtitleLanguage returns the language code of a subtitle file from its
// name (without extension): "Movie.en.srt" -> "en", "Movie.eng.forced.srt"
// -> "en", "2_English.srt" -> "en", "Movie.pt-BR.srt" -> "pt-BR".
// videoStem, when non-empty and a prefix of stem, is removed first.
func ParseSubtitleLanguage(stem, videoStem string) string {
	s := stem
	if videoStem != "" && len(s) > len(videoStem) && strings.EqualFold(s[:len(videoStem)], videoStem) {
		s = s[len(videoStem):]
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '.' || r == '_' || r == ' ' || r == '[' || r == ']' || r == '(' || r == ')'
	})
	checked := 0
	for i := len(parts) - 1; i >= 0 && checked < 4; i-- {
		t := strings.ToLower(parts[i])
		if subFlags[t] {
			continue
		}
		checked++
		if m := reLangRegion.FindStringSubmatch(parts[i]); m != nil {
			if code, ok := langCodes[strings.ToLower(m[1])]; ok {
				r := m[2]
				if len(r) == 2 {
					r = strings.ToUpper(r)
				} else {
					r = strings.ToUpper(r[:1]) + strings.ToLower(r[1:])
				}
				return code + "-" + r
			}
		}
		if code, ok := langCodes[t]; ok {
			return code
		}
		// "Movie-en" style: try the last dash-separated piece.
		if j := strings.LastIndexByte(t, '-'); j >= 0 {
			if code, ok := langCodes[t[j+1:]]; ok {
				return code
			}
		}
	}
	return ""
}

// ---- books / audiobooks ----

type bookName struct {
	Title    string
	Author   string
	Narrator string
	Series   string
	Index    float64
	Year     int
	ASIN     string
	ISBN     string
}

var (
	reNarrator    = regexp.MustCompile(`\{([^{}]+)\}`)
	reASIN        = regexp.MustCompile(`[\[(]\s*(B0[0-9A-Z]{8})\s*[\])]`)
	reISBNTag     = regexp.MustCompile(`(?i)[\[(]\s*(?:isbn[ :]*)?((?:97[89][\- ]?)?\d{1,5}[\- ]?\d{1,7}[\- ]?\d{1,7}[\- ]?[\dX])\s*[\])]`)
	reYearParen   = regexp.MustCompile(`[\[(]\s*(\d{4})\s*[\])]`)
	reCalibreID   = regexp.MustCompile(`\s*\((\d{1,7})\)\s*$`)
	reBrackets    = regexp.MustCompile(`\s*\[[^\[\]]*\]`)
	reAbridged    = regexp.MustCompile(`(?i)\s*[(\[]\s*(?:un)?abridged\s*[)\]]`)
	reSeriesParen = regexp.MustCompile(`(?i)\s*\(\s*([^()]+?)[,\s]*(?:#|book\s+|bk\.?\s*|vol\.?\s*|volume\s+|no\.?\s*|part\s+)(\d+(?:\.\d+)?)\s*\)\s*$`)
	reDashSplit   = regexp.MustCompile(`\s+[-–—]\s+`)
	reIndexOnly   = regexp.MustCompile(`(?i)^(?:book|bk|vol(?:ume)?|part|no|#)?\.?\s*#?\s*(\d{1,3}(?:\.\d+)?)$`)
	reSeriesHash  = regexp.MustCompile(`^(.+?)\s*#\s*(\d+(?:\.\d+)?)$`)
	reSeriesNum   = regexp.MustCompile(`(?i)^(.+?)[,\s]+(?:book\s+|bk\.?\s*|vol\.?\s*|volume\s+)?(\d{1,3}(?:\.\d+)?)$`)
	reLeadIndex   = regexp.MustCompile(`(?i)^(?:(?:book|bk|vol(?:ume)?|part)\.?\s*)?(\d{1,3}(?:\.\d+)?)(?:\.\s+|\s*[:]\s*)(.+)$`)
	reLeadWordIdx = regexp.MustCompile(`(?i)^(?:book|bk|vol(?:ume)?)\.?\s*(\d{1,3}(?:\.\d+)?)\s+(.+)$`)
	reByAuthor    = regexp.MustCompile(`(?i)\s+by\s+(.+)$`)
)

// sameAuthor compares author names loosely ("Tolkien, J.R.R." == "J.R.R. Tolkien").
func sameAuthor(a, b string) bool {
	ka, kb := strings.Fields(titleKey(a)), strings.Fields(titleKey(b))
	if len(ka) == 0 || len(ka) != len(kb) {
		return false
	}
	seen := map[string]int{}
	for _, w := range ka {
		seen[w]++
	}
	for _, w := range kb {
		if seen[w] == 0 {
			return false
		}
		seen[w]--
	}
	return true
}

func bookYearOK(y int) bool { return y >= 1400 && y <= maxYear }

// parseBookName parses a book / audiobook file or folder name. author is the
// author known from the folder hierarchy ("" when unknown: the name is then
// read as "Author - Title"). calibre strips a trailing "(123)" Calibre id.
func parseBookName(name, author string, calibre bool) bookName {
	b := bookName{Author: author}
	s := strings.TrimSpace(name)
	s = strings.ReplaceAll(s, "_ ", ": ") // Calibre writes ':' as '_'
	s = strings.ReplaceAll(s, "_", " ")
	if !strings.Contains(s, " ") && strings.Count(s, ".") >= 2 {
		s = strings.ReplaceAll(s, ".", " ")
	}
	if m := reNarrator.FindStringSubmatchIndex(s); m != nil {
		b.Narrator = strings.TrimSpace(s[m[2]:m[3]])
		s = s[:m[0]] + " " + s[m[1]:]
	}
	if m := reASIN.FindStringSubmatchIndex(s); m != nil {
		b.ASIN = s[m[2]:m[3]]
		s = s[:m[0]] + " " + s[m[1]:]
	}
	if m := reISBNTag.FindStringSubmatchIndex(s); m != nil {
		if isbn := normISBN(s[m[2]:m[3]]); isbn != "" {
			b.ISBN = isbn
			s = s[:m[0]] + " " + s[m[1]:]
		}
	}
	s = strings.TrimSpace(s)
	if calibre {
		if m := reCalibreID.FindStringSubmatchIndex(s); m != nil {
			s = strings.TrimSpace(s[:m[0]])
		}
	}
	for _, m := range reYearParen.FindAllStringSubmatchIndex(s, -1) {
		if y := atoi(s[m[2]:m[3]]); bookYearOK(y) {
			b.Year = y
			s = s[:m[0]] + " " + s[m[1]:]
			break
		}
	}
	s = reAbridged.ReplaceAllString(s, "")
	if t := reBrackets.ReplaceAllString(s, ""); hasAlnum(t) {
		s = t
	}
	s = collapseSpaces(s)
	if m := reSeriesParen.FindStringSubmatchIndex(s); m != nil && hasAlnum(s[:m[0]]) {
		b.Series = strings.TrimSpace(s[m[2]:m[3]])
		b.Index, _ = strconv.ParseFloat(s[m[4]:m[5]], 64)
		s = strings.TrimSpace(s[:m[0]])
	}

	parts := reDashSplit.Split(s, -1)
	if author != "" {
		var kept []string
		for _, p := range parts {
			if sameAuthor(p, author) && len(parts) > 1 {
				continue
			}
			kept = append(kept, p)
		}
		parts = kept
		if len(parts) == 1 {
			if m := reByAuthor.FindStringSubmatchIndex(parts[0]); m != nil && sameAuthor(parts[0][m[2]:m[3]], author) {
				parts[0] = parts[0][:m[0]]
			}
		}
	} else if len(parts) >= 2 {
		// Prefer "Author - Title"; fall back to "Title - Author" when the
		// first part is clearly a title (series marker or index).
		first := parts[0]
		if reIndexOnly.MatchString(first) || reSeriesHash.MatchString(first) {
			b.Author = parts[len(parts)-1]
			parts = parts[:len(parts)-1]
		} else {
			b.Author = first
			parts = parts[1:]
		}
	}

	// Leading index / year parts: "01 - Title", "Book 2 - Title",
	// "Vol 1 - 2015 - Title", "Series #3 - Title", "Series 03 - Title".
	for i := 0; i < 3 && len(parts) >= 2; i++ {
		first := strings.TrimSpace(parts[0])
		if m := reIndexOnly.FindStringSubmatch(first); m != nil {
			b.Index, _ = strconv.ParseFloat(m[1], 64)
			parts = parts[1:]
			continue
		}
		if len(first) == 4 && bookYearOK(atoi(first)) {
			b.Year = atoi(first)
			parts = parts[1:]
			continue
		}
		if m := reSeriesHash.FindStringSubmatch(first); m != nil {
			b.Series = strings.TrimSpace(m[1])
			b.Index, _ = strconv.ParseFloat(m[2], 64)
			parts = parts[1:]
			continue
		}
		if m := reSeriesNum.FindStringSubmatch(first); m != nil && hasLetter(m[1]) {
			b.Series = strings.TrimSpace(m[1])
			b.Index, _ = strconv.ParseFloat(m[2], 64)
			parts = parts[1:]
			continue
		}
		break
	}
	title := strings.TrimSpace(strings.Join(parts, " - "))
	if b.Index == 0 {
		if m := reLeadIndex.FindStringSubmatch(title); m != nil && hasLetter(m[2]) {
			b.Index, _ = strconv.ParseFloat(m[1], 64)
			title = m[2]
		} else if m := reLeadWordIdx.FindStringSubmatch(title); m != nil && hasLetter(m[2]) {
			b.Index, _ = strconv.ParseFloat(m[1], 64)
			title = m[2]
		}
	}
	if b.Series == "" {
		if m := reSeriesHash.FindStringSubmatch(title); m != nil && b.Index == 0 {
			// "Series Name #3" with nothing else: keep as title but record it.
			b.Series = strings.TrimSpace(m[1])
			b.Index, _ = strconv.ParseFloat(m[2], 64)
		}
	}
	b.Title = cleanTitle(title, false)
	if b.Title == "" {
		b.Title = cleanTitle(s, false)
	}
	return b
}

func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// normISBN returns a 10/13 character ISBN without separators, or "".
func normISBN(s string) string {
	s = strings.ToUpper(s)
	s = strings.TrimPrefix(s, "URN:ISBN:")
	s = strings.TrimPrefix(s, "ISBN:")
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == 'X' {
			b.WriteRune(r)
		} else if r != '-' && r != ' ' {
			return ""
		}
	}
	out := b.String()
	if len(out) == 13 && !strings.Contains(out, "X") && (strings.HasPrefix(out, "978") || strings.HasPrefix(out, "979")) {
		return out
	}
	if len(out) == 10 && !strings.Contains(out[:9], "X") {
		return out
	}
	return ""
}

var (
	reTrackWord  = regexp.MustCompile(`(?i)(?:^|[^\p{L}])(?:part|pt|track|chapter|chap|ch|kapitel)[ ._\-]*(\d{1,4})`)
	reDiscTrack  = regexp.MustCompile(`^\s*(\d{1,2})[-.](\d{2,3})(?:\D|$)`)
	reLeadTrack  = regexp.MustCompile(`^\s*(\d{1,4})(?:\D|$)`)
	reTrailTrack = regexp.MustCompile(`(\d{1,4})\s*$`)
	reDiscInName = regexp.MustCompile(`(?i)(?:^|[^\p{L}])(?:cd|disc|disk)[ ._\-]?(\d{1,2})`)
)

// parseTrack extracts (disc, track) numbers from an audio file name
// (without extension). Zero means unknown.
func parseTrack(stem string) (disc, track int) {
	s := strings.ReplaceAll(stem, "_", " ")
	if m := reDiscInName.FindStringSubmatchIndex(s); m != nil {
		disc = atoi(s[m[2]:m[3]])
		s = s[:m[0]] + " " + s[m[1]:]
	}
	if m := reDiscTrack.FindStringSubmatch(s); m != nil && disc == 0 {
		return atoi(m[1]), atoi(m[2])
	}
	if m := reTrackWord.FindStringSubmatch(s); m != nil {
		return disc, atoi(m[1])
	}
	if m := reLeadTrack.FindStringSubmatch(s); m != nil && !isYearNumber(m[1]) {
		return disc, atoi(m[1])
	}
	t := reTrailGroups.ReplaceAllString(s, "")
	if m := reTrailTrack.FindStringSubmatch(t); m != nil && !isYearNumber(m[1]) {
		return disc, atoi(m[1])
	}
	return disc, 0
}
