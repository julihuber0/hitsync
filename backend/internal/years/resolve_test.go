package years

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func intp(v int) *int { return &v }

func TestCombine(t *testing.T) {
	cases := []struct {
		name             string
		override, nd, mb *int
		maxBackdate      int
		wantYear         int
		wantSource       string
		wantOK           bool
	}{
		{"override wins", intp(1980), intp(1990), intp(1970), 0, 1980, SourceManual, true},
		{"both present, mb earlier", nil, intp(1990), intp(1970), 0, 1970, SourceMusicBrainz, true},
		{"both present, nd earlier", nil, intp(1970), intp(1990), 0, 1970, SourceLibrary, true},
		{"both present, equal", nil, intp(1975), intp(1975), 0, 1975, SourceBoth, true},
		{"only nd", nil, intp(1975), nil, 0, 1975, SourceLibrary, true},
		{"only mb", nil, nil, intp(1975), 0, 1975, SourceMusicBrainz, true},
		{"neither", nil, nil, nil, 0, 0, "", false},
		{"backdate guard triggers", nil, intp(2000), intp(1950), 10, 2000, SourceLibrary, true},
		{"backdate guard off", nil, intp(2000), intp(1950), 0, 1950, SourceMusicBrainz, true},
		{"backdate guard does not trigger under threshold", nil, intp(2000), intp(1995), 10, 1995, SourceMusicBrainz, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			year, source, ok := Combine(c.override, c.nd, c.mb, c.maxBackdate)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if year != c.wantYear || source != c.wantSource {
				t.Errorf("Combine() = (%d, %q), want (%d, %q)", year, source, c.wantYear, c.wantSource)
			}
		})
	}
}

type fakeMB struct {
	calls int32
	year  int
	found bool
}

func (f *fakeMB) Resolve(ctx context.Context, title, artist string) (*MBResult, error) {
	atomic.AddInt32(&f.calls, 1)
	if !f.found {
		return nil, nil
	}
	return &MBResult{Year: f.year, Source: SourceMusicBrainz}, nil
}

func TestResolverCachesRepeatedLookups(t *testing.T) {
	fake := &fakeMB{year: 1975, found: true}
	r := NewResolver(fake, true, 100, time.Hour)

	for i := 0; i < 5; i++ {
		year, found, err := r.ResolveMusicBrainzYear(context.Background(), "Wish You Were Here", "Pink Floyd")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if !found || year != 1975 {
			t.Fatalf("unexpected result: year=%d found=%v", year, found)
		}
	}
	if atomic.LoadInt32(&fake.calls) != 1 {
		t.Errorf("expected exactly 1 upstream call, got %d", fake.calls)
	}
}

func TestResolverCachesNegativeResults(t *testing.T) {
	fake := &fakeMB{found: false}
	r := NewResolver(fake, true, 100, time.Hour)

	for i := 0; i < 3; i++ {
		_, found, err := r.ResolveMusicBrainzYear(context.Background(), "Unknown Song", "Unknown Artist")
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if found {
			t.Fatal("expected not found")
		}
	}
	if atomic.LoadInt32(&fake.calls) != 1 {
		t.Errorf("expected exactly 1 upstream call for negative caching, got %d", fake.calls)
	}
}

func TestResolverDisabledNeverCallsMusicBrainz(t *testing.T) {
	fake := &fakeMB{year: 1975, found: true}
	r := NewResolver(fake, false, 100, time.Hour)

	_, found, err := r.ResolveMusicBrainzYear(context.Background(), "Wish You Were Here", "Pink Floyd")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if found {
		t.Fatal("expected not found when disabled")
	}
	if atomic.LoadInt32(&fake.calls) != 0 {
		t.Errorf("expected 0 upstream calls when disabled, got %d", fake.calls)
	}
}
