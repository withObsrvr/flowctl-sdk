package processor

import (
	"fmt"
	"time"
)

// generateID generates a unique ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}