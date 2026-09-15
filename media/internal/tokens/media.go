// Package tokens verifies the HMAC stream tokens minted by the backend
// (§10.2 of the spec). The media service never mints tokens itself.
package tokens

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Payload is the signed payload embedded in a media stream token.
type Payload struct {
	TrackID string `json:"t"`
	GameID  string `json:"g"`
	Exp     int64  `json:"exp"`
}

// Verifier checks HMAC-signed stream tokens using the shared secret.
type Verifier struct {
	secret []byte
}

// NewVerifier creates a Verifier bound to MEDIA_SHARED_SECRET.
func NewVerifier(secret string) *Verifier {
	return &Verifier{secret: []byte(secret)}
}

// Verify checks a stream token's signature and expiry, and that it matches
// the given track id. Uses constant-time comparison for the signature.
func (v *Verifier) Verify(token, wantTrackID string) (*Payload, error) {
	i := strings.IndexByte(token, '.')
	if i < 0 {
		return nil, errors.New("malformed token")
	}
	encPayload, sig := token[:i], token[i+1:]

	expectedSig := v.sign(encPayload)
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return nil, errors.New("bad signature")
	}

	raw, err := base64.RawURLEncoding.DecodeString(encPayload)
	if err != nil {
		return nil, err
	}
	var payload Payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if payload.TrackID != wantTrackID {
		return nil, errors.New("track id mismatch")
	}
	if time.Now().Unix() > payload.Exp {
		return nil, errors.New("token expired")
	}
	return &payload, nil
}

func (v *Verifier) sign(encPayload string) string {
	h := hmac.New(sha256.New, v.secret)
	h.Write([]byte(encPayload))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
