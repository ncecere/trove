package metadata

import (
	"strings"
	"unicode"
)

var fold = strings.NewReplacer(
	"à", "a", "á", "a", "â", "a", "ã", "a", "ä", "a", "å", "a", "æ", "ae", "ç", "c",
	"è", "e", "é", "e", "ê", "e", "ë", "e", "ì", "i", "í", "i", "î", "i", "ï", "i",
	"ñ", "n", "ò", "o", "ó", "o", "ô", "o", "õ", "o", "ö", "o", "ø", "o", "ù", "u",
	"ú", "u", "û", "u", "ü", "u", "ý", "y", "ÿ", "y", "ß", "ss", "ā", "a", "ē", "e",
	"ī", "i", "ō", "o", "ū", "u", "ł", "l", "ś", "s", "š", "s", "ž", "z", "ź", "z",
	"ż", "z", "č", "c", "ć", "c", "ř", "r", "ň", "n", "ń", "n", "œ", "oe",
	"’", "'", "‘", "'",
)

// normTitle lowercases, folds Latin diacritics, maps '&' to "and", drops
// punctuation and a leading English article.
func normTitle(s string) string {
	s = fold.Replace(strings.ToLower(s))
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

// similarity returns 0..1 for two titles (1 = equal after normalisation).
func similarity(a, b string) float64 {
	na, nb := normTitle(a), normTitle(b)
	if na == "" || nb == "" {
		return 0
	}
	if na == nb {
		return 1
	}
	lev := levRatio(na, nb)
	dice := tokenDice(na, nb)
	s := lev
	if dice > s {
		s = dice
	}
	return s
}

// titleSimilarity also compares the main title before a ':' subtitle.
func titleSimilarity(query, cand string) float64 {
	best := similarity(query, cand)
	for _, pair := range [][2]string{{query, mainTitle(cand)}, {mainTitle(query), cand}} {
		if s := similarity(pair[0], pair[1]) * 0.95; s > best {
			best = s
		}
	}
	return best
}

func mainTitle(s string) string {
	if i := strings.IndexAny(s, ":("); i > 0 {
		return s[:i]
	}
	return s
}

func levRatio(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 && len(rb) == 0 {
		return 1
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return 1 - float64(prev[len(rb)])/float64(max(len(ra), len(rb)))
}

func tokenDice(a, b string) float64 {
	ta, tb := strings.Fields(a), strings.Fields(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	count := map[string]int{}
	for _, t := range ta {
		count[t]++
	}
	inter := 0
	for _, t := range tb {
		if count[t] > 0 {
			inter++
			count[t]--
		}
	}
	return 2 * float64(inter) / float64(len(ta)+len(tb))
}

// personSimilarity compares author names, tolerating "Last, First" order
// and initials.
func personSimilarity(a, b string) float64 {
	ta, tb := strings.Fields(normTitle(a)), strings.Fields(normTitle(b))
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	sa := map[string]bool{}
	for _, t := range ta {
		sa[t] = true
	}
	inter := 0
	for _, t := range tb {
		if sa[t] {
			inter++
		}
	}
	s := 2 * float64(inter) / float64(len(ta)+len(tb))
	// Same surname (last token, or first token in "Last, First") is a strong hint.
	if s < 0.8 && (ta[len(ta)-1] == tb[len(tb)-1] || ta[0] == tb[len(tb)-1] || ta[len(ta)-1] == tb[0]) {
		s = max(s, 0.8)
	}
	return s
}

func yearOf(date string) int {
	if len(date) >= 4 {
		y := 0
		for _, c := range date[:4] {
			if c < '0' || c > '9' {
				return 0
			}
			y = y*10 + int(c-'0')
		}
		return y
	}
	return 0
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
