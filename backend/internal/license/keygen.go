// Package license generates license keys per SPEC.md §5:
// {product.key_prefix}-XXXX-XXXX-XXXX-XXXX, Crockford base32,
// 80 bits of crypto randomness. The generated key is shown ONCE;
// only its argon2id hash and last-4 hint are stored.
package license

import (
	"crypto/rand"
	"strings"
)

// Crockford base32: no I, L, O, U — unambiguous over the phone,
// which is exactly how support will read these out.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewKey returns e.g. "FL-9F4K-22XQ-7T1B-M2A8" for prefix "FL".
func NewKey(prefix string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	chars := make([]byte, 16)
	for i, v := range b {
		chars[i] = crockford[int(v)%32]
	}
	groups := []string{prefix, string(chars[0:4]), string(chars[4:8]), string(chars[8:12]), string(chars[12:16])}
	return strings.Join(groups, "-"), nil
}

// Hint returns the last 4 characters for support conversations.
func Hint(key string) string {
	if len(key) < 4 {
		return key
	}
	return key[len(key)-4:]
}
