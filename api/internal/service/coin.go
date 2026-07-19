package service

import "math/rand/v2"

// FlipCoin returns "heads" or "tails" with equal probability.
func FlipCoin() string {
	if rand.IntN(2) == 0 {
		return "heads"
	}
	return "tails"
}
