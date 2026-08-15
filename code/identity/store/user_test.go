package store

import "testing"

func TestContains(t *testing.T) {
	if !contains("duplicate key value", "duplicate key") {
		t.Fatal("expected match")
	}
	if contains("unique", "duplicate") {
		t.Fatal("unexpected match")
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if !isUniqueViolation(&testError{msg: "ERROR: 23505 duplicate key"}) {
		t.Fatal("expected unique violation")
	}
	if isUniqueViolation(nil) {
		t.Fatal("nil is not a unique violation")
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
