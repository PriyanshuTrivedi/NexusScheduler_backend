package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNullIfZero(t *testing.T) {
	assert.Nil(t, nullIfZero(time.Time{}))
	now := time.Unix(100, 0)
	got := nullIfZero(now)
	if assert.NotNil(t, got) {
		assert.Equal(t, now, *got)
	}
}

func TestContains(t *testing.T) {
	assert.True(t, contains("duplicate key value", "duplicate key"))
	assert.False(t, contains("unique", "duplicate"))
}

func TestIsUniqueViolation(t *testing.T) {
	assert.True(t, isUniqueViolation(&testError{msg: "ERROR: 23505 duplicate key"}))
	assert.False(t, isUniqueViolation(nil))
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
