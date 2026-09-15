package tokens

import (
	"strings"
	"testing"
	"time"
)

func TestMediaTokenRoundTrip(t *testing.T) {
	signer := NewMediaSigner("a-very-secret-key-that-is-32chars!")
	tok, err := signer.Issue("game1", "track1", 30*time.Minute)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	payload, err := signer.Verify(tok, "track1")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if payload.GameID != "game1" || payload.TrackID != "track1" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestMediaTokenTrackMismatch(t *testing.T) {
	signer := NewMediaSigner("a-very-secret-key-that-is-32chars!")
	tok, _ := signer.Issue("game1", "track1", 30*time.Minute)
	if _, err := signer.Verify(tok, "track2"); err == nil {
		t.Fatal("expected error for mismatched track id")
	}
}

func TestMediaTokenTampering(t *testing.T) {
	signer := NewMediaSigner("a-very-secret-key-that-is-32chars!")
	tok, _ := signer.Issue("game1", "track1", 30*time.Minute)

	parts := strings.SplitN(tok, ".", 2)
	tampered := parts[0] + "x." + parts[1]
	if _, err := signer.Verify(tampered, "track1"); err == nil {
		t.Fatal("expected error for tampered payload")
	}

	otherSigner := NewMediaSigner("a-different-secret-key-32-chars!!")
	if _, err := otherSigner.Verify(tok, "track1"); err == nil {
		t.Fatal("expected error for token signed with different secret")
	}
}

func TestMediaTokenExpiry(t *testing.T) {
	signer := NewMediaSigner("a-very-secret-key-that-is-32chars!")
	tok, _ := signer.Issue("game1", "track1", -1*time.Minute)
	if _, err := signer.Verify(tok, "track1"); err == nil {
		t.Fatal("expected error for expired token")
	}
}
