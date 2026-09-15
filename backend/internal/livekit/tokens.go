// Package livekit creates room-scoped, subscribe-only access tokens. The API
// secret remains server-side; browsers only receive short-lived JWTs.
package livekit

import (
	"time"

	"github.com/livekit/protocol/auth"
)

type TokenIssuer struct {
	apiKey, apiSecret string
	ttl               time.Duration
}

func NewTokenIssuer(apiKey, apiSecret string, ttl time.Duration) *TokenIssuer {
	return &TokenIssuer{apiKey: apiKey, apiSecret: apiSecret, ttl: ttl}
}

func (i *TokenIssuer) Issue(room, identity string) (string, error) {
	canSubscribe, canPublish := true, false
	at := auth.NewAccessToken(i.apiKey, i.apiSecret)
	at.AddGrant(&auth.VideoGrant{
		RoomJoin: true, Room: room, CanSubscribe: &canSubscribe, CanPublish: &canPublish, CanPublishData: &canPublish,
	}).SetIdentity(identity).SetValidFor(i.ttl)
	return at.ToJWT()
}

func RoomName(gameID string) string { return "hitsync-" + gameID }
