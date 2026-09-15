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

// MediaPayload is the signed payload embedded in a private broadcast-control token.
type MediaPayload struct {
	TrackID string `json:"t"`
	GameID  string `json:"g"`
	Exp     int64  `json:"exp"`
}

// MediaSigner mints HMAC-signed control tokens shared with the media worker.
type MediaSigner struct {
	secret []byte
}

// NewMediaSigner creates a MediaSigner bound to MEDIA_SHARED_SECRET.
func NewMediaSigner(secret string) *MediaSigner {
	return &MediaSigner{secret: []byte(secret)}
}

// Issue mints a stream token for (game, track), valid for the given TTL.
func (m *MediaSigner) Issue(gameID, trackID string, ttl time.Duration) (string, error) {
	payload := MediaPayload{
		TrackID: trackID,
		GameID:  gameID,
		Exp:     time.Now().Add(ttl).Unix(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encPayload := base64.RawURLEncoding.EncodeToString(raw)
	sig := m.sign(encPayload)
	return encPayload + "." + sig, nil
}

// Verify checks a stream token's signature and expiry, and that it matches
// the given track id. Uses constant-time comparison for the signature.
func (m *MediaSigner) Verify(token, wantTrackID string) (*MediaPayload, error) {
	i := strings.IndexByte(token, '.')
	if i < 0 {
		return nil, errors.New("malformed token")
	}
	encPayload, sig := token[:i], token[i+1:]

	expectedSig := m.sign(encPayload)
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return nil, errors.New("bad signature")
	}

	raw, err := base64.RawURLEncoding.DecodeString(encPayload)
	if err != nil {
		return nil, err
	}
	var payload MediaPayload
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

func (m *MediaSigner) sign(encPayload string) string {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(encPayload))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
