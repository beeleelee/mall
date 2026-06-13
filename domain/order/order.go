package order

import (
	"time"

	checkout "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
)

type OrderStatus string

const (
	OrderStatusConfirmed  OrderStatus = "confirmed"
	OrderStatusProcessing OrderStatus = "processing"
	OrderStatusShipped    OrderStatus = "shipped"
	OrderStatusDelivered  OrderStatus = "delivered"
	OrderStatusReturned   OrderStatus = "returned"
	OrderStatusCancelled  OrderStatus = "cancelled"
)

type Order struct {
	kernel.AggregateRoot
	UserID          kernel.ID
	CheckoutID      kernel.ID
	CartID          kernel.ID
	Items           []OrderLineItem
	ShippingAddress checkout.Address
	BillingAddress  checkout.Address
	ShippingOption  checkout.ShippingOption
	PaymentHandler  string
	Subtotal        int64
	ShippingCost    int64
	TaxAmount       int64
	GrandTotal      int64
	Status          OrderStatus
	TrackingNumber  string
	Carrier         string
	ConfirmedAt     time.Time
	ProcessingAt    *time.Time
	ShippedAt       *time.Time
	DeliveredAt     *time.Time
	ReturnedAt      *time.Time
	CancelledAt     *time.Time
}

type OrderLineItem struct {
	ProductID  kernel.ID `json:"product_id"`
	SKU        string    `json:"sku"`
	Name       string    `json:"name"`
	Quantity   int       `json:"quantity"`
	UnitPrice  int64     `json:"unit_price"`
	TotalPrice int64     `json:"total_price"`
}

func NewOrderFromCheckout(id kernel.ID, session *checkout.CheckoutSession) (*Order, error) {
	if session.Status != checkout.CheckoutStatusCompleted {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "checkout must be completed to create order")
	}
	if id <= 0 {
		return nil, kernel.NewDomainError(kernel.ErrInvalidArgument, "order id must be positive")
	}

	items := make([]OrderLineItem, len(session.CartSnapshot.Items))
	for i, item := range session.CartSnapshot.Items {
		items[i] = OrderLineItem{
			ProductID:  item.ProductID,
			SKU:        item.SKU,
			Name:       item.Name,
			Quantity:   item.Quantity,
			UnitPrice:  item.UnitPrice,
			TotalPrice: item.TotalPrice(),
		}
	}

	now := time.Now()
	shippingAddr := checkout.Address{}
	if session.ShippingAddress != nil {
		shippingAddr = *session.ShippingAddress
	}
	billingAddr := checkout.Address{}
	if session.BillingAddress != nil {
		billingAddr = *session.BillingAddress
	}
	shippingOpt := checkout.ShippingOption{}
	if session.ShippingOption != nil {
		shippingOpt = *session.ShippingOption
	}

	o := &Order{
		AggregateRoot:   kernel.NewAggregateRoot(id),
		UserID:          session.UserID,
		CheckoutID:      session.ID,
		CartID:          session.CartID,
		Items:           items,
		ShippingAddress: shippingAddr,
		BillingAddress:  billingAddr,
		ShippingOption:  shippingOpt,
		PaymentHandler:  session.PaymentHandler,
		Subtotal:        session.Subtotal,
		ShippingCost:    session.ShippingCost,
		TaxAmount:       session.TaxAmount,
		GrandTotal:      session.GrandTotal,
		Status:          OrderStatusConfirmed,
		ConfirmedAt:     now,
	}
	o.CreatedAt = now
	o.UpdatedAt = now
	o.AddEvent(OrderConfirmedEvent{OrderID: o.ID, UserID: o.UserID})
	return o, nil
}

func NewOrderFromSnapshot(
	id, userID, checkoutID, cartID kernel.ID,
	items []OrderLineItem,
	shippingAddr, billingAddr checkout.Address,
	shippingOpt checkout.ShippingOption,
	paymentHandler string,
	subtotal, shippingCost, taxAmount, grandTotal int64,
	status OrderStatus,
	trackingNumber, carrier string,
	confirmedAt time.Time,
	processingAt, shippedAt, deliveredAt, returnedAt, cancelledAt *time.Time,
	createdAt, updatedAt time.Time,
) *Order {
	o := &Order{
		AggregateRoot:   kernel.NewAggregateRoot(id),
		UserID:          userID,
		CheckoutID:      checkoutID,
		CartID:          cartID,
		Items:           items,
		ShippingAddress: shippingAddr,
		BillingAddress:  billingAddr,
		ShippingOption:  shippingOpt,
		PaymentHandler:  paymentHandler,
		Subtotal:        subtotal,
		ShippingCost:    shippingCost,
		TaxAmount:       taxAmount,
		GrandTotal:      grandTotal,
		Status:          status,
		TrackingNumber:  trackingNumber,
		Carrier:         carrier,
		ConfirmedAt:     confirmedAt,
		ProcessingAt:    processingAt,
		ShippedAt:       shippedAt,
		DeliveredAt:     deliveredAt,
		ReturnedAt:      returnedAt,
		CancelledAt:     cancelledAt,
	}
	o.CreatedAt = createdAt
	o.UpdatedAt = updatedAt
	return o
}

func (o *Order) StartProcessing() error {
	if o.Status != OrderStatusConfirmed {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only start processing from confirmed status, current: "+string(o.Status))
	}
	now := time.Now()
	o.Status = OrderStatusProcessing
	o.ProcessingAt = &now
	o.touch()
	o.AddEvent(OrderProcessingEvent{OrderID: o.ID, UserID: o.UserID})
	return nil
}

func (o *Order) Ship(trackingNumber, carrier string) error {
	if o.Status != OrderStatusProcessing {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only ship from processing status, current: "+string(o.Status))
	}
	if trackingNumber == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "tracking number must not be empty")
	}
	if carrier == "" {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "carrier must not be empty")
	}
	now := time.Now()
	o.Status = OrderStatusShipped
	o.TrackingNumber = trackingNumber
	o.Carrier = carrier
	o.ShippedAt = &now
	o.touch()
	o.AddEvent(OrderShippedEvent{OrderID: o.ID, UserID: o.UserID})
	return nil
}

func (o *Order) MarkDelivered() error {
	if o.Status != OrderStatusShipped {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only mark delivered from shipped status, current: "+string(o.Status))
	}
	now := time.Now()
	o.Status = OrderStatusDelivered
	o.DeliveredAt = &now
	o.touch()
	o.AddEvent(OrderDeliveredEvent{OrderID: o.ID, UserID: o.UserID})
	return nil
}

func (o *Order) Return() error {
	if o.Status != OrderStatusDelivered {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only return from delivered status, current: "+string(o.Status))
	}
	now := time.Now()
	o.Status = OrderStatusReturned
	o.ReturnedAt = &now
	o.touch()
	o.AddEvent(OrderReturnedEvent{OrderID: o.ID, UserID: o.UserID})
	return nil
}

func (o *Order) Cancel() error {
	if o.Status == OrderStatusCancelled {
		return kernel.NewDomainError(kernel.ErrConflict, "order already cancelled")
	}
	if o.Status != OrderStatusConfirmed && o.Status != OrderStatusProcessing {
		return kernel.NewDomainError(kernel.ErrInvalidArgument, "can only cancel from confirmed or processing status, current: "+string(o.Status))
	}
	now := time.Now()
	o.Status = OrderStatusCancelled
	o.CancelledAt = &now
	o.touch()
	o.AddEvent(OrderCancelledEvent{OrderID: o.ID, UserID: o.UserID})
	return nil
}

func (o *Order) touch() {
	o.UpdatedAt = time.Now()
}
