package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zeromicro/go-zero/rest/pathvar"

	domain "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
)

type CheckoutHandler struct {
	svc *domain.CheckoutService
	sf  *kernel.Snowflake
}

func NewCheckoutHandler(svc *domain.CheckoutService, sf *kernel.Snowflake) *CheckoutHandler {
	return &CheckoutHandler{svc: svc, sf: sf}
}

type createCheckoutRequest struct {
	CheckoutID int64                   `json:"checkout_id,omitempty"`
	CartID     int64                   `json:"cart_id"`
	Items      []cartSnapshotItemInput `json:"items"`
}

type cartSnapshotItemInput struct {
	ProductID int64  `json:"product_id"`
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
	ImageURL  string `json:"image_url,omitempty"`
}

type addressInput struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

type shippingOptionInput struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Cost      int64  `json:"cost"`
	Estimated string `json:"estimated,omitempty"`
}

type paymentHandlerInput struct {
	Handler string `json:"handler"`
}

func (h *CheckoutHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	var req createCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	checkoutID := kernel.ID(req.CheckoutID)
	if checkoutID <= 0 {
		id, err := h.sf.NextID()
		if err != nil {
			writeDomainError(w, err)
			return
		}
		checkoutID = id
	}

	items := make([]domain.CartSnapshotItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = domain.CartSnapshotItem{
			ProductID: kernel.ID(item.ProductID),
			SKU:       item.SKU,
			Name:      item.Name,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
			ImageURL:  item.ImageURL,
		}
	}

	session, err := h.svc.CreateCheckout(r.Context(), domain.CreateCheckoutInput{
		CheckoutID: checkoutID,
		UserID:     userID,
		CartID:     kernel.ID(req.CartID),
		CartItems:  items,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeCheckoutResponse(w, http.StatusCreated, session)
}

func (h *CheckoutHandler) GetCheckout(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid checkout id"))
		return
	}

	session, err := h.svc.GetCheckout(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	if session.UserID != userID {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrPermissionDenied, "checkout does not belong to user"))
		return
	}

	writeCheckoutResponse(w, http.StatusOK, session)
}

func (h *CheckoutHandler) SetShippingAddress(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid checkout id"))
		return
	}

	session, err := h.svc.GetCheckout(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if session.UserID != userID {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrPermissionDenied, "checkout does not belong to user"))
		return
	}

	var req addressInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	session, err = h.svc.SetShippingAddress(r.Context(), kernel.ID(id), domain.Address{
		Line1:      req.Line1,
		Line2:      req.Line2,
		City:       req.City,
		State:      req.State,
		PostalCode: req.PostalCode,
		Country:    req.Country,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeCheckoutResponse(w, http.StatusOK, session)
}

func (h *CheckoutHandler) SetBillingAddress(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid checkout id"))
		return
	}

	session, err := h.svc.GetCheckout(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if session.UserID != userID {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrPermissionDenied, "checkout does not belong to user"))
		return
	}

	var req addressInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	session, err = h.svc.SetBillingAddress(r.Context(), kernel.ID(id), domain.Address{
		Line1:      req.Line1,
		Line2:      req.Line2,
		City:       req.City,
		State:      req.State,
		PostalCode: req.PostalCode,
		Country:    req.Country,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeCheckoutResponse(w, http.StatusOK, session)
}

func (h *CheckoutHandler) SelectShippingOption(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid checkout id"))
		return
	}

	session, err := h.svc.GetCheckout(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if session.UserID != userID {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrPermissionDenied, "checkout does not belong to user"))
		return
	}

	var req shippingOptionInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	session, err = h.svc.SelectShippingOption(r.Context(), kernel.ID(id), domain.ShippingOption{
		ID:        req.ID,
		Name:      req.Name,
		Cost:      req.Cost,
		Estimated: req.Estimated,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeCheckoutResponse(w, http.StatusOK, session)
}

func (h *CheckoutHandler) SelectPaymentHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid checkout id"))
		return
	}

	session, err := h.svc.GetCheckout(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if session.UserID != userID {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrPermissionDenied, "checkout does not belong to user"))
		return
	}

	var req paymentHandlerInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	session, err = h.svc.SelectPaymentHandler(r.Context(), kernel.ID(id), req.Handler)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeCheckoutResponse(w, http.StatusOK, session)
}

type paymentTokenInput struct {
	WalletProvider string `json:"wallet_provider"`
	Token          string `json:"token"`
}

func (h *CheckoutHandler) SubmitPaymentToken(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid checkout id"))
		return
	}

	session, err := h.svc.GetCheckout(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if session.UserID != userID {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrPermissionDenied, "checkout does not belong to user"))
		return
	}

	var req paymentTokenInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	session, err = h.svc.SubmitPaymentToken(r.Context(), kernel.ID(id), req.WalletProvider, req.Token)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeCheckoutResponse(w, http.StatusOK, session)
}

type selectMandateInput struct {
	MandateID int64 `json:"mandate_id"`
}

func (h *CheckoutHandler) SelectMandate(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid checkout id"))
		return
	}

	session, err := h.svc.GetCheckout(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if session.UserID != userID {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrPermissionDenied, "checkout does not belong to user"))
		return
	}

	var req selectMandateInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDomainError(w, err)
		return
	}

	session, err = h.svc.SelectMandate(r.Context(), kernel.ID(id), kernel.ID(req.MandateID))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeCheckoutResponse(w, http.StatusOK, session)
}

type completeCheckoutRequest struct {
	ContinueURL string `json:"continue_url,omitempty"`
}

