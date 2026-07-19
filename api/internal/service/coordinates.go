package service

import (
	"fmt"
	"math/rand/v2"
)

// Coordinate represents a geographic coordinate pair.
type Coordinate struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// RandomCoordinates returns `count` random geographic coordinates.
// If landOnly is true, the function makes a best-effort attempt to return
// land coordinates by rejection-sampling against a bounding-box heuristic.
// count must be 1-100.
func RandomCoordinates(count int, landOnly bool) ([]Coordinate, error) {
	if count < 1 || count > 100 {
		return nil, fmt.Errorf("%w: count must be between 1 and 100; got %d", ErrCountOutOfRange, count)
	}
	results := make([]Coordinate, count)
	for i := range results {
		if landOnly {
			results[i] = randomLandCoordinate()
		} else {
			results[i] = randomCoordinate()
		}
	}
	return results, nil
}

func randomCoordinate() Coordinate {
	return Coordinate{
		Lat: rand.Float64()*180 - 90,
		Lon: rand.Float64()*360 - 180,
	}
}

// randomLandCoordinate returns a coordinate inside one of six rough continental
// bounding boxes. The boxes are intentional approximations - they do not trace
// coastlines precisely, so ocean points near coasts are possible. The loop
// structure is a placeholder for future rejection-sampling against a tighter
// land mask; this implementation returns on the first iteration.
func randomLandCoordinate() Coordinate {
	type bbox struct{ latMin, latMax, lonMin, lonMax float64 }
	landBoxes := []bbox{
		{35, 72, -10, 60},    // Europe
		{-35, 38, -18, 52},   // Africa
		{5, 78, 60, 180},     // Asia
		{-45, -10, 110, 155}, // Australia
		{15, 75, -170, -50},  // North America
		{-55, 15, -82, -34},  // South America
	}
	for range 20 {
		box := landBoxes[rand.IntN(len(landBoxes))]
		lat := box.latMin + rand.Float64()*(box.latMax-box.latMin)
		lon := box.lonMin + rand.Float64()*(box.lonMax-box.lonMin)
		return Coordinate{Lat: lat, Lon: lon}
	}
	return randomCoordinate()
}
