package fsrs

import (
	"math"

	gofsrs "github.com/open-spaced-repetition/go-fsrs/v3"
)

// curve is the forgetting curve of the scheduler Review builds. It is read from
// go-fsrs's default parameters, so the curve and the scheduler cannot drift apart.
var curve = gofsrs.DefaultParam()

// Retrievability is the chance, from 0 to 1, that a card is still recalled
// elapsedDays after its last review, given its stability in days.
//
// It is the FSRS forgetting curve, R = (1 + factor*t/S)^decay. R is 1 at no
// elapsed time and 0.9 when elapsedDays equals the stability. A stability of 0
// or less means the card was never learned, and gives 0. A negative elapsed time
// counts as none. Nothing is stored: callers compute it when they need it.
func Retrievability(stability, elapsedDays float64) float64 {
	if stability <= 0 || math.IsNaN(stability) || math.IsNaN(elapsedDays) {
		return 0
	}
	if elapsedDays < 0 {
		elapsedDays = 0
	}
	return math.Pow(1+curve.Factor*elapsedDays/stability, curve.Decay)
}
