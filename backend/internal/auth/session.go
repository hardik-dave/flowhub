package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// Session audience and lifetime constants (SPEC §3/§7).
const (
	AudienceDashboard = "DASHBOARD"
	AudienceApp       = "APP"

	DashboardTTL = 12 * 60 * 60 // 12h
	AppTTL       = 30 * 24 * 60 * 60
)

// NewSessionToken returns a random 256-bit token (sent to the client
// once) and the sha256-hex digest stored at rest (SPEC §9).
func NewSessionToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken is sha256-hex of the presented bearer token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
