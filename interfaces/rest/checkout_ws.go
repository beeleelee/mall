package rest

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/zeromicro/go-zero/rest/pathvar"

	domain "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/ecp"
	"github.com/beeleelee/mall/domain/kernel"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type CheckoutWSHandler struct {
	svc    *domain.CheckoutService
	logger kernel.Logger
}

func NewCheckoutWSHandler(svc *domain.CheckoutService, logger kernel.Logger) *CheckoutWSHandler {
	return &CheckoutWSHandler{svc: svc, logger: logger}
}

func (h *CheckoutWSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid checkout id", http.StatusBadRequest)
		return
	}
	checkoutID := kernel.ID(id)

	userID, err := userIDFromContext(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	defer conn.Close()

	var mu sync.Mutex
	writeJSON := func(v any) error {
		mu.Lock()
		defer mu.Unlock()
		return conn.WriteJSON(v)
	}

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg ecp.ECPMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			writeJSON(ecp.ECPResponse{
				JSONRPC: "2.0",
				Error:   &ecp.ECPError{Code: -32700, Message: "parse error"},
				ID:      nil,
			})
			continue
		}

		resp := h.handleMessage(r, checkoutID, userID, msg)
		if resp != nil {
			resp.JSONRPC = "2.0"
			if resp.ID == nil {
				resp.ID = msg.ID
			}
			writeJSON(resp)
		}
	}
}

func (h *CheckoutWSHandler) handleMessage(r *http.Request, checkoutID, userID kernel.ID, msg ecp.ECPMessage) *ecp.ECPResponse {
	switch msg.Method {
	case ecp.MethodStateUpdate:
		return h.handleStateUpdate(checkoutID, userID)
	case ecp.MethodCredentialsSubmit:
		return h.handleCredentialsSubmit(r, checkoutID, userID, msg)
	case ecp.MethodPaymentAuthorize:
		return h.handlePaymentAuthorize(r, checkoutID, userID, msg)
	case ecp.MethodAddressSelect:
		return h.handleAddressSelect(r, checkoutID, userID, msg)
	default:
		return &ecp.ECPResponse{
			Error: &ecp.ECPError{Code: -32601, Message: "method not found: " + msg.Method},
			ID:    msg.ID,
		}
	}
}

func (h *CheckoutWSHandler) handleStateUpdate(checkoutID, userID kernel.ID) *ecp.ECPResponse {
	session, err := h.svc.GetCheckout(context.Background(), checkoutID)
	if err != nil {
		return &ecp.ECPResponse{
			Error: &ecp.ECPError{Code: -32000, Message: err.Error()},
		}
	}
	if session.UserID != userID {
		return &ecp.ECPResponse{
			Error: &ecp.ECPError{Code: -32001, Message: "permission denied"},
		}
	}

	return &ecp.ECPResponse{
		Result: ecp.StateUpdateResult{
			CheckoutID:  checkoutID.Int64(),
			Status:      string(session.Status),
			ContinueURL: session.ContinueURL,
		},
	}
}

func (h *CheckoutWSHandler) handleCredentialsSubmit(r *http.Request, checkoutID, userID kernel.ID, msg ecp.ECPMessage) *ecp.ECPResponse {
	raw, err := json.Marshal(msg.Params)
	if err != nil {
		return &ecp.ECPResponse{
			Error: &ecp.ECPError{Code: -32602, Message: "invalid params"},
		}
	}

	var params ecp.CredentialsSubmitParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return &ecp.ECPResponse{
			Error: &ecp.ECPError{Code: -32602, Message: "invalid params"},
		}
	}

	if _, err := h.svc.SetShippingAddress(r.Context(), checkoutID, domain.Address{
		Line1:      params.ShippingLine1,
		Line2:      params.ShippingLine2,
		City:       params.ShippingCity,
		State:      params.ShippingState,
		PostalCode: params.ShippingPostal,
		Country:    params.ShippingCountry,
	}); err != nil {
		return &ecp.ECPResponse{
			Error: &ecp.ECPError{Code: -32000, Message: err.Error()},
		}
	}

	if _, err := h.svc.SetBillingAddress(r.Context(), checkoutID, domain.Address{
		Line1:      params.BillingLine1,
		Line2:      params.BillingLine2,
		City:       params.BillingCity,
		State:      params.BillingState,
		PostalCode: params.BillingPostal,
		Country:    params.BillingCountry,
	}); err != nil {
		return &ecp.ECPResponse{
			Error: &ecp.ECPError{Code: -32000, Message: err.Error()},
		}
	}

	session, err := h.svc.GetCheckout(r.Context(), checkoutID)
	if err != nil {
		return &ecp.ECPResponse{
			Error: &ecp.ECPError{Code: -32000, Message: err.Error()},
		}
	}

	return &ecp.ECPResponse{
		Result: ecp.CredentialsSubmitResult{
			CheckoutID: checkoutID.Int64(),
			Status:     string(session.Status),
		},
	}
}

