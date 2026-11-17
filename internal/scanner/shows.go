package scanner

import (
	"strings"

	"trove/internal/media"
)

func (b *builder) scanShows(root *dir, anime bool) {
	b.looseEpisodes(root, anime)
	for _, d := range root.dirs {
		if b.ctx.Err() != nil {
			return
		}
		if anime && isAnimeMovieDir(d) {
			b.buildMovie(d)
			continue
		}
		b.buildShow(d, anime)
	}
}

// isAnimeMovieDir: a top-level anime folder whose feature video(s) carry no
// episode marker (and that has no season folders) is a movie.
func isAnimeMovieDir(d *dir) bool {
	var mains []*file
	seasons := false
	var walk func(cur *dir, top bool)
	walk = func(cur *dir, top bool) {
		for _, f := range cur.files {
			if mainVideo(f) {
				mains = append(mains, f)
			}
		}
		for _, sd := range cur.dirs {
			if _, ok := ParseSeasonFolder(sd.name); ok {
				seasons = true
			}
			if isExtrasDir(sd.name, true) || isSubsDir(sd.name) {
				continue
			}
			walk(sd, false)
		}
	}
	walk(d, true)
	if seasons || len(mains) == 0 {
		return false
	}
	var features []*file
	for _, f := range mains {
		ep := ParseEpisode(f.stem, EpisodeOptions{Anime: true})
		if ep.Episode > 0 {
			return false
		}
		if !ep.Extra {
			features = append(features, f)
		}
	}
	if len(features) == 0 {
		return false
	}
	k := titleKey(ParseRelease(features[0].stem).Title)
	for _, f := range features[1:] {
		if titleKey(ParseRelease(f.stem).Title) != k {
			return false
		}
	}
	return true
}

// parseShowFolder parses a top-level show folder name. A trailing season
// marker ("Show.Name.S01.1080p", "Show 2nd Season") is removed from the title
// and returned as the default season for files directly inside.
func parseShowFolder(name string) (Release, int, bool) {
	r := ParseRelease(name)
	if m := reShowSeason.FindStringSubmatchIndex(r.Title); m != nil && hasAlnum(r.Title[:m[0]]) {
		season := 0
		for g := 2; g <= 6; g += 2 {
			if m[g] >= 0 {
				season = atoi(r.Title[m[g]:m[g+1]])
			}
		}
		r.Title = cleanTitle(r.Title[:m[0]], false)
		return r, season, true
	}
	return r, 0, false
}

type showCtx struct {
	season int
	known  bool
	extra  bool
}

type epVideo struct {
	dir  *dir
	stem string
	idx  int // index into item.Files
}

type subRef struct {
	f   *file
	dir *dir
	c   showCtx
}

func (b *builder) buildShow(d *dir, anime bool) {
	fr, season, hasSeason := parseShowFolder(d.name)
	it := media.ScannedItem{Kind: media.KindShow, RelPath: d.rel, Title: fr.Title, Year: fr.Year}
	it.IDs = mergeIDs(nil, fr.IDs)
	var vids []epVideo
	var subs []subRef
	var fallbackShow string
	var fallbackYear int

	var collect func(cur *dir, c showCtx)
	collect = func(cur *dir, c showCtx) {
		for _, f := range cur.files {
			switch {
			case isVideo(f):
				if isSampleName(f.stem) {
					continue
				}
				sf := newFile(f, media.RoleVideo)
				if c.extra || isExtraName(f.stem) {
					sf.Role = media.RoleExtra
					applyRelease(&sf, ParseRelease(f.stem))
				} else {
					ep := ParseEpisode(f.stem, EpisodeOptions{Anime: anime, SeasonFolder: c.known, Season: c.season})
					applyRelease(&sf, ep.Release)
					if ep.Episode > 0 && !ep.Extra {
						sf.Season, sf.Episode, sf.EpisodeEnd, sf.EpisodeTitle = ep.Season, ep.Episode, ep.EpisodeEnd, ep.Title
						if fallbackShow == "" && ep.Show != "" {
							fallbackShow, fallbackYear = ep.Show, ep.Year
						}
					} else {
						sf.Role = media.RoleExtra
					}
				}
				vids = append(vids, epVideo{cur, f.stem, len(it.Files)})
				it.Files = append(it.Files, sf)
			case isSubtitle(f):
				subs = append(subs, subRef{f, cur, c})
			}
		}
		for _, sd := range cur.dirs {
			nc := c
			if s, ok := ParseSeasonFolder(sd.name); ok && !c.extra {
				nc.season, nc.known = s, true
			} else if isExtrasDir(sd.name, false) {
				nc.extra = true
			}
			collect(sd, nc)
		}
	}
	collect(d, showCtx{season: season, known: hasSeason})

	for _, s := range subs {
		sf := newFile(s.f, media.RoleSubtitle)
		var stems []string
		byStem := map[string]int{}
		for _, v := range vids {
			if v.dir == s.dir {
				stems = append(stems, v.stem)
				byStem[v.stem] = v.idx
			}
		}
		st := matchVideoStem(s.f, stems)
		if st != "" {
			v := it.Files[byStem[st]]
			sf.Season, sf.Episode, sf.EpisodeEnd = v.Season, v.Episode, v.EpisodeEnd
		} else if !s.c.extra {
			ep := ParseEpisode(stripLangSuffix(s.f.stem), EpisodeOptions{Anime: anime, SeasonFolder: s.c.known, Season: s.c.season})
			if ep.Episode > 0 && !ep.Extra {
				sf.Season, sf.Episode, sf.EpisodeEnd = ep.Season, ep.Episode, ep.EpisodeEnd
			}
		}
		sf.Language = ParseSubtitleLanguage(s.f.stem, st)
		it.Files = append(it.Files, sf)
	}
	if it.Title == "" {
		it.Title, it.Year = fallbackShow, fallbackYear
	}
	if m, ok := findNFO(d, false, "tvshow.nfo"); ok {
		applyNFO(&it, m)
	}
	it.PosterRelPath, it.BackdropRelPath = findArt(d, true)
	b.add(it)
}

