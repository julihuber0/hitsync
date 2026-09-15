// Package broadcast turns a server-cached Navidrome track into one LiveKit
// audio publication. Browsers receive only the SFU's WebRTC stream.
package broadcast

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"

	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v3"
)

type Config struct {
	LiveKitURL, APIKey, APISecret string
	FFmpegPath                    string
}

type session struct {
	trackID string
	room    *lksdk.Room
	writer  *io.PipeWriter
	ctx     context.Context
	cancel  context.CancelFunc
	started bool
}

// Broadcaster owns at most one active broadcast per game room.
type Broadcaster struct {
	cfg      Config
	log      *slog.Logger
	mu       sync.Mutex
	sessions map[string]*session
}

func New(cfg Config, log *slog.Logger) *Broadcaster {
	return &Broadcaster{cfg: cfg, log: log, sessions: make(map[string]*session)}
}

func roomName(gameID string) string { return "hitsync-" + gameID }

// Prepare publishes an empty Opus track. The frontend can therefore connect
// and subscribe during the game PREPARING phase; Start later feeds it audio.
func (b *Broadcaster) Prepare(ctx context.Context, gameID, trackID string) error {
	b.mu.Lock()
	if current := b.sessions[gameID]; current != nil {
		b.mu.Unlock()
		if current.trackID == trackID {
			return nil
		}
		return fmt.Errorf("a different broadcast is still active for game %s", gameID)
	}
	b.mu.Unlock()

	reader, writer := io.Pipe()
	track, err := lksdk.NewLocalReaderTrack(reader, webrtc.MimeTypeOpus)
	if err != nil {
		_ = writer.Close()
		return fmt.Errorf("create LiveKit audio track: %w", err)
	}
	room, err := lksdk.ConnectToRoom(b.cfg.LiveKitURL, lksdk.ConnectInfo{
		APIKey:              b.cfg.APIKey,
		APISecret:           b.cfg.APISecret,
		RoomName:            roomName(gameID),
		ParticipantIdentity: "broadcast",
		ParticipantName:     "Hitsync broadcast",
	}, nil)
	if err != nil {
		_ = writer.Close()
		return fmt.Errorf("connect LiveKit room: %w", err)
	}
	if _, err = room.LocalParticipant.PublishTrack(track, &lksdk.TrackPublicationOptions{Name: "music"}); err != nil {
		_ = writer.Close()
		room.Disconnect()
		return fmt.Errorf("publish LiveKit audio track: %w", err)
	}

	playCtx, cancel := context.WithCancel(context.Background())
	b.mu.Lock()
	if b.sessions[gameID] != nil {
		b.mu.Unlock()
		cancel()
		_ = writer.Close()
		room.Disconnect()
		return fmt.Errorf("broadcast was prepared concurrently for game %s", gameID)
	}
	b.sessions[gameID] = &session{trackID: trackID, room: room, writer: writer, ctx: playCtx, cancel: cancel}
	b.mu.Unlock()
	_ = ctx // Room setup is complete once the track is published.
	return nil
}

// Start starts FFmpeg in real-time mode and copies an Ogg/Opus stream into
// the already-published track. This keeps media timing authoritative on the
// server rather than correcting each browser's MP3 clock.
func (b *Broadcaster) Start(gameID, trackID, sourcePath string) error {
	b.mu.Lock()
	s := b.sessions[gameID]
	if s == nil || s.trackID != trackID {
		b.mu.Unlock()
		return fmt.Errorf("no prepared broadcast for game %s and track %s", gameID, trackID)
	}
	if s.started {
		b.mu.Unlock()
		return nil
	}
	s.started = true
	b.mu.Unlock()

	cmd := exec.CommandContext(s.ctx, b.cfg.FFmpegPath,
		"-nostdin", "-hide_banner", "-loglevel", "warning", "-re", "-i", sourcePath,
		"-vn", "-c:a", "libopus", "-ar", "48000", "-ac", "2", "-page_duration", "20000", "-f", "ogg", "pipe:1")
	cmd.Stdout = s.writer
	if err := cmd.Start(); err != nil {
		b.Stop(gameID, trackID)
		return fmt.Errorf("start ffmpeg: %w", err)
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			b.log.Warn("ffmpeg broadcast ended", "game_id", gameID, "track_id", trackID, "error", err)
		}
		b.Stop(gameID, trackID)
	}()
	return nil
}

// Stop is idempotent and tears down the publisher, ending the SFU track.
func (b *Broadcaster) Stop(gameID, trackID string) {
	b.mu.Lock()
	s := b.sessions[gameID]
	if s == nil || (trackID != "" && s.trackID != trackID) {
		b.mu.Unlock()
		return
	}
	delete(b.sessions, gameID)
	b.mu.Unlock()
	s.cancel()
	_ = s.writer.Close()
	s.room.Disconnect()
}
