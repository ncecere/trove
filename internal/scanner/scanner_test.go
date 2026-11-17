package scanner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"trove/internal/media"
)

// mkTree creates files under root. Entries are "path" (empty file) or
// "path=content".
func mkTree(t *testing.T, root string, entries ...string) {
	t.Helper()
	for _, e := range entries {
		p, content, _ := strings.Cut(e, "=")
		full := filepath.Join(root, filepath.FromSlash(p))
		if strings.HasSuffix(p, "/") {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// dump renders scan results as stable text so tests can assert on every
// field of every item and file.
func dump(items []media.ScannedItem) string {
	var b strings.Builder
	for _, it := range items {
		fmt.Fprintf(&b, "%s %q", it.Kind, it.Title)
		if it.Year != 0 {
			fmt.Fprintf(&b, " (%d)", it.Year)
		}
		fmt.Fprintf(&b, " rel=%q sort=%q", it.RelPath, it.SortTitle)
		if it.Author != "" {
			fmt.Fprintf(&b, " author=%q", it.Author)
		}
		if it.Narrator != "" {
			fmt.Fprintf(&b, " narrator=%q", it.Narrator)
		}
		if it.Series != "" || it.SeriesIndex != 0 {
			fmt.Fprintf(&b, " series=%q #%g", it.Series, it.SeriesIndex)
		}
		if len(it.IDs) > 0 {
			var ks []string
			for k, v := range it.IDs {
				ks = append(ks, k+":"+v)
			}
			sort.Strings(ks)
			fmt.Fprintf(&b, " ids=%s", strings.Join(ks, ","))
		}
		if it.Overview != "" {
			fmt.Fprintf(&b, " overview=%q", it.Overview)
		}
		if len(it.Genres) > 0 {
			fmt.Fprintf(&b, " genres=%s", strings.Join(it.Genres, "|"))
		}
		if it.PosterRelPath != "" {
			fmt.Fprintf(&b, " poster=%q", it.PosterRelPath)
		}
		if it.BackdropRelPath != "" {
			fmt.Fprintf(&b, " backdrop=%q", it.BackdropRelPath)
		}
		b.WriteString("\n")
		for _, f := range it.Files {
			fmt.Fprintf(&b, "  %s %q", f.Role, f.RelPath)
			if f.Episode != 0 || f.Season != 0 {
				fmt.Fprintf(&b, " S%dE%d", f.Season, f.Episode)
				if f.EpisodeEnd != 0 {
					fmt.Fprintf(&b, "-%d", f.EpisodeEnd)
				}
			}
			if f.EpisodeTitle != "" {
				fmt.Fprintf(&b, " %q", f.EpisodeTitle)
			}
			if f.Track != 0 {
				fmt.Fprintf(&b, " track=%d", f.Track)
			}
			for _, kv := range [][2]string{{"res", f.Resolution}, {"src", f.Source}, {"vc", f.VideoCodec}, {"ac", f.AudioCodec}, {"hdr", f.HDR}, {"ed", f.Edition}, {"lang", f.Language}} {
				if kv[1] != "" {
					fmt.Fprintf(&b, " %s=%s", kv[0], kv[1])
				}
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func scanText(t *testing.T, root string, lt media.LibraryType) string {
	t.Helper()
	items, err := New().Scan(context.Background(), root, lt)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		for _, f := range it.Files {
			if f.Container == "" || strings.Contains(f.RelPath, "\\") || strings.HasPrefix(f.RelPath, "/") {
				t.Errorf("bad file %+v", f)
			}
		}
	}
	return dump(items)
}

func assertDump(t *testing.T, got, want string) {
	t.Helper()
	want = strings.TrimLeft(want, "\n")
	if got == want {
		return
	}
	gl, wl := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(gl) || i < len(wl); i++ {
		var g, w string
		if i < len(gl) {
			g = gl[i]
		}
		if i < len(wl) {
			w = wl[i]
		}
		if g != w {
			t.Errorf("first difference at line %d:\n got: %s\nwant: %s", i+1, g, w)
			break
		}
	}
	t.Errorf("full result:\n%s", got)
}

const movieNFO = `<?xml version="1.0" encoding="UTF-8"?>
<movie><title>Movie Title</title><year>2019</year><plot>A plot.</plot><genre>Drama</genre><uniqueid type="tmdb">111</uniqueid></movie>`

func TestScanMovies(t *testing.T) {
	root := t.TempDir()
	mkTree(t, root,
		// Radarr-style folder with sidecars, extras, sample, nfo and art.
		"Movie Title (2019)/Movie.Title.2019.1080p.BluRay.x265-GRP.mkv",
		"Movie Title (2019)/Movie.Title.2019.1080p.BluRay.x265-GRP.en.srt",
		"Movie Title (2019)/Movie.Title.2019.1080p.BluRay.x265-GRP.eng.forced.srt",
		"Movie Title (2019)/Movie.Title.2019.1080p.BluRay.x265-GRP.sample.mkv",
		"Movie Title (2019)/Movie-trailer.mp4",
		"Movie Title (2019)/Subs/2_English.srt",
		"Movie Title (2019)/Featurettes/Making Of.mkv",
		"Movie Title (2019)/Behind The Scenes/BTS.mkv",
		"Movie Title (2019)/Sample/sample.mkv",
		"Movie Title (2019)/poster.jpg",
		"Movie Title (2019)/fanart.jpg",
		"Movie Title (2019)/movie.nfo="+movieNFO,
		// multiple versions + edition tags + id tag in folder
		"Blade Runner 2049 (2017) {tmdb-335984}/Blade Runner 2049 (2017) - 2160p.mkv",
		"Blade Runner 2049 (2017) {tmdb-335984}/Blade Runner 2049 (2017) {edition-Director's Cut} - 1080p.mkv",
		"2001 A Space Odyssey (1968) [imdbid-tt0062622]/2001.A.Space.Odyssey.1968.REMASTERED.1080p.BluRay.x264.mkv",
		"2001 A Space Odyssey (1968) [imdbid-tt0062622]/folder.jpg",
		"1917 (2019)/1917.2019.2160p.UHD.BluRay.REMUX.HDR.HEVC.TrueHD.7.1.Atmos-GRP.mkv",
		// multi-part (file markers and folders)
		"Se7en (1995)/Se7en cd1.avi",
		"Se7en (1995)/Se7en cd2.avi",
		"Kill Bill Vol 1 (2003)/CD1/kb.avi",
		"Kill Bill Vol 1 (2003)/CD2/kb.avi",
		// collection folder
		"Alien Collection/poster.jpg",
		"Alien Collection/Alien (1979)/Alien.1979.mkv",
		"Alien Collection/Aliens (1986)/Aliens.1986.Extended.mkv",
		// scene folder and obfuscated file name
		"Movie.Scene.2018.720p.WEB-DL.DD5.1.H264-GRP/movie.scene.2018.720p.web-dl.mkv",
		"Obfuscated.Movie.2015.1080p.BluRay.x264-GRP/abc123.mkv",
		// release folder nested inside a clean folder
		"Inception (2010)/Inception.2010.1080p.BluRay.x264-GRP/Inception.2010.1080p.BluRay.x264-GRP.mkv",
		"Inception (2010)/Inception.2010.1080p.BluRay.x264-GRP/Inception.2010.1080p.BluRay.x264-GRP.nfo=https://www.imdb.com/title/tt1375666/",
		// disc structure
		"Disc Movie (2005)/BDMV/STREAM/00001.m2ts",
		"Disc Movie (2005)/BDMV/index.bdmv",
		// loose root files
		"Loose Movie (2010).mkv",
		"Loose Movie (2010)-poster.jpg",
		"Loose Movie (2010).en.srt",
		"Loose Movie (2010).nfo=https://www.themoviedb.org/movie/4242-loose",
		"The.Loose.Scene.2012.1080p.WEB.H264-GRP.mkv",
		// junk that must be ignored
		"Empty Folder/",
		".hidden/Hidden (2000).mkv",
		"@eaDir/Thumb (2000).mkv",
		"#recycle/Deleted (2000).mkv",
		"$RECYCLE.BIN/Deleted (2001).mkv",
		"lost+found/x.mkv",
		"Movie Title (2019)/.DS_Store",
		"Movie Title (2019)/Thumbs.db",
		"Movie Title (2019)/@eaDir/Movie.Title.2019.mkv",
		"Downloading (2020)/Downloading.2020.mkv.part",
	)
	got := scanText(t, root, media.LibraryMovies)
	assertDump(t, got, `
movie "1917" (2019) rel="1917 (2019)" sort="1917"
  video "1917 (2019)/1917.2019.2160p.UHD.BluRay.REMUX.HDR.HEVC.TrueHD.7.1.Atmos-GRP.mkv" res=2160p src=Remux vc=x265 ac=TrueHD Atmos hdr=HDR
movie "2001 A Space Odyssey" (1968) rel="2001 A Space Odyssey (1968) [imdbid-tt0062622]" sort="2001 a space odyssey" ids=imdb:tt0062622 poster="2001 A Space Odyssey (1968) [imdbid-tt0062622]/folder.jpg"
  video "2001 A Space Odyssey (1968) [imdbid-tt0062622]/2001.A.Space.Odyssey.1968.REMASTERED.1080p.BluRay.x264.mkv" res=1080p src=BluRay vc=x264 ed=Remastered
movie "Alien" (1979) rel="Alien Collection/Alien (1979)" sort="alien"
  video "Alien Collection/Alien (1979)/Alien.1979.mkv"
movie "Aliens" (1986) rel="Alien Collection/Aliens (1986)" sort="aliens"
  video "Alien Collection/Aliens (1986)/Aliens.1986.Extended.mkv" ed=Extended
movie "Blade Runner 2049" (2017) rel="Blade Runner 2049 (2017) {tmdb-335984}" sort="blade runner 2049" ids=tmdb:335984
  video "Blade Runner 2049 (2017) {tmdb-335984}/Blade Runner 2049 (2017) - 2160p.mkv" res=2160p
  video "Blade Runner 2049 (2017) {tmdb-335984}/Blade Runner 2049 (2017) {edition-Director's Cut} - 1080p.mkv" res=1080p ed=Director's Cut
movie "Disc Movie" (2005) rel="Disc Movie (2005)" sort="disc movie"
  video "Disc Movie (2005)/BDMV/STREAM/00001.m2ts"
movie "Inception" (2010) rel="Inception (2010)" sort="inception" ids=imdb:tt1375666
  video "Inception (2010)/Inception.2010.1080p.BluRay.x264-GRP/Inception.2010.1080p.BluRay.x264-GRP.mkv" res=1080p src=BluRay vc=x264
movie "Kill Bill Vol 1" (2003) rel="Kill Bill Vol 1 (2003)" sort="kill bill vol 1"
  video "Kill Bill Vol 1 (2003)/CD1/kb.avi" track=1
  video "Kill Bill Vol 1 (2003)/CD2/kb.avi" track=2
movie "Loose Movie" (2010) rel="Loose Movie (2010).mkv" sort="loose movie" ids=tmdb:4242 poster="Loose Movie (2010)-poster.jpg"
  subtitle "Loose Movie (2010).en.srt" lang=en
  video "Loose Movie (2010).mkv"
movie "The Loose Scene" (2012) rel="The.Loose.Scene.2012.1080p.WEB.H264-GRP.mkv" sort="loose scene"
  video "The.Loose.Scene.2012.1080p.WEB.H264-GRP.mkv" res=1080p src=WEB-DL vc=x264
movie "Movie Scene" (2018) rel="Movie.Scene.2018.720p.WEB-DL.DD5.1.H264-GRP" sort="movie scene"
  video "Movie.Scene.2018.720p.WEB-DL.DD5.1.H264-GRP/movie.scene.2018.720p.web-dl.mkv" res=720p src=WEB-DL
movie "Movie Title" (2019) rel="Movie Title (2019)" sort="movie title" ids=tmdb:111 overview="A plot." genres=Drama poster="Movie Title (2019)/poster.jpg" backdrop="Movie Title (2019)/fanart.jpg"
  extra "Movie Title (2019)/Behind The Scenes/BTS.mkv"
  extra "Movie Title (2019)/Featurettes/Making Of.mkv"
  extra "Movie Title (2019)/Movie-trailer.mp4"
  subtitle "Movie Title (2019)/Movie.Title.2019.1080p.BluRay.x265-GRP.en.srt" lang=en
  subtitle "Movie Title (2019)/Movie.Title.2019.1080p.BluRay.x265-GRP.eng.forced.srt" lang=en
  video "Movie Title (2019)/Movie.Title.2019.1080p.BluRay.x265-GRP.mkv" res=1080p src=BluRay vc=x265
  subtitle "Movie Title (2019)/Subs/2_English.srt" lang=en
movie "Obfuscated Movie" (2015) rel="Obfuscated.Movie.2015.1080p.BluRay.x264-GRP" sort="obfuscated movie"
  video "Obfuscated.Movie.2015.1080p.BluRay.x264-GRP/abc123.mkv"
movie "Se7en" (1995) rel="Se7en (1995)" sort="se7en"
  video "Se7en (1995)/Se7en cd1.avi" track=1
  video "Se7en (1995)/Se7en cd2.avi" track=2
`)
}

const tvshowNFO = `<tvshow><title>Show Name</title><year>2010</year><plot>Show plot.</plot><genre>Comedy</genre><uniqueid type="tvdb">7777</uniqueid></tvshow>`

func TestScanTV(t *testing.T) {
	root := t.TempDir()
	mkTree(t, root,
		"Show Name (2010)/tvshow.nfo="+tvshowNFO,
		"Show Name (2010)/poster.jpg",
		"Show Name (2010)/backdrop.png",
		"Show Name (2010)/Season 01/Show.Name.S01E01.Pilot.720p.HDTV.x264-GRP.mkv",
		"Show Name (2010)/Season 01/Show.Name.S01E01.Pilot.720p.HDTV.x264-GRP.en.srt",
		"Show Name (2010)/Season 01/Show.Name.S01E01.sample.mkv",
		"Show Name (2010)/Season 01/Show.Name.S01E02E03.720p.mkv",
		"Show Name (2010)/Season 01/Show Name - 1x04 - The Fourth.mkv",
		"Show Name (2010)/Season 01/Show Name S01E05.es.srt",
		"Show Name (2010)/Season 01/Show Name - S01E06-07 - Double [WEBDL-1080p].mkv",
		"Show Name (2010)/Season 2/02 - Second Episode.mkv",
		"Show Name (2010)/Season 2/Episode 3.mkv",
		"Show Name (2010)/Season 2/Season 2 Episode 10.mkv",
		"Show Name (2010)/S03/Show.Name.S03.E01.mkv",
		"Show Name (2010)/Specials/Show Name - S00E01 - Christmas Special.mkv",
		"Show Name (2010)/Specials/03 - Another Special.mkv",
		"Show Name (2010)/Season 0/Gag Reel.mkv",
		"Show Name (2010)/Extras/Blooper Reel.mkv",
		"Show Name (2010)/Show.Name.S04E01.Loose.In.Show.Folder.mkv",
		"Show Name (2010)/Show.Name.S01E01-behindthescenes.mkv",
		"The Daily Show/The.Daily.Show.2024.03.15.Guest.Name.720p.WEB.mkv",
		"The Daily Show/Season 2024/The Daily Show - 2024-03-16 - Other Guest.mkv",
		"Other.Show.S02.1080p.WEB-DL-GRP/Other.Show.S02E01.1080p.WEB-DL-GRP.mkv",
		"Other.Show.S02.1080p.WEB-DL-GRP/Other.Show.S02E02.1080p.WEB-DL-GRP.mkv",
		"Root.Show.S01E01.720p.mkv",
		"Root.Show.S01E02.720p.mkv",
		"Root.Show.S01E02.720p.en.srt",
		"Empty Show/",
	)
	got := scanText(t, root, media.LibraryTV)
	assertDump(t, got, `
show "The Daily Show" rel="The Daily Show" sort="daily show"
  video "The Daily Show/Season 2024/The Daily Show - 2024-03-16 - Other Guest.mkv" S2024E316 "Other Guest"
  video "The Daily Show/The.Daily.Show.2024.03.15.Guest.Name.720p.WEB.mkv" S2024E315 "Guest Name" res=720p src=WEB-DL
show "Other Show" rel="Other.Show.S02.1080p.WEB-DL-GRP" sort="other show"
  video "Other.Show.S02.1080p.WEB-DL-GRP/Other.Show.S02E01.1080p.WEB-DL-GRP.mkv" S2E1 res=1080p src=WEB-DL
  video "Other.Show.S02.1080p.WEB-DL-GRP/Other.Show.S02E02.1080p.WEB-DL-GRP.mkv" S2E2 res=1080p src=WEB-DL
show "Root Show" rel="Root.Show.S01E01.720p.mkv" sort="root show"
  video "Root.Show.S01E01.720p.mkv" S1E1 res=720p
  subtitle "Root.Show.S01E02.720p.en.srt" S1E2 lang=en
  video "Root.Show.S01E02.720p.mkv" S1E2 res=720p
show "Show Name" (2010) rel="Show Name (2010)" sort="show name" ids=tvdb:7777 overview="Show plot." genres=Comedy poster="Show Name (2010)/poster.jpg" backdrop="Show Name (2010)/backdrop.png"
  extra "Show Name (2010)/Extras/Blooper Reel.mkv"
  video "Show Name (2010)/S03/Show.Name.S03.E01.mkv" S3E1
  extra "Show Name (2010)/Season 0/Gag Reel.mkv"
  video "Show Name (2010)/Season 01/Show Name - 1x04 - The Fourth.mkv" S1E4 "The Fourth"
  video "Show Name (2010)/Season 01/Show Name - S01E06-07 - Double [WEBDL-1080p].mkv" S1E6-7 "Double" res=1080p src=WEB-DL
  subtitle "Show Name (2010)/Season 01/Show Name S01E05.es.srt" S1E5 lang=es
  subtitle "Show Name (2010)/Season 01/Show.Name.S01E01.Pilot.720p.HDTV.x264-GRP.en.srt" S1E1 lang=en
  video "Show Name (2010)/Season 01/Show.Name.S01E01.Pilot.720p.HDTV.x264-GRP.mkv" S1E1 "Pilot" res=720p src=HDTV vc=x264
  video "Show Name (2010)/Season 01/Show.Name.S01E02E03.720p.mkv" S1E2-3 res=720p
  video "Show Name (2010)/Season 2/02 - Second Episode.mkv" S2E2 "Second Episode"
  video "Show Name (2010)/Season 2/Episode 3.mkv" S2E3
  video "Show Name (2010)/Season 2/Season 2 Episode 10.mkv" S2E10
  extra "Show Name (2010)/Show.Name.S01E01-behindthescenes.mkv"
  video "Show Name (2010)/Show.Name.S04E01.Loose.In.Show.Folder.mkv" S4E1 "Loose In Show Folder"
  video "Show Name (2010)/Specials/03 - Another Special.mkv" S0E3 "Another Special"
  video "Show Name (2010)/Specials/Show Name - S00E01 - Christmas Special.mkv" S0E1 "Christmas Special"
`)
}

func TestScanAnime(t *testing.T) {
	root := t.TempDir()
	mkTree(t, root,
		"Show Name/[SubsPlease] Show Name - 01 (1080p) [ABCD1234].mkv",
		"Show Name/[SubsPlease] Show Name - 01 (1080p) [ABCD1234].ass",
		"Show Name/[SubsPlease] Show Name - 02v2 (1080p) [0123ABCD].mkv",
		"Show Name/[Group] Show Name - NCOP1 [1080p].mkv",
		"Show Name/[Group] Show Name - OVA [1080p].mkv",
		"Show Name/Season 2/[Group] Show Name S2 - 01 [1080p][HEVC].mkv",
		"Show Name/Season 2/[Group] Show Name - 02 [1080p][HEVC].mkv",
		"Show Name/Specials/[Group] Show Name - OVA 01 [1080p].mkv",
		"Show Name/Extras/NCED.mkv",
		"Show Name/poster.jpg",
		"Second Show/[Group] Second Show 2nd Season - 03 [720p].mkv",
		"Second Show/[Group] Second Show - 12.5 [720p].mkv",
		"Third Show/Third Show - S02E05 - Title.mkv",
		"Your Name (2016)/[Group] Kimi no Na wa (2016) [BD 1080p][x265].mkv",
		"Your Name (2016)/Extras/[Group] Kimi no Na wa - PV [1080p].mkv",
		"[Group] Loose Movie (2020) [1080p].mkv",
		"[Group] Loose Show - 05 [1080p].mkv",
		"[Group] Loose Show - 06 [1080p].mkv",
	)
	got := scanText(t, root, media.LibraryAnime)
	assertDump(t, got, `
movie "Loose Movie" (2020) rel="[Group] Loose Movie (2020) [1080p].mkv" sort="loose movie"
  video "[Group] Loose Movie (2020) [1080p].mkv" res=1080p
show "Loose Show" rel="[Group] Loose Show - 05 [1080p].mkv" sort="loose show"
  video "[Group] Loose Show - 05 [1080p].mkv" S1E5 res=1080p
  video "[Group] Loose Show - 06 [1080p].mkv" S1E6 res=1080p
show "Second Show" rel="Second Show" sort="second show"
  extra "Second Show/[Group] Second Show - 12.5 [720p].mkv" res=720p
  video "Second Show/[Group] Second Show 2nd Season - 03 [720p].mkv" S2E3 res=720p
show "Show Name" rel="Show Name" sort="show name" poster="Show Name/poster.jpg"
  extra "Show Name/[Group] Show Name - NCOP1 [1080p].mkv" res=1080p
  extra "Show Name/[Group] Show Name - OVA [1080p].mkv" res=1080p
  subtitle "Show Name/[SubsPlease] Show Name - 01 (1080p) [ABCD1234].ass" S1E1
  video "Show Name/[SubsPlease] Show Name - 01 (1080p) [ABCD1234].mkv" S1E1 res=1080p
  video "Show Name/[SubsPlease] Show Name - 02v2 (1080p) [0123ABCD].mkv" S1E2 res=1080p
  extra "Show Name/Extras/NCED.mkv"
  video "Show Name/Season 2/[Group] Show Name - 02 [1080p][HEVC].mkv" S2E2 res=1080p vc=x265
  video "Show Name/Season 2/[Group] Show Name S2 - 01 [1080p][HEVC].mkv" S2E1 res=1080p vc=x265
  video "Show Name/Specials/[Group] Show Name - OVA 01 [1080p].mkv" S0E1 res=1080p
show "Third Show" rel="Third Show" sort="third show"
  video "Third Show/Third Show - S02E05 - Title.mkv" S2E5 "Title"
movie "Your Name" (2016) rel="Your Name (2016)" sort="your name"
  video "Your Name (2016)/[Group] Kimi no Na wa (2016) [BD 1080p][x265].mkv" res=1080p src=BluRay vc=x265
  extra "Your Name (2016)/Extras/[Group] Kimi no Na wa - PV [1080p].mkv" res=1080p
`)
}

const calibreOPF = `<?xml version='1.0' encoding='utf-8'?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>The Way of Kings</dc:title>
    <dc:creator opf:role="aut">Brandon Sanderson</dc:creator>
    <dc:description>&lt;p&gt;Roshar.&lt;/p&gt;</dc:description>
    <dc:subject>Fantasy</dc:subject>
    <dc:identifier opf:scheme="ISBN">9780765326355</dc:identifier>
    <dc:date>2010-08-31T00:00:00+00:00</dc:date>
    <meta name="calibre:series" content="The Stormlight Archive"/>
    <meta name="calibre:series_index" content="1"/>
  </metadata>
</package>`

func TestScanBooks(t *testing.T) {
	root := t.TempDir()
	mkTree(t, root,
		// Calibre
		"Brandon Sanderson/The Way of Kings (42)/The Way of Kings - Brandon Sanderson.epub",
		"Brandon Sanderson/The Way of Kings (42)/The Way of Kings - Brandon Sanderson.mobi",
		"Brandon Sanderson/The Way of Kings (42)/metadata.opf="+calibreOPF,
		"Brandon Sanderson/The Way of Kings (42)/cover.jpg",
		// series folders
		"Brandon Sanderson/Mistborn/01 - The Final Empire/The Final Empire.epub",
		"Brandon Sanderson/Mistborn/02 - The Well of Ascension/The Well of Ascension.epub",
		"Brandon Sanderson/Mistborn/02 - The Well of Ascension/cover.png",
		// Readarr
		"Andy Weir/The Martian (2011)/The Martian - Andy Weir.epub",
		"Andy Weir/The Martian (2011)/The Martian - Andy Weir.azw3",
		// loose files in an author folder
		"Andy Weir/Artemis.epub",
		"Andy Weir/Artemis.pdf",
		"Andy Weir/Project Hail Mary.epub",
		"Andy Weir/Project Hail Mary.jpg",
		// series markers in file names
		"Frank Herbert/Dune Chronicles #2 - Dune Messiah.epub",
		"Frank Herbert/Children of Dune (Dune Chronicles #3).azw3",
		// single "Author - Title" folder in the root
		"Ursula K. Le Guin - A Wizard of Earthsea/A Wizard of Earthsea.epub",
		// flat files in the root
		"George Orwell - 1984.epub",
		"Neil Gaiman - American Gods.mobi",
		"Neil Gaiman - American Gods.epub",
		// noise
		"Andy Weir/notes.docx",
		"Brandon Sanderson/.calnotes/x.epub",
	)
	got := scanText(t, root, media.LibraryBooks)
	assertDump(t, got, `
book "1984" rel="George Orwell - 1984.epub" sort="1984" author="George Orwell"
  ebook "George Orwell - 1984.epub"
book "American Gods" rel="Neil Gaiman - American Gods.epub" sort="american gods" author="Neil Gaiman"
  ebook "Neil Gaiman - American Gods.epub"
  ebook "Neil Gaiman - American Gods.mobi"
book "Artemis" rel="Andy Weir/Artemis.epub" sort="artemis" author="Andy Weir"
  ebook "Andy Weir/Artemis.epub"
  ebook "Andy Weir/Artemis.pdf"
book "Children of Dune" rel="Frank Herbert/Children of Dune (Dune Chronicles #3).azw3" sort="children of dune" author="Frank Herbert" series="Dune Chronicles" #3
  ebook "Frank Herbert/Children of Dune (Dune Chronicles #3).azw3"
book "Dune Messiah" rel="Frank Herbert/Dune Chronicles #2 - Dune Messiah.epub" sort="dune messiah" author="Frank Herbert" series="Dune Chronicles" #2
  ebook "Frank Herbert/Dune Chronicles #2 - Dune Messiah.epub"
book "The Final Empire" rel="Brandon Sanderson/Mistborn/01 - The Final Empire" sort="final empire" author="Brandon Sanderson" series="Mistborn" #1
  ebook "Brandon Sanderson/Mistborn/01 - The Final Empire/The Final Empire.epub"
book "The Martian" (2011) rel="Andy Weir/The Martian (2011)" sort="martian" author="Andy Weir"
  ebook "Andy Weir/The Martian (2011)/The Martian - Andy Weir.azw3"
  ebook "Andy Weir/The Martian (2011)/The Martian - Andy Weir.epub"
book "Project Hail Mary" rel="Andy Weir/Project Hail Mary.epub" sort="project hail mary" author="Andy Weir" poster="Andy Weir/Project Hail Mary.jpg"
  ebook "Andy Weir/Project Hail Mary.epub"
book "The Way of Kings" (2010) rel="Brandon Sanderson/The Way of Kings (42)" sort="way of kings" author="Brandon Sanderson" series="The Stormlight Archive" #1 ids=isbn:9780765326355 overview="Roshar." genres=Fantasy poster="Brandon Sanderson/The Way of Kings (42)/cover.jpg"
  ebook "Brandon Sanderson/The Way of Kings (42)/The Way of Kings - Brandon Sanderson.epub"
  ebook "Brandon Sanderson/The Way of Kings (42)/The Way of Kings - Brandon Sanderson.mobi"
book "The Well of Ascension" rel="Brandon Sanderson/Mistborn/02 - The Well of Ascension" sort="well of ascension" author="Brandon Sanderson" series="Mistborn" #2 poster="Brandon Sanderson/Mistborn/02 - The Well of Ascension/cover.png"
  ebook "Brandon Sanderson/Mistborn/02 - The Well of Ascension/The Well of Ascension.epub"
book "A Wizard of Earthsea" rel="Ursula K. Le Guin - A Wizard of Earthsea" sort="wizard of earthsea" author="Ursula K. Le Guin"
  ebook "Ursula K. Le Guin - A Wizard of Earthsea/A Wizard of Earthsea.epub"
`)
}

const absJSON = `{"title":"Project Hail Mary","authors":["Andy Weir"],"narrators":["Ray Porter"],"series":[],"genres":["Science Fiction"],"publishedYear":"2021","description":"Ryland Grace.","isbn":null,"asin":"B08G9PRS1K"}`

func TestScanAudiobooks(t *testing.T) {
	root := t.TempDir()
	mkTree(t, root,
		"Andy Weir/Project Hail Mary/01 - Chapter 1.mp3",
		"Andy Weir/Project Hail Mary/02 - Chapter 2.mp3",
		"Andy Weir/Project Hail Mary/10 - Chapter 10.mp3",
		"Andy Weir/Project Hail Mary/cover.jpg",
		"Andy Weir/Project Hail Mary/metadata.json="+absJSON,
		"Andy Weir/Project Hail Mary/booklet.pdf",
		"Andy Weir/Project Hail Mary/info.nfo",
		"Andy Weir/Project Hail Mary/book.cue",
		"Brandon Sanderson/The Stormlight Archive/Book 1 - The Way of Kings (2010) {Michael Kramer, Kate Reading}/Disc 1/01.mp3",
		"Brandon Sanderson/The Stormlight Archive/Book 1 - The Way of Kings (2010) {Michael Kramer, Kate Reading}/Disc 1/02.mp3",
		"Brandon Sanderson/The Stormlight Archive/Book 1 - The Way of Kings (2010) {Michael Kramer, Kate Reading}/Disc 2/01.mp3",
		"Brandon Sanderson/The Stormlight Archive/Book 1 - The Way of Kings (2010) {Michael Kramer, Kate Reading}/folder.jpg",
		"Terry Pratchett - Guards! Guards!/Guards Guards.m4b",
		"Stephen King/The Shining.m4b",
		"Stephen King/It.m4b",
		"Neil Gaiman/Good Omens [B07ABCDE12]/Good Omens Part 1.m4b",
		"Neil Gaiman/Good Omens [B07ABCDE12]/Good Omens Part 2.m4b",
		"Dennis E. Taylor/Bobiverse/Book 1 - We Are Legion.m4b",
		"Dennis E. Taylor/Bobiverse/Book 2 - For We Are Many.m4b",
		"Mary Roach - Stiff.m4b",
		"Frank Herbert/Dune/Dune Part 01.mp3",
		"Frank Herbert/Dune/Dune Part 02.mp3",
		"Frank Herbert/Dune/Dune Part 11.mp3",
	)
	got := scanText(t, root, media.LibraryAudiobooks)
	assertDump(t, got, `
audiobook "Dune" rel="Frank Herbert/Dune" sort="dune" author="Frank Herbert"
  audio "Frank Herbert/Dune/Dune Part 01.mp3" track=1
  audio "Frank Herbert/Dune/Dune Part 02.mp3" track=2
  audio "Frank Herbert/Dune/Dune Part 11.mp3" track=3
audiobook "For We Are Many" rel="Dennis E. Taylor/Bobiverse/Book 2 - For We Are Many.m4b" sort="for we are many" author="Dennis E. Taylor" series="Bobiverse" #2
  audio "Dennis E. Taylor/Bobiverse/Book 2 - For We Are Many.m4b" track=1
audiobook "Good Omens" rel="Neil Gaiman/Good Omens [B07ABCDE12]" sort="good omens" author="Neil Gaiman" ids=asin:B07ABCDE12
  audio "Neil Gaiman/Good Omens [B07ABCDE12]/Good Omens Part 1.m4b" track=1
  audio "Neil Gaiman/Good Omens [B07ABCDE12]/Good Omens Part 2.m4b" track=2
audiobook "Guards! Guards!" rel="Terry Pratchett - Guards! Guards!" sort="guards! guards!" author="Terry Pratchett"
  audio "Terry Pratchett - Guards! Guards!/Guards Guards.m4b" track=1
audiobook "It" rel="Stephen King/It.m4b" sort="it" author="Stephen King"
  audio "Stephen King/It.m4b" track=1
audiobook "Project Hail Mary" (2021) rel="Andy Weir/Project Hail Mary" sort="project hail mary" author="Andy Weir" narrator="Ray Porter" ids=asin:B08G9PRS1K overview="Ryland Grace." genres=Science Fiction poster="Andy Weir/Project Hail Mary/cover.jpg"
  audio "Andy Weir/Project Hail Mary/01 - Chapter 1.mp3" track=1
  audio "Andy Weir/Project Hail Mary/02 - Chapter 2.mp3" track=2
  audio "Andy Weir/Project Hail Mary/10 - Chapter 10.mp3" track=3
  extra "Andy Weir/Project Hail Mary/booklet.pdf"
audiobook "The Shining" rel="Stephen King/The Shining.m4b" sort="shining" author="Stephen King"
  audio "Stephen King/The Shining.m4b" track=1
audiobook "Stiff" rel="Mary Roach - Stiff.m4b" sort="stiff" author="Mary Roach"
  audio "Mary Roach - Stiff.m4b" track=1
audiobook "The Way of Kings" (2010) rel="Brandon Sanderson/The Stormlight Archive/Book 1 - The Way of Kings (2010) {Michael Kramer, Kate Reading}" sort="way of kings" author="Brandon Sanderson" narrator="Michael Kramer, Kate Reading" series="The Stormlight Archive" #1 poster="Brandon Sanderson/The Stormlight Archive/Book 1 - The Way of Kings (2010) {Michael Kramer, Kate Reading}/folder.jpg"
  audio "Brandon Sanderson/The Stormlight Archive/Book 1 - The Way of Kings (2010) {Michael Kramer, Kate Reading}/Disc 1/01.mp3" track=1
  audio "Brandon Sanderson/The Stormlight Archive/Book 1 - The Way of Kings (2010) {Michael Kramer, Kate Reading}/Disc 1/02.mp3" track=2
  audio "Brandon Sanderson/The Stormlight Archive/Book 1 - The Way of Kings (2010) {Michael Kramer, Kate Reading}/Disc 2/01.mp3" track=3
audiobook "We Are Legion" rel="Dennis E. Taylor/Bobiverse/Book 1 - We Are Legion.m4b" sort="we are legion" author="Dennis E. Taylor" series="Bobiverse" #1
  audio "Dennis E. Taylor/Bobiverse/Book 1 - We Are Legion.m4b" track=1
`)
}

func TestScanErrorsAndRobustness(t *testing.T) {
	s := New()
	if _, err := s.Scan(context.Background(), t.TempDir(), "music"); err == nil {
		t.Error("expected error for unknown library type")
	}
	if _, err := s.Scan(context.Background(), filepath.Join(t.TempDir(), "missing"), media.LibraryMovies); err == nil {
		t.Error("expected error for missing root")
	}
	root := t.TempDir()
	mkTree(t, root, "A (2001)/A.2001.mkv")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Scan(ctx, root, media.LibraryMovies); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled scan err = %v", err)
	}
	items, err := s.Scan(context.Background(), t.TempDir(), media.LibraryMovies)
	if err != nil || len(items) != 0 {
		t.Errorf("empty root = %v, %v", items, err)
	}
}

func TestScanSymlinkLoopAndUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks")
	}
	root := t.TempDir()
	mkTree(t, root, "A (2001)/A.2001.mkv", "B (2002)/B.2002.mkv", "Locked (2003)/Locked.2003.mkv")
	if err := os.Symlink(root, filepath.Join(root, "A (2001)", "loop")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "B (2002)"), filepath.Join(root, "B link")); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(root, "Locked (2003)")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755)
	var errs []string
	items, err := New(WithErrorHandler(func(p string, err error) { errs = append(errs, p) })).Scan(context.Background(), root, media.LibraryMovies)
	if err != nil {
		t.Fatal(err)
	}
	var rels []string
	for _, it := range items {
		rels = append(rels, it.RelPath)
	}
	want := []string{"A (2001)", "B (2002)"}
	if os.Geteuid() == 0 { // root can read the locked folder
		want = append(want, "Locked (2003)")
	} else if len(errs) != 1 {
		t.Errorf("unreadable dir not reported: %v", errs)
	}
	if strings.Join(rels, ",") != strings.Join(want, ",") {
		t.Errorf("items = %v, want %v", rels, want)
	}
}

func TestScanFileMetadata(t *testing.T) {
	root := t.TempDir()
	mkTree(t, root, "M (2001)/M.2001.mkv=12345")
	items, err := New().Scan(context.Background(), root, media.LibraryMovies)
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	f := items[0].Files[0]
	if f.Size != 5 || f.Container != "mkv" || f.ModTime.IsZero() || f.RelPath != "M (2001)/M.2001.mkv" {
		t.Errorf("file = %+v", f)
	}
}

func TestScanMoviesMoreLayouts(t *testing.T) {
	root := t.TempDir()
	mkTree(t, root,
		// collection folder with loose movie files
		"Marvel/Iron Man (2008).mkv",
		"Marvel/Iron Man (2008)-poster.jpg",
		"Marvel/Thor (2011) - 1080p.mkv",
		"Marvel/Thor (2011) - 2160p.mkv",
		// year-less folder: year from the release file
		"Alien/Alien.1979.1080p.BluRay.x264.mkv",
		// versions + extras in one folder, edition from scene tokens
		"Aliens (1986)/Aliens.1986.Theatrical.1080p.mkv",
		"Aliens (1986)/Aliens.1986.Special.Edition.2160p.mkv",
		"Aliens (1986)/Trailers/Aliens Trailer.mkv",
		"Aliens (1986)/aliens.1986.1080p-sample.mkv",
		"Aliens (1986)/Aliens-poster.jpg",
	)
	got := scanText(t, root, media.LibraryMovies)
	assertDump(t, got, `
movie "Alien" (1979) rel="Alien" sort="alien"
  video "Alien/Alien.1979.1080p.BluRay.x264.mkv" res=1080p src=BluRay vc=x264
movie "Aliens" (1986) rel="Aliens (1986)" sort="aliens"
  video "Aliens (1986)/Aliens.1986.Special.Edition.2160p.mkv" res=2160p ed=Special Edition
  video "Aliens (1986)/Aliens.1986.Theatrical.1080p.mkv" res=1080p ed=Theatrical
  extra "Aliens (1986)/Trailers/Aliens Trailer.mkv"
movie "Iron Man" (2008) rel="Marvel/Iron Man (2008).mkv" sort="iron man" poster="Marvel/Iron Man (2008)-poster.jpg"
  video "Marvel/Iron Man (2008).mkv"
movie "Thor" (2011) rel="Marvel/Thor (2011) - 1080p.mkv" sort="thor"
  video "Marvel/Thor (2011) - 1080p.mkv" res=1080p
  video "Marvel/Thor (2011) - 2160p.mkv" res=2160p
`)
}

func TestScanBooksMoreLayouts(t *testing.T) {
	root := t.TempDir()
	mkTree(t, root,
		// Readarr folder whose year looks like a Calibre id, no OPF: a year
		"George Orwell/Nineteen Eighty-Four (1949)/Nineteen Eighty-Four - George Orwell.epub",
		// formats split into sub-folders
		"Terry Pratchett/Mort/epub/Mort.epub",
		"Terry Pratchett/Mort/pdf/Mort.pdf",
		// series folder with loose numbered files
		"Robin Hobb/Farseer Trilogy/01 - Assassin's Apprentice.epub",
		"Robin Hobb/Farseer Trilogy/02 - Royal Assassin.epub",
		// single-book folder at the root named after the book
		"Dracula/Dracula - Bram Stoker.epub",
		"Dracula/cover.jpg",
	)
	got := scanText(t, root, media.LibraryBooks)
	assertDump(t, got, `
book "Assassin's Apprentice" rel="Robin Hobb/Farseer Trilogy/01 - Assassin's Apprentice.epub" sort="assassin's apprentice" author="Robin Hobb" series="Farseer Trilogy" #1
  ebook "Robin Hobb/Farseer Trilogy/01 - Assassin's Apprentice.epub"
book "Dracula" rel="Dracula" sort="dracula" author="Bram Stoker" poster="Dracula/cover.jpg"
  ebook "Dracula/Dracula - Bram Stoker.epub"
book "Mort" rel="Terry Pratchett/Mort" sort="mort" author="Terry Pratchett"
  ebook "Terry Pratchett/Mort/epub/Mort.epub"
  ebook "Terry Pratchett/Mort/pdf/Mort.pdf"
book "Nineteen Eighty-Four" (1949) rel="George Orwell/Nineteen Eighty-Four (1949)" sort="nineteen eighty-four" author="George Orwell"
  ebook "George Orwell/Nineteen Eighty-Four (1949)/Nineteen Eighty-Four - George Orwell.epub"
book "Royal Assassin" rel="Robin Hobb/Farseer Trilogy/02 - Royal Assassin.epub" sort="royal assassin" author="Robin Hobb" series="Farseer Trilogy" #2
  ebook "Robin Hobb/Farseer Trilogy/02 - Royal Assassin.epub"
`)
}
