package auth

import (
	"regexp"
	"testing"
	"time"
)

func TestPrepareCodesAreSixDigitsHashedAndDistinct(t *testing.T) {
	clock := func() time.Time { return time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC) }
	o := NewOTP(nil, nil, clock)
	sixDigits := regexp.MustCompile(`^[0-9]{6}$`)
	seen := make(map[string]bool, 20)

	for i := 0; i < 20; i++ {
		rec, code := o.Prepare(1, PurposeRegistration)
		if !sixDigits.MatchString(code) {
			t.Fatalf("code %q is not 6 digits", code)
		}
		if rec.CodeHash == code {
			t.Fatal("OTP stored in plaintext")
		}
		ok, err := VerifySecret(rec.CodeHash, code)
		if err != nil || !ok {
			t.Fatalf("stored hash does not verify its code (ok=%v err=%v)", ok, err)
		}
		if want := clock().Add(OTPTTL); !rec.ExpiresAt.Equal(want) {
			t.Fatalf("expiry = %v, want %v", rec.ExpiresAt, want)
		}
		if seen[code] {
			t.Fatal("duplicate code generated")
		}
		seen[code] = true
	}
}
