//go:build race

package placement

// Seed counts for go test -race. The race detector makes each simulated run
// much slower, and the assertions do not depend on the full non-race counts.
// race_off_test.go keeps 200, 1000, 300, and 400.
const (
	simSeedsLevelThree = 20
	simSeedsNoLexeme   = 100
	simSeedsRightPlace = 30
	simSeedsGuesser    = 40
)
