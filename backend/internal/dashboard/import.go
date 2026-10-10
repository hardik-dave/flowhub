package dashboard

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flowos/hub/internal/auth"
	"github.com/flowos/hub/internal/httpx"
	"github.com/flowos/hub/internal/license"
	"github.com/flowos/hub/internal/store"
)

const (
	importMaxBytes = 5 << 20
	importMaxRows  = 5000
)

var validPlans = map[string]bool{"MONTHLY": true, "QUARTERLY": true, "ANNUAL": true}

type importRowError struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

type importCreated struct {
	Row        int    `json:"row"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	LicenseKey string `json:"license_key"`
}

// ImportCSV implements POST /dash/imports (SPEC §8): a multipart CSV with
// a header row. Per-row failures never abort the batch; the response
// lists one-time initial passwords + license keys for the tenant to
// distribute. Imported users are mobile_verified=false so their first
// app login drives the FIRST_LOGIN OTP flow.
func (h *Handler) ImportCSV(w http.ResponseWriter, r *http.Request) {
	sc, ok := mustScope(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	if err := r.ParseMultipartForm(importMaxBytes); err != nil {
		httpx.Error(w, http.StatusBadRequest, "Expected a multipart form with a CSV 'file' field.")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "A CSV 'file' field is required.")
		return
	}
	defer file.Close()
	filename := header.Filename

	reader := csv.NewReader(io.LimitReader(file, importMaxBytes))
	reader.FieldsPerRecord = -1
	headerRow, err := reader.Read()
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "Could not read the CSV header row.")
		return
	}
	idx := map[string]int{}
	for i, name := range headerRow {
		idx[strings.ToLower(strings.TrimSpace(name))] = i
	}
	if _, ok := idx["username"]; !ok {
		httpx.Error(w, http.StatusBadRequest, "CSV header must include 'username'.")
		return
	}
	if _, ok := idx["mobile"]; !ok {
		httpx.Error(w, http.StatusBadRequest, "CSV header must include 'mobile'.")
		return
	}

	errorsOut := []importRowError{}
	createdOut := []importCreated{}
	seenUsernames := map[string]bool{}
	seenMobiles := map[string]bool{}

	var batchID int64
	var rowCount int
	err = h.store.InTx(ctx, func(tx store.DBTX) error {
		id, err := h.store.InsertImportBatch(ctx, tx, &store.ImportBatch{
			TenantID: sc.TenantID, Filename: filename, CreatedBy: sc.UserID,
		})
		if err != nil {
			return err
		}
		batchID = id

		row := 0
		for {
			rec, err := reader.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			row++
			if row > importMaxRows {
				return fmt.Errorf("CSV exceeds %d data rows", importMaxRows)
			}
			if isEmptyRow(rec) {
				continue
			}
			rowCount++

			get := func(name string) string {
				if i, ok := idx[name]; ok && i < len(rec) {
					return strings.TrimSpace(rec[i])
				}
				return ""
			}
			username := get("username")
			mobile := get("mobile")

			if !usernameRe.MatchString(username) {
				errorsOut = append(errorsOut, importRowError{row, "username must be 3-64 characters: letters, digits, dot, underscore or hyphen."})
				continue
			}
			if !e164Re.MatchString(mobile) {
				errorsOut = append(errorsOut, importRowError{row, "mobile must be E.164, e.g. +919876543210"})
				continue
			}
			if seenUsernames[username] {
				errorsOut = append(errorsOut, importRowError{row, "duplicate username within the file"})
				continue
			}
			if _, err := h.store.UserByUsername(ctx, username); err == nil {
				errorsOut = append(errorsOut, importRowError{row, "username already exists"})
				continue
			} else if !errors.Is(err, store.ErrNotFound) {
				return err
			}

			productCode := get("product_code")
			if productCode == "" {
				productCode = "flowos"
			}
			product, err := h.store.ProductByCode(ctx, productCode)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					errorsOut = append(errorsOut, importRowError{row, "unknown product: " + productCode})
					continue
				}
				return err
			}
			granted, err := h.store.ProductGrantedToTenant(ctx, sc.TenantID, product.ID)
			if err != nil {
				return err
			}
			if !granted {
				errorsOut = append(errorsOut, importRowError{row, "tenant does not distribute product " + productCode})
				continue
			}
			if seenMobiles[mobile] {
				errorsOut = append(errorsOut, importRowError{row, "duplicate mobile within the file"})
				continue
			}
			hasMobile, err := h.store.TenantHasMobile(ctx, sc.TenantID, mobile)
			if err != nil {
				return err
			}
			if hasMobile {
				errorsOut = append(errorsOut, importRowError{row, "mobile already exists in this tenant"})
				continue
			}

			plan := strings.ToUpper(get("plan"))
			paidAtStr := get("paid_at")
			validFromStr := get("valid_from")
			var paidAt, validFrom, validUntil time.Time
			if plan != "" || paidAtStr != "" || validFromStr != "" {
				if plan == "" || paidAtStr == "" || validFromStr == "" {
					errorsOut = append(errorsOut, importRowError{row, "plan, paid_at and valid_from must all be provided together"})
					continue
				}
				if !validPlans[plan] {
					errorsOut = append(errorsOut, importRowError{row, "plan must be MONTHLY, QUARTERLY or ANNUAL"})
					continue
				}
				paidAt, err = time.Parse("2006-01-02", paidAtStr)
				if err != nil {
					errorsOut = append(errorsOut, importRowError{row, "paid_at must be YYYY-MM-DD"})
					continue
				}
				validFrom, err = time.Parse("2006-01-02", validFromStr)
				if err != nil {
					errorsOut = append(errorsOut, importRowError{row, "valid_from must be YYYY-MM-DD"})
					continue
				}
				validUntil = addPlan(validFrom, plan)
			}

			password, err := auth.RandomPassword()
			if err != nil {
				return err
			}
			passwordHash, err := auth.HashSecret(password)
			if err != nil {
				return err
			}
			key, err := license.NewKey(product.KeyPrefix)
			if err != nil {
				return err
			}
			keyHash, err := auth.HashSecret(key)
			if err != nil {
				return err
			}

			uid, err := h.store.InsertUser(ctx, tx, &store.User{
				Username: username, PasswordHash: passwordHash,
				FirstName: get("first_name"), LastName: get("last_name"), ContactNo: mobile,
				MobileVerified: false, Email: get("email"), City: get("city"), State: get("state"),
				BrokerClientCode: get("broker_client_code"), ReferralCode: get("referral_code"),
				Status: "PENDING_VERIFICATION", Imported: true, ImportBatchID: &batchID,
			})
			if err != nil {
				if isDuplicate(err) {
					errorsOut = append(errorsOut, importRowError{row, "username already exists"})
					continue
				}
				return err
			}
			if err := h.store.InsertMembership(ctx, tx, sc.TenantID, uid, store.RoleUser); err != nil {
				return err
			}
			lid, err := h.store.InsertLicense(ctx, tx, &store.License{
				TenantID: sc.TenantID, UserID: uid, ProductID: product.ID,
				LicenseKeyHash: keyHash, LicenseKeyHint: license.Hint(key), Status: "ACTIVE",
			})
			if err != nil {
				return err
			}
			if !validUntil.IsZero() {
				if err := h.store.UpdateLicenseValidUntil(ctx, tx, sc.TenantID, lid, validUntil); err != nil {
					return err
				}
				if _, err := h.store.InsertPayment(ctx, tx, &store.Payment{
					TenantID: sc.TenantID, UserID: uid, LicenseID: lid,
					AmountMinorUnits: 0, Currency: "INR", Method: "MANUAL", Plan: plan,
					PaidAt: paidAt, ValidFrom: validFrom, ValidUntil: validUntil,
					RecordedBy: sc.UserID, Note: "imported",
				}); err != nil {
					return err
				}
			}
			if err := h.store.InsertAudit(ctx, tx, &store.Audit{
				TenantID: sc.TenantID, ActorUserID: &sc.UserID, Action: "USER_IMPORTED", SubjectUserID: &uid,
				After:  mustJSON(map[string]any{"username": username, "product_code": productCode, "batch_id": batchID}),
				Reason: "Imported via CSV",
			}); err != nil {
				return err
			}

			seenUsernames[username] = true
			seenMobiles[mobile] = true
			createdOut = append(createdOut, importCreated{Row: row, Username: username, Password: password, LicenseKey: key})
		}

		return h.store.UpdateImportBatch(ctx, tx, batchID, len(createdOut), len(errorsOut), mustJSON(errorsOut))
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not complete the import.")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{
		"batch_id": batchID, "filename": filename, "row_count": rowCount,
		"ok_count": len(createdOut), "error_count": len(errorsOut),
		"errors": errorsOut, "created": createdOut,
	})
}

func addPlan(from time.Time, plan string) time.Time {
	switch plan {
	case "MONTHLY":
		return from.AddDate(0, 1, 0)
	case "QUARTERLY":
		return from.AddDate(0, 3, 0)
	case "ANNUAL":
		return from.AddDate(0, 12, 0)
	}
	return from
}

func isEmptyRow(rec []string) bool {
	for _, v := range rec {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}
