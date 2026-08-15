package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionKey(t *testing.T) {
	assert.Equal(t, "resource:search:version:org-1", versionKey("org-1"))
}
