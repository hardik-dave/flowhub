package user

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/flowos/hub/internal/auth"
	"github.com/flowos/hub/internal/config"
	"github.com/flowos/hub/internal/store"
)

// HouseTenantID is the reserved house-tenant id (SPEC §3 seed).
const HouseTenantID int64 = 1

// Bootstrap creates the first-boot platform admin (house tenant, role
// SUPERADMIN) from env, once and idempotently. Returns true if it
// created the account this run. A missing username/password is a no-op.
func Bootstrap(ctx context.Context, st *store.Store, admin config.PlatformAdmin) (bool, error) {
	if admin.Username == "" || admin.Password == "" {
		return false, nil
	}
	if _, err := st.UserByUsername(ctx, admin.Username); err == nil {
		return false, nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return false, err
	}

	hash, err := auth.HashSecret(admin.Password)
	if err != nil {
		return false, err
	}

	err = st.InTx(ctx, func(tx store.DBTX) error {
		id, err := st.InsertUser(ctx, tx, &store.User{
			Username:       admin.Username,
			PasswordHash:   hash,
			FirstName:      "Platform",
			LastName:       "Admin",
			ContactNo:      admin.Mobile,
			MobileVerified: admin.Mobile != "",
			Status:         "ACTIVE",
		})
		if err != nil {
			return err
		}
		if err := st.InsertMembership(ctx, tx, HouseTenantID, id, store.RoleSuperadmin); err != nil {
			return err
		}
		after, _ := json.Marshal(map[string]any{"username": admin.Username, "role": string(store.RoleSuperadmin)})
		return st.InsertAudit(ctx, tx, &store.Audit{
			TenantID:      HouseTenantID,
			Action:        "PLATFORM_ADMIN_BOOTSTRAPPED",
			SubjectUserID: &id,
			After:         after,
			Reason:        "First-boot platform admin",
		})
	})
	if err != nil {
		if isDuplicate(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
