package cards

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/julianhuber/hitsync/backend/internal/navidrome"
)

func intPtr(v int) *int { return &v }

type fakeLibrary struct {
	songs []navidrome.Song
	err   error
}

func (f *fakeLibrary) SearchPage(_ context.Context, offset, count int) ([]navidrome.Song, error) {
	if f.err != nil {
		return nil, f.err
	}
	if offset >= len(f.songs) {
		return nil, nil
	}
	return f.songs[offset:min(offset+count, len(f.songs))], nil
}

func newTestCollection(t *testing.T, lib Library) (*Collection, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config", "cards.json")
	return NewCollection(path, lib, 45*time.Second, 600*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))), path
}

func TestMergeKeepsHandEditedFields(t *testing.T) {
	existing := []Card{
		{NavidromeID: "a", Title: "Old Title", Artist: "X", Year: intPtr(1999), DurationSec: 200, HitsyncYear: intPtr(1975), Excluded: true},
		{NavidromeID: "gone", Title: "Removed", Artist: "Y", DurationSec: 200},
		{NavidromeID: "same", Title: "Same", Artist: "Z", Year: intPtr(2001), DurationSec: 180},
	}
	scanned := []Card{
		{NavidromeID: "a", Title: "New Title", Artist: "X", Year: intPtr(2005), DurationSec: 201},
		{NavidromeID: "same", Title: "Same", Artist: "Z", Year: intPtr(2001), DurationSec: 180},
		{NavidromeID: "new", Title: "Fresh", Artist: "A", DurationSec: 120, HitsyncYear: intPtr(1), Excluded: true},
		{NavidromeID: "new", Title: "Fresh", Artist: "A", DurationSec: 120},
	}

	merged, summary := Merge(existing, scanned)

	if summary != (ScanSummary{Added: 1, Updated: 1, Removed: 1, Total: 3}) {
		t.Fatalf("summary = %+v", summary)
	}
	byID := map[string]Card{}
	for _, c := range merged {
		byID[c.NavidromeID] = c
	}
	a := byID["a"]
	if a.Title != "New Title" || *a.Year != 2005 || a.DurationSec != 201 {
		t.Errorf("library fields not refreshed: %+v", a)
	}
	if a.HitsyncYear == nil || *a.HitsyncYear != 1975 || !a.Excluded {
		t.Errorf("hand-edited fields not preserved: %+v", a)
	}
	if n := byID["new"]; n.HitsyncYear != nil || n.Excluded {
		t.Errorf("new card must start with hitsyncyear null and excluded false: %+v", n)
	}
	if merged[0].Artist != "A" {
		t.Errorf("cards not sorted by artist: first is %q", merged[0].Artist)
	}
}

func TestGameYearPrefersHitsyncYear(t *testing.T) {
	cases := []struct {
		card       Card
		wantYear   int
		wantSource string
		wantOK     bool
	}{
		{Card{Year: intPtr(1999), HitsyncYear: intPtr(1975)}, 1975, SourceHitsyncYear, true},
		{Card{Year: intPtr(1999)}, 1999, SourceNavidrome, true},
		{Card{HitsyncYear: intPtr(1975)}, 1975, SourceHitsyncYear, true},
		{Card{}, 0, "", false},
	}
	for _, c := range cases {
		year, source, ok := c.card.GameYear()
		if year != c.wantYear || source != c.wantSource || ok != c.wantOK {
			t.Errorf("GameYear(%+v) = %d, %q, %v", c.card, year, source, ok)
		}
	}
}

func TestReadFileIsStrict(t *testing.T) {
	dir := t.TempDir()
	write := func(content string) string {
		path := filepath.Join(dir, "cards.json")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	if cards, err := ReadFile(filepath.Join(dir, "missing.json")); err != nil || cards != nil {
		t.Errorf("missing file: %v, %v", cards, err)
	}
	if _, err := ReadFile(write(`[{"navidromeId":"a","hitsync_year":1975}]`)); err == nil {
		t.Error("expected error for misspelled field")
	}
	if _, err := ReadFile(write(`[{"navidromeId":"a"},{"navidromeId":"a"}]`)); err == nil {
		t.Error("expected error for duplicate id")
	}
	if _, err := ReadFile(write(`[{"title":"no id"}]`)); err == nil {
		t.Error("expected error for missing id")
	}
}

func TestScanCreatesFileWithNullHitsyncYear(t *testing.T) {
	lib := &fakeLibrary{songs: []navidrome.Song{
		{ID: "a", Title: "Song & Dance", Artist: "X", Album: "LP", Year: 1984, Duration: 200},
		{ID: "b", Title: "No Year", Artist: "Y", Duration: 200},
	}}
	c, path := newTestCollection(t, lib)

	summary, err := c.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if summary.Added != 2 {
		t.Errorf("summary = %+v", summary)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"hitsyncyear": null`, `"excluded": false`, `"year": null`, `"Song & Dance"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("file lacks %s:\n%s", want, data)
		}
	}
	// "b" has no year from either source, so it is not playable.
	if n := c.PlayableCount(); n != 1 {
		t.Errorf("playable = %d, want 1", n)
	}
}

