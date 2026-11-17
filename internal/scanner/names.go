package scanner

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// This file holds the pure release-name parsing functions shared by every
// library type. Nothing here touches the filesystem.

const (
	bl = `(?:^|[^\p{L}\p{N}])` // token boundary (left)
	br = `(?:$|[^\p{L}\p{N}])` // token boundary (right)
)

// tok compiles a case-insensitive token pattern wrapped in word boundaries.
// Group 1 is the token itself.
func tok(p string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)` + bl + `(` + p + `)` + br)
}

// allTok returns the submatch indexes of every match of a tok-style regexp.
// Unlike FindAll it lets adjacent tokens share a single separator
// ("2049 2017", "1080p x265").
func allTok(re *regexp.Regexp, s string) [][]int {
	var out [][]int
	pos := 0
	for pos <= len(s) {
		m := re.FindStringSubmatchIndex(s[pos:])
		if m == nil {
			break
		}
		for i := range m {
			if m[i] >= 0 {
				m[i] += pos
			}
		}
		out = append(out, m)
		next := m[3]
		if next <= pos {
			next = pos + 1
		}
		pos = next
	}
	return out
}

// firstTitleTok returns the start of the first match that leaves a non-empty
// title in front of it, or -1.
func firstTitleTok(re *regexp.Regexp, s string) int {
	for _, m := range allTok(re, s) {
		if hasAlnum(s[:m[2]]) {
			return m[2]
		}
	}
	return -1
}

// firstTok returns the start of the first match, or -1.
func firstTok(re *regexp.Regexp, s string) int {
	if m := re.FindStringSubmatchIndex(s); m != nil {
		return m[2]
	}
	return -1
}

func hasAlnum(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// maxYear is the largest number accepted as a release year.
var maxYear = time.Now().Year() + 3

var (
	// strongRe matches release tokens that never occur in real titles; a title
	// is cut at the first one.
	strongRe = tok(`\d{3,4}[pi]|[48]k|uhd|\d{3,4}x\d{3,4}|blu[ \-]?ray|bd[ \-]?rip|br[ \-]?rip|bd[ \-]?remux|remux|bd25|bd50|web[ \-]?dl|web[ \-]?rip|webdl|hdtv(?:rip)?|pdtv|sdtv|dvd[ \-]?rip|dvd[ \-]?scr|dvdr|dvd5|dvd9|hd[ \-]?rip|hd[ \-]?dvd|x[ .]?26[45]|h[ .]?26[45]|hevc|xvid|divx|av1|vp9|avc|true[ \-]?hd|dts(?:[ \-]?hd)?(?:[ .\-]?ma)?|dts[ \-]?x|e[ \-]?ac[ \-]?3|ac[ \-]?3|dd[p+]?[1-9][ .][01]|ddp|dd\+|aac(?:[1-9][ .][01])?|flac|hdr10(?:\+|plus)?|hdr|dolby[ .\-]?vision|dovi|hlg|1[02][ \-]?bit|8[ \-]?bit|hi10p?`)
	// sceneTagRe matches scene tags that only cut a title in dotted names.
	sceneTagRe = tok(`proper|repack\d?|rerip|internal|dubbed|subbed|multisubs?|multi|dual[ \-]?audio|amzn|dsnp|hmax|atvp|pcok|nf|vostfr|truefrench|hardsubs?|readnfo|nfofix|dirfix|hsbs|limited|complete`)
	yearRe     = tok(`18[89]\d|19\d\d|20\d\d`)
	partRe     = tok(`(?:cd|dvd|disc|disk|part|pt)[ .\-]?(\d{1,2})`)
)

// Release is what ParseRelease extracts from a movie file or folder name.
type Release struct {
	Title      string
	Year       int
	Resolution string
	Source     string
	VideoCodec string
	AudioCodec string
	HDR        string
	Edition    string
	IDs        map[string]string // tmdb, imdb, tvdb, anidb
	Part       int               // cd1 / pt2 stacking marker; 0 = none
	Group      string            // leading "[Group]" (fansub) tag
}

var (
	reIDTag      = regexp.MustCompile(`(?i)[\[{(]\s*(tmdb|tmdbid|imdb|imdbid|tvdb|tvdbid|anidb|anidbid)\s*[-=:\s]\s*([a-z0-9]+)\s*[\]})]`)
	reEditionTag = regexp.MustCompile(`(?i)\{\s*edition\s*[-=:]\s*([^}]*)\}`)
	reSitePrefix = regexp.MustCompile(`(?i)^\s*[\[(]?\s*www\.[^\s\])]+\.[a-z]{2,6}\s*[\])]?\s*(?:[-–]\s*)?`)
	reLeadGroup  = regexp.MustCompile(`^\s*[\[【]([^\]】]*)[\]】]\s*`)
	reCRC        = regexp.MustCompile(`[\[(]([0-9A-Fa-f]{8})[\])]`)
	reBareIMDB   = regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}])(tt\d{7,9})(?:$|[^\p{L}\p{N}])`)
)

