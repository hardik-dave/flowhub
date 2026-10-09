package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/flowos/hub/internal/store"
)

// §6.4 lifecycle constants: 6 digits, 5-minute expiry, 5 attempts,
// 60-second resend cooldown.
const (
	OTPTTL         = 5 * time.Minute
	OTPCooldown    = 60 * time.Second
	OTPMaxAttempts = 5

	PurposeRegistration = "REGISTRATION"
	PurposeFirstLogin   = "FIRST_LOGIN"
)

var (
	// ErrCooldown means a code was requested again inside the 60s window.
	ErrCooldown = errors.New("auth: otp cooldown")
	// ErrOTPInvalid covers wrong, expired, consumed, or over-attempted.
	ErrOTPInvalid = errors.New("auth: otp invalid or expired")
)

// OTP issues and checks one-time codes. Codes are argon2id-hashed at
// rest and never logged; delivery goes through SmsSender.
type OTP struct {
	store  *store.Store
	sender SmsSender
	now    func() time.Time
}

func NewOTP(st *store.Store, sender SmsSender, now func() time.Time) *OTP {
	if now == nil {
		now = time.Now
	}
	return &OTP{store: st, sender: sender, now: now}
}

// Prepare builds an unsaved OTP row plus the plaintext code. The caller
// inserts the row inside its own transaction; Deliver sends the code.
func (o *OTP) Prepare(userID int64, purpose string) (*store.OTPCode, string) {
	code := randomCode()
	hash, err := HashSecret(code)
	if err != nil {
		// HashSecret only fails if the system CSPRNG fails.
		panic("auth: hash otp: " + err.Error())
	}
	return &store.OTPCode{
		UserID:    userID,
		Purpose:   purpose,
		CodeHash:  hash,
		ExpiresAt: o.now().UTC().Add(OTPTTL),
	}, code
}

// Deliver hands the code to the SMS transport. Errors are logged
// WITHOUT the code (rule 8) and never fail the caller.
func (o *OTP) Deliver(mobile, code string) {
	if o.sender == nil {
		return
	}
	if err := o.sender.SendOTP(mobile, code); err != nil {
		slog.Error("otp delivery failed", "err", err)
	}
}

// Issue checks the cooldown, stores a fresh code, and sends it.
func (o *OTP) Issue(ctx context.Context, userID int64, mobile, purpose string) (expiresInSeconds int, err error) {
	last, err := o.store.LatestOTP(ctx, userID, purpose)
	switch {
	case err == nil:
		if o.now().Sub(last.CreatedAt) < OTPCooldown {
			return 0, ErrCooldown
		}
	case errors.Is(err, store.ErrNotFound):
	default:
		return 0, err
	}

	rec, code := o.Prepare(userID, purpose)
	if err := o.store.InTx(ctx, func(tx store.DBTX) error {
		_, e := o.store.InsertOTP(ctx, tx, rec)
		return e
	}); err != nil {
		return 0, err
	}
	o.Deliver(mobile, code)
	return int(OTPTTL / time.Second), nil
}

// Check validates the presented code against the newest OTP row for
// user/purpose. On a wrong code it increments the attempt counter. It
// returns the row so the caller can consume it inside a transaction
// that also writes the audit row.
func (o *OTP) Check(ctx context.Context, userID int64, purpose, code string) (*store.OTPCode, error) {
	rec, err := o.store.LatestOTP(ctx, userID, purpose)
	if err != nil {
		return nil, ErrOTPInvalid
	}
	if rec.ConsumedAt != nil || rec.Attempts >= OTPMaxAttempts || o.now().After(rec.ExpiresAt) {
		return nil, ErrOTPInvalid
	}
	match, err := VerifySecret(rec.CodeHash, code)
	if err != nil || !match {
		_ = o.store.IncrementOTPAttempts(ctx, rec.ID)
		return nil, ErrOTPInvalid
	}
	return rec, nil
}

func randomCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		panic("auth: otp rng: " + err.Error())
	}
	return fmt.Sprintf("%06d", n.Int64())
}
