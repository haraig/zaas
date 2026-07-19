package service

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode"

	"zaas/api/internal/service/wordlists"
)

// ErrInvalidWordStyle is returned when an unsupported style is given.
var ErrInvalidWordStyle = errors.New("invalid word style")

// ErrSeparatorTooLong is returned when separator exceeds 5 characters.
var ErrSeparatorTooLong = errors.New("separator too long")

// WordsParams holds the resolved parameters for word generation.
type WordsParams struct {
	Style      string
	Words      int
	Separator  string
	Capitalize bool
	Count      int
}

// RandomWords generates `count` random word strings.
// style: "random", "docker", "ubuntu".
// For "docker" and "ubuntu", the words param is ignored (always 2 words).
// Separator and capitalize override style defaults when explicitly provided.
// count must be 1-100.
func RandomWords(p WordsParams) ([]string, error) {
	if p.Style != "random" && p.Style != "docker" && p.Style != "ubuntu" {
		return nil, fmt.Errorf("%w: style must be one of random, docker, ubuntu; got %q", ErrInvalidWordStyle, p.Style)
	}
	if len(p.Separator) > 5 {
		return nil, fmt.Errorf("%w: separator must be at most 5 characters; got %d", ErrSeparatorTooLong, len(p.Separator))
	}
	if p.Style == "random" && (p.Words < 1 || p.Words > 20) {
		return nil, fmt.Errorf("%w: words must be between 1 and 20; got %d", ErrInvalidParam, p.Words)
	}
	if p.Count < 1 || p.Count > 100 {
		return nil, fmt.Errorf("%w: count must be between 1 and 100; got %d", ErrCountOutOfRange, p.Count)
	}

	results := make([]string, p.Count)
	for i := range results {
		results[i] = generateWordString(p)
	}
	return results, nil
}

func generateWordString(p WordsParams) string {
	var parts []string
	switch p.Style {
	case "docker":
		adj := wordlists.DockerAdjectives[rand.IntN(len(wordlists.DockerAdjectives))]
		name := wordlists.DockerNames[rand.IntN(len(wordlists.DockerNames))]
		parts = []string{adj, name}
	case "ubuntu":
		adj := wordlists.DockerAdjectives[rand.IntN(len(wordlists.DockerAdjectives))]
		animal := wordlists.UbuntuAnimals[rand.IntN(len(wordlists.UbuntuAnimals))]
		parts = []string{adj, animal}
	default:
		parts = make([]string, p.Words)
		for i := range parts {
			parts[i] = wordlists.Random[rand.IntN(len(wordlists.Random))]
		}
	}

	if p.Capitalize {
		for i, w := range parts {
			if len(w) > 0 {
				runes := []rune(w)
				runes[0] = unicode.ToUpper(runes[0])
				parts[i] = string(runes)
			}
		}
	}
	return strings.Join(parts, p.Separator)
}
