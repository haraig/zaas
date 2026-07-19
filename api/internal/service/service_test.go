package service_test

import (
	"os"
	"strings"
	"testing"

	"zaas/api/internal/service"
)

// --- Dice ---

func TestRollDice_ValidSides(t *testing.T) {
	t.Parallel()
	for _, sides := range []int{4, 6, 8, 10, 12, 20, 100} {
		result, err := service.RollDice(sides, 1)
		if err != nil {
			t.Fatalf("sides=%d: unexpected error: %v", sides, err)
		}
		if len(result) != 1 {
			t.Fatalf("sides=%d: expected 1 result, got %d", sides, len(result))
		}
		if result[0] < 1 || result[0] > sides {
			t.Errorf("sides=%d: result %d out of range [1,%d]", sides, result[0], sides)
		}
	}
}

func TestRollDice_InvalidSides(t *testing.T) {
	t.Parallel()
	_, err := service.RollDice(7, 1)
	if err == nil {
		t.Error("expected error for sides=7, got nil")
	}
}

func TestRollDice_Count(t *testing.T) {
	t.Parallel()
	results, err := service.RollDice(6, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}
}

func TestRollDice_CountTooHigh(t *testing.T) {
	t.Parallel()
	_, err := service.RollDice(6, 101)
	if err == nil {
		t.Error("expected error for count=101")
	}
}

// --- Coin ---

func TestFlipCoin(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		result := service.FlipCoin()
		if result != "heads" && result != "tails" {
			t.Fatalf("unexpected coin result: %q", result)
		}
		seen[result] = true
	}
	if !seen["heads"] || !seen["tails"] {
		t.Error("after 100 flips, expected both heads and tails")
	}
}

// --- Number ---

func TestRandomInt_Range(t *testing.T) {
	t.Parallel()
	results, err := service.RandomInts(0, 100, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results))
	}
	for _, v := range results {
		if v < 0 || v > 100 {
			t.Errorf("value %d out of range [0,100]", v)
		}
	}
}

func TestRandomInt_MinGreaterThanMax(t *testing.T) {
	t.Parallel()
	_, err := service.RandomInts(100, 0, 1)
	if err == nil {
		t.Error("expected error when min > max")
	}
}

func TestRandomInt_CountTooHigh(t *testing.T) {
	t.Parallel()
	_, err := service.RandomInts(0, 100, 101)
	if err == nil {
		t.Error("expected error for count=101")
	}
}

func TestRandomFloat_Range(t *testing.T) {
	t.Parallel()
	results, err := service.RandomFloats(0, 100, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}
	for _, v := range results {
		if v < 0 || v > 100 {
			t.Errorf("value %f out of range [0,100]", v)
		}
	}
}

func TestRandomInt_BoundsEnforced(t *testing.T) {
	t.Parallel()
	_, err := service.RandomInts(-2_000_000_000, 0, 1)
	if err == nil {
		t.Error("expected error for min below -1_000_000_000")
	}
}

// --- UUID ---

func TestGenerateUUID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		count     int
		format    string
		wantErr   bool
		checkFunc func(t *testing.T, results []string)
	}{
		{
			name:  "standard format has length 36",
			count: 1, format: "standard",
			checkFunc: func(t *testing.T, results []string) {
				t.Helper()
				if len(results[0]) != 36 {
					t.Errorf("standard UUID length: got %d, want 36", len(results[0]))
				}
			},
		},
		{
			name:  "no-dashes format has length 32",
			count: 1, format: "no-dashes",
			checkFunc: func(t *testing.T, results []string) {
				t.Helper()
				if len(results[0]) != 32 {
					t.Errorf("no-dashes UUID length: got %d, want 32", len(results[0]))
				}
			},
		},
		{
			name:  "urn format has urn:uuid: prefix",
			count: 1, format: "urn",
			checkFunc: func(t *testing.T, results []string) {
				t.Helper()
				if len(results[0]) < 9 || results[0][:9] != "urn:uuid:" {
					t.Errorf("urn UUID: got %q, want prefix urn:uuid:", results[0])
				}
			},
		},
		{
			name:  "invalid format returns error",
			count: 1, format: "base64",
			wantErr: true,
		},
		{
			name:  "count=5 returns 5 unique UUIDs",
			count: 5, format: "standard",
			checkFunc: func(t *testing.T, results []string) {
				t.Helper()
				if len(results) != 5 {
					t.Fatalf("expected 5 results, got %d", len(results))
				}
				seen := map[string]bool{}
				for _, u := range results {
					if seen[u] {
						t.Errorf("duplicate UUID: %s", u)
					}
					seen[u] = true
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results, err := service.GenerateUUIDs(tt.count, tt.format)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, results)
			}
		})
	}
}

