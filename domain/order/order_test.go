package order

import (
	"testing"
	"time"

	checkout "github.com/beeleelee/mall/domain/checkout"
	"github.com/beeleelee/mall/domain/kernel"
)

func completedCheckout(t *testing.T, id kernel.ID) *checkout.CheckoutSession {
	t.Helper()
	snapshot := checkout.NewCartSnapshot([]checkout.CartSnapshotItem{
		{ProductID: 100, SKU: "SKU001", Name: "Product 1", Quantity: 2, UnitPrice: 1000},
	})
	s, err := checkout.NewCheckoutSession(id, 42, 10, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	addr := checkout.Address{Line1: "123 Main St", City: "Portland", State: "OR", PostalCode: "97201", Country: "US"}
	s.SetShippingAddress(addr)
	s.SetBillingAddress(addr)
	s.SelectShippingOption(checkout.ShippingOption{ID: "std", Name: "Standard", Cost: 500, Estimated: "5-7 days"})
	s.SelectPaymentHandler("stripe")
	s.MarkReady()
	s.Complete()
	return s
}

func TestNewOrderFromCheckout_Success(t *testing.T) {
	session := completedCheckout(t, 1)
	order, err := NewOrderFromCheckout(1, session)
	if err != nil {
		t.Fatal(err)
	}
	if order.ID != 1 {
		t.Errorf("expected ID 1, got %d", order.ID)
	}
	if order.UserID != 42 {
		t.Errorf("expected user 42, got %d", order.UserID)
	}
	if order.Status != OrderStatusConfirmed {
		t.Errorf("expected confirmed, got %s", order.Status)
	}
	if len(order.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(order.Items))
	}
	if order.Items[0].ProductID != 100 {
		t.Errorf("expected product 100, got %d", order.Items[0].ProductID)
	}
	if order.GrandTotal != 2500 {
		t.Errorf("expected grand total 2500 (2000+500+0), got %d", order.GrandTotal)
	}
}

func TestNewOrderFromCheckout_NotCompleted(t *testing.T) {
	snapshot := checkout.NewCartSnapshot([]checkout.CartSnapshotItem{
		{ProductID: 100, Quantity: 1, UnitPrice: 1000},
	})
	session, _ := checkout.NewCheckoutSession(1, 42, 10, snapshot)
	_, err := NewOrderFromCheckout(1, session)
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestOrder_StartProcessing(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)

	if err := order.StartProcessing(); err != nil {
		t.Fatal(err)
	}
	if order.Status != OrderStatusProcessing {
		t.Errorf("expected processing, got %s", order.Status)
	}
	if order.ProcessingAt == nil {
		t.Error("expected non-nil processing_at")
	}
}

func TestOrder_StartProcessing_WrongState(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)
	order.StartProcessing()
	err := order.StartProcessing()
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestOrder_Ship(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)
	order.StartProcessing()

	if err := order.Ship("TRACK123", "UPS"); err != nil {
		t.Fatal(err)
	}
	if order.Status != OrderStatusShipped {
		t.Errorf("expected shipped, got %s", order.Status)
	}
	if order.TrackingNumber != "TRACK123" {
		t.Errorf("expected TRACK123, got %s", order.TrackingNumber)
	}
	if order.Carrier != "UPS" {
		t.Errorf("expected UPS, got %s", order.Carrier)
	}
}

func TestOrder_Ship_WrongState(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)
	err := order.Ship("TRACK123", "UPS")
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestOrder_Ship_EmptyTracking(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)
	order.StartProcessing()
	err := order.Ship("", "UPS")
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestOrder_Ship_EmptyCarrier(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)
	order.StartProcessing()
	err := order.Ship("TRACK123", "")
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestOrder_MarkDelivered(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)
	order.StartProcessing()
	order.Ship("TRK", "FedEx")

	if err := order.MarkDelivered(); err != nil {
		t.Fatal(err)
	}
	if order.Status != OrderStatusDelivered {
		t.Errorf("expected delivered, got %s", order.Status)
	}
}

func TestOrder_MarkDelivered_WrongState(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)
	err := order.MarkDelivered()
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestOrder_Return(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)
	order.StartProcessing()
	order.Ship("TRK", "UPS")
	order.MarkDelivered()

	if err := order.Return(); err != nil {
		t.Fatal(err)
	}
	if order.Status != OrderStatusReturned {
		t.Errorf("expected returned, got %s", order.Status)
	}
}

func TestOrder_Return_WrongState(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)
	err := order.Return()
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestOrder_Cancel_FromConfirmed(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)

	if err := order.Cancel(); err != nil {
		t.Fatal(err)
	}
	if order.Status != OrderStatusCancelled {
		t.Errorf("expected cancelled, got %s", order.Status)
	}
}

func TestOrder_Cancel_FromProcessing(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)
	order.StartProcessing()

	if err := order.Cancel(); err != nil {
		t.Fatal(err)
	}
	if order.Status != OrderStatusCancelled {
		t.Errorf("expected cancelled, got %s", order.Status)
	}
}

func TestOrder_Cancel_FromShipped(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)
	order.StartProcessing()
	order.Ship("TRK", "UPS")

	err := order.Cancel()
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestOrder_Cancel_AlreadyCancelled(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)
	order.Cancel()
	err := order.Cancel()
	if !kernel.IsConflict(err) {
		t.Errorf("expected conflict, got %v", err)
	}
}

func TestOrder_FullLifecycle(t *testing.T) {
	session := completedCheckout(t, 1)
	order, _ := NewOrderFromCheckout(1, session)

	if order.Status != OrderStatusConfirmed { t.Fatal("expected confirmed") }
	order.StartProcessing()
	if order.Status != OrderStatusProcessing { t.Fatal("expected processing") }
	order.Ship("1Z999AA10123456784", "UPS")
	if order.Status != OrderStatusShipped { t.Fatal("expected shipped") }
	order.MarkDelivered()
	if order.Status != OrderStatusDelivered { t.Fatal("expected delivered") }
	order.Return()
	if order.Status != OrderStatusReturned { t.Fatal("expected returned") }
}

func TestNewOrderFromSnapshot(t *testing.T) {
	now := time.Now()
	items := []OrderLineItem{{ProductID: 100, SKU: "A", Name: "P", Quantity: 1, UnitPrice: 1000, TotalPrice: 1000}}
	addr := checkout.Address{Line1: "1 St", City: "NYC", State: "NY", PostalCode: "10001", Country: "US"}
	opt := checkout.ShippingOption{ID: "std", Name: "Std", Cost: 500}
	o := NewOrderFromSnapshot(1, 42, 10, 5, items, addr, addr, opt, "stripe", 1000, 500, 0, 1500, OrderStatusConfirmed, "", "", now, nil, nil, nil, nil, nil, now, now)
	if o.ID != 1 || o.UserID != 42 || o.Status != OrderStatusConfirmed {
		t.Error("snapshot reconstitution failed")
	}
}
