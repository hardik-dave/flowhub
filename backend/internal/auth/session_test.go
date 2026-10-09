package auth

import "testing"

func TestNewSessionTokenUniqueHashed(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		token, hash, err := NewSessionToken()
		if err != nil {
			t.Fatalf("NewSessionToken: %v", err)
		}
		if len(hash) != 64 {
			t.Fatalf("token_hash len = %d, want 64 (sha256 hex)", len(hash))
		}
		if hash != HashToken(token) {
			t.Fatal("stored hash does not match HashToken(token)")
		}
		if seen[token] {
			t.Fatal("duplicate session token generated")
		}
		seen[token] = true
	}
}
