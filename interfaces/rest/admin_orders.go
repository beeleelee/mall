package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"github.com/beeleelee/mall/domain/kernel"
)

func (h *AdminHandler) ProcessOrder(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid order id"))
		return
	}

	order, err := h.orderSvc.StartProcessing(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeOrderResponse(w, http.StatusOK, order)
}

func (h *AdminHandler) ShipOrder(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid order id"))
		return
	}

	var req shipOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	order, err := h.orderSvc.Ship(r.Context(), kernel.ID(id), req.TrackingNumber, req.Carrier)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeOrderResponse(w, http.StatusOK, order)
}

func (h *AdminHandler) DeliverOrder(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid order id"))
		return
	}

	order, err := h.orderSvc.MarkDelivered(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeOrderResponse(w, http.StatusOK, order)
}

func (h *AdminHandler) ReturnOrder(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid order id"))
		return
	}

	order, err := h.orderSvc.ReturnOrder(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeOrderResponse(w, http.StatusOK, order)
}

func (h *AdminHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid order id"))
		return
	}

	order, err := h.orderSvc.Cancel(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeOrderResponse(w, http.StatusOK, order)
}

type processRefundRequest struct {
	MandateID int64  `json:"mandate_id,omitempty"`
	Reason    string `json:"reason"`
}

func (h *AdminHandler) ProcessRefund(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid order id"))
		return
	}

	var req processRefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	order, err := h.orderSvc.ReturnOrder(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	refundID, err := h.sf.NextID()
	if err != nil {
		writeDomainError(w, err)
		return
	}

	refund, err := h.refundSvc.ProcessRefund(r.Context(), refundID, order, kernel.ID(req.MandateID), req.Reason)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(refund)
}


