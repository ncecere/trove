package scanner

import (
	"regexp"
	"sort"
	"strings"

	"trove/internal/media"
)

type bookCtx struct {
	author string
	series string
}

// formatPriority orders ebook formats when one has to represent the book.
var formatPriority = map[string]int{"epub": 0, "azw3": 1, "azw": 2, "kfx": 3, "mobi": 4, "fb2": 5, "pdf": 6, "djvu": 7, "cbz": 8, "cbr": 9, "cb7": 10, "txt": 11}

func betterFormat(a, b *file) bool {
	pa, pb := formatPriority[a.ext], formatPriority[b.ext]
	if pa != pb {
		return pa < pb
	}
	return NaturalLess(a.rel, b.rel)
}

var reParenGroup = regexp.MustCompile(`\s*[(\[][^()\[\]]*[)\]]`)

func cleanAuthor(name string) string {
	s := strings.ReplaceAll(name, "_", " ")
	if t := reParenGroup.ReplaceAllString(s, ""); hasAlnum(t) {
		s = t
	}
	return collapseSpaces(s)
}

func cleanSeries(name string) string { return cleanAuthor(name) }

func (b *builder) scanBooks(root *dir) {
	b.looseBooks(root, bookCtx{})
	for _, d := range root.dirs {
		if b.ctx.Err() != nil {
			return
		}
		if !d.hasFile(isEbook) {
			continue
		}
		if isSingleBookDir(d, true) {
			b.buildBook(d, bookCtx{})
		} else {
			b.bookContainer(d, bookCtx{author: cleanAuthor(d.name)})
		}
	}
}

func (b *builder) bookContainer(d *dir, c bookCtx) {
	b.looseBooks(d, c)
	for _, sd := range d.dirs {
		if !sd.hasFile(isEbook) {
			continue
		}
		if isSingleBookDir(sd, false) {
			b.buildBook(sd, c)
		} else {
			nc := c
			nc.series = cleanSeries(sd.name)
			b.bookContainer(sd, nc)
		}
	}
}

func isFormatDir(name string) bool { return ebookExts[squash(name)] }

// isSingleBookDir decides whether d holds exactly one book (in one or more
// formats). top is true for folders directly in the library root, where a
// folder with a single book file is more likely an author folder unless its
// name says otherwise.
func isSingleBookDir(d *dir, top bool) bool {
	var direct []*file
	for _, f := range d.files {
		if isEbook(f) {
			direct = append(direct, f)
		}
	}
	for _, sd := range d.dirs {
		if sd.hasFile(isEbook) && !isFormatDir(sd.name) {
			return false
		}
	}
	if d.fileByName("metadata.opf") != nil && d.hasFile(isEbook) {
		return true
	}
	if len(direct) == 0 {
		return d.hasFile(isEbook) // formats in epub/ pdf/ subfolders
	}
	keys := map[string]bool{}
	for _, f := range direct {
		keys[strings.ToLower(f.stem)] = true
	}
	if len(keys) != 1 {
		return false
	}
	if !top {
		return true
	}
	if reDashSplit.MatchString(d.name) {
		return true // "Author - Title/"
	}
	ft := titleKey(parseBookName(d.name, "", false).Title)
	for _, p := range reDashSplit.Split(direct[0].stem, -1) {
		if titleKey(parseBookName(p, "", false).Title) == ft {
			return true
		}
	}
	return false
}

// authorFromStem finds the author in "Title - Author" / "Author - Title" file
// names when the title is already known from the folder.
func authorFromStem(stem, title string) string {
	parts := reDashSplit.Split(stem, -1)
	if len(parts) != 2 {
		return ""
	}
	tk := titleKey(title)
	switch {
	case titleKey(parts[0]) == tk:
		return strings.TrimSpace(parts[1])
	case titleKey(parts[1]) == tk:
		return strings.TrimSpace(parts[0])
	}
	return ""
}

