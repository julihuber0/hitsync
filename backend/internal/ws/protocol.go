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
	TypeChallenge        = "challenge"
	TypeChallengePreview = "challenge_preview"
	TypePassChallenge    = "pass_challenge"
	TypeSkipTrack        = "skip_track"
	TypeKickPlayer       = "kick_player"
	TypeEndGame          = "end_game"
	TypePlayAgain        = "play_again"
	TypeLeave            = "leave"
)

// Server -> client message types.
const (
	TypePong         = "pong"
	TypeState        = "state"
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

// ReadyPayload acknowledges a LiveKit room subscription during PREPARING.
type ReadyPayload struct {
	PrepareID string `json:"prepareId"`
}

// UpdateSettingsPayload carries lobby setting changes.
type UpdateSettingsPayload struct {
	TargetCards     *int  `json:"targetCards,omitempty"`
	StartTokens     *int  `json:"startTokens,omitempty"`
	EnableSongGuess *bool `json:"enableSongGuess,omitempty"`
}

// PlaceCardPayload is the active player's placement submission.
type PlaceCardPayload struct {
	SlotIndex   int     `json:"slotIndex"`
	TitleGuess  *string `json:"titleGuess,omitempty"`
	ArtistGuess *string `json:"artistGuess,omitempty"`
}

// ChallengePayload claims a slot during CHALLENGING.
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

// ErrorPayload reports a machine-readable error (§12).
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// KickedPayload explains why a socket was kicked.
type KickedPayload struct {
	Reason string `json:"reason"`
}

// TrackPreparePayload gives a player a short-lived, subscribe-only LiveKit
// token. GuessOptions is only populated for the active player's socket.
type TrackPreparePayload struct {
	PrepareID    string        `json:"prepareId"`
	TrackID      string        `json:"trackId"`
	LiveKitURL   string        `json:"livekitUrl"`
	LiveKitToken string        `json:"livekitToken"`
	RoomName     string        `json:"roomName"`
	DurationMs   int64         `json:"durationMs"`
	GuessOptions *GuessOptions `json:"guessOptions,omitempty"`
}

// GuessOptions is the song-guess multiple-choice panel content (§8.6).
type GuessOptions struct {
	Titles  []string `json:"titles"`
	Artists []string `json:"artists"`
}

// TrackStartPayload marks the point where the server begins feeding the
// already-subscribed LiveKit track. It intentionally has no client clock.
type TrackStartPayload struct {
	PrepareID string `json:"prepareId"`
}

// TrackStopPayload tells clients to fade out and stop.
type TrackStopPayload struct {
	FadeMs int `json:"fadeMs"`
}

func newEnvelope(msgType string, payload any) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{Type: msgType, Payload: raw}, nil
}
