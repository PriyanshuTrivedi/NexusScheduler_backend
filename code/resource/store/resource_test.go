package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNullIfZero(t *testing.T) {
	t.Run("zero time returns nil", func(t *testing.T) {
		assert.Nil(t, nullIfZero(time.Time{}))
	})

	t.Run("non-zero time returns pointer", func(t *testing.T) {
		now := time.Unix(100, 0)

		got := nullIfZero(now)

		if assert.NotNil(t, got) {
			assert.Equal(t, now, *got)
		}
	})
}

func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{
			name:   "substring exists",
			s:      "duplicate key value",
			substr: "duplicate key",
			want:   true,
		},
		{
			name:   "substring does not exist",
			s:      "unique",
			substr: "duplicate",
			want:   false,
		},
		{
			name:   "exact match",
			s:      "duplicate key",
			substr: "duplicate key",
			want:   true,
		},
		{
			name:   "substring at beginning",
			s:      "duplicate key value",
			substr: "duplicate",
			want:   true,
		},
		{
			name:   "substring at end",
			s:      "duplicate key",
			substr: "key",
			want:   true,
		},
		{
			name:   "empty substring",
			s:      "abc",
			substr: "",
			want:   true,
		},
		{
			name:   "empty string",
			s:      "",
			substr: "abc",
			want:   false,
		},
		{
			name:   "both empty",
			s:      "",
			substr: "",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, contains(tt.s, tt.substr))
		})
	}
}

func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "postgres unique violation",
			err:  &testError{msg: "ERROR: 23505 duplicate key"},
			want: true,
		},
		{
			name: "duplicate key error",
			err:  &testError{msg: "duplicate key value violates unique constraint"},
			want: true,
		},
		{
			name: "unrelated error",
			err:  &testError{msg: "connection failed"},
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "23505 without duplicate key text",
			err:  &testError{msg: "ERROR: 23505"},
			want: true,
		},
		{
			name: "duplicate key without postgres code",
			err:  &testError{msg: "duplicate key"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isUniqueViolation(tt.err))
		})
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
