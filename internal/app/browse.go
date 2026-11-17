package app

import (
	"fmt"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"

	"trove/internal/media"
	"trove/internal/store"
)

type itemSummaryJSON struct {
	ID           int64    `json:"id"`
	LibraryID    int64    `json:"libraryId"`
	LibraryType  string   `json:"libraryType"`
	Kind         string   `json:"kind"`
	Title        string   `json:"title"`
	Year         *int     `json:"year"`
	Author       *string  `json:"author"`
	PosterURL    *string  `json:"posterUrl"`
	Rating       *float64 `json:"rating"`
	AddedAt      string   `json:"addedAt"`
	Size         int64    `json:"size"`
	FileCount    int      `json:"fileCount"`
	SeasonCount  *int     `json:"seasonCount"`
	EpisodeCount *int     `json:"episodeCount"`
}

type mediaFileJSON struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	RelPath    string  `json:"relPath"`
	Size       int64   `json:"size"`
	Role       string  `json:"role"`
	Container  string  `json:"container"`
	Resolution *string `json:"resolution"`
	Source     *string `json:"source"`
	VideoCodec *string `json:"videoCodec"`
	AudioCodec *string `json:"audioCodec"`
	HDR        *string `json:"hdr"`
	Edition    *string `json:"edition"`
	Language   *string `json:"language"`
	Track      *int    `json:"track"`
	ModifiedAt string  `json:"modifiedAt"`
}

type episodeJSON struct {
	Season     int             `json:"season"`
	Episode    int             `json:"episode"`
	EpisodeEnd *int            `json:"episodeEnd"`
	Title      *string         `json:"title"`
	Overview   *string         `json:"overview"`
	AirDate    *string         `json:"airDate"`
	File       mediaFileJSON   `json:"file"`
	Subtitles  []mediaFileJSON `json:"subtitles"`
}

type seasonJSON struct {
	Season       int           `json:"season"`
	EpisodeCount int           `json:"episodeCount"`
	Size         int64         `json:"size"`
	Episodes     []episodeJSON `json:"episodes"`
}

