package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	passwordMemory  = 64 * 1024
	passwordTime    = 3
	passwordThreads = 2
	passwordSaltLen = 16
	passwordKeyLen  = 32
)

// HashPassword returns a salted Argon2id hash using the standard PHC encoding.
func HashPassword(password string) (string, error) {
	if len(password) == 0 || len(password) > 4096 {
		return "", ErrAuthInvalidInput
	}
	salt := make([]byte, passwordSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, passwordTime, passwordMemory, passwordThreads, passwordKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, passwordMemory, passwordTime, passwordThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func verifyPassword(hash, password string) (valid, legacy bool) {
	if legacyPasswordMatches(hash, password) {
		return true, true
	}
	// Accept only the bounded parameters that this application writes. Malformed
	// database values must not choose arbitrary memory or CPU costs.
	if len(hash) > 255 {
		return false, false
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" ||
		parts[2] != fmt.Sprintf("v=%d", argon2.Version) ||
		parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", passwordMemory, passwordTime, passwordThreads) {
		return false, false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != passwordSaltLen {
		return false, false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) != passwordKeyLen {
		return false, false
	}
	key := argon2.IDKey([]byte(password), salt, passwordTime, passwordMemory, passwordThreads, passwordKeyLen)
	return subtle.ConstantTimeCompare(key, expected) == 1, false
}
