// Package media produces the audio files players download for a turn. Each
// Navidrome track is transcoded once by FFmpeg into a constant-bitrate MP3
// with every tag stripped (so the file cannot reveal title, artist, or year)
// and kept in an on-disk LRU cache shared by all games.
package media

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ErrInvalidTrackID is returned for an id that is unsafe to use as a file name.
var ErrInvalidTrackID = errors.New("invalid track id")

// Navidrome ids are hex or base62 strings; anything else never reaches disk.
var trackIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// SourceURLFunc returns an authenticated URL for a track's original file.
type SourceURLFunc func(trackID string) (string, error)

// TranscoderConfig configures a Transcoder.
type TranscoderConfig struct {
	Cache       *Cache
	SourceURL   SourceURLFunc
	FFmpegPath  string
	BitrateKbps int
	// MaxConcurrent bounds simultaneous FFmpeg processes.
	MaxConcurrent int
	// Timeout bounds a single transcode, including the source download.
	Timeout time.Duration
}

// Transcoder de-duplicates concurrent requests for the same track: every
// caller waits on one shared FFmpeg run.
type Transcoder struct {
	cfg   TranscoderConfig
	log   *slog.Logger
	slots chan struct{}

	mu       sync.Mutex
	inFlight map[string]*job
}

type job struct {
	done chan struct{}
	err  error
}

// NewTranscoder creates a Transcoder.
func NewTranscoder(cfg TranscoderConfig, log *slog.Logger) *Transcoder {
	if cfg.MaxConcurrent < 1 {
		cfg.MaxConcurrent = 1
	}
	return &Transcoder{
		cfg:      cfg,
		log:      log,
		slots:    make(chan struct{}, cfg.MaxConcurrent),
		inFlight: map[string]*job{},
	}
}

// Ensure returns the path of the transcoded file for trackID, transcoding it
// first if it is not cached. Cancelling ctx stops waiting but not the
// transcode itself, which other players are likely waiting on too.
func (t *Transcoder) Ensure(ctx context.Context, trackID string) (string, error) {
	if !trackIDPattern.MatchString(trackID) {
		return "", ErrInvalidTrackID
	}
	if t.cfg.Cache.Has(trackID) {
		t.cfg.Cache.Touch(trackID)
		return t.cfg.Cache.Path(trackID), nil
	}

	t.mu.Lock()
	j, ok := t.inFlight[trackID]
	if !ok {
		j = &job{done: make(chan struct{})}
		t.inFlight[trackID] = j
		go t.run(trackID, j)
	}
	t.mu.Unlock()

	select {
	case <-j.done:
		if j.err != nil {
			return "", j.err
		}
		return t.cfg.Cache.Path(trackID), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Warm transcodes trackID in the background so a later download is served
// straight from the cache.
func (t *Transcoder) Warm(trackID string) {
	go func() {
		if _, err := t.Ensure(context.Background(), trackID); err != nil {
			t.log.Warn("failed to pre-transcode track", "track_id", trackID, "error", err)
		}
	}()
}

func (t *Transcoder) run(trackID string, j *job) {
	defer func() {
		t.mu.Lock()
		delete(t.inFlight, trackID)
		t.mu.Unlock()
		close(j.done)
	}()
	// A previous job may have committed the file between Ensure's cache
	// check and this job's registration.
	if t.cfg.Cache.Has(trackID) {
		return
	}

	t.slots <- struct{}{}
	defer func() { <-t.slots }()

	ctx, cancel := context.WithTimeout(context.Background(), t.cfg.Timeout)
	defer cancel()
	started := time.Now()
	j.err = t.transcode(ctx, trackID)
	if j.err == nil {
		t.log.Debug("transcoded track", "track_id", trackID, "took", time.Since(started))
	}
}

func (t *Transcoder) transcode(ctx context.Context, trackID string) error {
	src, err := t.cfg.SourceURL(trackID)
	if err != nil {
		return fmt.Errorf("build source url: %w", err)
	}
	tmp := t.cfg.Cache.TempPath(trackID)
	defer os.Remove(tmp) // no-op once renamed into place

	// FFmpeg reads the source itself rather than from a pipe so it can seek:
	// MP4/M4A files often keep their index at the end of the file.
	cmd := exec.CommandContext(ctx, t.cfg.FFmpegPath,
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-rw_timeout", "30000000", // microseconds without source progress
		"-i", src,
		"-map", "0:a:0", "-vn", "-sn", "-dn",
		"-map_metadata", "-1", "-map_chapters", "-1",
		"-ac", "2", "-ar", "44100",
		"-c:a", "libmp3lame", "-b:a", fmt.Sprintf("%dk", t.cfg.BitrateKbps),
		// No ID3 tags; keep the Xing/LAME header so browsers know the exact
		// duration and trim encoder padding, which keeps loops seamless.
		"-id3v2_version", "0", "-write_id3v1", "0", "-write_xing", "1",
		"-f", "mp3", "-y", tmp,
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// The source URL carries Navidrome credentials; keep it out of logs.
		msg := strings.ReplaceAll(strings.TrimSpace(stderr.String()), src, "<source>")
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return fmt.Errorf("ffmpeg: %w: %s", err, msg)
	}

	info, err := os.Stat(tmp)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return errors.New("ffmpeg produced an empty file")
	}
	if err := os.Rename(tmp, t.cfg.Cache.Path(trackID)); err != nil {
		return err
	}
	t.cfg.Cache.Commit(trackID, info.Size())
	return nil
}
