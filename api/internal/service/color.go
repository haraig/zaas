package service

import (
	"errors"
	"fmt"
	"math/rand/v2"
)

// ErrInvalidColorFormat is returned when an unsupported color format is given.
var ErrInvalidColorFormat = errors.New("invalid color format")

var validColorFormats = map[string]bool{"hex": true, "rgb": true, "hsl": true}

// RandomColors returns `count` random colors in the requested format.
// format: "hex" (#rrggbb), "rgb" (rgb(r, g, b)), "hsl" (hsl(h, s%, l%)).
// count must be 1-100.
func RandomColors(count int, format string) ([]string, error) {
	if !validColorFormats[format] {
		return nil, fmt.Errorf("%w: format must be one of hex, rgb, hsl; got %q", ErrInvalidColorFormat, format)
	}
	if count < 1 || count > 100 {
		return nil, fmt.Errorf("%w: count must be between 1 and 100; got %d", ErrCountOutOfRange, count)
	}
	results := make([]string, count)
	for i := range results {
		r := rand.IntN(256)
		g := rand.IntN(256)
		b := rand.IntN(256)
		switch format {
		case "hex":
			results[i] = fmt.Sprintf("#%02x%02x%02x", r, g, b)
		case "rgb":
			results[i] = fmt.Sprintf("rgb(%d, %d, %d)", r, g, b)
		case "hsl":
			h, s, l := rgbToHSL(r, g, b)
			results[i] = fmt.Sprintf("hsl(%d, %d%%, %d%%)", h, s, l)
		}
	}
	return results, nil
}

// rgbToHSL converts RGB values (0-255) to HSL (hue 0-360, saturation 0-100,
// lightness 0-100) using the standard algorithm.
// Hue is computed from the dominant channel: red → segment 0/6, green → 2,
// blue → 4, each normalized to the 0-1 range before scaling to degrees.
// Saturation uses the formula d / (1 − |2L − 1|) which avoids division by
// zero because we return early for achromatic colors (maxV == minV).
func rgbToHSL(r, g, b int) (int, int, int) {
	rf := float64(r) / 255
	gf := float64(g) / 255
	bf := float64(b) / 255

	maxV := rf
	if gf > maxV {
		maxV = gf
	}
	if bf > maxV {
		maxV = bf
	}
	minV := rf
	if gf < minV {
		minV = gf
	}
	if bf < minV {
		minV = bf
	}

	l := (maxV + minV) / 2
	if maxV == minV {
		return 0, 0, int(l * 100)
	}

	d := maxV - minV
	s := d / (1 - abs64(2*l-1))

	var h float64
	switch maxV {
	case rf:
		h = (gf - bf) / d
		if gf < bf {
			h += 6
		}
	case gf:
		h = (bf-rf)/d + 2
	default:
		h = (rf-gf)/d + 4
	}
	h /= 6

	return int(h * 360), int(s * 100), int(l * 100)
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
