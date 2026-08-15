package util

import "testing"

func TestPasswordHashAndCheck(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "correct-password") {
		t.Fatal("password should match")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("wrong password matched")
	}
}
