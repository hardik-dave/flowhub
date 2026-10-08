// Package verify is the §4 access rule as a pure function: no I/O,
// no clock, no logs. The service loads rows, builds an AccessInput,
// and renders the verdict; the desktop client switches on the exact
// status strings from SPEC.md §6.1 (law — never rename).
package verify

import (
	"fmt"
	"time"

	"github.com/flowos/hub/internal/workingdays"
)

const (
	StatusValid    = "valid"
	StatusPaused   = "paused"
	StatusRevoked  = "revoked"
	StatusNotFound = "not_found"
)

const (
	TenantActive    = "ACTIVE"
	ProductActive   = "ACTIVE"
	UserPending     = "PENDING_VERIFICATION"
	UserActive      = "ACTIVE"
	UserDeactivated = "DEACTIVATED"
	UserBanned      = "BANNED"
	LicenseActive   = "ACTIVE"
	LicenseDisabled = "DISABLED"
)

// Reason and warning templates — EXACT strings from SPEC.md §4.
const (
	ReasonNotFound       = "License not recognised. Check your User ID and license key."
	ReasonAwaiting       = "Account awaiting activation or payment. Contact %s."
	ReasonDeactivated    = "Deactivated by %s: %s"
	ReasonAccessRemoved  = "Access removed by %s. Contact them to restore access."
	ReasonLicenseRemoved = "This product's license was removed by %s."
	ReasonExpired        = "Subscription expired on %s — renew to continue. Contact %s."
	WarningGrace         = "Subscription expired on %s — %d working day(s) of grace remain. Renew to avoid interruption."
)

const dateFmt = "02 Jan 2006"

// AccessInput is everything §4 needs, loaded by the caller in one
// tenant-scoped pass. Status fields carry the raw DB enum values.
type AccessInput struct {
	TenantFound        bool
	TenantName         string
	TenantStatus       string
	ProductFound       bool
	ProductGranted     bool
	ProductStatus      string
	UserFound          bool
	UserStatus         string
	MobileVerified     bool
	LicenseFound       bool
	LicenseStatus      string
	KeyMatches         bool
	ValidUntil         *time.Time
	DeactivationReason string
	GraceWorkingDays   int
}

// Verdict maps 1:1 onto the §6.1 response fields (entitlements and
// the subscription object are assembled by the service from the DB
// rows it already loaded). GraceUntil is nil unless GraceActive.
type Verdict struct {
	Status      string
	Reason      string
	Warning     string
	GraceActive bool
	GraceUntil  *time.Time
}

// Evaluate applies the §4 mapping table. Identification failures
// (not_found) win over everything so a wrong key never discloses that
// an account exists; revoked beats paused when both hold.
func Evaluate(in AccessInput, today time.Time) Verdict {
	if !in.TenantFound || !in.ProductFound || !in.ProductGranted ||
		in.ProductStatus != ProductActive || !in.UserFound ||
		!in.LicenseFound || !in.KeyMatches {
		return Verdict{Status: StatusNotFound, Reason: ReasonNotFound}
	}

	switch {
	case in.TenantStatus != TenantActive || in.UserStatus == UserBanned:
		return Verdict{Status: StatusRevoked, Reason: fmt.Sprintf(ReasonAccessRemoved, in.TenantName)}
	case in.LicenseStatus != LicenseActive:
		return Verdict{Status: StatusRevoked, Reason: fmt.Sprintf(ReasonLicenseRemoved, in.TenantName)}
	case in.UserStatus == UserDeactivated:
		return Verdict{Status: StatusPaused, Reason: fmt.Sprintf(ReasonDeactivated, in.TenantName, in.DeactivationReason)}
	case in.UserStatus != UserActive || !in.MobileVerified:
		return Verdict{Status: StatusPaused, Reason: fmt.Sprintf(ReasonAwaiting, in.TenantName)}
	case in.ValidUntil == nil:
		return Verdict{Status: StatusPaused, Reason: fmt.Sprintf(ReasonAwaiting, in.TenantName)}
	}

	d := dateOnly(today)
	v := dateOnly(*in.ValidUntil)
	graceEnd := workingdays.Add(v, in.GraceWorkingDays)

	switch {
	case !d.After(v):
		return Verdict{Status: StatusValid}
	case !d.After(graceEnd):
		n := workingdays.Count(d, graceEnd)
		until := graceEnd
		return Verdict{
			Status:      StatusValid,
			Warning:     fmt.Sprintf(WarningGrace, v.Format(dateFmt), n),
			GraceActive: true,
			GraceUntil:  &until,
		}
	default:
		return Verdict{
			Status: StatusPaused,
			Reason: fmt.Sprintf(ReasonExpired, v.Format(dateFmt), in.TenantName),
		}
	}
}

func dateOnly(t time.Time) time.Time {
	y, m, day := t.Date()
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}
