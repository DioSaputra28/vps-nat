package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func GenerateSessionToken() (plain string, hashed string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}

	plain = hex.EncodeToString(buf)
	sum := sha256.Sum256([]byte(plain))
	hashed = hex.EncodeToString(sum[:])

	return plain, hashed, nil
}

func HashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
