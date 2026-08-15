package util

import "testing"

func TestNewOTP(t *testing.T) {
	otp, err := NewOTP()
	if err != nil {
		t.Fatal(err)
	}
	if len(otp) != 6 {
		t.Fatalf("expected 6 digits, got %q", otp)
	}
}
