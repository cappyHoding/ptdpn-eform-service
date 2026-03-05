// Package crypto provides cryptographic utilities:
//   - Bcrypt password hashing for internal staff passwords
//   - SHA-256 token hashing for customer session lookup
//
// WHY TWO DIFFERENT HASHING STRATEGIES?
//
//	Bcrypt (passwords): Designed to be SLOW. Includes a salt automatically.
//	                    Makes brute-force attacks computationally expensive.
//	                    You cannot reverse it — only verify.
//
//	SHA-256 (tokens):   Designed to be FAST. The token itself is already
//	                    a long, random, high-entropy value so we don't need
//	                    the slowness of bcrypt. We store the hash so that
//	                    even if the DB is compromised, raw tokens are safe.
package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is the work factor for bcrypt.
	// 12 is a good balance: ~250ms per hash on modern hardware.
	// High enough to deter brute force; low enough not to slow down login.
	BcryptCost = 12
)

// HashPassword hashes a plaintext password using bcrypt.
// Store the returned hash in the database. Never store the plaintext.
//
// Returns an error if hashing fails (very rare, usually indicates system issue).
func HashPassword(plaintext string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword verifies a plaintext password against a stored bcrypt hash.
// Returns nil if the password matches, error otherwise.
//
// TIMING SAFETY: bcrypt.CompareHashAndPassword uses constant-time comparison
// internally, so it's safe against timing attacks.
func CheckPassword(plaintext, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
}

// GenerateSecureToken creates a cryptographically secure random token.
// Used for customer session tokens.
//
// The token is base64url-encoded for safe use in URLs and HTTP headers.
// Length: byteLength bytes of randomness → encoded to a longer string.
// Recommended: 32 bytes = 256 bits of entropy = extremely secure.
func GenerateSecureToken(byteLength int) (string, error) {
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate secure token: %w", err)
	}
	// base64.URLEncoding is safe for use in URLs (no +, /, = characters)
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// HashToken creates a SHA-256 hash of a token for database storage.
// Store this hash in token_hash column, never the raw token itself.
//
// The raw token is returned to the customer; we only ever store/compare the hash.
func HashToken(rawToken string) string {
	hash := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(hash[:])
}
