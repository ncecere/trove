package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"trove/internal/media"
)

func openTest(t *testing.T) (*Store, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "trove.db")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, p
}

func TestMigrationsIdempotent(t *testing.T) {
	s, p := openTest(t)
	ctx := context.Background()
	v, err := s.SchemaVersion(ctx)
	if err != nil || v != 1 {
		t.Fatalf("version %d %v", v, err)
	}
	s.Close()
	s2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	var fk int
	s2.db.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	var mode string
	s2.db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if fk != 1 || mode != "wal" {
		t.Fatalf("pragmas: fk=%d mode=%s", fk, mode)
	}
}

func TestSortTitle(t *testing.T) {
	for in, want := range map[string]string{"The Matrix": "matrix", "A Quiet Place": "quiet place", "An Education": "education", "Alien": "alien", "The": "the", "'71": "71"} {
		if got := SortTitle(in); got != want {
			t.Errorf("SortTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIndexUpsertAndPrune(t *testing.T) {
	s, _ := openTest(t)
	ctx := context.Background()
	lib, err := s.CreateLibrary(ctx, "Movies", media.LibraryMovies, "/x")
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	mk := func(rel string, files ...string) IndexItem {
		it := media.ScannedItem{Kind: media.KindMovie, RelPath: rel, Title: rel}
		for _, f := range files {
			it.Files = append(it.Files, media.ScannedFile{RelPath: rel + "/" + f, Size: 10, ModTime: old, Role: media.RoleVideo, Container: "mkv"})
		}
		return IndexItem{Item: it}
	}
	gen, _ := s.NextScanGen(ctx, lib.ID)
	if err := s.IndexItems(ctx, lib.ID, gen, []IndexItem{mk("A", "a.mkv", "b.mkv"), mk("B", "b.mkv")}, true, nil); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.PruneLibrary(ctx, lib.ID, gen); n != 0 {
		t.Fatalf("pruned %d", n)
	}
	items, total, _ := s.QueryItems(ctx, ItemQuery{Sort: "title"})
	if total != 2 || !items[0].AddedAt.Equal(old) || items[0].Size != 20 {
		t.Fatalf("first scan: %+v", items)
	}
	idA := items[0].ID
	if err := s.SetRemoteMetadata(ctx, idA, &media.Metadata{Provider: "x", Title: "Alpha", Genres: []string{"Drama"}}); err != nil {
		t.Fatal(err)
	}

	gen, _ = s.NextScanGen(ctx, lib.ID)
	if err := s.IndexItems(ctx, lib.ID, gen, []IndexItem{mk("A", "a.mkv")}, false, nil); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.PruneLibrary(ctx, lib.ID, gen); n != 1 {
		t.Fatalf("pruned %d, want 1", n)
	}
	it, err := s.GetItem(ctx, idA)
	if err != nil {
		t.Fatal(err)
	}
	if it.Title != "Alpha" || it.FileCount != 1 || it.Remote == nil || !it.AddedAt.Equal(old) {
		t.Fatalf("after rescan: %+v", it)
	}
	files, _ := s.ItemFiles(ctx, idA)
	if len(files) != 1 || files[0].RelPath != "A/a.mkv" {
		t.Fatalf("files: %+v", files)
	}
	g, _ := s.Genres(ctx, nil, 0)
	if len(g) != 1 || g[0] != "Drama" {
		t.Fatalf("genres: %v", g)
	}
}
