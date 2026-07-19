// Package service contains the business logic for each ZaaS randomness endpoint.
package service

import (
	"errors"
	"fmt"
	"math/rand/v2"
)

// ErrInvalidSides is returned when a dice sides value is not in the valid set.
var ErrInvalidSides = errors.New("invalid sides value")

// ErrCountOutOfRange is returned when count is not between 1 and 100.
var ErrCountOutOfRange = errors.New("count out of range")

var validSides = map[int]bool{4: true, 6: true, 8: true, 10: true, 12: true, 20: true, 100: true}

// RollDice rolls `count` dice each with `sides` sides. Returns each roll result.
// sides must be one of 4,6,8,10,12,20,100. count must be 1-100.
func RollDice(sides, count int) ([]int, error) {
	if !validSides[sides] {
		return nil, fmt.Errorf("%w: sides must be one of 4,6,8,10,12,20,100; got %d", ErrInvalidSides, sides)
	}
	if count < 1 || count > 100 {
		return nil, fmt.Errorf("%w: count must be between 1 and 100; got %d", ErrCountOutOfRange, count)
	}
	results := make([]int, count)
	for i := range results {
		results[i] = rand.IntN(sides) + 1
	}
	return results, nil
}