// prepared is a release name with ids / edition tags / group / CRC removed.
type prepared struct {
	s       string // cleaned name
	n       string // s with separators normalised; same byte length as s
	dotted  bool   // dots (or underscores) are word separators
	ids     map[string]string
	edition string
	group   string
}

func prepare(name string, keepGroup bool) prepared {
	p := prepared{ids: map[string]string{}}
	s := strings.TrimSpace(strings.TrimPrefix(name, "\ufeff"))
	s = reIDTag.ReplaceAllStringFunc(s, func(m string) string {
		sm := reIDTag.FindStringSubmatch(m)
		key := strings.TrimSuffix(strings.ToLower(sm[1]), "id") // tmdbid -> tmdb
		val := sm[2]
		if key == "imdb" {
			val = strings.ToLower(val)
			if !strings.HasPrefix(val, "tt") {
				val = "tt" + val
			}
		}
		if _, ok := p.ids[key]; !ok {
			p.ids[key] = val
		}
		return " "
	})
	if m := reEditionTag.FindStringSubmatchIndex(s); m != nil {
		p.edition = strings.TrimSpace(s[m[2]:m[3]])
		s = s[:m[0]] + " " + s[m[1]:]
	}
	s = reSitePrefix.ReplaceAllString(s, "")
	if !keepGroup {
		if m := reLeadGroup.FindStringSubmatchIndex(s); m != nil {
			g := strings.TrimSpace(s[m[2]:m[3]])
			rest := s[m[1]:]
			if g != "" && hasAlnum(rest) && !looksLikeTag(g) {
				p.group = g
				s = rest
			}
		}
	}
	s = reCRC.ReplaceAllString(s, " ")
	if m := reBareIMDB.FindStringSubmatchIndex(s); m != nil {
		if _, ok := p.ids["imdb"]; !ok {
			p.ids["imdb"] = strings.ToLower(s[m[2]:m[3]])
		}
		s = s[:m[2]] + s[m[3]:]
	}
	s = strings.TrimSpace(s)
	p.s = s
	p.dotted = isDotted(s)
	p.n = normSeps(s, p.dotted)
	return p
}

// looksLikeTag reports whether a leading [..] group is release info rather
// than a fansub group name.
func looksLikeTag(g string) bool {
	if strongRe.MatchString(g) || reCRC.MatchString("["+g+"]") {
		return true
	}
	if n, err := strconv.Atoi(g); err == nil && n > 0 {
		return true
	}
	return false
}

// isDotted reports whether dots act as word separators ("Movie.Title.2019").
func isDotted(s string) bool {
	dots := strings.Count(s, ".")
	return dots >= 1 && dots > strings.Count(s, " ")
}

