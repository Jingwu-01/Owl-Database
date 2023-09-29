package authentication

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type SessionInfo struct {
	Username  string
	ExpiresAt time.Time
}

// generateToken is a helper function that generates a secure, random token
func generateToken() (string, error) {
	// 128 bits
	token := make([]byte, 16)
	// Fill the slide with cryptographically secure random bytes
	_, err := rand.Read(token)
	if err != nil {
		return "", err
	}
	// Convert the random bytes to a hexadecimal string
	return hex.EncodeToString(token), nil
}
