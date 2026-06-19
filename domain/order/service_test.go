package order

import (
	"context"
	"testing"

	"github.com/beeleelee/mall/domain/kernel"
)

func newTestService() *OrderService {
	return NewOrderService(newFakeOrderRepo(), newFakeOrderPublisher(), fakeLoggerOrder{})
}

func TestOrderService_CreateOrder_Success(t *testing.T) {
	svc := newTestService()
	session := completedCheckout(t, 1)
	ctx := context.Background()

	order, err := svc.CreateOrder(ctx, 1, session)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != OrderStatusConfirmed {
		t.Errorf("expected confirmed, got %s", order.Status)
	}
}

func TestOrderService_GetOrder(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	session := completedCheckout(t, 1)
	svc.CreateOrder(ctx, 1, session)

	found, err := svc.GetOrder(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != 1 {
		t.Errorf("expected ID 1, got %d", found.ID)
	}
}

func TestOrderService_GetOrder_NotFound(t *testing.T) {
	svc := newTestService()
	_, err := svc.GetOrder(context.Background(), 999)
	if !kernel.IsNotFound(err) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestOrderService_GetOrdersByUser(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	s1 := completedCheckout(t, 1)
	s2 := completedCheckout(t, 2)
	svc.CreateOrder(ctx, 1, s1)
	svc.CreateOrder(ctx, 2, s2)

	orders, err := svc.GetOrdersByUser(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) != 2 {
		t.Errorf("expected 2 orders, got %d", len(orders))
	}
}

func TestOrderService_FullLifecycle(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	session := completedCheckout(t, 1)
	order, _ := svc.CreateOrder(ctx, 1, session)

	order, _ = svc.StartProcessing(ctx, order.ID)
	if order.Status != OrderStatusProcessing {
		t.Fatal("expected processing")
	}

	order, _ = svc.Ship(ctx, order.ID, "TRK123", "UPS")
	if order.Status != OrderStatusShipped {
		t.Fatal("expected shipped")
	}

	order, _ = svc.MarkDelivered(ctx, order.ID)
	if order.Status != OrderStatusDelivered {
		t.Fatal("expected delivered")
	}

	order, _ = svc.ReturnOrder(ctx, order.ID)
	if order.Status != OrderStatusReturned {
		t.Fatal("expected returned")
	}
}

func TestOrderService_Ship_InvalidArgs(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	session := completedCheckout(t, 1)
	order, _ := svc.CreateOrder(ctx, 1, session)
	svc.StartProcessing(ctx, order.ID)

	_, err := svc.Ship(ctx, order.ID, "", "UPS")
	if !kernel.IsInvalidArgument(err) {
		t.Errorf("expected invalid argument, got %v", err)
	}
}

func TestOrderService_Cancel(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	session := completedCheckout(t, 1)
	order, _ := svc.CreateOrder(ctx, 1, session)

	order, err := svc.Cancel(ctx, order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != OrderStatusCancelled {
		t.Errorf("expected cancelled, got %s", order.Status)
	}
}
