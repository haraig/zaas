package service

import (
	"errors"
	"fmt"
	"math/rand/v2"
)

const maxBound = 1_000_000_000

// ErrMinGreaterThanMax is returned when min > max.
var ErrMinGreaterThanMax = errors.New("min must be <= max")

// ErrBoundsExceeded is returned when min or max exceeds ±1,000,000,000.
var ErrBoundsExceeded = errors.New("bounds exceeded")

// RandomInts returns `count` random integers in [lo, hi]. count must be 1-100.
// lo and hi are capped to ±1,000,000,000.
func RandomInts(lo, hi, count int) ([]int, error) {
	if err := validateNumberParams(lo, hi, count); err != nil {
		return nil, err
	}
	results := make([]int, count)
	for i := range results {
		results[i] = lo + rand.IntN(hi-lo+1)
	}
	return results, nil
}

// RandomFloats returns `count` random float64 values in [lo, hi]. count must be 1-100.
func RandomFloats(lo, hi float64, count int) ([]float64, error) {
	if lo > hi {
		return nil, fmt.Errorf("%w", ErrMinGreaterThanMax)
	}
	if count < 1 || count > 100 {
		return nil, fmt.Errorf("%w: count must be between 1 and 100; got %d", ErrCountOutOfRange, count)
	}
	results := make([]float64, count)
	for i := range results {
		results[i] = lo + rand.Float64()*(hi-lo)
	}
	return results, nil
}

func validateNumberParams(lo, hi, count int) error {
	if lo < -maxBound || hi > maxBound {
		return fmt.Errorf("%w: min and max must be within ±%d", ErrBoundsExceeded, maxBound)
	}
	if lo > hi {
		return fmt.Errorf("%w: got min=%d max=%d", ErrMinGreaterThanMax, lo, hi)
	}
	if count < 1 || count > 100 {
		return fmt.Errorf("%w: count must be between 1 and 100; got %d", ErrCountOutOfRange, count)
	}
	return nil
}
