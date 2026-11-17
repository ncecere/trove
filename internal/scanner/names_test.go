package scanner

import (
	"reflect"
	"sort"
	"testing"
)

func TestParseReleaseTitleYear(t *testing.T) {
	cases := []struct {
		in    string
		title string
		year  int
	}{
		{"Movie Title (2019)", "Movie Title", 2019},
		{"Movie.Title.2019.1080p.BluRay.x265-GRP", "Movie Title", 2019},
		{"Movie_Title_2019_720p", "Movie Title", 2019},
		{"2001 A Space Odyssey (1968)", "2001 A Space Odyssey", 1968},
		{"2001.A.Space.Odyssey.1968.2160p.UHD.BluRay.x265-GRP", "2001 A Space Odyssey", 1968},
		{"Blade Runner 2049 (2017)", "Blade Runner 2049", 2017},
		{"Blade.Runner.2049.2017.1080p.WEB-DL.DDP5.1.H.264-GRP", "Blade Runner 2049", 2017},
		{"Blade Runner 2049", "Blade Runner 2049", 0},
		{"1917 (2019)", "1917", 2019},
		{"1917.2019.1080p.BluRay.x264", "1917", 2019},
		{"1917", "1917", 0},
		{"Se7en (1995)", "Se7en", 1995},
		{"Se7en.1995.REMASTERED.1080p.BluRay", "Se7en", 1995},
		{"2012 (2009)", "2012", 2009},
		{"2046 (2004)", "2046", 2004},
		{"Wonder Woman 1984 (2020)", "Wonder Woman 1984", 2020},
		{"Mr. Holland's Opus (1995)", "Mr. Holland's Opus", 1995},
		{"Charlotte's Web (2006)", "Charlotte's Web", 2006},
		{"Charlotte's.Web.2006.1080p.WEBRip", "Charlotte's Web", 2006},
		{"Mad.Max.Fury.Road.2015.1080p", "Mad Max Fury Road", 2015},
		{"It.2017.1080p.BluRay", "It", 2017},
		{"Real.Steel.2011.720p", "Real Steel", 2011},
		{"Internal Affairs (1990)", "Internal Affairs", 1990},
		{"S.W.A.T. (2017)", "S.W.A.T.", 2017},
		{"Marvel's.The.Avengers.2012.1080p", "Marvel's The Avengers", 2012},
		{"Amélie (2001)", "Amélie", 2001},
		{"Movie Title (2019) {tmdb-12345}", "Movie Title", 2019},
		{"Movie Title (2019) [imdbid-tt1234567]", "Movie Title", 2019},
		{"Movie Title (2019) {edition-Director's Cut}", "Movie Title", 2019},
		{"Movie Title (2019) - Director's Cut", "Movie Title", 2019},
		{"Movie Title (2019) [1080p]", "Movie Title", 2019},
		{"Movie.Title.Extended.1080p.BluRay", "Movie Title", 0},
		{"Movie Title", "Movie Title", 0},
		{"[REC] (2007)", "[REC]", 2007},
		{"Birdman or (The Unexpected Virtue of Ignorance) (2014)", "Birdman or (The Unexpected Virtue of Ignorance)", 2014},
		{"www.Torrenting.com - Movie.Title.2019.720p", "Movie Title", 2019},
		{"The.Matrix.1999.1080p.BluRay.REMUX.AVC.DTS-HD.MA.5.1-GRP", "The Matrix", 1999},
		{"Movie.Title.2019.PROPER.1080p", "Movie Title", 2019},
		{"Movie Title cd1", "Movie Title", 0},
		{"Movie.Title.2019.CD2", "Movie Title", 2019},
		{"Harry Potter and the Deathly Hallows Part 1 (2010)", "Harry Potter and the Deathly Hallows Part 1", 2010},
		{"Dr. Strangelove or How I Learned to Stop Worrying and Love the Bomb (1964)", "Dr. Strangelove or How I Learned to Stop Worrying and Love the Bomb", 1964},
		{"Ocean's Eleven (2001)", "Ocean's Eleven", 2001},
		{"Ocean's.Eleven.2001.1080p", "Ocean's Eleven", 2001},
		{"Fantastic Four (2015)", "Fantastic Four", 2015},
		{"Star Wars Episode IV - A New Hope (1977)", "Star Wars Episode IV - A New Hope", 1977},
		{"Kill.Bill.Vol.1.2003.1080p", "Kill Bill Vol 1", 2003},
		{"Movie Title 2019 1080p", "Movie Title", 2019},
	}
	for _, c := range cases {
		r := ParseRelease(c.in)
		if r.Title != c.title || r.Year != c.year {
			t.Errorf("ParseRelease(%q) = %q (%d), want %q (%d)", c.in, r.Title, r.Year, c.title, c.year)
		}
	}
}

