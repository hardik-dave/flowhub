package verify

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/flowos/hub/internal/auth"
	"github.com/flowos/hub/internal/store"
)

const isoDate = "2006-01-02"

// Response is the §6.1 wire body. Field names and the status strings
// are law (AGENTS.md rule 1) — never rename or remove, only add.
type Response struct {
	Status       string          `json:"status"`
	Reason       string          `json:"reason"`
	Warning      string          `json:"warning"`
	Entitlements json.RawMessage `json:"entitlements"`
	Subscription Subscription    `json:"subscription"`
}

// Subscription mirrors §6.1's subscription object.
type Subscription struct {
	ValidUntil *string `json:"valid_until"`
	Grace      Grace   `json:"grace"`
}

type Grace struct {
	Active bool    `json:"active"`
	Until  *string `json:"until"`
}

// Service loads the rows §4 needs and renders a §6.1 verdict.
type Service struct {
	store *store.Store
	now   func() time.Time
}

func NewService(st *store.Store, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{store: st, now: now}
}

// CheckLicense evaluates a presented license key (§6.1 POST /app/verify).
func (s *Service) CheckLicense(ctx context.Context, tenantSlug, username, productCode, licenseKey string) (Response, error) {
	return s.check(ctx, tenantSlug, username, productCode, &licenseKey)
}

// CheckAccount evaluates an already-authenticated user's license for a
// product (§6.2 login: the password proved identity, no key presented).
func (s *Service) CheckAccount(ctx context.Context, tenantSlug, username, productCode string) (Response, error) {
	return s.check(ctx, tenantSlug, username, productCode, nil)
}

func (s *Service) check(ctx context.Context, tenantSlug, username, productCode string, licenseKey *string) (Response, error) {
	notFound := Response{
		Status:       StatusNotFound,
		Reason:       ReasonNotFound,
		Entitlements: nil,
		Subscription: Subscription{Grace: Grace{}},
	}

	tenant, err := s.store.TenantBySlug(ctx, tenantSlug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return notFound, nil
		}
		return Response{}, err
	}
	product, err := s.store.ProductByCode(ctx, productCode)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return notFound, nil
		}
		return Response{}, err
	}
	granted, err := s.store.ProductGrantedToTenant(ctx, tenant.ID, product.ID)
	if err != nil {
		return Response{}, err
	}
	user, err := s.store.UserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return notFound, nil
		}
		return Response{}, err
	}
	license, err := s.store.LicenseByUserProduct(ctx, tenant.ID, user.ID, product.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return notFound, nil
		}
		return Response{}, err
	}

	keyMatches := true
	if licenseKey != nil {
		match, verr := auth.VerifySecret(license.LicenseKeyHash, *licenseKey)
		keyMatches = verr == nil && match
	}

	in := AccessInput{
		TenantFound:      true,
		TenantName:       tenant.Name,
		TenantStatus:     tenant.Status,
		ProductFound:     true,
		ProductGranted:   granted,
		ProductStatus:    product.Status,
		UserFound:        true,
		UserStatus:       user.Status,
		MobileVerified:   user.MobileVerified,
		LicenseFound:     true,
		LicenseStatus:    license.Status,
		KeyMatches:       keyMatches,
		ValidUntil:       license.SubscriptionValidUntil,
		GraceWorkingDays: tenant.GraceWorkingDays,
	}
	if user.Status == UserDeactivated {
		if reason, rerr := s.store.LatestStatusReason(ctx, tenant.ID, user.ID); rerr == nil {
			in.DeactivationReason = reason
		}
	}

	verdict := Evaluate(in, s.now())
	out := Response{
		Status:       verdict.Status,
		Reason:       verdict.Reason,
		Warning:      verdict.Warning,
		Entitlements: license.Entitlements,
	}
	if len(out.Entitlements) == 0 {
		out.Entitlements = nil
	}
	if license.SubscriptionValidUntil != nil {
		d := license.SubscriptionValidUntil.UTC().Format(isoDate)
		out.Subscription.ValidUntil = &d
	}
	if verdict.GraceActive && verdict.GraceUntil != nil {
		d := verdict.GraceUntil.UTC().Format(isoDate)
		out.Subscription.Grace.Active = true
		out.Subscription.Grace.Until = &d
	}
	return out, nil
}
