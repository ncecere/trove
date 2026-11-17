package scanner

import (
	"regexp"
	"sort"

	"trove/internal/media"
)

var reTrackish = regexp.MustCompile(`(?i)(?:part|pt|disc|disk|cd|track|chapter|ch)?[ ._\-]*\d+`)

// multipleSingleFileBooks: a folder of several .m4b files whose names differ
// beyond their numbers is a series/author folder of single-file books, not
// one book split in parts.
func multipleSingleFileBooks(direct []*file) bool {
	if len(direct) < 2 {
		return false
	}
	keys := map[string]bool{}
	for _, f := range direct {
		if f.ext != "m4b" {
			return false
		}
		keys[titleKey(reTrackish.ReplaceAllString(f.stem, " "))] = true
	}
	return len(keys) == len(direct)
}

func isAudiobookDir(d *dir) bool {
	var direct []*file
	for _, f := range d.files {
		if isAudio(f) {
			direct = append(direct, f)
		}
	}
	discs := false
	for _, sd := range d.dirs {
		if !sd.hasFile(isAudio) {
			continue
		}
		if partDirNumber(sd.name) == 0 {
			return false // a real sub-book: d is an author / series folder
		}
		discs = true
	}
	if len(direct) > 0 {
		return !multipleSingleFileBooks(direct)
	}
	return discs
}

func (b *builder) scanAudiobooks(root *dir) {
	b.looseAudio(root, bookCtx{})
	for _, d := range root.dirs {
		if b.ctx.Err() != nil {
			return
		}
		if !d.hasFile(isAudio) {
			continue
		}
		if isAudiobookDir(d) {
			b.buildAudiobook(d, bookCtx{})
		} else {
			b.audioContainer(d, bookCtx{author: cleanAuthor(d.name)})
		}
	}
}

func (b *builder) audioContainer(d *dir, c bookCtx) {
	b.looseAudio(d, c)
	for _, sd := range d.dirs {
		if !sd.hasFile(isAudio) {
			continue
		}
		if isAudiobookDir(sd) {
			b.buildAudiobook(sd, c)
		} else {
			nc := c
			nc.series = cleanSeries(sd.name)
			b.audioContainer(sd, nc)
		}
	}
}

type abTrack struct {
	f           *file
	disc, track int
}

func (b *builder) buildAudiobook(d *dir, c bookCtx) {
	var tracks []abTrack
	var extras []*file
	var walk func(cur *dir, disc int)
	walk = func(cur *dir, disc int) {
		for _, f := range cur.files {
			switch {
			case isAudio(f):
				fd, ft := parseTrack(f.stem)
				if disc > 0 {
					fd = disc
				}
				tracks = append(tracks, abTrack{f, fd, ft})
			case f.ext == "pdf" || f.ext == "epub":
				extras = append(extras, f)
			}
		}
		for _, sd := range cur.dirs {
			nd := disc
			if n := partDirNumber(sd.name); n > 0 {
				nd = n
			}
			walk(sd, nd)
		}
	}
	walk(d, 0)
	if len(tracks) == 0 {
		return
	}
	orderTracks(tracks)

	bn := parseBookName(d.name, c.author, false)
	if !hasLetter(bn.Title) {
		fb := parseBookName(tracks[0].f.stem, c.author, false)
		bn.Title = fb.Title
	}
	it := media.ScannedItem{Kind: media.KindAudiobook, RelPath: d.rel, Title: bn.Title, Year: bn.Year, Author: bn.Author, Narrator: bn.Narrator, Series: c.series, SeriesIndex: bn.Index}
	if it.Author == "" {
		it.Author = c.author
	}
	if bn.Series != "" {
		it.Series = bn.Series
	}
	it.IDs = bookIDs(bn)
	for i, t := range tracks {
		sf := newFile(t.f, media.RoleAudio)
		sf.Track = i + 1
		it.Files = append(it.Files, sf)
	}
	for _, f := range extras {
		it.Files = append(it.Files, newFile(f, media.RoleExtra))
	}
	if f := d.fileByName("metadata.opf"); f != nil {
		if m, ok := parseOPF(readSmall(f.abs)); ok {
			applyBookMeta(&it, m)
		}
	}
	if f := d.fileByName("metadata.json"); f != nil {
		if m, ok := parseABSMetadata(readSmall(f.abs)); ok {
			applyBookMeta(&it, m)
		}
	}
	it.PosterRelPath, it.BackdropRelPath = findArt(d, true)
	b.add(it)
}

// orderTracks sorts by (disc, track) when every file has a track number,
// otherwise by natural path order (which also keeps "Disc 1/" before "Disc 2/").
func orderTracks(ts []abTrack) {
	all := true
	for _, t := range ts {
		if t.track == 0 {
			all = false
		}
	}
	sort.SliceStable(ts, func(i, j int) bool {
		a, b := ts[i], ts[j]
		if all {
			if a.disc != b.disc {
				return a.disc < b.disc
			}
			if a.track != b.track {
				return a.track < b.track
			}
		}
		return NaturalLess(a.f.rel, b.f.rel)
	})
}

// looseAudio: every audio file lying directly in d is its own audiobook.
func (b *builder) looseAudio(d *dir, c bookCtx) {
	for _, f := range d.files {
		if !isAudio(f) {
			continue
		}
		bn := parseBookName(f.stem, c.author, false)
		it := media.ScannedItem{Kind: media.KindAudiobook, RelPath: f.rel, Title: bn.Title, Year: bn.Year, Author: bn.Author, Narrator: bn.Narrator, Series: c.series, SeriesIndex: bn.Index}
		if it.Author == "" {
			it.Author = c.author
		}
		if bn.Series != "" {
			it.Series = bn.Series
		}
		it.IDs = bookIDs(bn)
		sf := newFile(f, media.RoleAudio)
		sf.Track = 1
		it.Files = []media.ScannedFile{sf}
		it.PosterRelPath, it.BackdropRelPath = findArt(d, false, f.stem)
		b.add(it)
	}
}