// normSeps replaces '_' (and '.' in dotted names) with spaces, preserving
// byte offsets.
func normSeps(s string, dotted bool) string {
	b := []byte(s)
	for i, c := range b {
		if c == '_' || (dotted && c == '.') {
			b[i] = ' '
		}
	}
	return string(b)
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// cleanTitle tidies a raw title fragment.
func cleanTitle(s string, dotted bool) string {
	s = collapseSpaces(s)
	for {
		old := s
		s = strings.TrimRight(s, " -–—:,;._~([{+/")
		s = strings.TrimLeft(s, " -–—:,;._~)]}+/")
		if strings.HasSuffix(s, "]") {
			if i := strings.LastIndex(s, "["); i > 0 {
				s = s[:i]
			}
		}
		s = strings.ReplaceAll(s, "()", "")
		s = strings.ReplaceAll(s, "[]", "")
		s = collapseSpaces(s)
		if s == old {
			break
		}
	}
	if dotted {
		s = mergeAcronyms(s)
	}
	return s
}

// mergeAcronyms turns "S H I E L D" (from "S.H.I.E.L.D") back into
// "S.H.I.E.L.D.".
func mergeAcronyms(s string) string {
	words := strings.Fields(s)
	single := func(w string) bool {
		r := []rune(w)
		return len(r) == 1 && unicode.IsUpper(r[0])
	}
	var out []string
	for i := 0; i < len(words); {
		j := i
		for j < len(words) && single(words[j]) {
			j++
		}
		if j-i >= 2 {
			out = append(out, strings.Join(words[i:j], ".")+".")
			i = j
			continue
		}
		out = append(out, words[i])
		i++
	}
	return strings.Join(out, " ")
}

// ParseRelease parses a movie folder or file name (without extension) such as
// "Movie.Title.2019.1080p.BluRay.x265-GRP" or "Movie Title (2019) {tmdb-123}".
func ParseRelease(name string) Release {
	r := parseRelease(name, false)
	if r.Group != "" && (r.Title == "" || (r.Year == 0 && yearRe.MatchString(r.Title) && !hasLetter(r.Title))) {
		// "[REC] (2007)": the bracket was the title, not a group.
		if r2 := parseRelease(name, true); hasLetter(r2.Title) {
			r = r2
		}
	}
	return r
}

func parseRelease(name string, keepGroup bool) Release {
	p := prepare(name, keepGroup)
	r := Release{IDs: p.ids, Edition: p.edition, Group: p.group}
	n := p.n

	cutQ := len(n)
	if i := firstTitleTok(strongRe, n); i >= 0 {
		cutQ = i
	}
	if p.dotted {
		if i := firstTitleTok(sceneTagRe, n); i >= 0 && i < cutQ {
			cutQ = i
		}
	}
	cutB := cutQ
	for i, c := range n {
		if i >= cutB {
			break
		}
		if (c == '[' || c == '{') && hasAlnum(n[:i]) {
			cutB = i
			break
		}
	}

	var paren, bare []int
	for _, m := range allTok(yearRe, n) {
		y, _ := strconv.Atoi(n[m[2]:m[3]])
		if y > maxYear || !hasAlnum(n[:m[2]]) {
			continue
		}
		isParen := m[2] > 0 && strings.ContainsRune("([", rune(n[m[2]-1])) && m[3] < len(n) && strings.ContainsRune(")]", rune(n[m[3]]))
		if isParen && m[2] < cutQ {
			paren = m
		} else if !isParen && m[2] < cutB {
			bare = m
		}
	}
	titleEnd, remStart := cutB, cutB
	if ym := paren; ym != nil || bare != nil {
		if ym == nil {
			ym = bare
		}
		r.Year, _ = strconv.Atoi(n[ym[2]:ym[3]])
		titleEnd, remStart = ym[2], ym[3]
	}

	// Stacking marker (cd1, part2).
	if parts := allTok(partRe, n); len(parts) > 0 {
		m := parts[len(parts)-1]
		v, _ := strconv.Atoi(n[m[4]:m[5]])
		switch {
		case m[2] >= titleEnd:
			r.Part = v
		case r.Year == 0 && strings.Trim(n[m[3]:titleEnd], " -.([") == "" && hasAlnum(n[:m[2]]):
			r.Part = v
			titleEnd = m[2]
		}
	}

	r.Title = cleanTitle(n[:titleEnd], p.dotted)
	rem := n[remStart:]
	if r.Edition == "" {
		r.Edition = findEdition(rem)
	}
	var ed string
	r.Title, ed = stripTrailingEdition(r.Title)
	if r.Edition == "" {
		r.Edition = ed
	}
	parseQuality(rem, &r.Resolution, &r.Source, &r.VideoCodec, &r.AudioCodec, &r.HDR)
	if r.Title == "" && r.Group == "" {
		r.Title = cleanTitle(n, p.dotted)
	}
	return r
}

type namedRe struct {
	re   *regexp.Regexp
	name string
}

var editions = []namedRe{
	{tok(`director'?s?[ \-]?(?:cut|edition)`), "Director's Cut"},
	{tok(`extended(?:[ \-](?:cut|edition|version))?`), "Extended"},
	{tok(`unrated`), "Unrated"},
	{tok(`uncut`), "Uncut"},
	{tok(`(?:\d{1,2}k[ \-])?remaster(?:ed)?`), "Remastered"},
	{tok(`imax(?:[ \-]edition)?`), "IMAX"},
	{tok(`theatrical(?:[ \-](?:cut|edition|version))?`), "Theatrical"},
	{tok(`final[ \-]cut`), "Final Cut"},
	{tok(`ultimate[ \-](?:cut|edition)`), "Ultimate Edition"},
	{tok(`special[ \-]edition`), "Special Edition"},
	{tok(`collector'?s?[ \-]edition`), "Collector's Edition"},
	{tok(`\d{1,3}(?:th)?[ \-]anniversary(?:[ \-]edition)?|anniversary[ \-]edition`), "Anniversary Edition"},
	{tok(`criterion(?:[ \-]collection)?`), "Criterion"},
	{tok(`open[ \-]matte`), "Open Matte"},
}

// findEdition returns the edition tokens in s, in order of appearance.
func findEdition(s string) string {
	type hit struct {
		pos  int
		name string
	}
	var hits []hit
	for _, e := range editions {
		if m := e.re.FindStringSubmatchIndex(s); m != nil {
			hits = append(hits, hit{m[2], e.name})
		}
	}
	if len(hits) == 0 {
		return ""
	}
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].pos < hits[j-1].pos; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	names := make([]string, len(hits))
	for i, h := range hits {
		names[i] = h.name
	}
	return strings.Join(names, " ")
}

