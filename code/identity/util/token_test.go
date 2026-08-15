package util

import "testing"

func TestInviteTokenHash(t *testing.T) {
	raw, hash, err := NewInviteToken()
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || hash == "" {
		t.Fatal("token should not be empty")
	}
	if HashToken(raw) != hash {
		t.Fatal("token hash mismatch")
	}
}