func TestScanPreservesHandEditsAndAppliesThem(t *testing.T) {
	lib := &fakeLibrary{songs: []navidrome.Song{
		{ID: "a", Title: "A", Artist: "X", Year: 2010, Duration: 200},
		{ID: "b", Title: "B", Artist: "Y", Year: 2011, Duration: 200},
	}}
	c, path := newTestCollection(t, lib)
	if _, err := c.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	cards, _ := ReadFile(path)
	for i := range cards {
		switch cards[i].NavidromeID {
		case "a":
			cards[i].HitsyncYear = intPtr(1971)
		case "b":
			cards[i].Excluded = true
		}
	}
	if err := WriteFile(path, cards); err != nil {
		t.Fatal(err)
	}
	lib.songs = append(lib.songs, navidrome.Song{ID: "c", Title: "C", Artist: "Z", Year: 2012, Duration: 200})

	summary, err := c.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.Added != 1 || summary.Total != 3 {
		t.Errorf("summary = %+v", summary)
	}
	if c.IsPlayable("b") {
		t.Error("excluded card is playable")
	}
	for i := 0; i < 20; i++ {
		card, ok := c.Draw(map[string]bool{"c": true})
		if !ok || card.NavidromeID != "a" {
			t.Fatalf("Draw = %+v, %v; want only a", card, ok)
		}
		if year, _, _ := card.GameYear(); year != 1971 {
			t.Fatalf("hitsyncyear not applied: %d", year)
		}
	}
}

func TestScanFailureKeepsFileAndFallsBackToIt(t *testing.T) {
	lib := &fakeLibrary{songs: []navidrome.Song{{ID: "a", Title: "A", Artist: "X", Year: 2010, Duration: 200}}}
	first, path := newTestCollection(t, lib)
	if _, err := first.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)

	// A fresh process whose Navidrome is unreachable still serves the file.
	lib.err = errors.New("connection refused")
	second := NewCollection(path, lib, 45*time.Second, 600*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := second.Scan(context.Background()); err == nil {
		t.Fatal("expected scan error")
	}
	if second.PlayableCount() != 1 {
		t.Error("did not fall back to the existing file")
	}
	if stats := second.Stats(); stats.LastScanError == "" {
		t.Error("scan error not reported in stats")
	}

	// An empty library answer must not wipe the file.
	lib.err = nil
	lib.songs = nil
	if _, err := second.Scan(context.Background()); err == nil {
		t.Error("expected refusal for empty library")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Error("file changed by a failed scan")
	}
}

func TestScanRejectsBrokenFile(t *testing.T) {
	lib := &fakeLibrary{songs: []navidrome.Song{{ID: "a", Title: "A", Artist: "X", Year: 2010, Duration: 200}}}
	c, path := newTestCollection(t, lib)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	broken := `[{"navidromeId": "a", "excluded": tru}]`
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Scan(context.Background()); err == nil {
		t.Fatal("expected error for broken file")
	}
	if data, _ := os.ReadFile(path); string(data) != broken {
		t.Error("broken file was overwritten")
	}
}

func TestDurationBoundsAffectPlayability(t *testing.T) {
	lib := &fakeLibrary{songs: []navidrome.Song{
		{ID: "short", Title: "S", Artist: "X", Year: 2000, Duration: 30},
		{ID: "long", Title: "L", Artist: "X", Year: 2000, Duration: 900},
		{ID: "ok", Title: "O", Artist: "X", Year: 2000, Duration: 200},
	}}
	c, _ := newTestCollection(t, lib)
	if _, err := c.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.PlayableCount() != 1 || !c.IsPlayable("ok") {
		t.Errorf("playable = %d", c.PlayableCount())
	}
	if stats := c.Stats(); stats.Total != 3 {
		t.Errorf("total = %d; out-of-range songs must stay in the file", stats.Total)
	}
}