func (h *CheckoutWSHandler) handlePaymentAuthorize(r *http.Request, checkoutID, userID kernel.ID, msg ecp.ECPMessage) *ecp.ECPResponse {
	raw, err := json.Marshal(msg.Params)
	if err != nil {
		return &ecp.ECPResponse{
			Error: &ecp.ECPError{Code: -32602, Message: "invalid params"},
		}
	}

	var params ecp.PaymentAuthorizeParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return &ecp.ECPResponse{
			Error: &ecp.ECPError{Code: -32602, Message: "invalid params"},
		}
	}

	if params.MandateID != "" {
		if _, err := h.svc.SelectPaymentHandler(r.Context(), checkoutID, "ap2:"+params.MandateID); err != nil {
			return &ecp.ECPResponse{
				Error: &ecp.ECPError{Code: -32000, Message: err.Error()},
			}
		}
	}

	session, err := h.svc.Complete(r.Context(), checkoutID)
	if err != nil {
		return &ecp.ECPResponse{
			Error: &ecp.ECPError{Code: -32000, Message: err.Error()},
		}
	}

	return &ecp.ECPResponse{
		Result: ecp.PaymentAuthorizeResult{
			CheckoutID: checkoutID.Int64(),
			Status:     string(session.Status),
			Token:      "mock_token_" + checkoutID.String(),
		},
	}
}

func (h *CheckoutWSHandler) handleAddressSelect(r *http.Request, checkoutID, userID kernel.ID, msg ecp.ECPMessage) *ecp.ECPResponse {
	raw, err := json.Marshal(msg.Params)
	if err != nil {
		return &ecp.ECPResponse{
			Error: &ecp.ECPError{Code: -32602, Message: "invalid params"},
		}
	}

	var params ecp.AddressSelectParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return &ecp.ECPResponse{
			Error: &ecp.ECPError{Code: -32602, Message: "invalid params"},
		}
	}

	addr := domain.Address{
		Line1:      params.Line1,
		Line2:      params.Line2,
		City:       params.City,
		State:      params.State,
		PostalCode: params.PostalCode,
		Country:    params.Country,
	}

	switch params.AddressType {
	case "shipping":
		if _, err := h.svc.SetShippingAddress(r.Context(), checkoutID, addr); err != nil {
			return &ecp.ECPResponse{
				Error: &ecp.ECPError{Code: -32000, Message: err.Error()},
			}
		}
	case "billing":
		if _, err := h.svc.SetBillingAddress(r.Context(), checkoutID, addr); err != nil {
			return &ecp.ECPResponse{
				Error: &ecp.ECPError{Code: -32000, Message: err.Error()},
			}
		}
	default:
		return &ecp.ECPResponse{
			Error: &ecp.ECPError{Code: -32602, Message: "invalid address_type: " + params.AddressType},
		}
	}

	return &ecp.ECPResponse{
		Result: ecp.AddressSelectResult{
			CheckoutID:  checkoutID.Int64(),
			AddressType: params.AddressType,
			Status:      "updated",
		},
	}
}
