package utility

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func GenerateRandomFilename(extension string) string {
	bytes := make([]byte, 8) // 8 bytes = 16 hex characters
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s.%s", hex.EncodeToString(bytes), extension)
}
