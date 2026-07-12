package rest

import (
	"encoding/json"
	"net/http"
	"time"

	domain "github.com/beeleelee/mall/domain/discount"
	"github.com/beeleelee/mall/domain/kernel"
)

type DiscountHandler struct {
	svc *domain.DiscountService
	sf  *kernel.Snowflake
}

func NewDiscountHandler(svc *domain.DiscountService, sf *kernel.Snowflake) *DiscountHandler {
	return &DiscountHandler{svc: svc, sf: sf}
}

type createDiscountRequest struct {
	Code        string `json:"code"`
	Type        string `json:"type"`
	Value       int64  `json:"value"`
	MinPurchase int64  `json:"min_purchase"`
	MaxUsages   int    `json:"max_usages"`
	Expiry      string `json:"expiry"`
	Stackable   bool   `json:"stackable"`
}

type applyDiscountRequest struct {
	Code     string `json:"code"`
	Subtotal int64  `json:"subtotal"`
}

type validateDiscountRequest struct {
	Code     string `json:"code"`
	Subtotal int64  `json:"subtotal"`
}

type deactivateDiscountRequest struct {
	Code string `json:"code"`
}

func (h *DiscountHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createDiscountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	expiry, err := time.Parse(time.RFC3339, req.Expiry)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid expiry format, use RFC3339"))
		return
	}

	id, err := h.sf.NextID()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	dc, err := h.svc.CreateCode(r.Context(), id, req.Code, domain.DiscountType(req.Type), req.Value, req.MinPurchase, req.MaxUsages, expiry, req.Stackable)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	json.NewEncoder(w).Encode(discountResponse(dc))
}

func (h *DiscountHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var req validateDiscountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	dc, valid := h.svc.ValidateCode(r.Context(), req.Code, req.Subtotal)
	if dc == nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrNotFound, "discount code not found"))
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"valid": valid,
		"code":  discountResponse(dc),
	})
}

func (h *DiscountHandler) Apply(w http.ResponseWriter, r *http.Request) {
	var req applyDiscountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	final, applied, err := h.svc.ApplyCode(r.Context(), req.Code, req.Subtotal)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"applied":  applied,
		"original": req.Subtotal,
		"final":    final,
		"discount": req.Subtotal - final,
	})
}

func (h *DiscountHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	var req deactivateDiscountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	if err := h.svc.DeactivateCode(r.Context(), req.Code); err != nil {
		writeDomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type discountResponseBody struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	Type        string `json:"type"`
	Value       int64  `json:"value"`
	MinPurchase int64  `json:"min_purchase"`
	MaxUsages   int    `json:"max_usages"`
	UsedCount   int    `json:"used_count"`
	Expiry      string `json:"expiry"`
	Active      bool   `json:"active"`
	Stackable   bool   `json:"stackable"`
}

func discountResponse(d *domain.DiscountCode) discountResponseBody {
	return discountResponseBody{
		ID:          d.ID.Int64(),
		Code:        d.Code,
		Type:        string(d.Type),
		Value:       d.Value,
		MinPurchase: d.MinPurchase,
		MaxUsages:   d.MaxUsages,
		UsedCount:   d.UsedCount,
		Expiry:      d.Expiry.Format(time.RFC3339),
		Active:      d.Active,
		Stackable:   d.Stackable,
	}
}
