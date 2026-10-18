package db

import (
	"fmt"
	"time"
)

// timestampLayout is the UTC text stored in every timestamp column this
// process writes. The fractional part is always nine digits, so the strings
// sort in time order. time.RFC3339Nano omits trailing zeros, and a text
// comparison such as `due <= ?` then disagrees with time order: an exact
// second ("...01Z") sorts after one nanosecond later ("...01.000000001Z").
const timestampLayout = "2006-01-02T15:04:05.000000000Z"

// FormatTime formats t in UTC with timestampLayout.
// Lexical order of the result is time order.
func FormatTime(t time.Time) string {
	return t.UTC().Format(timestampLayout)
}

// ParseTime parses a timestamp written by FormatTime.
// Rows written earlier may be RFC3339 or RFC3339Nano, without a fixed
// fractional width, and those still parse.
func ParseTime(s string) (time.Time, error) {
	for _, layout := range []string{timestampLayout, time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, s)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse time %q", s)
}
