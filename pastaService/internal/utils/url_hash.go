package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

func Hash(objectName string) string {
	hash := sha256.Sum256([]byte(objectName))
	return hex.EncodeToString(hash[:])
}