type itemDetailJSON struct {
	itemSummaryJSON
	OriginalTitle  *string           `json:"originalTitle"`
	Tagline        *string           `json:"tagline"`
	Overview       *string           `json:"overview"`
	Genres         []string          `json:"genres"`
	RuntimeMinutes *int              `json:"runtimeMinutes"`
	ReleaseDate    *string           `json:"releaseDate"`
	Status         *string           `json:"status"`
	Studio         *string           `json:"studio"`
	Narrator       *string           `json:"narrator"`
	Series         *string           `json:"series"`
	SeriesIndex    *float64          `json:"seriesIndex"`
	BackdropURL    *string           `json:"backdropUrl"`
	ExternalIDs    map[string]string `json:"externalIds"`
	Files          []mediaFileJSON   `json:"files"`
	Seasons        []seasonJSON      `json:"seasons"`
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intPtr(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}

func floatPtr(f float64) *float64 {
	if f == 0 {
		return nil
	}
	return &f
}

func toSummaryJSON(it *store.ItemSummary) itemSummaryJSON {
	j := itemSummaryJSON{
		ID: it.ID, LibraryID: it.LibraryID, LibraryType: string(it.LibraryType), Kind: string(it.Kind),
		Title: it.Title, Year: intPtr(it.Year), Rating: floatPtr(it.Rating), AddedAt: fmtTime(it.AddedAt),
		Size: it.Size, FileCount: it.FileCount,
		PosterURL: apiURL(fmt.Sprintf("/items/%d/poster", it.ID), it.PosterV),
	}
	if it.Kind == media.KindBook || it.Kind == media.KindAudiobook {
		j.Author = strPtr(it.Author)
	}
	if it.Kind == media.KindShow {
		sc, ec := it.SeasonCount, it.EpisodeCount
		j.SeasonCount, j.EpisodeCount = &sc, &ec
	}
	return j
}

func toSummaries(items []store.ItemSummary) []itemSummaryJSON {
	out := make([]itemSummaryJSON, 0, len(items))
	for i := range items {
		out = append(out, toSummaryJSON(&items[i]))
	}
	return out
}

func toFileJSON(itemRel string, f *store.File) mediaFileJSON {
	j := mediaFileJSON{
		ID: f.ID, Name: path.Base(f.RelPath), RelPath: relToItem(itemRel, f.RelPath), Size: f.Size,
		Role: string(f.Role), Container: f.Container,
		Resolution: strPtr(f.Resolution), Source: strPtr(f.Source), VideoCodec: strPtr(f.VideoCodec),
		AudioCodec: strPtr(f.AudioCodec), HDR: strPtr(f.HDR), Edition: strPtr(f.Edition),
		Language: strPtr(f.Language), Track: intPtr(f.Track), ModifiedAt: fmtTime(f.ModTime),
	}
	return j
}

var roleOrder = map[media.FileRole]int{media.RoleVideo: 0, media.RoleAudio: 1, media.RoleEbook: 2, media.RoleSubtitle: 3, media.RoleExtra: 4}

func roleRank(r media.FileRole) int {
	if n, ok := roleOrder[r]; ok {
		return n
	}
	return 5
}

func sortFiles(files []store.File) {
	sort.SliceStable(files, func(i, j int) bool {
		a, b := &files[i], &files[j]
		if ra, rb := roleRank(a.Role), roleRank(b.Role); ra != rb {
			return ra < rb
		}
		ta, tb := a.Track, b.Track
		if ta == 0 {
			ta = 1 << 30
		}
		if tb == 0 {
			tb = 1 << 30
		}
		if ta != tb {
			return ta < tb
		}
		na, nb := strings.ToLower(path.Base(a.RelPath)), strings.ToLower(path.Base(b.RelPath))
		if na != nb {
			return na < nb
		}
		return a.RelPath < b.RelPath
	})
}

func (a *App) buildDetail(r *http.Request, it *store.Item) (*itemDetailJSON, error) {
	files, err := a.store.ItemFiles(r.Context(), it.ID)
	if err != nil {
		return nil, err
	}
	d := it.Details()
	j := &itemDetailJSON{
		itemSummaryJSON: toSummaryJSON(&it.ItemSummary),
		OriginalTitle:   strPtr(d.OriginalTitle),
		Tagline:         strPtr(d.Tagline),
		Overview:        strPtr(d.Overview),
		Genres:          d.Genres,
		RuntimeMinutes:  intPtr(d.RuntimeMinutes),
		ReleaseDate:     strPtr(d.ReleaseDate),
		Status:          strPtr(d.Status),
		Studio:          strPtr(d.Studio),
		Narrator:        strPtr(d.Narrator),
		Series:          strPtr(d.Series),
		SeriesIndex:     floatPtr(d.SeriesIndex),
		BackdropURL:     apiURL(fmt.Sprintf("/items/%d/backdrop", it.ID), it.BackdropV),
		ExternalIDs:     d.ExternalIDs,
		Files:           []mediaFileJSON{},
		Seasons:         []seasonJSON{},
	}
	if j.Genres == nil {
		j.Genres = []string{}
	}
	sortFiles(files)

	if it.Kind != media.KindShow {
		for i := range files {
			j.Files = append(j.Files, toFileJSON(it.RelPath, &files[i]))
		}
		return j, nil
	}

	eps, err := a.store.ItemEpisodes(r.Context(), it.ID)
	if err != nil {
		return nil, err
	}
	type key = store.EpisodeKey
	// Subtitles that belong to an episode.
	subs := map[key][]*store.File{}
	videoKeys := map[key]bool{}
	for i := range files {
		f := &files[i]
		if f.Role == media.RoleVideo && f.Episode > 0 {
			videoKeys[key{Season: f.Season, Episode: f.Episode}] = true
		}
	}
	seasons := map[int]*seasonJSON{}
	for i := range files {
		f := &files[i]
		k := key{Season: f.Season, Episode: f.Episode}
		if f.Role == media.RoleSubtitle && f.Episode > 0 && videoKeys[k] {
			subs[k] = append(subs[k], f)
		}
	}
	for i := range files {
		f := &files[i]
		k := key{Season: f.Season, Episode: f.Episode}
		switch {
		case f.Role == media.RoleVideo && f.Episode > 0:
			s := seasons[f.Season]
			if s == nil {
				s = &seasonJSON{Season: f.Season, Episodes: []episodeJSON{}}
				seasons[f.Season] = s
			}
			e := episodeJSON{Season: f.Season, Episode: f.Episode, File: toFileJSON(it.RelPath, f), Subtitles: []mediaFileJSON{}}
			if f.EpisodeEnd > f.Episode {
				end := f.EpisodeEnd
				e.EpisodeEnd = &end
			}
			if m, ok := eps[k]; ok {
				e.Title, e.Overview, e.AirDate = strPtr(m.Title), strPtr(m.Overview), strPtr(m.AirDate)
			}
			if e.Title == nil {
				e.Title = strPtr(f.EpisodeTitle)
			}
			s.Size += f.Size
			s.Episodes = append(s.Episodes, e)
		case f.Role == media.RoleSubtitle && f.Episode > 0 && videoKeys[k]:
			// attached below
		default:
			j.Files = append(j.Files, toFileJSON(it.RelPath, f))
		}
	}
	for _, s := range seasons {
		for ei := range s.Episodes {
			e := &s.Episodes[ei]
			for _, sf := range subs[key{Season: e.Season, Episode: e.Episode}] {
				e.Subtitles = append(e.Subtitles, toFileJSON(it.RelPath, sf))
			}
		}
		// Subtitles of a (season, episode) are attached to every version of
		// that episode; count their size once per season.
		for k, list := range subs {
			if k.Season == s.Season {
				for _, sf := range list {
					s.Size += sf.Size
				}
			}
		}
		sort.SliceStable(s.Episodes, func(a, b int) bool {
			if s.Episodes[a].Episode != s.Episodes[b].Episode {
				return s.Episodes[a].Episode < s.Episodes[b].Episode
			}
			return s.Episodes[a].File.Name < s.Episodes[b].File.Name
		})
		s.EpisodeCount = len(s.Episodes)
		j.Seasons = append(j.Seasons, *s)
	}
	sort.Slice(j.Seasons, func(a, b int) bool { return j.Seasons[a].Season < j.Seasons[b].Season })
	return j, nil
}

func (a *App) handleItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		notFound(w)
		return
	}
	it, _, ok := a.loadAccessibleItem(w, r, id)
	if !ok {
		return
	}
	d, err := a.buildDetail(r, it)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