// stripTrailingEdition removes an edition phrase from the end of a title
// ("Movie Extended" -> "Movie", "Extended").
func stripTrailingEdition(title string) (string, string) {
	var found []string
	for {
		stripped := false
		for _, e := range editions {
			ms := allTok(e.re, title)
			if len(ms) == 0 {
				continue
			}
			m := ms[len(ms)-1]
			if strings.Trim(title[m[3]:], " )]-") != "" || !hasAlnum(title[:m[2]]) {
				continue
			}
			found = append([]string{e.name}, found...)
			title = cleanTitle(title[:m[2]], false)
			stripped = true
			break
		}
		if !stripped {
			break
		}
	}
	return title, strings.Join(found, " ")
}

var (
	reRes      = tok(`(\d{3,4})([pi])|([48])k|uhd|(\d{3,4})x(\d{3,4})`)
	webAlone   = tok(`web`)
	atmosRe    = tok(`atmos`)
	dvRe       = tok(`dv|dovi|dolby[ .\-]?vision`)
	sourceList = []namedRe{
		{tok(`bd[ \-]?remux|remux`), "Remux"},
		{tok(`blu[ \-]?ray|bd[ \-]?rip|br[ \-]?rip|bd25|bd50|bd|bdmv`), "BluRay"},
		{tok(`web[ \-]?dl|webdl`), "WEB-DL"},
		{tok(`web[ \-]?rip`), "WEBRip"},
		{tok(`hdtv(?:rip)?|pdtv|sdtv|dsr|tvrip`), "HDTV"},
		{tok(`dvd[ \-]?rip|dvd[ \-]?scr|dvdr|dvd5|dvd9|dvd`), "DVD"},
	}
	codecList = []namedRe{
		{tok(`x[ .]?265|h[ .]?265|hevc`), "x265"},
		{tok(`x[ .]?264|h[ .]?264|avc`), "x264"},
		{tok(`av1`), "AV1"},
		{tok(`xvid|divx`), "XviD"},
		{tok(`vp9`), "VP9"},
		{tok(`mpeg[ \-]?2`), "MPEG-2"},
	}
	audioList = func() []namedRe {
		ch := `(?:[ .]?[1-9][ .][01])?`
		return []namedRe{
			{tok(`true[ \-]?hd` + ch), "TrueHD"},
			{tok(`dts[ \-]?hd[ .\-]?ma` + ch), "DTS-HD MA"},
			{tok(`dts[ \-:]?x` + ch), "DTS-X"},
			{tok(`dts[ \-]?hd(?:[ .\-]?hra?)?` + ch), "DTS-HD"},
			{tok(`dts` + ch), "DTS"},
			{tok(`(?:ddp|dd\+|e[ \-]?ac[ \-]?3)` + ch), "DD+"},
			{tok(`(?:dd|ac[ \-]?3|dolby[ .]digital)` + ch), "DD"},
			{tok(`aac(?:[ \-]?lc)?` + ch), "AAC"},
			{tok(`flac` + ch), "FLAC"},
			{tok(`opus` + ch), "Opus"},
			{tok(`mp3`), "MP3"},
			{tok(`l?pcm` + ch), "PCM"},
		}
	}()
	hdrList = []namedRe{
		{tok(`hdr10(?:\+|plus)`), "HDR10+"},
		{tok(`hdr10`), "HDR10"},
		{tok(`hdr`), "HDR"},
		{tok(`hlg`), "HLG"},
	}
)

