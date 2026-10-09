package verify

import (
	"net/http"

	"github.com/flowos/hub/internal/httpx"
)

// Handler serves §6.1 POST /app/verify.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type verifyRequest struct {
	TenantSlug  string `json:"tenant_slug"`
	Username    string `json:"username"`
	ProductCode string `json:"product_code"`
	LicenseKey  string `json:"license_key"`
}

// Verify always answers 200 for a processed check — the status field
// carries the verdict (SPEC §6.1).
func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.ProductCode == "" {
		httpx.Error(w, http.StatusBadRequest, "product_code is required.")
		return
	}
	if req.TenantSlug == "" || req.Username == "" || req.LicenseKey == "" {
		httpx.Error(w, http.StatusBadRequest, "tenant_slug, username and license_key are required.")
		return
	}
	resp, err := h.svc.CheckLicense(r.Context(), req.TenantSlug, req.Username, req.ProductCode, req.LicenseKey)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Could not process the license check.")
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}
