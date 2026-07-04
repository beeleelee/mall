package rest

import (
	"encoding/json"
	"net/http"

	checkout "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/inventory"
	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/order"
	"github.com/beeleelee/mall/domain/payment"
)

type sagaReserveReq struct {
	Items []sagaItem `json:"items"`
}

type sagaItem struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

type sagaPaymentReq struct {
	MandateID int64  `json:"mandate_id"`
	Token     string `json:"token"`
}

type sagaOrderCreateReq struct {
	OrderID    int64 `json:"order_id"`
	CheckoutID int64 `json:"checkout_id"`
	UserID     int64 `json:"user_id"`
	CartID     int64 `json:"cart_id"`
}

type SagaHandler struct {
	inventorySvc *inventory.InventoryService
	paymentSvc   *payment.PaymentService
	orderSvc     *domain.OrderService
	checkoutSvc  *checkout.CheckoutService
}

func NewSagaHandler(
	inv *inventory.InventoryService,
	pay *payment.PaymentService,
	chk *checkout.CheckoutService,
	ord *domain.OrderService,
) *SagaHandler {
	return &SagaHandler{
		inventorySvc: inv,
		paymentSvc:   pay,
		checkoutSvc:  chk,
		orderSvc:     ord,
	}
}

func writeSagaError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeSagaOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (h *SagaHandler) ReserveInventory(w http.ResponseWriter, r *http.Request) {
	var req sagaReserveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSagaError(w, "invalid request: "+err.Error())
		return
	}
	for _, item := range req.Items {
		if _, err := h.inventorySvc.Reserve(r.Context(), kernel.ID(item.ProductID), item.Quantity); err != nil {
			writeSagaError(w, err.Error())
			return
		}
	}
	writeSagaOK(w)
}

func (h *SagaHandler) ReleaseInventory(w http.ResponseWriter, r *http.Request) {
	var req sagaReserveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSagaError(w, "invalid request: "+err.Error())
		return
	}
	for _, item := range req.Items {
		if _, err := h.inventorySvc.ReleaseReservation(r.Context(), kernel.ID(item.ProductID), item.Quantity); err != nil {
			writeSagaError(w, err.Error())
			return
		}
	}
	writeSagaOK(w)
}

func (h *SagaHandler) ConfirmInventory(w http.ResponseWriter, r *http.Request) {
	var req sagaReserveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSagaError(w, "invalid request: "+err.Error())
		return
	}
	for _, item := range req.Items {
		if _, err := h.inventorySvc.ConfirmReservation(r.Context(), kernel.ID(item.ProductID), item.Quantity); err != nil {
			writeSagaError(w, err.Error())
			return
		}
	}
	writeSagaOK(w)
}

func (h *SagaHandler) VerifyPayment(w http.ResponseWriter, r *http.Request) {
	var req sagaPaymentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSagaError(w, "invalid request: "+err.Error())
		return
	}
	m, err := h.paymentSvc.GetMandate(r.Context(), kernel.ID(req.MandateID))
	if err != nil {
		writeSagaError(w, "mandate not found: "+err.Error())
		return
	}
	if m.Status != payment.MandateStatusExecuted && m.Status != payment.MandateStatusSettled {
		writeSagaError(w, "mandate not in executed/settled state")
		return
	}
	writeSagaOK(w)
}

func (h *SagaHandler) CancelPayment(w http.ResponseWriter, r *http.Request) {
	var req sagaPaymentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSagaError(w, "invalid request: "+err.Error())
		return
	}
	if _, err := h.paymentSvc.CancelMandate(r.Context(), kernel.ID(req.MandateID)); err != nil {
		writeSagaError(w, err.Error())
		return
	}
	writeSagaOK(w)
}

func (h *SagaHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req sagaOrderCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSagaError(w, "invalid request: "+err.Error())
		return
	}

	session, err := h.checkoutSvc.GetCheckout(r.Context(), kernel.ID(req.CheckoutID))
	if err != nil {
		writeSagaError(w, "checkout not found: "+err.Error())
		return
	}

	if _, err := h.orderSvc.CreateOrder(r.Context(), kernel.ID(req.OrderID), session); err != nil {
		writeSagaError(w, err.Error())
		return
	}
	writeSagaOK(w)
}

func (h *SagaHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID int64 `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSagaError(w, "invalid request: "+err.Error())
		return
	}
	if _, err := h.orderSvc.Cancel(r.Context(), kernel.ID(req.OrderID)); err != nil {
		writeSagaError(w, err.Error())
		return
	}
	writeSagaOK(w)
}

type sagaMandateReq struct {
	MandateID int64  `json:"mandate_id"`
	Token     string `json:"token"`
}

func (h *SagaHandler) ExecuteMandate(w http.ResponseWriter, r *http.Request) {
	var req sagaMandateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSagaError(w, "invalid request: "+err.Error())
		return
	}
	if _, err := h.paymentSvc.ExecuteMandate(r.Context(), kernel.ID(req.MandateID), req.Token); err != nil {
		writeSagaError(w, err.Error())
		return
	}
	writeSagaOK(w)
}

func (h *SagaHandler) SettleMandate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MandateID int64 `json:"mandate_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSagaError(w, "invalid request: "+err.Error())
		return
	}
	if _, err := h.paymentSvc.SettleMandate(r.Context(), kernel.ID(req.MandateID)); err != nil {
		writeSagaError(w, err.Error())
		return
	}
	writeSagaOK(w)
}

func (h *SagaHandler) RollbackMandateSettle(w http.ResponseWriter, r *http.Request) {
	writeSagaOK(w)
}
