package idgen

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateId creates a random ID with the given prefix and length
func GenerateId(prefix string, length int) (string, error) {
	// Calculate how many random bytes we need
	randomBytesNeeded := (length * 3) / 4 + 1
	
	// Generate random bytes
	randomBytes := make([]byte, randomBytesNeeded)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	
	// Encode to base64
	encoded := base64.RawURLEncoding.EncodeToString(randomBytes)
	
	// Trim to desired length and add prefix
	if len(encoded) > length {
		encoded = encoded[:length]
	}
	
	return prefix + encoded, nil
}