func (h *CheckoutHandler) Complete(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid checkout id"))
		return
	}

	var req completeCheckoutRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	session, escalated, err := h.svc.StartComplete(r.Context(), kernel.ID(id), req.ContinueURL)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if session.UserID != userID {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrPermissionDenied, "checkout does not belong to user"))
		return
	}

	if escalated {
		resp := map[string]any{
			"checkout_id":  session.ID.Int64(),
			"status":       string(session.Status),
			"continue_url": session.ContinueURL,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(resp)
		return
	}

	writeCheckoutResponse(w, http.StatusOK, session)
}

type createPaymentIntentResponse struct {
	ClientSecret string `json:"client_secret"`
	IntentID     string `json:"intent_id"`
	Amount       int64  `json:"amount"`
}

func (h *CheckoutHandler) CreateStripePaymentIntent(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid checkout id"))
		return
	}

	session, err := h.svc.GetCheckout(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if session.UserID != userID {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrPermissionDenied, "checkout does not belong to user"))
		return
	}

	clientSecret, intentID, err := h.svc.CreateStripePaymentIntent(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createPaymentIntentResponse{
		ClientSecret: clientSecret,
		IntentID:     intentID,
		Amount:       session.GrandTotal,
	})
}

func (h *CheckoutHandler) CheckoutSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "payment_completed", "message": "payment completed successfully"})
}

func (h *CheckoutHandler) CheckoutCancel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "payment_cancelled", "message": "payment was cancelled"})
}

func (h *CheckoutHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid checkout id"))
		return
	}

	session, err := h.svc.GetCheckout(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if session.UserID != userID {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrPermissionDenied, "checkout does not belong to user"))
		return
	}

	session, err = h.svc.Cancel(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeCheckoutResponse(w, http.StatusOK, session)
}

type checkoutResponse struct {
	ID              int64                   `json:"id"`
	UserID          int64                   `json:"user_id"`
	CartID          int64                   `json:"cart_id"`
	Items           []checkoutItemResponse  `json:"items"`
	ShippingAddress *addressResponse        `json:"shipping_address,omitempty"`
	BillingAddress  *addressResponse        `json:"billing_address,omitempty"`
	ShippingOption  *shippingOptionResponse `json:"shipping_option,omitempty"`
	PaymentHandler  string                  `json:"payment_handler,omitempty"`
	WalletProvider  string                  `json:"wallet_provider,omitempty"`
	WalletToken     string                  `json:"wallet_token,omitempty"`
	Subtotal        int64                   `json:"subtotal"`
	ShippingCost    int64                   `json:"shipping_cost"`
	TaxAmount       int64                   `json:"tax_amount"`
	GrandTotal      int64                   `json:"grand_total"`
	Status          string                  `json:"status"`
	CompletedAt     *int64                  `json:"completed_at,omitempty"`
	CreatedAt       int64                   `json:"created_at"`
	UpdatedAt       int64                   `json:"updated_at"`
}

type checkoutItemResponse struct {
	ProductID int64  `json:"product_id"`
	SKU       string `json:"sku"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
	ImageURL  string `json:"image_url,omitempty"`
}

type addressResponse struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

type shippingOptionResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Cost      int64  `json:"cost"`
	Estimated string `json:"estimated,omitempty"`
}

func writeCheckoutResponse(w http.ResponseWriter, status int, session *domain.CheckoutSession) {
	items := make([]checkoutItemResponse, len(session.CartSnapshot.Items))
	for i, item := range session.CartSnapshot.Items {
		items[i] = checkoutItemResponse{
			ProductID: item.ProductID.Int64(),
			SKU:       item.SKU,
			Name:      item.Name,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
			ImageURL:  item.ImageURL,
		}
	}

	var sa *addressResponse
	if session.ShippingAddress != nil {
		sa = &addressResponse{
			Line1:      session.ShippingAddress.Line1,
			Line2:      session.ShippingAddress.Line2,
			City:       session.ShippingAddress.City,
			State:      session.ShippingAddress.State,
			PostalCode: session.ShippingAddress.PostalCode,
			Country:    session.ShippingAddress.Country,
		}
	}

	var ba *addressResponse
	if session.BillingAddress != nil {
		ba = &addressResponse{
			Line1:      session.BillingAddress.Line1,
			Line2:      session.BillingAddress.Line2,
			City:       session.BillingAddress.City,
			State:      session.BillingAddress.State,
			PostalCode: session.BillingAddress.PostalCode,
			Country:    session.BillingAddress.Country,
		}
	}

	var so *shippingOptionResponse
	if session.ShippingOption != nil {
		so = &shippingOptionResponse{
			ID:        session.ShippingOption.ID,
			Name:      session.ShippingOption.Name,
			Cost:      session.ShippingOption.Cost,
			Estimated: session.ShippingOption.Estimated,
		}
	}

	var completedAt *int64
	if session.CompletedAt != nil {
		t := session.CompletedAt.UnixMilli()
		completedAt = &t
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(checkoutResponse{
		ID:              session.ID.Int64(),
		UserID:          session.UserID.Int64(),
		CartID:          session.CartID.Int64(),
		Items:           items,
		ShippingAddress: sa,
		BillingAddress:  ba,
		ShippingOption:  so,
		PaymentHandler:  session.PaymentHandler,
		WalletProvider:  session.WalletProvider,
		WalletToken:     session.WalletToken,
		Subtotal:        session.Subtotal,
		ShippingCost:    session.ShippingCost,
		TaxAmount:       session.TaxAmount,
		GrandTotal:      session.GrandTotal,
		Status:          string(session.Status),
		CompletedAt:     completedAt,
		CreatedAt:       session.CreatedAt.UnixMilli(),
		UpdatedAt:       session.UpdatedAt.UnixMilli(),
	})
}
