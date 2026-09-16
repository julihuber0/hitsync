package gamesvc

import (
	"crypto/rand"
	"math/big"
	"time"

	"github.com/google/uuid"
)

func nowMs() int64 {
	return time.Now().UnixMilli()
}

func newID() string {
	return uuid.NewString()
}

// crockfordAlphabet excludes ambiguous glyphs (§8.2).
const crockfordAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// newInviteCode generates a 6-character Crockford base32 code (§8.2).
func newInviteCode() (string, error) {
	b := make([]byte, 6)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(crockfordAlphabet))))
		if err != nil {
			return "", err
		}
		b[i] = crockfordAlphabet[n.Int64()]
	}
	return string(b), nil
}

// normalizeInviteCode uppercases and strips hyphens/spaces from user input
// (§8.2: "Input is case-insensitive and strips hyphens/spaces").
func normalizeInviteCode(s string) string {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r == '-' || r == ' ':
			continue
		case r >= 'a' && r <= 'z':
			out = append(out, byte(r-'a'+'A'))
		default:
			out = append(out, byte(r))
		}
	}
	return string(out)
}
