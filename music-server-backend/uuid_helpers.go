package main

import (
	"crypto/md5"
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/google/uuid"
)

const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GenerateArtistID generates a consistent ID for an artist name using MD5 hash
func GenerateArtistID(artistName string) string {
	if artistName == "" {
		return ""
	}
	// Normalize the artist name (trim whitespace, lowercase for consistency)
	normalized := strings.TrimSpace(artistName)

	// Generate MD5 hash
	hasher := md5.New()
	hasher.Write([]byte(normalized))
	hash := hasher.Sum(nil)

	// Return as hex string (32 characters, same format as Navidrome)
	return hex.EncodeToString(hash)
}

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
