package db

import (
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

// timeLayout is RFC 3339 in UTC at second precision. A fixed width keeps text
// comparison in SQL (e.g. expires_at <= ?) equal to time order.
const timeLayout = "2006-01-02T15:04:05Z"

// FormatTime renders t for storage.
func FormatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

// ParseTime reads a stored timestamp.
func ParseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse stored time %q: %w", s, err)
	}
	return t.UTC(), nil
}

// NewID returns a new ULID: sortable by creation time and safe to expose in URLs.
func NewID() string { return ulid.Make().String() }
