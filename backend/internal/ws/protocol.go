// Package ws implements the Hitsync WebSocket protocol (§13): envelope
// codec, connection lifecycle, and the clock-sync fast path. Game rules and
// state live in internal/gamesvc; this package only moves bytes and dumbly
// dispatches typed messages to a GameHandler.
package ws

import "encoding/json"

// Envelope is the wire shape used in both directions (§13).
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Client -> server message types.
const (
	TypeHello            = "hello"
	TypePing             = "ping"
	TypeReady            = "ready"
	TypeUpdateSettings   = "update_settings"
	TypeStartGame        = "start_game"
	TypePlaceCard        = "place_card"
	TypeSongGuess        = "song_guess"
	TypePlacePreview     = "place_preview"
	TypeClaimSteal       = "claim_steal"
	TypeChallenge        = "challenge"
	TypeChallengePreview = "challenge_preview"
	TypePassChallenge    = "pass_challenge"
	TypeSkipTrack        = "skip_track"
	TypeKickPlayer       = "kick_player"
	TypeAdjustTokens     = "adjust_tokens"
	TypeEndGame          = "end_game"
	TypePlayAgain        = "play_again"
	TypeLeave            = "leave"
)

// Server -> client message types.
const (
	TypePong         = "pong"
	TypeState        = "state"
	TypeTrackPreload = "track_preload"
	TypeTrackPrepare = "track_prepare"
	TypeTrackStart   = "track_start"
	TypeTrackStop    = "track_stop"
	TypeReveal       = "reveal"
	TypeError        = "error"
	TypeKicked       = "kicked"
)

// HelloPayload authenticates the socket (§13.1).
type HelloPayload struct {
	PlayerToken string `json:"playerToken"`
}

// PingPayload is the client's clock-sync probe (§10.3).
type PingPayload struct {
	C0 int64 `json:"c0"`
}

// PongPayload answers a clock-sync probe.
type PongPayload struct {
	C0 int64 `json:"c0"`
	S  int64 `json:"s"`
}

// ReadyPayload acknowledges during PREPARING that the client has downloaded
// the turn's track and can start playing it.
type ReadyPayload struct {
	PrepareID string `json:"prepareId"`
}

// UpdateSettingsPayload carries lobby setting changes.
type UpdateSettingsPayload struct {
	TargetCards     *int  `json:"targetCards,omitempty"`
	StartTokens     *int  `json:"startTokens,omitempty"`
	MaxTokens       *int  `json:"maxTokens,omitempty"`
	EnableSongGuess *bool `json:"enableSongGuess,omitempty"`
}

// PlaceCardPayload is the active player's placement submission.
type PlaceCardPayload struct {
	SlotIndex int `json:"slotIndex"`
}

// SongGuessPayload is the active player's title/artist guess for the token
// bonus. It replaces any earlier guess this turn and is checked at the
// reveal (§8.6).
type SongGuessPayload struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

// PlacePreviewPayload shares the active player's revisable intended
// placement slot. It never submits; TypePlaceCard is the final submission.
type PlacePreviewPayload struct {
	SlotIndex int `json:"slotIndex"`
}

// ChallengePayload places a claimed steal on a slot during CHALLENGING.
type ChallengePayload struct {
	SlotIndex int `json:"slotIndex"`
}

// ChallengePreviewPayload shares a player's revisable intended steal slot.
// It never spends a token; TypeChallenge is the final submission.
type ChallengePreviewPayload struct {
	SlotIndex int `json:"slotIndex"`
}

// KickPlayerPayload names a player to remove.
type KickPlayerPayload struct {
	PlayerID string `json:"playerId"`
}

// AdjustTokensPayload lets the host manually award or remove a token from a
// player, typically for off-band correct title/artist guesses.
type AdjustTokensPayload struct {
	PlayerID string `json:"playerId"`
	Delta    int    `json:"delta"`
}

// ErrorPayload reports a machine-readable error (§12).
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// KickedPayload explains why a socket was kicked.
type KickedPayload struct {
	Reason string `json:"reason"`
}

// TrackPreloadPayload asks clients to download the next turn's track in
// the background while the current turn is still playing.
type TrackPreloadPayload struct {
	TrackID  string `json:"trackId"`
	MediaURL string `json:"mediaUrl"`
}

// TrackPreparePayload starts a turn's PREPARING phase: clients download the
// track (or reuse their preloaded copy) and answer with TypeReady.
type TrackPreparePayload struct {
	PrepareID  string `json:"prepareId"`
	TrackID    string `json:"trackId"`
	MediaURL   string `json:"mediaUrl"`
	DurationMs int64  `json:"durationMs"`
}

// TrackStartPayload fixes the shared playback start on the server clock.
// Every client plays its local copy at position
// (serverNow - StartAtServerMs) modulo the track length, so clients that
// join or finish downloading late still play in sync.
type TrackStartPayload struct {
	PrepareID       string `json:"prepareId"`
	StartAtServerMs int64  `json:"startAtServerMs"`
}

// TrackStopPayload tells clients to fade out, stop, and discard their copy
// of the track.
type TrackStopPayload struct {
	PrepareID string `json:"prepareId"`
	FadeMs    int    `json:"fadeMs"`
}

func newEnvelope(msgType string, payload any) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{Type: msgType, Payload: raw}, nil
}
