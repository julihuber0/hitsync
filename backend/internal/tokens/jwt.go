// Package tokens issues and verifies the JWTs used for the app-access gate,
// admin sessions, and player identity, plus the HMAC media stream tokens
// shared with the media service.
package tokens

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken is returned for any malformed, expired, or wrong-scope token.
var ErrInvalidToken = errors.New("invalid token")

// Issuer signs and verifies HS256 JWTs using a single shared secret.
type Issuer struct {
	secret []byte
}

// NewIssuer creates an Issuer bound to the given secret.
func NewIssuer(secret string) *Issuer {
	return &Issuer{secret: []byte(secret)}
}

// ScopeClaims backs the app-access and admin session cookies.
type ScopeClaims struct {
	Scope string `json:"scope"`
	jwt.RegisteredClaims
}

// IssueScope issues a JWT asserting the given scope ("app" or "admin").
func (i *Issuer) IssueScope(scope string, ttl time.Duration) (string, error) {
	claims := ScopeClaims{
		Scope: scope,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
}

// VerifyScope checks a scope token and returns its scope.
func (i *Issuer) VerifyScope(tokenStr string, wantScope string) (*ScopeClaims, error) {
	claims := &ScopeClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, i.keyFunc)
	if err != nil || !tok.Valid {
		return nil, ErrInvalidToken
	}
	if claims.Scope != wantScope {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// PlayerClaims backs the per-player reconnection token.
type PlayerClaims struct {
	GameID   string `json:"gameId"`
	PlayerID string `json:"playerId"`
	jwt.RegisteredClaims
}

// IssuePlayer issues a player token for the given game/player pair.
func (i *Issuer) IssuePlayer(gameID, playerID string, ttl time.Duration) (string, error) {
	claims := PlayerClaims{
		GameID:   gameID,
		PlayerID: playerID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
}

// VerifyPlayer checks a player token and returns its claims.
func (i *Issuer) VerifyPlayer(tokenStr string) (*PlayerClaims, error) {
	claims := &PlayerClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, i.keyFunc)
	if err != nil || !tok.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (i *Issuer) keyFunc(tok *jwt.Token) (interface{}, error) {
	if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, ErrInvalidToken
	}
	return i.secret, nil
}