// --- Color ---

func TestRandomColors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		count     int
		format    string
		wantErr   bool
		checkFunc func(t *testing.T, results []string)
	}{
		{
			name:  "hex format starts with # and has length 7",
			count: 3, format: "hex",
			checkFunc: func(t *testing.T, results []string) {
				t.Helper()
				if len(results) != 3 {
					t.Fatalf("expected 3 results, got %d", len(results))
				}
				for _, c := range results {
					if len(c) != 7 || c[0] != '#' {
						t.Errorf("hex color format wrong: %q", c)
					}
				}
			},
		},
		{
			name:  "rgb format starts with rgb(",
			count: 1, format: "rgb",
			checkFunc: func(t *testing.T, results []string) {
				t.Helper()
				if len(results[0]) < 10 || results[0][:4] != "rgb(" {
					t.Errorf("rgb color format wrong: %q", results[0])
				}
			},
		},
		{
			name:  "hsl format starts with hsl(",
			count: 1, format: "hsl",
			checkFunc: func(t *testing.T, results []string) {
				t.Helper()
				if len(results[0]) < 10 || results[0][:4] != "hsl(" {
					t.Errorf("hsl color format wrong: %q", results[0])
				}
			},
		},
		{
			name:  "invalid format returns error",
			count: 1, format: "cmyk",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results, err := service.RandomColors(tt.count, tt.format)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, results)
			}
		})
	}
}

// --- Coordinates ---

func TestRandomCoordinates_Range(t *testing.T) {
	t.Parallel()
	results, err := service.RandomCoordinates(5, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	for _, c := range results {
		if c.Lat < -90 || c.Lat > 90 {
			t.Errorf("lat %f out of range", c.Lat)
		}
		if c.Lon < -180 || c.Lon > 180 {
			t.Errorf("lon %f out of range", c.Lon)
		}
	}
}

func TestRandomCoordinates_CountLimit(t *testing.T) {
	t.Parallel()
	_, err := service.RandomCoordinates(101, false)
	if err == nil {
		t.Error("expected error for count=101")
	}
}

// --- Password ---

// TestGeneratePasswords_NoCryptoRandRegression asserts that password.go does
// not import math/rand or math/rand/v2, which are not suitable for generating
// security-sensitive passwords.
func TestGeneratePasswords_NoCryptoRandRegression(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("password.go")
	if err != nil {
		t.Fatalf("failed to read password.go: %v", err)
	}
	content := string(src)
	for _, bad := range []string{`"math/rand"`, `"math/rand/v2"`} {
		if strings.Contains(content, bad) {
			t.Errorf("password.go must not import %s; use crypto/rand instead", bad)
		}
	}
}

func TestGeneratePasswords_ReturnsExpectedCount(t *testing.T) {
	t.Parallel()
	results, err := service.GeneratePasswords(5, 16, true, true, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 passwords, got %d", len(results))
	}
}

func TestGeneratePasswords_LengthEnforced(t *testing.T) {
	t.Parallel()
	results, err := service.GeneratePasswords(1, 24, true, true, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results[0]) != 24 {
		t.Errorf("expected length 24, got %d", len(results[0]))
	}
}

func TestGeneratePasswords_NoCharacterSets_Error(t *testing.T) {
	t.Parallel()
	_, err := service.GeneratePasswords(1, 16, false, false, false, false)
	if err == nil {
		t.Error("expected error when no character sets selected")
	}
}

func TestGeneratePasswords_LengthTooShort_Error(t *testing.T) {
	t.Parallel()
	_, err := service.GeneratePasswords(1, 7, true, true, true, false)
	if err == nil {
		t.Error("expected error for length=7")
	}
}

func TestGeneratePasswords_LengthTooLong_Error(t *testing.T) {
	t.Parallel()
	_, err := service.GeneratePasswords(1, 129, true, true, true, false)
	if err == nil {
		t.Error("expected error for length=129")
	}
}

func TestGeneratePasswords_UppercaseOnly(t *testing.T) {
	t.Parallel()
	results, err := service.GeneratePasswords(1, 12, true, false, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, ch := range results[0] {
		if ch < 'A' || ch > 'Z' {
			t.Errorf("expected uppercase only, got char %q", ch)
		}
	}
}

func TestGeneratePasswords_SymbolsOnly(t *testing.T) {
	t.Parallel()
	symbols := "!@#$%^&*()-_=+"
	results, err := service.GeneratePasswords(1, 10, false, false, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, ch := range results[0] {
		found := false
		for _, s := range symbols {
			if ch == s {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("unexpected char %q in symbols-only password", ch)
		}
	}
}