func firstNamed(list []namedRe, s string) string {
	for _, e := range list {
		if e.re.MatchString(s) {
			return e.name
		}
	}
	return ""
}

// parseQuality extracts release info from the part of a name after the title.
func parseQuality(s string, res, src, vc, ac, hdr *string) {
	*res = parseResolution(s)
	*src = firstNamed(sourceList, s)
	if *src == "" && webAlone.MatchString(s) && strongRe.MatchString(s) {
		*src = "WEB-DL"
	}
	*vc = firstNamed(codecList, s)
	*ac = firstNamed(audioList, s)
	if atmosRe.MatchString(s) {
		if *ac == "" {
			*ac = "Atmos"
		} else {
			*ac += " Atmos"
		}
	}
	h := firstNamed(hdrList, s)
	if dvRe.MatchString(s) {
		if h != "" {
			h = "DV " + h
		} else {
			h = "DV"
		}
	}
	*hdr = h
}

func parseResolution(s string) string {
	for _, m := range allTok(reRes, s) {
		switch {
		case m[4] >= 0:
			v := s[m[4]:m[5]]
			i := strings.ToLower(s[m[6]:m[7]]) == "i"
			switch v {
			case "2160":
				return "2160p"
			case "4320":
				return "4320p"
			case "1080":
				if i {
					return "1080i"
				}
				return "1080p"
			case "1440", "720", "576", "540", "480", "360":
				return v + "p"
			}
		case m[8] >= 0:
			if s[m[8]:m[9]] == "8" {
				return "4320p"
			}
			return "2160p"
		case m[10] >= 0:
			switch s[m[10]:m[11]] {
			case "3840", "4096":
				return "2160p"
			case "1920", "1440":
				return "1080p"
			case "1280":
				return "720p"
			case "720", "704", "640":
				return "480p"
			}
		default: // uhd
			return "2160p"
		}
	}
	return ""
}

