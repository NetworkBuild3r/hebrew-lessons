//go:build !race

package placement

// Full seed counts. go test -race builds with the race tag and uses
// race_on_test.go instead, so a non-race run still walks every seed.
const (
	simSeedsLevelThree = 200
	simSeedsNoLexeme   = 1000
	simSeedsRightPlace = 300
	simSeedsGuesser    = 400
)
