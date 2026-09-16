package gamesvc

import "time"

// Config holds the gameplay tunables the manager needs, sourced from
// internal/config (§6.6, §6.5).
type Config struct {
	MinPlayers, MaxPlayers, MaxConcurrentGames int
	DefaultTargetCards, DefaultStartTokens     int
	DefaultMaxTokens                           int
	EnableSongGuess                            bool

	TurnPlacementTimeout         time.Duration
	DisconnectedPlacementTimeout time.Duration // fixed 20s per §8.10
	TurnChallengeWindow          time.Duration
	RevealDuration               time.Duration
	PreparingCap                 time.Duration // fixed 8s per §8.5; bounds waiting for clients' downloads
	StartAtLeadMs                int64         // fixed 400ms per §8.5; delay between track_start and the shared start
	PlayerReconnectGrace         time.Duration
	LobbyIdleTimeout             time.Duration
	SkipRateLimit                time.Duration // fixed 10s per §8.11

	AppDomain string
	MediaTTL  time.Duration // lifetime of a media download URL
}
