package gamesvc

import "time"

// Config holds the gameplay tunables the manager needs, sourced from
// internal/config (§6.6, §6.5).
type Config struct {
	MinPlayers, MaxPlayers, MaxConcurrentGames int
	DefaultTargetCards, DefaultStartTokens     int
	MaxTokens                                  int
	EnableSongGuess                            bool

	TurnPlacementTimeout         time.Duration
	DisconnectedPlacementTimeout time.Duration // fixed 20s per §8.10
	TurnChallengeWindow          time.Duration
	RevealDuration               time.Duration
	PreparingCap                 time.Duration // fixed 8s per §8.5
	StartAtLeadMs                int64         // fixed 400ms per §8.5
	PlayerReconnectGrace         time.Duration
	LobbyIdleTimeout             time.Duration
	SkipRateLimit                time.Duration // fixed 10s per §8.11

	YearLookaheadDepth int
	YearLookupTimeout  time.Duration

	AppDomain       string
	LiveKitURL      string
	LiveKitTokenTTL time.Duration
	MediaTTL        time.Duration
}
