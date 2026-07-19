package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
)

// ErrInvalidParam is returned for generic invalid parameter values.
var ErrInvalidParam = errors.New("invalid parameter")

// ErrNoCharacterSets is returned when all character set flags are false.
var ErrNoCharacterSets = errors.New("at least one character set must be enabled")

const passwordSymbols = "!@#$%^&*()-_=+"

// GeneratePasswords returns `count` random passwords of the given length.
// At least one of uppercase, lowercase, digits, symbols must be true.
// length must be 8-128, count must be 1-100.
func GeneratePasswords(count, length int, uppercase, lowercase, digits, symbols bool) ([]string, error) {
	if !uppercase && !lowercase && !digits && !symbols {
		return nil, ErrNoCharacterSets
	}
	if length < 8 || length > 128 {
		return nil, fmt.Errorf("%w: length must be between 8 and 128; got %d", ErrInvalidParam, length)
	}
	if count < 1 || count > 100 {
		return nil, fmt.Errorf("%w: count must be between 1 and 100; got %d", ErrCountOutOfRange, count)
	}

	var charset []byte
	if uppercase {
		for c := byte('A'); c <= 'Z'; c++ {
			charset = append(charset, c)
		}
	}
	if lowercase {
		for c := byte('a'); c <= 'z'; c++ {
			charset = append(charset, c)
		}
	}
	if digits {
		for c := byte('0'); c <= '9'; c++ {
			charset = append(charset, c)
		}
	}
	if symbols {
		charset = append(charset, []byte(passwordSymbols)...)
	}

	results := make([]string, count)
	buf := make([]byte, length)
	for i := range results {
		for j := range buf {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			if err != nil {
				return nil, fmt.Errorf("crypto/rand failed: %w", err)
			}
			buf[j] = charset[n.Int64()]
		}
		results[i] = string(buf)
	}
	return results, nil
}
