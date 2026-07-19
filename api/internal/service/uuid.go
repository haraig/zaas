package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ErrInvalidFormat is returned when an unsupported format string is given.
var ErrInvalidFormat = errors.New("invalid format")

var validUUIDFormats = map[string]bool{"standard": true, "no-dashes": true, "urn": true}

// GenerateUUIDs returns `count` UUID v4 strings in the requested format.
// format: "standard" (8-4-4-4-12), "no-dashes" (32 hex chars), "urn" (urn:uuid:...).
// count must be 1-100.
func GenerateUUIDs(count int, format string) ([]string, error) {
	if !validUUIDFormats[format] {
		return nil, fmt.Errorf("%w: format must be one of standard, no-dashes, urn; got %q", ErrInvalidFormat, format)
	}
	if count < 1 || count > 100 {
		return nil, fmt.Errorf("%w: count must be between 1 and 100; got %d", ErrCountOutOfRange, count)
	}
	results := make([]string, count)
	for i := range results {
		u := uuid.New()
		switch format {
		case "standard":
			results[i] = u.String()
		case "no-dashes":
			results[i] = strings.ReplaceAll(u.String(), "-", "")
		case "urn":
			results[i] = "urn:uuid:" + u.String()
		}
	}
	return results, nil
}
