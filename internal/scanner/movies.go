package scanner

import (
	"strconv"
	"strings"

	"trove/internal/media"
)

type movieCtx int

const (
	ctxMain movieCtx = iota
	ctxExtra
	ctxSubs
)

var discStructDirs = setOf("bdmv", "video_ts", "hvdvd_ts", "certificate")

// mainVideo: a video that can be the feature (not a sample / -trailer).
func mainVideo(f *file) bool {
	return isVideo(f) && !isSampleName(f.stem) && !isExtraName(f.stem)
}

func hasMainVideo(d *dir) bool {
	for _, f := range d.files {
		if mainVideo(f) {
			return true
		}
	}
	return false
}

// isLooseCollection: a folder holding several different movies as loose
// files ("Marvel/Iron Man (2008).mkv", "Marvel/Thor (2011).mkv"). Every file
// must carry a year, the titles must differ, and none may match the folder.
func isLooseCollection(d *dir) bool {
	keys := map[string]bool{}
	folder := titleKey(ParseRelease(d.name).Title)
	for _, f := range d.files {
		if !mainVideo(f) {
			continue
		}
		r := ParseRelease(f.stem)
		k := titleKey(r.Title)
		if r.Year == 0 || k == folder {
			return false
		}
		keys[k+"|"+strconv.Itoa(r.Year)] = true
	}
	return len(keys) >= 2
}

func sameTitle(a, b string) bool {
	ka, kb := titleKey(ParseRelease(a).Title), titleKey(ParseRelease(b).Title)
	return ka != "" && ka == kb
}

// isMovieFolder decides whether d is one movie (as opposed to a collection
// folder holding several movie folders).
func isMovieFolder(d *dir) bool {
	if hasMainVideo(d) {
		return !isLooseCollection(d)
	}
	var videoDirs, plain []*dir
	for _, sd := range d.dirs {
		if !sd.hasFile(isVideo) {
			continue
		}
		if discStructDirs[strings.ToLower(sd.name)] || partDirNumber(sd.name) > 0 {
			return true
		}
		videoDirs = append(videoDirs, sd)
		if !isExtrasDir(sd.name, true) && !isSubsDir(sd.name) {
			plain = append(plain, sd)
		}
	}
	if len(videoDirs) == 0 {
		return false
	}
	if len(plain) == 0 {
		return true // only extras: still one movie
	}
	// "Movie (2019)/Movie.2019.1080p.BluRay-GRP/movie.mkv"
	return len(plain) == 1 && hasMainVideo(plain[0]) && sameTitle(d.name, plain[0].name)
}

func (b *builder) scanMovies(root *dir) {
	var vids []*file
	for _, f := range root.files {
		if isVideo(f) {
			vids = append(vids, f)
		}
	}
	b.looseMovies(root, vids)
	for _, d := range root.dirs {
		b.movieDir(d)
	}
}

func (b *builder) movieDir(d *dir) {
	if b.ctx.Err() != nil {
		return
	}
	if isMovieFolder(d) {
		b.buildMovie(d)
		return
	}
	var vids []*file
	for _, f := range d.files {
		if isVideo(f) {
			vids = append(vids, f)
		}
	}
	b.looseMovies(d, vids)
	for _, sd := range d.dirs {
		b.movieDir(sd)
	}
}

type mvFile struct {
	f    *file
	role media.FileRole
	part int
	rel  Release
	dir  *dir
}