var diacritics = strings.NewReplacer(
	"à", "a", "á", "a", "â", "a", "ã", "a", "ä", "a", "å", "a", "æ", "ae", "ç", "c",
	"è", "e", "é", "e", "ê", "e", "ë", "e", "ì", "i", "í", "i", "î", "i", "ï", "i",
	"ð", "d", "ñ", "n", "ò", "o", "ó", "o", "ô", "o", "õ", "o", "ö", "o", "ø", "o",
	"ù", "u", "ú", "u", "û", "u", "ü", "u", "ý", "y", "ÿ", "y", "þ", "th", "ß", "ss",
	"ā", "a", "ă", "a", "ą", "a", "ć", "c", "č", "c", "ď", "d", "đ", "d", "ē", "e",
	"ė", "e", "ę", "e", "ě", "e", "ğ", "g", "ī", "i", "į", "i", "ı", "i", "ł", "l",
	"ľ", "l", "ń", "n", "ň", "n", "ō", "o", "ő", "o", "œ", "oe", "ř", "r", "ś", "s",
	"š", "s", "ş", "s", "ș", "s", "ť", "t", "ţ", "t", "ț", "t", "ū", "u", "ů", "u",
	"ű", "u", "ų", "u", "ź", "z", "ż", "z", "ž", "z",
	"‘", "'", "’", "'", "“", "\"", "”", "\"",
)

// SortTitle returns the sort key for a title: lowercase, Latin diacritics
// folded, leading punctuation and a leading English article removed.
func SortTitle(title string) string {
	s := diacritics.Replace(strings.ToLower(strings.TrimSpace(title)))
	s = strings.TrimLeftFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	for _, a := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(s, a) && hasAlnum(s[len(a):]) {
			s = s[len(a):]
			break
		}
	}
	s = collapseSpaces(s)
	if s == "" {
		return strings.ToLower(strings.TrimSpace(title))
	}
	return s
}

// titleKey is a loose comparison key: alphanumerics only, folded, no article.
func titleKey(s string) string {
	s = diacritics.Replace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "&", " and ")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, s)
	f := strings.Fields(s)
	if len(f) > 1 && (f[0] == "the" || f[0] == "a" || f[0] == "an") {
		f = f[1:]
	}
	return strings.Join(f, " ")
}

// NaturalLess orders strings case-insensitively with embedded numbers
// compared by value ("Episode 2" < "Episode 10").
func NaturalLess(a, b string) bool {
	if c := naturalCompare(a, b); c != 0 {
		return c < 0
	}
	return a < b
}

func isDigitByte(c byte) bool { return c >= '0' && c <= '9' }

func naturalCompare(a, b string) int {
	a, b = strings.ToLower(a), strings.ToLower(b)
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if isDigitByte(a[i]) && isDigitByte(b[j]) {
			si, sj := i, j
			for i < len(a) && isDigitByte(a[i]) {
				i++
			}
			for j < len(b) && isDigitByte(b[j]) {
				j++
			}
			na, nb := strings.TrimLeft(a[si:i], "0"), strings.TrimLeft(b[sj:j], "0")
			if len(na) != len(nb) {
				if len(na) < len(nb) {
					return -1
				}
				return 1
			}
			if c := strings.Compare(na, nb); c != 0 {
				return c
			}
			continue
		}
		ca, cb := a[i], b[j]
		// '/' sorts first so a folder's contents stay together.
		if ca == '/' {
			ca = 0
		}
		if cb == '/' {
			cb = 0
		}
		if ca != cb {
			if ca < cb {
				return -1
			}
			return 1
		}
		i++
		j++
	}
	switch {
	case len(a)-i < len(b)-j:
		return -1
	case len(a)-i > len(b)-j:
		return 1
	}
	return 0
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
