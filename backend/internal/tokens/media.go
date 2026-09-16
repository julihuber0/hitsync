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

// MediaPayload is the signed payload of a media download token.
type MediaPayload struct {
	TrackID string `json:"t"`
	GameID  string `json:"g"`
	Exp     int64  `json:"exp"`
}

// MediaSigner mints the short-lived tokens embedded in the media URLs sent to
// players, so a browser can only download tracks its game handed out.
type MediaSigner struct {
	secret []byte
}

// NewMediaSigner creates a MediaSigner. Its HMAC key is derived from secret
// with a fixed label, keeping it independent of the JWT signing key.
func NewMediaSigner(secret string) *MediaSigner {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte("hitsync media token v1"))
	return &MediaSigner{secret: h.Sum(nil)}
}

// Issue mints a media token for (game, track), valid for the given TTL.
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

// Verify checks a media token's signature and expiry and returns its payload.
// Uses constant-time comparison for the signature.
func (m *MediaSigner) Verify(token string) (*MediaPayload, error) {
	encPayload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return nil, errors.New("malformed token")
	}

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
