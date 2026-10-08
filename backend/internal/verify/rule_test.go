package verify

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func tp(t time.Time) *time.Time { return &t }

// base is an all-checks-pass input whose subscription sits in the
// §6.1 example: valid_until 12 Sep 2026 (Sat), grace_working_days 4
// → grace ends Thu 17 Sep 2026, exactly the example's until field.
func base() AccessInput {
	return AccessInput{
		TenantFound:      true,
		TenantName:       "Alpha Broker",
		TenantStatus:     TenantActive,
		ProductFound:     true,
		ProductGranted:   true,
		ProductStatus:    ProductActive,
		UserFound:        true,
		UserStatus:       UserActive,
		MobileVerified:   true,
		LicenseFound:     true,
		LicenseStatus:    LicenseActive,
		KeyMatches:       true,
		ValidUntil:       tp(day(2026, time.September, 12)),
		GraceWorkingDays: 4,
	}
}

const (
	revokedAccess  = "Access removed by Alpha Broker. Contact them to restore access."
	revokedLicense = "This product's license was removed by Alpha Broker."
	pausedAwaiting = "Account awaiting activation or payment. Contact Alpha Broker."
	notFoundReason = "License not recognised. Check your User ID and license key."
	graceWarning   = "Subscription expired on 12 Sep 2026 — 3 working day(s) of grace remain. Renew to avoid interruption."
	expiredReason  = "Subscription expired on 12 Sep 2026 — renew to continue. Contact Alpha Broker."
)