type pageJSON struct {
	Items any `json:"items"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

func parsePaging(r *http.Request, defLimit, maxLimit int) (page, limit int, msg string) {
	page, limit = 1, defLimit
	q := r.URL.Query()
	if s := q.Get("page"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, 0, "page must be an integer"
		}
		page = max(n, 1)
	}
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, 0, "limit must be an integer"
		}
		limit = min(max(n, 1), maxLimit)
	}
	return page, limit, ""
}

// libraryParam parses an optional ?library= the user can access.
func (a *App) libraryParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	s := r.URL.Query().Get("library")
	if s == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		badRequest(w, "library must be a library id")
		return 0, false
	}
	if !userFrom(r).CanAccess(id) {
		notFound(w)
		return 0, false
	}
	if _, err := a.store.GetLibrary(r.Context(), id); err != nil {
		a.storeErr(w, r, err)
		return 0, false
	}
	return id, true
}

func (a *App) handleItems(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	qv := r.URL.Query()
	libID, ok := a.libraryParam(w, r)
	if !ok {
		return
	}
	q := store.ItemQuery{Accessible: accessFilter(u), LibraryID: libID, Q: qv.Get("q"), Genre: qv.Get("genre")}
	if k := qv.Get("kind"); k != "" {
		switch media.Kind(k) {
		case media.KindMovie, media.KindShow, media.KindBook, media.KindAudiobook:
			q.Kind = media.Kind(k)
		default:
			badRequest(w, "kind must be one of movie, show, book, audiobook")
			return
		}
	}
	q.Sort = qv.Get("sort")
	switch q.Sort {
	case "":
		q.Sort = "title"
	case "title", "added", "year", "rating", "size":
	default:
		badRequest(w, "sort must be one of title, added, year, rating, size")
		return
	}
	switch qv.Get("order") {
	case "":
		q.Desc = q.Sort != "title"
	case "asc":
	case "desc":
		q.Desc = true
	default:
		badRequest(w, "order must be asc or desc")
		return
	}
	if len(q.Q) > 200 {
		q.Q = q.Q[:200]
	}
	page, limit, msg := parsePaging(r, 48, 100)
	if msg != "" {
		badRequest(w, msg)
		return
	}
	q.Limit, q.Offset = limit, (page-1)*limit
	items, total, err := a.store.QueryItems(r.Context(), q)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pageJSON{Items: toSummaries(items), Total: total, Page: page, Limit: limit})
}

func (a *App) handleHome(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	recent, _, err := a.store.QueryItems(r.Context(), store.ItemQuery{Accessible: accessFilter(u), Sort: "added", Desc: true, Limit: 20})
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	libs, err := a.accessibleLibraries(r)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	type libRecent struct {
		Library libraryJSON       `json:"library"`
		Recent  []itemSummaryJSON `json:"recent"`
	}
	out := []libRecent{}
	for _, l := range libs {
		items, _, err := a.store.QueryItems(r.Context(), store.ItemQuery{LibraryID: l.ID, Sort: "added", Desc: true, Limit: 20})
		if err != nil {
			a.internalError(w, r, err)
			return
		}
		out = append(out, libRecent{Library: a.toLibraryJSON(l, u), Recent: toSummaries(items)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"recentlyAdded": toSummaries(recent), "libraries": out})
}

func (a *App) handleGenres(w http.ResponseWriter, r *http.Request) {
	libID, ok := a.libraryParam(w, r)
	if !ok {
		return
	}
	g, err := a.store.Genres(r.Context(), accessFilter(userFrom(r)), libID)
	if err != nil {
		a.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"genres": g})
}
