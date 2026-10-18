package path

import "github.com/NetworkBuild3r/hebrew-lessons/go/internal/textkey"

// ExactKey is textkey.ExactKey. The path word line uses the same string
// key as the library and the plan checkers.
func ExactKey(s string) string {
	return textkey.ExactKey(s)
}