func applyBookMeta(it *media.ScannedItem, m localMeta) {
	if m.Title != "" {
		it.Title = m.Title
	}
	if len(m.Authors) > 0 {
		it.Author = strings.Join(m.Authors, " & ")
	}
	if len(m.Narrators) > 0 {
		it.Narrator = strings.Join(m.Narrators, ", ")
	}
	if m.Year != 0 {
		it.Year = m.Year
	}
	if m.Overview != "" {
		it.Overview = m.Overview
	}
	if len(m.Genres) > 0 {
		it.Genres = m.Genres
	}
	if m.Series != "" {
		it.Series = m.Series
		it.SeriesIndex = m.SeriesIndex
	}
	if len(m.IDs) > 0 {
		it.IDs = mergeIDs(m.IDs, it.IDs)
	}
}

func bookIDs(bn bookName) map[string]string {
	ids := map[string]string{}
	if bn.ISBN != "" {
		ids["isbn"] = bn.ISBN
	}
	if bn.ASIN != "" {
		ids["asin"] = bn.ASIN
	}
	return ids
}

func (b *builder) buildBook(d *dir, c bookCtx) {
	var opf localMeta
	hasOPF := false
	if f := d.fileByName("metadata.opf"); f != nil {
		if data := readSmall(f.abs); data != nil {
			opf, hasOPF = parseOPF(data)
		}
	}
	var files []*file
	var walk func(cur *dir)
	walk = func(cur *dir) {
		for _, f := range cur.files {
			if isEbook(f) {
				files = append(files, f)
			}
		}
		for _, sd := range cur.dirs {
			walk(sd)
		}
	}
	walk(d)
	if len(files) == 0 {
		return
	}
	primary := files[0]
	for _, f := range files[1:] {
		if betterFormat(f, primary) {
			primary = f
		}
	}
	bn := parseBookName(d.name, c.author, hasOPF)
	if !hasLetter(bn.Title) {
		fb := parseBookName(primary.stem, c.author, false)
		bn.Title = fb.Title
		if bn.Author == "" {
			bn.Author = fb.Author
		}
	}
	it := media.ScannedItem{Kind: media.KindBook, RelPath: d.rel, Title: bn.Title, Year: bn.Year, Author: bn.Author, Series: c.series, SeriesIndex: bn.Index}
	if it.Author == "" {
		it.Author = c.author
	}
	if it.Author == "" {
		it.Author = authorFromStem(primary.stem, bn.Title)
	}
	if bn.Series != "" {
		it.Series = bn.Series
	}
	it.IDs = bookIDs(bn)
	for _, f := range files {
		it.Files = append(it.Files, newFile(f, media.RoleEbook))
	}
	if hasOPF {
		applyBookMeta(&it, opf)
	}
	it.PosterRelPath, it.BackdropRelPath = findArt(d, true, primary.stem)
	b.add(it)
}

// looseBooks makes one item per distinct basename among the ebook files lying
// directly in d, grouping formats of the same basename.
func (b *builder) looseBooks(d *dir, c bookCtx) {
	groups := map[string][]*file{}
	var order []string
	for _, f := range d.files {
		if !isEbook(f) {
			continue
		}
		k := strings.ToLower(f.stem)
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], f)
	}
	sort.Strings(order)
	for _, k := range order {
		fs := groups[k]
		primary := fs[0]
		for _, f := range fs[1:] {
			if betterFormat(f, primary) {
				primary = f
			}
		}
		bn := parseBookName(primary.stem, c.author, false)
		it := media.ScannedItem{Kind: media.KindBook, RelPath: primary.rel, Title: bn.Title, Year: bn.Year, Author: bn.Author, Series: c.series, SeriesIndex: bn.Index}
		if it.Author == "" {
			it.Author = c.author
		}
		if bn.Series != "" {
			it.Series = bn.Series
		}
		it.IDs = bookIDs(bn)
		for _, f := range fs {
			it.Files = append(it.Files, newFile(f, media.RoleEbook))
		}
		if f := d.fileByName(primary.stem + ".opf"); f != nil {
			if m, ok := parseOPF(readSmall(f.abs)); ok {
				applyBookMeta(&it, m)
			}
		}
		it.PosterRelPath, it.BackdropRelPath = findArt(d, false, primary.stem)
		b.add(it)
	}
}
