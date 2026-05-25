package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"github.com/beeleelee/mall/domain/kernel"
	domain "github.com/beeleelee/mall/domain/order"
)

type OrderHandler struct {
	svc *domain.OrderService
}

func NewOrderHandler(svc *domain.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	vars := pathvar.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeDomainError(w, kernel.NewDomainError(kernel.ErrInvalidArgument, "invalid order id"))
		return
	}

	order, err := h.svc.GetOrder(r.Context(), kernel.ID(id))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeOrderResponse(w, http.StatusOK, order)
}

func (h *OrderHandler) ListByUser(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromContext(r)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	orders, err := h.svc.GetOrdersByUser(r.Context(), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	resp := make([]orderResponse, len(orders))
	for i, o := range orders {
		resp[i] = buildOrderResponse(o)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

type orderLineItemResponse struct {
	ProductID  int64  `json:"product_id"`
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	Quantity   int    `json:"quantity"`
	UnitPrice  int64  `json:"unit_price"`
	TotalPrice int64  `json:"total_price"`
}

type orderResponse struct {
	ID              int64                   `json:"id"`
	UserID          int64                   `json:"user_id"`
	CheckoutID      int64                   `json:"checkout_id"`
	CartID          int64                   `json:"cart_id"`
	Items           []orderLineItemResponse `json:"items"`
	ShippingAddress addressResponse         `json:"shipping_address"`
	BillingAddress  addressResponse         `json:"billing_address"`
	ShippingOption  shippingOptionResponse  `json:"shipping_option"`
	PaymentHandler  string                  `json:"payment_handler"`
	Subtotal        int64                   `json:"subtotal"`
	ShippingCost    int64                   `json:"shipping_cost"`
	TaxAmount       int64                   `json:"tax_amount"`
	GrandTotal      int64                   `json:"grand_total"`
	Status          string                  `json:"status"`
	TrackingNumber  string                  `json:"tracking_number,omitempty"`
	Carrier         string                  `json:"carrier,omitempty"`
	ConfirmedAt     int64                   `json:"confirmed_at"`
	ProcessingAt    *int64                  `json:"processing_at,omitempty"`
	ShippedAt       *int64                  `json:"shipped_at,omitempty"`
	DeliveredAt     *int64                  `json:"delivered_at,omitempty"`
	ReturnedAt      *int64                  `json:"returned_at,omitempty"`
	CancelledAt     *int64                  `json:"cancelled_at,omitempty"`
	CreatedAt       int64                   `json:"created_at"`
	UpdatedAt       int64                   `json:"updated_at"`
}

func buildOrderResponse(order *domain.Order) orderResponse {
	items := make([]orderLineItemResponse, len(order.Items))
	for i, item := range order.Items {
		items[i] = orderLineItemResponse{
			ProductID:  item.ProductID.Int64(),
			SKU:        item.SKU,
			Name:       item.Name,
			Quantity:   item.Quantity,
			UnitPrice:  item.UnitPrice,
			TotalPrice: item.TotalPrice,
		}
	}

	return orderResponse{
		ID:         order.ID.Int64(),
		UserID:     order.UserID.Int64(),
		CheckoutID: order.CheckoutID.Int64(),
		CartID:     order.CartID.Int64(),
		Items:      items,
		ShippingAddress: addressResponse{
			Line1:      order.ShippingAddress.Line1,
			Line2:      order.ShippingAddress.Line2,
			City:       order.ShippingAddress.City,
			State:      order.ShippingAddress.State,
			PostalCode: order.ShippingAddress.PostalCode,
			Country:    order.ShippingAddress.Country,
		},
		BillingAddress: addressResponse{
			Line1:      order.BillingAddress.Line1,
			Line2:      order.BillingAddress.Line2,
			City:       order.BillingAddress.City,
			State:      order.BillingAddress.State,
			PostalCode: order.BillingAddress.PostalCode,
			Country:    order.BillingAddress.Country,
		},
		ShippingOption: shippingOptionResponse{
			ID:        order.ShippingOption.ID,
			Name:      order.ShippingOption.Name,
			Cost:      order.ShippingOption.Cost,
			Estimated: order.ShippingOption.Estimated,
		},
		PaymentHandler: order.PaymentHandler,
		Subtotal:       order.Subtotal,
		ShippingCost:   order.ShippingCost,
		TaxAmount:      order.TaxAmount,
		GrandTotal:     order.GrandTotal,
		Status:         string(order.Status),
		TrackingNumber: order.TrackingNumber,
		Carrier:        order.Carrier,
		ConfirmedAt:    order.ConfirmedAt.UnixMilli(),
		ProcessingAt:   optionalUnixMilli(order.ProcessingAt),
		ShippedAt:      optionalUnixMilli(order.ShippedAt),
		DeliveredAt:    optionalUnixMilli(order.DeliveredAt),
		ReturnedAt:     optionalUnixMilli(order.ReturnedAt),
		CancelledAt:    optionalUnixMilli(order.CancelledAt),
		CreatedAt:      order.CreatedAt.UnixMilli(),
		UpdatedAt:      order.UpdatedAt.UnixMilli(),
	}
}

func writeOrderResponse(w http.ResponseWriter, status int, order *domain.Order) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(buildOrderResponse(order))
}

func optionalUnixMilli(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	v := t.UnixMilli()
	return &v
}