func (b *builder) buildMovie(d *dir) {
	fr := ParseRelease(d.name)
	var vids []mvFile
	var subs []*file
	var collect func(cur *dir, c movieCtx, part int)
	collect = func(cur *dir, c movieCtx, part int) {
		for _, f := range cur.files {
			switch {
			case isVideo(f):
				if isSampleName(f.stem) {
					continue
				}
				r := ParseRelease(f.stem)
				role := media.RoleVideo
				if c != ctxMain || isExtraName(f.stem) {
					role = media.RoleExtra
				}
				p := r.Part
				if part > 0 {
					p = part
				}
				vids = append(vids, mvFile{f, role, p, r, cur})
			case isSubtitle(f):
				subs = append(subs, f)
			}
		}
		for _, sd := range cur.dirs {
			nc, np := c, part
			switch {
			case isExtrasDir(sd.name, true):
				nc = ctxExtra
			case isSubsDir(sd.name):
				nc = ctxSubs
			case c == ctxMain && partDirNumber(sd.name) > 0:
				np = partDirNumber(sd.name)
			case c == ctxMain && cur == d && !discStructDirs[strings.ToLower(sd.name)] && isSeparateMovie(sd, fr):
				b.movieDir(sd)
				continue
			}
			collect(sd, nc, np)
		}
	}
	collect(d, ctxMain, 0)
	if len(vids) == 0 {
		return
	}

	var primary *mvFile
	parts := 0
	for i := range vids {
		v := &vids[i]
		if v.role != media.RoleVideo {
			continue
		}
		if v.part > 0 {
			parts++
		}
		if primary == nil || v.f.size > primary.f.size || (v.f.size == primary.f.size && NaturalLess(v.f.rel, primary.f.rel)) {
			primary = v
		}
	}
	titleSrc := primary
	if titleSrc == nil {
		titleSrc = &vids[0]
	}

	title, year := fr.Title, fr.Year
	clean := !strongRe.MatchString(normSeps(d.name, isDotted(d.name)))
	// A scene-named folder is as good as the file name; only an obfuscated or
	// year-less folder defers to the release file name.
	if (!clean && year == 0 && titleSrc.rel.Year > 0 && titleSrc.rel.Title != "") || title == "" {
		title, year = titleSrc.rel.Title, titleSrc.rel.Year
	}
	if year == 0 && titleKey(titleSrc.rel.Title) == titleKey(title) {
		year = titleSrc.rel.Year
	}

	it := media.ScannedItem{Kind: media.KindMovie, RelPath: d.rel, Title: title, Year: year}
	it.IDs = mergeIDs(nil, fr.IDs, titleSrc.rel.IDs)
	var stems []string
	for _, v := range vids {
		sf := newFile(v.f, v.role)
		applyRelease(&sf, v.rel)
		if sf.Edition == "" && v.role == media.RoleVideo {
			sf.Edition = fr.Edition
		}
		if parts >= 2 && v.role == media.RoleVideo {
			sf.Track = v.part
		}
		it.Files = append(it.Files, sf)
		stems = append(stems, v.f.stem)
	}
	for _, s := range subs {
		sf := newFile(s, media.RoleSubtitle)
		sf.Language = ParseSubtitleLanguage(s.stem, matchVideoStem(s, stems))
		it.Files = append(it.Files, sf)
	}
	m, ok := findNFO(d, true, "movie.nfo", titleSrc.f.stem+".nfo")
	if !ok && titleSrc.dir != d {
		m, ok = findNFO(titleSrc.dir, true, "movie.nfo", titleSrc.f.stem+".nfo")
	}
	if ok {
		applyNFO(&it, m)
	}
	it.PosterRelPath, it.BackdropRelPath = findArt(d, true, titleSrc.f.stem)
	if it.PosterRelPath == "" && it.BackdropRelPath == "" && titleSrc.dir != d {
		it.PosterRelPath, it.BackdropRelPath = findArt(titleSrc.dir, true, titleSrc.f.stem)
	}
	b.add(it)
}

// isSeparateMovie: a subfolder of a movie folder that is clearly a different
// movie ("Collection/Movie A (2001)" next to a stray video).
func isSeparateMovie(sd *dir, parent Release) bool {
	if !hasMainVideo(sd) {
		return false
	}
	r := ParseRelease(sd.name)
	return r.Year > 0 && titleKey(r.Title) != titleKey(parent.Title)
}

// looseMovies turns video files lying directly in d (a library root) into one
// item per movie; versions / parts of the same title+year are grouped.
func (b *builder) looseMovies(d *dir, vids []*file) {
	type group struct {
		files []mvFile
	}
	var order []string
	groups := map[string]*group{}
	for _, f := range vids {
		if isSampleName(f.stem) {
			continue
		}
		base := reExtraSuffix.ReplaceAllString(f.stem, "")
		r := ParseRelease(base)
		full := ParseRelease(f.stem)
		r.Edition = full.Edition
		role := media.RoleVideo
		if isExtraName(f.stem) {
			role = media.RoleExtra
		}
		key := titleKey(r.Title) + "|" + strconv.Itoa(r.Year)
		g := groups[key]
		if g == nil {
			g = &group{}
			groups[key] = g
			order = append(order, key)
		}
		g.files = append(g.files, mvFile{f, role, r.Part, r, d})
	}
	var allStems []string
	owner := map[string]string{}
	for _, k := range order {
		for _, v := range groups[k].files {
			allStems = append(allStems, v.f.stem)
			owner[v.f.stem] = k
		}
	}
	subsBy := map[string][]*file{}
	for _, f := range d.files {
		if isSubtitle(f) {
			if st := matchVideoStem(f, allStems); st != "" {
				subsBy[owner[st]] = append(subsBy[owner[st]], f)
			}
		}
	}
	for _, k := range order {
		g := groups[k]
		var primary *mvFile
		parts := 0
		for i := range g.files {
			v := &g.files[i]
			if v.role == media.RoleVideo {
				if v.part > 0 {
					parts++
				}
				if primary == nil || NaturalLess(v.f.rel, primary.f.rel) {
					primary = v
				}
			}
		}
		if primary == nil {
			primary = &g.files[0]
		}
		it := media.ScannedItem{Kind: media.KindMovie, RelPath: primary.f.rel, Title: primary.rel.Title, Year: primary.rel.Year}
		it.IDs = mergeIDs(nil, primary.rel.IDs)
		var stems []string
		for _, v := range g.files {
			sf := newFile(v.f, v.role)
			applyRelease(&sf, v.rel)
			if parts >= 2 && v.role == media.RoleVideo {
				sf.Track = v.part
			}
			it.Files = append(it.Files, sf)
			stems = append(stems, v.f.stem)
		}
		for _, s := range subsBy[k] {
			sf := newFile(s, media.RoleSubtitle)
			sf.Language = ParseSubtitleLanguage(s.stem, matchVideoStem(s, stems))
			it.Files = append(it.Files, sf)
		}
		if m, ok := findNFO(d, false, primary.f.stem+".nfo"); ok {
			applyNFO(&it, m)
		}
		it.PosterRelPath, it.BackdropRelPath = findArt(d, false, primary.f.stem)
		b.add(it)
	}
}