// TestEvaluateTable is the M-B acceptance suite: one case per row of
// the SPEC.md §4 mapping table (plus the product/edge cases §10 M-B
// demands), asserting EXACT status strings and reason templates.
func TestEvaluateTable(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*AccessInput)
		today  time.Time
		want   Verdict
	}{
		{
			name:   "§4 row 1: all pass, subscription current",
			mutate: func(in *AccessInput) {},
			today:  day(2026, time.September, 11),
			want:   Verdict{Status: StatusValid},
		},
		{
			name:   "§4 row 2: in grace — §6.1 example verbatim",
			mutate: func(in *AccessInput) {},
			today:  day(2026, time.September, 14),
			want: Verdict{
				Status:      StatusValid,
				Warning:     graceWarning,
				GraceActive: true,
				GraceUntil:  tp(day(2026, time.September, 17)),
			},
		},
		{
			name:   "§4 row 3a: grace exhausted → paused (expired)",
			mutate: func(in *AccessInput) {},
			today:  day(2026, time.September, 18),
			want:   Verdict{Status: StatusPaused, Reason: expiredReason},
		},
		{
			name: "§4 row 3b: never paid → paused (awaiting)",
			mutate: func(in *AccessInput) {
				in.ValidUntil = nil
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusPaused, Reason: pausedAwaiting},
		},
		{
			name: "§4 row 4: user DEACTIVATED → paused with audit reason",
			mutate: func(in *AccessInput) {
				in.UserStatus = UserDeactivated
				in.DeactivationReason = "payment overdue"
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusPaused, Reason: "Deactivated by Alpha Broker: payment overdue"},
		},
		{
			name: "§4 row 5a: user BANNED → revoked",
			mutate: func(in *AccessInput) {
				in.UserStatus = UserBanned
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusRevoked, Reason: revokedAccess},
		},
		{
			name: "§4 row 5b: tenant INACTIVE → revoked",
			mutate: func(in *AccessInput) {
				in.TenantStatus = "INACTIVE"
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusRevoked, Reason: revokedAccess},
		},
		{
			name: "§4 row 6: license DISABLED → revoked (this product)",
			mutate: func(in *AccessInput) {
				in.LicenseStatus = LicenseDisabled
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusRevoked, Reason: revokedLicense},
		},
		{
			name: "§4 row 7a: no such user → not_found",
			mutate: func(in *AccessInput) {
				in.UserFound = false
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusNotFound, Reason: notFoundReason},
		},
		{
			name: "§4 row 7b: wrong key → not_found",
			mutate: func(in *AccessInput) {
				in.KeyMatches = false
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusNotFound, Reason: notFoundReason},
		},
		{
			name: "§4 row 7c: no license for this product → not_found",
			mutate: func(in *AccessInput) {
				in.LicenseFound = false
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusNotFound, Reason: notFoundReason},
		},
		{
			name: "§4 row 7d: product not granted to tenant → not_found",
			mutate: func(in *AccessInput) {
				in.ProductGranted = false
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusNotFound, Reason: notFoundReason},
		},
		{
			name: "tenant not found → not_found",
			mutate: func(in *AccessInput) {
				in.TenantFound = false
				in.TenantName = ""
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusNotFound, Reason: notFoundReason},
		},
		{
			name: "product not found → not_found",
			mutate: func(in *AccessInput) {
				in.ProductFound = false
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusNotFound, Reason: notFoundReason},
		},
		{
			name: "product RETIRED → not_found (DECISIONS.md)",
			mutate: func(in *AccessInput) {
				in.ProductStatus = "RETIRED"
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusNotFound, Reason: notFoundReason},
		},
		{
			name: "user PENDING_VERIFICATION → paused (awaiting)",
			mutate: func(in *AccessInput) {
				in.UserStatus = UserPending
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusPaused, Reason: pausedAwaiting},
		},
		{
			name: "mobile unverified → paused (awaiting)",
			mutate: func(in *AccessInput) {
				in.MobileVerified = false
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusPaused, Reason: pausedAwaiting},
		},
		{
			name:   "boundary: today == valid_until is current",
			mutate: func(in *AccessInput) {},
			today:  day(2026, time.September, 12),
			want:   Verdict{Status: StatusValid},
		},
		{
			name:   "boundary: today == grace end, 0 days remain",
			mutate: func(in *AccessInput) {},
			today:  day(2026, time.September, 17),
			want: Verdict{
				Status:      StatusValid,
				Warning:     "Subscription expired on 12 Sep 2026 — 0 working day(s) of grace remain. Renew to avoid interruption.",
				GraceActive: true,
				GraceUntil:  tp(day(2026, time.September, 17)),
			},
		},
		{
			name: "default grace_working_days = 5 → 4 days, until 18 Sep",
			mutate: func(in *AccessInput) {
				in.GraceWorkingDays = 5
			},
			today: day(2026, time.September, 14),
			want: Verdict{
				Status:      StatusValid,
				Warning:     "Subscription expired on 12 Sep 2026 — 4 working day(s) of grace remain. Renew to avoid interruption.",
				GraceActive: true,
				GraceUntil:  tp(day(2026, time.September, 18)),
			},
		},
		{
			name: "revoked beats paused: DEACTIVATED + license DISABLED",
			mutate: func(in *AccessInput) {
				in.UserStatus = UserDeactivated
				in.DeactivationReason = "payment overdue"
				in.LicenseStatus = LicenseDisabled
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusRevoked, Reason: revokedLicense},
		},
		{
			name: "not_found beats revoked: wrong key + tenant INACTIVE",
			mutate: func(in *AccessInput) {
				in.KeyMatches = false
				in.TenantStatus = "INACTIVE"
			},
			today: day(2026, time.September, 14),
			want:  Verdict{Status: StatusNotFound, Reason: notFoundReason},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := base()
			c.mutate(&in)
			got := Evaluate(in, c.today)
			if got.Status != c.want.Status || got.Reason != c.want.Reason ||
				got.Warning != c.want.Warning || got.GraceActive != c.want.GraceActive {
				t.Errorf("Evaluate() =\n status %q\n reason %q\n warning %q\n grace_active %v\nwant:\n status %q\n reason %q\n warning %q\n grace_active %v",
					got.Status, got.Reason, got.Warning, got.GraceActive,
					c.want.Status, c.want.Reason, c.want.Warning, c.want.GraceActive)
			}
			switch {
			case got.GraceUntil == nil && c.want.GraceUntil == nil:
			case got.GraceUntil != nil && c.want.GraceUntil != nil:
				if !got.GraceUntil.Equal(*c.want.GraceUntil) {
					t.Errorf("grace until = %s, want %s",
						got.GraceUntil.Format("2006-01-02"), c.want.GraceUntil.Format("2006-01-02"))
				}
			default:
				t.Errorf("grace until = %v, want %v", got.GraceUntil, c.want.GraceUntil)
			}
		})
	}
}

// M-B: the second product is unaffected by the first product's lapse —
// each license is evaluated independently.
func TestSecondProductUnaffectedByFirstLapse(t *testing.T) {
	today := day(2026, time.September, 14)

	first := base()
	first.LicenseStatus = LicenseDisabled
	if got := Evaluate(first, today); got.Status != StatusRevoked {
		t.Fatalf("first product: status = %q, want %q", got.Status, StatusRevoked)
	}

	second := base()
	got := Evaluate(second, today)
	if got.Status != StatusValid || got.Warning == "" {
		t.Fatalf("second product: status = %q warning = %q, want %q with grace warning", got.Status, got.Warning, StatusValid)
	}
}
