// Package auth owns the credential machinery: argon2id hashing for
// passwords AND license keys/OTP codes (SPEC §9), sha256 session
// tokens, the OTP lifecycle, the SMS transport, and the bearer-token
// middleware. Nothing here ever logs a secret (AGENTS.md rule 8).
package auth

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/alexedwards/argon2id"
)

// HashParams are the argon2id costs used for every secret we store.
// Fixed (not runtime.NumCPU()) so hashes are portable and reviewable;
// chosen to the OWASP argon2id floor. Recorded in DECISIONS.md.
var HashParams = &argon2id.Params{
	Memory:      64 * 1024,
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// HashSecret returns an encoded argon2id hash ($argon2id$v=19$…).
func HashSecret(secret string) (string, error) {
	return argon2id.CreateHash(secret, HashParams)
}

// VerifySecret performs a constant-time comparison against a stored
// hash. A malformed stored hash returns (false, err), never a panic.
func VerifySecret(encoded, secret string) (bool, error) {
	return argon2id.ComparePasswordAndHash(secret, encoded)
}

// RandomPassword returns a 16-char URL-safe random password shown once
// to an admin who creates a user or onboards a tenant (SPEC §7/§8).
func RandomPassword() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
