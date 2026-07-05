package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/payment"
)

type PaymentHandler struct {
	svc *domain.PaymentService
	sf  *kernel.Snowflake
}

func NewPaymentHandler(svc *domain.PaymentService, sf *kernel.Snowflake) *PaymentHandler {
	return &PaymentHandler{svc: svc, sf: sf}
}

type createMandateRequest struct {
	MandateID       int64    `json:"mandate_id,omitempty"`
	MaxAmount       int64    `json:"max_amount"`
	MerchantID      int64    `json:"merchant_id"`
	Expiry          string   `json:"expiry"`
	AllowedHandlers []string `json:"allowed_handlers,omitempty"`
}

type approveMandateRequest struct {
	Signature string `json:"signature"`
}

type executeMandateRequest struct {
	Token string `json:"token"`
}

func (h *PaymentHandler) CreateMandate(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	var req createMandateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	id := kernel.ID(req.MandateID)
	if id <= 0 {
		id, err = h.sf.NextID()
		if err != nil {
			writeDomainError(w, err)
			return
		}
	}

	expiry, err := parseTime(req.Expiry)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid expiry format, use RFC3339"))
		return
	}

	scope := domain.MandateScope{
		MaxAmount:       req.MaxAmount,
		MerchantID:      kernel.ID(req.MerchantID),
		Expiry:          expiry,
		AllowedHandlers: req.AllowedHandlers,
	}

	m, err := h.svc.RequestMandate(r.Context(), id, userID, scope)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	json.NewEncoder(w).Encode(mandateResponse(m))
}

func (h *PaymentHandler) GetMandate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(pathvar.Vars(r)["id"], 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid mandate id"))
		return
	}

	m, err := h.svc.GetMandate(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	json.NewEncoder(w).Encode(mandateResponse(m))
}

func (h *PaymentHandler) ListMandates(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	mandates, err := h.svc.ListUserMandates(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	resp := make([]mandateResponseBody, 0, len(mandates))
	for _, m := range mandates {
		resp = append(resp, mandateResponse(m))
	}

	json.NewEncoder(w).Encode(resp)
}

func (h *PaymentHandler) ApproveMandate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(pathvar.Vars(r)["id"], 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid mandate id"))
		return
	}

	var req approveMandateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	m, err := h.svc.ApproveMandate(r.Context(), kernel.ID(id), req.Signature)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	json.NewEncoder(w).Encode(mandateResponse(m))
}

func (h *PaymentHandler) ExecuteMandate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(pathvar.Vars(r)["id"], 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid mandate id"))
		return
	}

	var req executeMandateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	m, err := h.svc.ExecuteMandate(r.Context(), kernel.ID(id), req.Token)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	json.NewEncoder(w).Encode(mandateResponse(m))
}

func (h *PaymentHandler) SettleMandate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(pathvar.Vars(r)["id"], 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid mandate id"))
		return
	}

	m, err := h.svc.SettleMandate(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	json.NewEncoder(w).Encode(mandateResponse(m))
}

func (h *PaymentHandler) CancelMandate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(pathvar.Vars(r)["id"], 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid mandate id"))
		return
	}

	m, err := h.svc.CancelMandate(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	json.NewEncoder(w).Encode(mandateResponse(m))
}

type mandateResponseBody struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Status     string `json:"status"`
	MaxAmount  int64  `json:"max_amount"`
	MerchantID int64  `json:"merchant_id"`
	Expiry     string `json:"expiry"`
	Signature  string `json:"signature,omitempty"`
	Token      string `json:"token,omitempty"`
}

func mandateResponse(m *domain.Mandate) mandateResponseBody {
	return mandateResponseBody{
		ID:         m.ID.Int64(),
		UserID:     m.UserID.Int64(),
		Status:     string(m.Status),
		MaxAmount:  m.Scope.MaxAmount,
		MerchantID: m.Scope.MerchantID.Int64(),
		Expiry:     m.Scope.Expiry.Format("2006-01-02T15:04:05Z"),
		Signature:  m.Signature,
		Token:      m.Token,
	}
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
