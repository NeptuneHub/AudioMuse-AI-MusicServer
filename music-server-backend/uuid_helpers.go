package main

import (
	"errors"
	"math/big"
	"strings"

	"github.com/google/uuid"
)

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GenerateBase62UUID generates a new UUID and encodes it as a base62 string
func GenerateBase62UUID() string {
	id := uuid.New()
	return UUIDToBase62(id)
}

// UUIDToBase62 converts a UUID to a base62 encoded string
func UUIDToBase62(id uuid.UUID) string {
	// Convert UUID bytes to a big integer
	var intValue big.Int
	intValue.SetBytes(id[:])

	// Convert to base62
	return toBase62(&intValue)
}

// Base62ToUUID converts a base62 string back to a UUID
func Base62ToUUID(base62Str string) (uuid.UUID, error) {
	// Convert base62 string to big integer
	intValue, err := fromBase62(base62Str)
	if err != nil {
		return uuid.Nil, err
	}

	// Convert big integer to UUID bytes
	bytes := intValue.Bytes()

	// Pad to 16 bytes if necessary
	var uuidBytes [16]byte
	copy(uuidBytes[16-len(bytes):], bytes)

	return uuid.FromBytes(uuidBytes[:])
}

// toBase62 converts a big integer to base62 string
func toBase62(num *big.Int) string {
	if num.Sign() == 0 {
		return "0"
	}

	var result strings.Builder
	base := big.NewInt(62)
	zero := big.NewInt(0)
	mod := new(big.Int)
	n := new(big.Int).Set(num)

	for n.Cmp(zero) > 0 {
		n.DivMod(n, base, mod)
		result.WriteByte(base62Alphabet[mod.Int64()])
	}

	// Reverse the string
	s := result.String()
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes)
}

// fromBase62 converts a base62 string to a big integer
func fromBase62(s string) (*big.Int, error) {
	result := big.NewInt(0)
	base := big.NewInt(62)

	for _, char := range s {
		result.Mul(result, base)
		idx := strings.IndexRune(base62Alphabet, char)
		if idx == -1 {
			return nil, errors.New("invalid base62 character")
		}
		result.Add(result, big.NewInt(int64(idx)))
	}

	return result, nil
}