func TestParseReleaseQuality(t *testing.T) {
	type q struct{ res, src, vc, ac, hdr, ed string }
	cases := []struct {
		in   string
		want q
	}{
		{"Movie.2019.1080p.BluRay.x265-GRP", q{"1080p", "BluRay", "x265", "", "", ""}},
		{"Movie.2019.2160p.UHD.BluRay.REMUX.HDR.HEVC.TrueHD.7.1.Atmos-GRP", q{"2160p", "Remux", "x265", "TrueHD Atmos", "HDR", ""}},
		{"Movie.2019.2160p.WEB-DL.DDP5.1.Atmos.DV.HDR10.H.265-GRP", q{"2160p", "WEB-DL", "x265", "DD+ Atmos", "DV HDR10", ""}},
		{"Movie.2019.4K.WEBRip.x264.AAC2.0", q{"2160p", "WEBRip", "x264", "AAC", "", ""}},
		{"Movie (2019) [Bluray-1080p][DTS-HD MA 5.1][x264]-GRP", q{"1080p", "BluRay", "x264", "DTS-HD MA", "", ""}},
		{"Movie.2019.720p.HDTV.x264.AC3", q{"720p", "HDTV", "x264", "DD", "", ""}},
		{"Movie.2019.DVDRip.XviD.MP3", q{"", "DVD", "XviD", "MP3", "", ""}},
		{"Movie.2019.1080i.HDTV.DD5.1.MPEG2", q{"1080i", "HDTV", "MPEG-2", "DD", "", ""}},
		{"Movie.2019.576p.BDRip.AV1.Opus", q{"576p", "BluRay", "AV1", "Opus", "", ""}},
		{"Movie.2019.480p.WEB.VP9.FLAC", q{"480p", "WEB-DL", "VP9", "FLAC", "", ""}},
		{"Movie.2019.2160p.BluRay.HDR10+.DTS-X.x265", q{"2160p", "BluRay", "x265", "DTS-X", "HDR10+", ""}},
		{"Movie.2019.2160p.BluRay.HLG.DTS.x265", q{"2160p", "BluRay", "x265", "DTS", "HLG", ""}},
		{"Movie.2019.2160p.WEB-DL.Dolby.Vision.EAC3", q{"2160p", "WEB-DL", "", "DD+", "DV", ""}},
		{"Movie.2019.Extended.Cut.1080p.BRRip", q{"1080p", "BluRay", "", "", "", "Extended"}},
		{"Movie.2019.Directors.Cut.1080p", q{"1080p", "", "", "", "", "Director's Cut"}},
		{"Movie.2019.UNRATED.720p", q{"720p", "", "", "", "", "Unrated"}},
		{"Movie.2019.IMAX.2160p", q{"2160p", "", "", "", "", "IMAX"}},
		{"Movie.2019.Theatrical.1080p", q{"1080p", "", "", "", "", "Theatrical"}},
		{"Movie (2019) {edition-Final Cut} [1080p]", q{"1080p", "", "", "", "", "Final Cut"}},
		{"Movie.2019.1920x1080.H264", q{"1080p", "", "x264", "", "", ""}},
		{"Movie.2019.REMASTERED.1080p", q{"1080p", "", "", "", "", "Remastered"}},
	}
	for _, c := range cases {
		r := ParseRelease(c.in)
		got := q{r.Resolution, r.Source, r.VideoCodec, r.AudioCodec, r.HDR, r.Edition}
		if got != c.want {
			t.Errorf("ParseRelease(%q) quality = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestParseReleaseIDsAndPart(t *testing.T) {
	cases := []struct {
		in   string
		ids  map[string]string
		part int
	}{
		{"Movie (2019) {tmdb-603}", map[string]string{"tmdb": "603"}, 0},
		{"Movie (2019) [tmdbid-603]", map[string]string{"tmdb": "603"}, 0},
		{"Movie (2019) {imdb-tt0133093}", map[string]string{"imdb": "tt0133093"}, 0},
		{"Movie (2019) [imdbid-tt0133093] [tmdbid-603]", map[string]string{"imdb": "tt0133093", "tmdb": "603"}, 0},
		{"Show (2010) [tvdbid-12345]", map[string]string{"tvdb": "12345"}, 0},
		{"Movie (2019) tt0133093", map[string]string{"imdb": "tt0133093"}, 0},
		{"Movie (2019) - cd1", map[string]string{}, 1},
		{"Movie.2019.1080p.pt2", map[string]string{}, 2},
		{"Movie (2019) - part 3", map[string]string{}, 3},
		{"Movie (2019) disc2", map[string]string{}, 2},
	}
	for _, c := range cases {
		r := ParseRelease(c.in)
		if !reflect.DeepEqual(r.IDs, c.ids) || r.Part != c.part {
			t.Errorf("ParseRelease(%q) ids=%v part=%d, want %v %d", c.in, r.IDs, r.Part, c.ids, c.part)
		}
	}
}

func TestParseEpisode(t *testing.T) {
	type want struct {
		show            string
		season, ep, end int
		title           string
	}
	cases := []struct {
		in   string
		opt  EpisodeOptions
		want want
	}{
		{"Show.Name.S01E02.Episode.Title.720p.HDTV.x264-GRP", EpisodeOptions{}, want{"Show Name", 1, 2, 0, "Episode Title"}},
		{"Show Name - S01E02 - Episode Title [WEBDL-1080p]", EpisodeOptions{}, want{"Show Name", 1, 2, 0, "Episode Title"}},
		{"Show Name (2010) - S01E01 - Pilot (1) [Bluray-1080p][x265]-GRP", EpisodeOptions{}, want{"Show Name", 1, 1, 0, "Pilot (1)"}},
		{"show.name.s01e02e03.720p", EpisodeOptions{}, want{"show name", 1, 2, 3, ""}},
		{"Show Name - S01E02-E03 - Title", EpisodeOptions{}, want{"Show Name", 1, 2, 3, "Title"}},
		{"Show Name - S01E02-03", EpisodeOptions{}, want{"Show Name", 1, 2, 3, ""}},
		{"Show.Name.S01E01E02E03.1080p", EpisodeOptions{}, want{"Show Name", 1, 1, 3, ""}},
		{"Show.Name.S01E02.1080p.WEB-DL", EpisodeOptions{}, want{"Show Name", 1, 2, 0, ""}},
		{"Show.Name.S01E02-1080p", EpisodeOptions{}, want{"Show Name", 1, 2, 0, ""}},
		{"Show Name 1x02 Title", EpisodeOptions{}, want{"Show Name", 1, 2, 0, "Title"}},
		{"Show Name - 10x15 - Title", EpisodeOptions{}, want{"Show Name", 10, 15, 0, "Title"}},
		{"Show Name 1x02-1x03", EpisodeOptions{}, want{"Show Name", 1, 2, 3, ""}},
		{"Show.Name.S01.E02.720p", EpisodeOptions{}, want{"Show Name", 1, 2, 0, ""}},
		{"Show Name Season 1 Episode 2", EpisodeOptions{}, want{"Show Name", 1, 2, 0, ""}},
		{"Show Name S2024E0315", EpisodeOptions{}, want{"Show Name", 2024, 315, 0, ""}},
		{"The.Daily.Show.2024.03.15.Guest.Name.720p.WEB", EpisodeOptions{}, want{"The Daily Show", 2024, 315, 0, "Guest Name"}},
		{"Show (2010) - 2024-03-15 - Title", EpisodeOptions{}, want{"Show", 2024, 315, 0, "Title"}},
		{"Show.Name.S00E05.Special", EpisodeOptions{}, want{"Show Name", 0, 5, 0, "Special"}},
		{"24.S01E01.720p", EpisodeOptions{}, want{"24", 1, 1, 0, ""}},
		{"1923.S01E01.1080p", EpisodeOptions{}, want{"1923", 1, 1, 0, ""}},
		{"Doctor.Who.2005.S01E01.Rose", EpisodeOptions{}, want{"Doctor Who", 1, 1, 0, "Rose"}},
		{"Marvels.Agents.of.S.H.I.E.L.D.S01E01.720p", EpisodeOptions{}, want{"Marvels Agents of S.H.I.E.L.D.", 1, 1, 0, ""}},
		// season folder context
		{"02 - Title", EpisodeOptions{SeasonFolder: true, Season: 3}, want{"", 3, 2, 0, "Title"}},
		{"02. Title", EpisodeOptions{SeasonFolder: true, Season: 3}, want{"", 3, 2, 0, "Title"}},
		{"Episode 2", EpisodeOptions{SeasonFolder: true, Season: 1}, want{"", 1, 2, 0, ""}},
		{"Ep02 - Title", EpisodeOptions{SeasonFolder: true, Season: 2}, want{"", 2, 2, 0, "Title"}},
		{"E05", EpisodeOptions{SeasonFolder: true, Season: 0}, want{"", 0, 5, 0, ""}},
		{"105 - Title", EpisodeOptions{SeasonFolder: true, Season: 1}, want{"", 1, 5, 0, "Title"}},
		{"Show Name - 03 - Title", EpisodeOptions{SeasonFolder: true, Season: 2}, want{"Show Name", 2, 3, 0, "Title"}},
		{"Show Name S01E02", EpisodeOptions{SeasonFolder: true, Season: 4}, want{"Show Name", 1, 2, 0, ""}},
		// anime
		{"[SubsPlease] Show Name - 01 (1080p) [ABCD1234]", EpisodeOptions{Anime: true}, want{"Show Name", 1, 1, 0, ""}},
		{"[Group] Show Name - 01 [1080p][HEVC]", EpisodeOptions{Anime: true}, want{"Show Name", 1, 1, 0, ""}},
		{"[Group] Show Name - 01v2 (BD 1080p)", EpisodeOptions{Anime: true}, want{"Show Name", 1, 1, 0, ""}},
		{"[Group] Show Name - 12 - The Ending [1080p]", EpisodeOptions{Anime: true}, want{"Show Name", 1, 12, 0, "The Ending"}},
		{"Show Name - S02E05", EpisodeOptions{Anime: true}, want{"Show Name", 2, 5, 0, ""}},
		{"[Group] Show Name 2nd Season - 03 [720p]", EpisodeOptions{Anime: true}, want{"Show Name", 2, 3, 0, ""}},
		{"[Group] Show Name S3 - 07 [720p]", EpisodeOptions{Anime: true}, want{"Show Name", 3, 7, 0, ""}},
		{"[Group] One Piece - 1071 [1080p]", EpisodeOptions{Anime: true}, want{"One Piece", 1, 1071, 0, ""}},
		{"[Group] Show Name - 05", EpisodeOptions{Anime: true, SeasonFolder: true, Season: 2}, want{"Show Name", 2, 5, 0, ""}},
		{"[Group] Show Name 03 [1080p]", EpisodeOptions{Anime: true}, want{"Show Name", 1, 3, 0, ""}},
		{"[Group] Mob Psycho 100 - 05 [1080p]", EpisodeOptions{Anime: true}, want{"Mob Psycho 100", 1, 5, 0, ""}},
		{"[Group] 86 - 02 [1080p]", EpisodeOptions{Anime: true}, want{"86", 1, 2, 0, ""}},
		{"Show Name - Episode 05 - Title", EpisodeOptions{Anime: true}, want{"Show Name", 1, 5, 0, "Title"}},
		{"[Group] Show Name - OVA 02 [1080p]", EpisodeOptions{Anime: true}, want{"Show Name", 0, 2, 0, ""}},
		{"[Group] Show Name OVA - 02 [1080p]", EpisodeOptions{Anime: true}, want{"Show Name", 0, 2, 0, ""}},
		{"[Group] Show Name - SP1 [1080p]", EpisodeOptions{Anime: true}, want{"Show Name", 0, 1, 0, ""}},
		{"[Group] Show Name - 05 [OVA][1080p]", EpisodeOptions{Anime: true}, want{"Show Name", 0, 5, 0, ""}},
	}
	for _, c := range cases {
		e := ParseEpisode(c.in, c.opt)
		got := want{e.Show, e.Season, e.Episode, e.EpisodeEnd, e.Title}
		if got != c.want || e.Extra {
			t.Errorf("ParseEpisode(%q, %+v) = %+v (extra=%v), want %+v", c.in, c.opt, got, e.Extra, c.want)
		}
	}
}

func TestParseEpisodeDetails(t *testing.T) {
	e := ParseEpisode("The.Daily.Show.2024.03.15.Guest.Name.720p.WEB", EpisodeOptions{})
	if e.AirDate != "2024-03-15" {
		t.Errorf("AirDate = %q", e.AirDate)
	}
	e = ParseEpisode("Show (2010) - S01E01 - Pilot [Bluray-1080p][DTS-HD MA 5.1][x265]", EpisodeOptions{})
	if e.Year != 2010 || e.Release.Resolution != "1080p" || e.Release.Source != "BluRay" || e.Release.VideoCodec != "x265" || e.Release.AudioCodec != "DTS-HD MA" {
		t.Errorf("details = %+v", e)
	}
	e = ParseEpisode("[SubsPlease] Show - 01 (1080p) [ABCD1234]", EpisodeOptions{Anime: true})
	if e.Release.Group != "SubsPlease" || !e.Absolute {
		t.Errorf("anime details = %+v", e)
	}
	// "The Web" in an episode title must not become a WEB source.
	e = ParseEpisode("Show.S01E01.The.Web.1080p.HDTV.x264", EpisodeOptions{})
	if e.Title != "The Web" || e.Release.Source != "HDTV" {
		t.Errorf("web title = %+v", e)
	}
}

func TestParseEpisodeNonEpisodes(t *testing.T) {
	cases := []struct {
		in    string
		opt   EpisodeOptions
		extra bool
	}{
		{"Show Name Behind the Scenes", EpisodeOptions{}, false},
		{"Movie.Title.2019.1080p", EpisodeOptions{}, false},
		{"02 - Title", EpisodeOptions{}, false}, // bare number outside a season folder (tv)
		{"[Group] Show Name - NCOP1 [1080p]", EpisodeOptions{Anime: true}, true},
		{"[Group] Show Name - NCED [1080p]", EpisodeOptions{Anime: true}, true},
		{"[Group] Show Name NCOP 2 [1080p]", EpisodeOptions{Anime: true}, true},
		{"[Group] Show Name - Creditless Opening [1080p]", EpisodeOptions{Anime: true}, true},
		{"[Group] Show Name - OP2 [1080p]", EpisodeOptions{Anime: true}, true},
		{"[Group] Show Name - PV [1080p]", EpisodeOptions{Anime: true}, true},
		{"[Group] Show Name - Preview 02 [1080p]", EpisodeOptions{Anime: true}, true},
		{"[Group] Show Name - OVA [1080p]", EpisodeOptions{Anime: true}, true},
		{"[Group] Show Name - 12.5 [1080p]", EpisodeOptions{Anime: true}, true},
		{"[Group] Movie Name (2019) [BD 1080p]", EpisodeOptions{Anime: true}, false},
		{"Kimi no Na wa (2016) [1080p]", EpisodeOptions{Anime: true}, false},
		{"Show S01E00 Pilot", EpisodeOptions{}, false},
	}
	for _, c := range cases {
		e := ParseEpisode(c.in, c.opt)
		if e.Episode != 0 || e.Extra != c.extra {
			t.Errorf("ParseEpisode(%q) = ep %d extra %v, want ep 0 extra %v", c.in, e.Episode, e.Extra, c.extra)
		}
	}
}

func TestParseSeasonFolder(t *testing.T) {
	cases := []struct {
		in string
		s  int
		ok bool
	}{
		{"Season 1", 1, true}, {"Season 01", 1, true}, {"season.02", 2, true}, {"S01", 1, true},
		{"S03 (2015)", 3, true}, {"Specials", 0, true}, {"Season 0", 0, true}, {"Season 00", 0, true},
		{"Staffel 2", 2, true}, {"Season 2024", 2024, true}, {"Show.Name.S02.1080p.WEB-DL-GRP", 2, true},
		{"Show Name 2nd Season", 2, true}, {"OVA", 0, true}, {"Extras", 0, false}, {"Featurettes", 0, false},
		{"Show.Name.S01E01.720p", 0, false}, {"Subs", 0, false}, {"Disc 1", 0, false},
	}
	for _, c := range cases {
		s, ok := ParseSeasonFolder(c.in)
		if s != c.s || ok != c.ok {
			t.Errorf("ParseSeasonFolder(%q) = %d,%v want %d,%v", c.in, s, ok, c.s, c.ok)
		}
	}
}

func TestSortTitle(t *testing.T) {
	cases := map[string]string{
		"The Matrix":        "matrix",
		"A Quiet Place":     "quiet place",
		"An American Tail":  "american tail",
		"The":               "the",
		"Amélie":            "amelie",
		"Éclair":            "eclair",
		"'Salem's Lot":      "salem's lot",
		"  The   Godfather": "godfather",
		"A.I. Artificial":   "a.i. artificial",
		"千と千尋の神隠し":          "千と千尋の神隠し",
		"Theater Camp":      "theater camp",
		"Ánother":           "another",
	}
	for in, want := range cases {
		if got := SortTitle(in); got != want {
			t.Errorf("SortTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNaturalLess(t *testing.T) {
	in := []string{"Episode 10.mkv", "episode 2.mkv", "Episode 1.mkv", "Disc 2/01.mp3", "Disc 1/10.mp3", "Disc 1/2.mp3", "Season 10/a", "Season 9/a"}
	want := []string{"Disc 1/2.mp3", "Disc 1/10.mp3", "Disc 2/01.mp3", "Episode 1.mkv", "episode 2.mkv", "Episode 10.mkv", "Season 9/a", "Season 10/a"}
	sort.Slice(in, func(i, j int) bool { return NaturalLess(in[i], in[j]) })
	if !reflect.DeepEqual(in, want) {
		t.Errorf("natural sort = %v", in)
	}
}

func TestParseSubtitleLanguage(t *testing.T) {
	cases := []struct{ stem, video, want string }{
		{"Movie.en", "Movie", "en"},
		{"Movie.eng.forced", "Movie", "en"},
		{"Movie.English", "Movie", "en"},
		{"Movie.2019.1080p.BluRay.x264.spa", "Movie.2019.1080p.BluRay.x264", "es"},
		{"2_English", "", "en"},
		{"3_French", "", "fr"},
		{"Movie.pt-BR", "Movie", "pt-BR"},
		{"Movie.zh-Hans.sdh", "Movie", "zh-Hans"},
		{"Movie.de.sdh", "Movie", "de"},
		{"Movie", "Movie", ""},
		{"Show.S01E01.ger.forced", "", "de"},
		{"Movie.nl.hi", "Movie", "nl"},
	}
	for _, c := range cases {
		if got := ParseSubtitleLanguage(c.stem, c.video); got != c.want {
			t.Errorf("ParseSubtitleLanguage(%q,%q) = %q, want %q", c.stem, c.video, got, c.want)
		}
	}
}

func TestParseBookName(t *testing.T) {
	cases := []struct {
		in, author string
		calibre    bool
		want       bookName
	}{
		{"Book Title - Author Name", "Author Name", false, bookName{Title: "Book Title", Author: "Author Name"}},
		{"Book Title (2001)", "Author Name", false, bookName{Title: "Book Title", Author: "Author Name", Year: 2001}},
		{"Book Title (123)", "Author Name", true, bookName{Title: "Book Title", Author: "Author Name"}},
		{"Author Name - Book Title", "", false, bookName{Title: "Book Title", Author: "Author Name"}},
		{"Series Name #3 - Book Title", "Author", false, bookName{Title: "Book Title", Author: "Author", Series: "Series Name", Index: 3}},
		{"Book Title (Series Name #3)", "Author", false, bookName{Title: "Book Title", Author: "Author", Series: "Series Name", Index: 3}},
		{"Book Title (Series Name, Book 2)", "Author", false, bookName{Title: "Book Title", Author: "Author", Series: "Series Name", Index: 2}},
		{"Series Name 03 - Book Title", "Author", false, bookName{Title: "Book Title", Author: "Author", Series: "Series Name", Index: 3}},
		{"01 - Book Title", "Author", false, bookName{Title: "Book Title", Author: "Author", Index: 1}},
		{"Book 2 - Book Title", "Author", false, bookName{Title: "Book Title", Author: "Author", Index: 2}},
		{"Vol 3 - Book Title", "Author", false, bookName{Title: "Book Title", Author: "Author", Index: 3}},
		{"Vol. 1 - 2015 - Book Title", "Author", false, bookName{Title: "Book Title", Author: "Author", Index: 1, Year: 2015}},
		{"1. Book Title", "Author", false, bookName{Title: "Book Title", Author: "Author", Index: 1}},
		{"1984", "George Orwell", false, bookName{Title: "1984", Author: "George Orwell"}},
		{"1984 - George Orwell", "George Orwell", false, bookName{Title: "1984", Author: "George Orwell"}},
		{"George Orwell - 1984", "", false, bookName{Title: "1984", Author: "George Orwell"}},
		{"Catch 22 - Joseph Heller", "Joseph Heller", false, bookName{Title: "Catch 22", Author: "Joseph Heller"}},
		{"Fahrenheit 451", "Ray Bradbury", false, bookName{Title: "Fahrenheit 451", Author: "Ray Bradbury"}},
		{"The Hobbit - Tolkien, J.R.R.", "J.R.R. Tolkien", false, bookName{Title: "The Hobbit", Author: "J.R.R. Tolkien"}},
		{"Title_ A Subtitle - Author", "Author", false, bookName{Title: "Title: A Subtitle", Author: "Author"}},
		{"Book Title {Narrator Name} [B00ABCDEF1] (2015)", "Author", false, bookName{Title: "Book Title", Author: "Author", Narrator: "Narrator Name", ASIN: "B00ABCDEF1", Year: 2015}},
		{"Author Name - Book Title {Jim Dale}", "", false, bookName{Title: "Book Title", Author: "Author Name", Narrator: "Jim Dale"}},
		{"Book Title (Unabridged)", "Author", false, bookName{Title: "Book Title", Author: "Author"}},
		{"Series #3 - Title - Author", "", false, bookName{Title: "Title", Author: "Author", Series: "Series", Index: 3}},
		{"Book Title [9780306406157]", "Author", false, bookName{Title: "Book Title", Author: "Author", ISBN: "9780306406157"}},
		{"Book 1.5 - Novella", "Author", false, bookName{Title: "Novella", Author: "Author", Index: 1.5}},
	}
	for _, c := range cases {
		if got := parseBookName(c.in, c.author, c.calibre); got != c.want {
			t.Errorf("parseBookName(%q,%q) = %+v, want %+v", c.in, c.author, got, c.want)
		}
	}
}

func TestParseTrack(t *testing.T) {
	cases := []struct {
		in          string
		disc, track int
	}{
		{"01 - Chapter One", 0, 1},
		{"Track 05", 0, 5},
		{"Book Title - Part 03", 0, 3},
		{"Book Title 12", 0, 12},
		{"Book Title (Unabridged) 07 [64k]", 0, 7},
		{"CD2 - 04", 2, 4},
		{"1-07 Title", 1, 7},
		{"Disc 3 Track 2", 3, 2},
		{"Chapter 10 - The End", 0, 10},
		{"Intro", 0, 0},
	}
	for _, c := range cases {
		d, tr := parseTrack(c.in)
		if d != c.disc || tr != c.track {
			t.Errorf("parseTrack(%q) = %d,%d want %d,%d", c.in, d, tr, c.disc, c.track)
		}
	}
}

func TestNFO(t *testing.T) {
	xmlNFO := `<?xml version="1.0" encoding="UTF-8" standalone="yes" ?>
<movie>
  <title>The Matrix</title>
  <year>1999</year>
  <plot>A hacker learns the truth.</plot>
  <genre>Action</genre>
  <genre>Science Fiction / Thriller</genre>
  <uniqueid type="imdb" default="true">tt0133093</uniqueid>
  <uniqueid type="tmdb">603</uniqueid>
</movie>
https://www.imdb.com/title/tt0133093/`
	m := parseNFO([]byte(xmlNFO))
	if m.Title != "The Matrix" || m.Year != 1999 || m.Overview != "A hacker learns the truth." ||
		!reflect.DeepEqual(m.Genres, []string{"Action", "Science Fiction", "Thriller"}) ||
		!reflect.DeepEqual(m.IDs, map[string]string{"imdb": "tt0133093", "tmdb": "603"}) {
		t.Errorf("xml nfo = %+v", m)
	}
	m = parseNFO([]byte("https://www.themoviedb.org/movie/603-the-matrix\n"))
	if !reflect.DeepEqual(m.IDs, map[string]string{"tmdb": "603"}) || m.Title != "" {
		t.Errorf("url nfo = %+v", m)
	}
	m = parseNFO([]byte("<tvshow><title>Show</title><premiered>2010-05-01</premiered><id>12345</id></tvshow>"))
	if m.Year != 2010 || m.IDs["tvdb"] != "12345" {
		t.Errorf("tvshow nfo = %+v", m)
	}
	m = parseNFO([]byte("Scene release NFO\n\nIMDB: http://www.imdb.com/title/tt1234567/\nRuntime: 120 min\x00\xff"))
	if m.IDs["imdb"] != "tt1234567" {
		t.Errorf("scene nfo = %+v", m)
	}
}

func TestOPF(t *testing.T) {
	opf := `<?xml version='1.0' encoding='utf-8'?>
<package xmlns="http://www.idpf.org/2007/opf" unique-identifier="uuid_id" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf" xmlns:calibre="http://calibre.kovidgoyal.net/2009/metadata">
    <dc:identifier opf:scheme="calibre" id="calibre_id">42</dc:identifier>
    <dc:identifier opf:scheme="ISBN">978-0-306-40615-7</dc:identifier>
    <dc:identifier opf:scheme="MOBI-ASIN">B000FC0SIM</dc:identifier>
    <dc:title>The Way of Kings</dc:title>
    <dc:creator opf:file-as="Sanderson, Brandon" opf:role="aut">Brandon Sanderson</dc:creator>
    <dc:creator opf:role="nrt">Michael Kramer</dc:creator>
    <dc:date>2010-08-31T04:00:00+00:00</dc:date>
    <dc:description>&lt;p&gt;Epic &amp;amp; long.&lt;/p&gt;&lt;p&gt;Second para.&lt;/p&gt;</dc:description>
    <dc:subject>Fantasy</dc:subject>
    <dc:subject>Epic</dc:subject>
    <meta name="calibre:series" content="The Stormlight Archive"/>
    <meta name="calibre:series_index" content="1.0"/>
  </metadata>
</package>`
	m, ok := parseOPF([]byte(opf))
	want := localMeta{
		Title: "The Way of Kings", Year: 2010, Overview: "Epic & long.\nSecond para.",
		Genres: []string{"Fantasy", "Epic"}, IDs: map[string]string{"isbn": "9780306406157", "asin": "B000FC0SIM"},
		Authors: []string{"Brandon Sanderson"}, Narrators: []string{"Michael Kramer"},
		Series: "The Stormlight Archive", SeriesIndex: 1,
	}
	if !ok || !reflect.DeepEqual(m, want) {
		t.Errorf("opf =\n %+v\nwant\n %+v", m, want)
	}
}

func TestABSMetadata(t *testing.T) {
	js := `{"tags":[],"chapters":[],"title":"Project Hail Mary","subtitle":null,"authors":["Andy Weir"],"narrators":["Ray Porter"],"series":["Standalone #1"],"genres":["Science Fiction"],"publishedYear":"2021","publishedDate":null,"publisher":null,"description":"<p>A lone astronaut.</p>","isbn":null,"asin":"B08G9PRS1K","language":null,"explicit":false}`
	m, ok := parseABSMetadata([]byte(js))
	want := localMeta{Title: "Project Hail Mary", Year: 2021, Overview: "A lone astronaut.", Genres: []string{"Science Fiction"},
		IDs: map[string]string{"asin": "B08G9PRS1K"}, Authors: []string{"Andy Weir"}, Narrators: []string{"Ray Porter"}, Series: "Standalone", SeriesIndex: 1}
	if !ok || !reflect.DeepEqual(m, want) {
		t.Errorf("abs = %+v", m)
	}
	m, _ = parseABSMetadata([]byte(`{"title":"X","authors":[{"name":"A"}],"series":[{"name":"S","sequence":"2.5"}],"publishedYear":1999}`))
	if m.Series != "S" || m.SeriesIndex != 2.5 || m.Year != 1999 || m.Authors[0] != "A" {
		t.Errorf("abs objects = %+v", m)
	}
}
