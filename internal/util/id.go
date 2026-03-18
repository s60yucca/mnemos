package util

import (
	"math/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// NewID generates a new ULID string with monotonic entropy
func NewID() string {
	t := time.Now().UTC()
	entropy := ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0) //nolint:gosec
	return ulid.MustNew(ulid.Timestamp(t), entropy).String()
}
