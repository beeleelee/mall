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