// stripLangSuffix removes trailing ".en.forced" style tokens from a subtitle
// stem so the episode parser sees the video name.
func stripLangSuffix(stem string) string {
	for i := 0; i < 3; i++ {
		j := strings.LastIndexByte(stem, '.')
		if j <= 0 {
			break
		}
		t := strings.ToLower(stem[j+1:])
		if _, ok := langCodes[t]; !ok && !subFlags[t] && !reLangRegion.MatchString(t) {
			break
		}
		stem = stem[:j]
	}
	return stem
}

// looseEpisodes groups video files lying directly in a tv/anime root by show
// title. In anime libraries, loose videos without an episode marker are
// movies.
func (b *builder) looseEpisodes(root *dir, anime bool) {
	type group struct {
		title string
		year  int
		files []media.ScannedFile
		stems []string
	}
	var order []string
	groups := map[string]*group{}
	var movies []*file
	for _, f := range root.files {
		if !isVideo(f) || isSampleName(f.stem) {
			continue
		}
		ep := ParseEpisode(f.stem, EpisodeOptions{Anime: anime})
		if anime && ep.Episode == 0 && !ep.Extra && !isExtraName(f.stem) {
			movies = append(movies, f)
			continue
		}
		sf := newFile(f, media.RoleVideo)
		applyRelease(&sf, ep.Release)
		if ep.Episode > 0 && !ep.Extra && !isExtraName(f.stem) {
			sf.Season, sf.Episode, sf.EpisodeEnd, sf.EpisodeTitle = ep.Season, ep.Episode, ep.EpisodeEnd, ep.Title
		} else {
			sf.Role = media.RoleExtra
		}
		title := ep.Show
		if title == "" {
			title = ParseRelease(f.stem).Title
		}
		key := titleKey(title)
		g := groups[key]
		if g == nil {
			g = &group{title: title, year: ep.Year}
			groups[key] = g
			order = append(order, key)
		}
		g.files = append(g.files, sf)
		g.stems = append(g.stems, f.stem)
	}
	if len(movies) > 0 {
		b.looseMovies(root, movies)
	}
	for _, f := range root.files {
		if !isSubtitle(f) {
			continue
		}
		for _, k := range order {
			g := groups[k]
			if st := matchVideoStem(f, g.stems); st != "" {
				sf := newFile(f, media.RoleSubtitle)
				for i, s := range g.stems {
					if s == st {
						sf.Season, sf.Episode, sf.EpisodeEnd = g.files[i].Season, g.files[i].Episode, g.files[i].EpisodeEnd
					}
				}
				sf.Language = ParseSubtitleLanguage(f.stem, st)
				g.files = append(g.files, sf)
				break
			}
		}
	}
	for _, k := range order {
		g := groups[k]
		first := ""
		for _, sf := range g.files {
			if sf.Role != media.RoleSubtitle && (first == "" || NaturalLess(sf.RelPath, first)) {
				first = sf.RelPath
			}
		}
		b.add(media.ScannedItem{Kind: media.KindShow, RelPath: first, Title: g.title, Year: g.year, Files: g.files})
	}
}
