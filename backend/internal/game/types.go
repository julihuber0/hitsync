// Package game implements Hitsync's pure turn-based rules engine (§8 of the
// spec). It performs no I/O and knows nothing about timers, websockets, or
// track selection — callers (internal/gamesvc) drive phase transitions by
// calling exported methods, including ones representing timeouts.
package game

import (
	"errors"
	"regexp"
	"strings"
)

// Phase is one of the turn states from §8.5.
type Phase string

const (
	PhaseLobby       Phase = "LOBBY"
	PhasePreparing   Phase = "PREPARING"
	PhasePlacing     Phase = "PLACING"
	PhaseChallenging Phase = "CHALLENGING"
	PhaseRevealing   Phase = "REVEALING"
	PhaseGameOver    Phase = "GAME_OVER"
)

// Card is a revealed track (§8.1).
type Card struct {
	TrackID string
	Title   string
	Artist  string
	Year    int
}

// Player is a participant with a timeline and token count (§8.1, §8.3).
type Player struct {
	ID        string
	Name      string
	Colour    string
	Tokens    int
	Timeline  []Card
	Connected bool
	IsHost    bool

	joinOrder int
}

// Settings are the lobby-configurable game settings (§8.3).
type Settings struct {
	TargetCards     int
	StartTokens     int
	MaxTokens       int
	EnableSongGuess bool
}

// Palette is the fixed 12-colour player palette (§8.3).
var Palette = []string{
	"#e8734a", "#3fb984", "#e05263", "#4a90e2", "#f5a623", "#9b59b6",
	"#1abc9c", "#e67e22", "#2ecc71", "#e74c3c", "#3498db", "#f1c40f",
}

// Turn holds the state of the in-progress turn (§8.5).
type Turn struct {
	Number int

	ActivePlayerID string
	Track          Card

	PlacementSubmitted bool
	PlacementSlot      int // -1 means auto-fail (timeout)
	// PlacementPreviewSlot is the active player's revisable, public slot
	// selection before final submission. -2 means none selected.
	PlacementPreviewSlot int

	// Order is the seat order snapshot used to resolve challenges starting
	// from the seat after the active player (§8.8 rule 2).
	Order []string

	// StealClaims are the players who pressed Steal while the steal window
	// was open. The token is spent on claiming; placing has no time limit.
	StealClaims map[string]bool
	// StealWindowClosed is set once the steal window has elapsed; no new
	// claims or passes are accepted afterwards.
	StealWindowClosed bool
	Challenges        map[string]int // playerID -> placed steal slot
	// ChallengePreviews are revisable, public slot selections of claimants
	// who haven't placed yet.
	ChallengePreviews map[string]int
	Passed            map[string]bool
	Spent             map[string]int // playerID -> tokens spent stealing this turn
}

// Reveal is the outcome of resolving a turn (§8.5 REVEALING, §8.8).
type Reveal struct {
	Card             Card
	ActivePlayerID   string
	ActivePlacement  int
	ActiveCorrect    bool
	WinnerPlayerID   string // player who received the card, "" if discarded
	Challenges       []ChallengeOutcome
	TokenChanges     map[string]int // playerID -> delta applied
	SongGuessCorrect bool
	SongGuessAwarded bool
}

// ChallengeOutcome describes one player's challenge result at reveal time.
type ChallengeOutcome struct {
	PlayerID string
	Slot     int
	Correct  bool
}

// Game is the full authoritative state of one match (§8.1).
type Game struct {
	ID         string
	InviteCode string
	Phase      Phase
	Settings   Settings
	HostID     string

	Players         []*Player
	ActivePlayerIdx int

	Turn *Turn

	WinnerID        string
	UsedTrackIDs    map[string]bool
	TracksUsedCount int

	nextJoinOrder     int
	currentTurnNumber int
}

var (
	ErrWrongPhase       = errors.New("wrong phase for this action")
	ErrNotActivePlayer  = errors.New("only the active player may do this")
	ErrIsActivePlayer   = errors.New("the active player may not do this")
	ErrPlayerNotFound   = errors.New("player not found")
	ErrInvalidSlot      = errors.New("invalid slot index")
	ErrSlotTaken        = errors.New("slot already claimed")
	ErrSlotIsActiveSlot = errors.New("cannot challenge the active player's own slot")
	ErrNoTokens         = errors.New("no tokens remaining")
	ErrAlreadyActed     = errors.New("already challenged or passed this turn")
	ErrStealWindowOver  = errors.New("the steal window has closed")
	ErrStealNotClaimed  = errors.New("press steal before placing a steal")
	ErrDuplicateName    = errors.New("display name already in use")
	ErrInvalidName      = errors.New("invalid display name")
	ErrInvalidSettings  = errors.New("invalid game settings")
	ErrPaletteExhausted = errors.New("no colours left in the palette")
	ErrNotHost          = errors.New("host only")
	ErrTooFewPlayers    = errors.New("not enough players")
	ErrTooManyPlayers   = errors.New("game is full")
)

var controlCharPattern = regexp.MustCompile(`[\x00-\x1f\x7f]`)

// ValidateDisplayName trims and validates a display name per §8.3: 2-20
// characters after trimming, no control characters.
func ValidateDisplayName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if len(trimmed) < 2 || len(trimmed) > 20 {
		return "", ErrInvalidName
	}
	if controlCharPattern.MatchString(trimmed) {
		return "", ErrInvalidName
	}
	return trimmed, nil
}
