package auth

import "testing"

func TestHashAndVerifySecret(t *testing.T) {
	const secret = "correct horse battery staple"
	hash, err := HashSecret(secret)
	if err != nil {
		t.Fatalf("HashSecret: %v", err)
	}
	if hash == secret {
		t.Fatal("secret stored in plaintext")
	}
	if len(hash) < 40 {
		t.Fatalf("hash looks too short: %q", hash)
	}

	ok, err := VerifySecret(hash, secret)
	if err != nil {
		t.Fatalf("VerifySecret(correct): %v", err)
	}
	if !ok {
		t.Fatal("VerifySecret(correct) = false, want true")
	}

	ok, err = VerifySecret(hash, "wrong password")
	if err != nil {
		t.Fatalf("VerifySecret(wrong): %v", err)
	}
	if ok {
		t.Fatal("VerifySecret(wrong) = true, want false")
	}
}

func TestVerifySecretMalformedHash(t *testing.T) {
	if _, err := VerifySecret("not-an-argon2-hash", "x"); err == nil {
		t.Fatal("VerifySecret(malformed) = nil error, want error")
	}
}
