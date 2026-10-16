package rank

import (
	"math"
	"time"
)

// halfLife is how long a use stays worth half as much: a thing used every day stays on top, one not
// touched for a couple of months fades.
const halfLife = 14 * 24 * time.Hour

// Frecency is how much a thing's history says it matters: each use counts 1 when it just happened and
// half as much for every half-life since.
func Frecency(uses []time.Time, now time.Time) float64 {
	var total float64
	for _, at := range uses {
		age := max(now.Sub(at), 0)
		total += math.Exp2(-float64(age) / float64(halfLife))
	}
	return total
}

// Combine lifts a match score by frecency: the first uses count a lot and later ones less, so a
// favourite beats an equal match without a thing used thousands of times burying every new one.
func Combine(match, frecency float64) float64 {
	return match * (1 + math.Log1p(frecency))
}
