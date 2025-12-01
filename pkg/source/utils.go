package source

import (
	"crypto/rand"
	"encoding/hex"
)

// generateID generates a random ID
func generateID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
