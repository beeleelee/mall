package main

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"

	_ "github.com/jackc/pgx/v5/stdlib"

	appOrder "github.com/beeleelee/mall/application/order"
	domainCart "github.com/beeleelee/mall/domain/cart"
	domainCheckout "github.com/beeleelee/mall/domain/checkout"
	domainIdentity "github.com/beeleelee/mall/domain/identity"
	domainOrder "github.com/beeleelee/mall/domain/order"
	"github.com/beeleelee/mall/domain/kernel"
	"github.com/beeleelee/mall/infrastructure/database"
	infraCart "github.com/beeleelee/mall/infrastructure/cart"
	infraCheckout "github.com/beeleelee/mall/infrastructure/checkout"
	infraIdentity "github.com/beeleelee/mall/infrastructure/identity"
	infraOrder "github.com/beeleelee/mall/infrastructure/order"
)

const e2eTimeout = 10 * time.Second

func TestE2E_FullPurchaseFlow(t *testing.T) {
	if !servicesUp() {
		t.Skip("e2e: need 'docker compose up postgres redis nats' running")
	}

	db, schema := connectPostgresWithSchema(t)
	defer db.Close()

	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 5})
	defer rdb.Close()

	nc := connectNATS(t)
	defer nc.Close()

	js := setupJetStream(t, nc, "e2e")
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		t.Fatal(err)
	}

	logger := stdLogger{}
	ctx := context.Background()

	idSeq := sf.NextID

	userRepo := infraIdentity.NewPostgresUserRepository(db, rdb)
	domainSvc := domainIdentity.NewIdentityService(userRepo, logger)

	userID, err := idSeq()
	if err != nil {
		t.Fatal(err)
	}
	user, err := domainSvc.Register(ctx, userID, fmt.Sprintf("e2e-%d@example.com", time.Now().UnixMilli()), "E2E User", "testpass123")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("registered user %d", user.ID.Int64())

	cartRepo := infraCart.NewPostgresCartRepository(db, rdb)
	cartPub := infraCart.NewNATSCartEventPublisher(nc)
	cartSvc := domainCart.NewCartService(cartRepo, cartPub, logger)

	cartID, err := idSeq()
	if err != nil {
		t.Fatal(err)
	}
	_, err = cartSvc.AddItem(ctx, domainCart.AddItemInput{
		CartID:    cartID,
		UserID:    user.ID,
		ProductID: 100,
		SKU:       "SKU-E2E-001",
		Name:      "E2E Product 1",
		Quantity:  2,
		UnitPrice: 1500,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("created cart %d with 1 item", cartID.Int64())

	_, err = cartSvc.AddItem(ctx, domainCart.AddItemInput{
		CartID:    cartID,
		UserID:    user.ID,
		ProductID: 101,
		SKU:       "SKU-E2E-002",
		Name:      "E2E Product 2",
		Quantity:  1,
		UnitPrice: 3000,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("added second item to cart")

	checkoutRepo := infraCheckout.NewPostgresCheckoutRepository(db, rdb)
	checkoutPub := infraCheckout.NewNATSCheckoutEventPublisher(js)
	taxSvc := domainCheckout.NewDefaultTaxService()
	priceCalc := domainCheckout.NewDefaultPriceCalculator()
	checkoutSvc := domainCheckout.NewCheckoutService(checkoutRepo, taxSvc, priceCalc, checkoutPub, logger)

	checkoutID, err := idSeq()
	if err != nil {
		t.Fatal(err)
	}
	snapshotItems := []domainCheckout.CartSnapshotItem{
		{ProductID: 100, SKU: "SKU-E2E-001", Name: "E2E Product 1", Quantity: 2, UnitPrice: 1500},
		{ProductID: 101, SKU: "SKU-E2E-002", Name: "E2E Product 2", Quantity: 1, UnitPrice: 3000},
	}
	session, err := checkoutSvc.CreateCheckout(ctx, domainCheckout.CreateCheckoutInput{
		CheckoutID: checkoutID,
		UserID:     user.ID,
		CartID:     cartID,
		CartItems:  snapshotItems,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("created checkout %d", session.ID.Int64())

	addr := domainCheckout.Address{Line1: "456 Oak St", City: "Seattle", State: "WA", PostalCode: "98101", Country: "US"}
	session, err = checkoutSvc.SetShippingAddress(ctx, session.ID, addr)
	if err != nil {
		t.Fatal(err)
	}
	session, err = checkoutSvc.SetBillingAddress(ctx, session.ID, addr)
	if err != nil {
		t.Fatal(err)
	}
	session, err = checkoutSvc.SelectShippingOption(ctx, session.ID, domainCheckout.ShippingOption{ID: "std", Name: "Standard", Cost: 500})
	if err != nil {
		t.Fatal(err)
	}
	session, err = checkoutSvc.SelectPaymentHandler(ctx, session.ID, "stripe")
	if err != nil {
		t.Fatal(err)
	}
	session, err = checkoutSvc.CalculateTax(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	session, err = checkoutSvc.MarkReady(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("checkout ready for completion: subtotal=%d shipping=%d tax=%d grand_total=%d",
		session.Subtotal, session.ShippingCost, session.TaxAmount, session.GrandTotal)

	orderRepo := infraOrder.NewPostgresOrderRepository(db, rdb)
	orderPub := infraOrder.NewNATSOrderEventPublisher(js)
	orderSvc := domainOrder.NewOrderService(orderRepo, orderPub, logger)
	saga := appOrder.NewCheckoutCompletedSaga(orderSvc, sf, logger)

	cons := startSagaConsumer(t, js, saga)
	defer cons.Stop()

	session, err = checkoutSvc.Complete(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("checkout %d completed", session.ID.Int64())

	var order *domainOrder.Order
	deadline := time.Now().Add(e2eTimeout)
	for time.Now().Before(deadline) {
		order, err = orderRepo.FindByCheckoutID(ctx, session.ID)
		if err == nil && order != nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if order == nil {
		t.Fatalf("order not created within %v (saga may not have consumed the event)", e2eTimeout)
	}
	t.Logf("order %d created from checkout %d", order.ID.Int64(), order.CheckoutID.Int64())

	if order.ID.Int64() == order.CheckoutID.Int64() {
		t.Error("order.ID should differ from checkout_id (new Snowflake ID)")
	}
	if order.Status != domainOrder.OrderStatusConfirmed {
		t.Errorf("expected confirmed, got %s", order.Status)
	}
	if order.UserID != user.ID {
		t.Errorf("expected user %d, got %d", user.ID.Int64(), order.UserID.Int64())
	}
	if order.CartID != cartID {
		t.Errorf("expected cart %d, got %d", cartID.Int64(), order.CartID.Int64())
	}
	if order.PaymentHandler != "stripe" {
		t.Errorf("expected payment handler stripe, got %s", order.PaymentHandler)
	}
	if order.GrandTotal != session.GrandTotal {
		t.Errorf("expected grand_total %d, got %d", session.GrandTotal, order.GrandTotal)
	}
	if len(order.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(order.Items))
	}
	if order.Items[0].ProductID != 100 {
		t.Errorf("expected first product 100, got %d", order.Items[0].ProductID)
	}

	if _, err := db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema)); err != nil {
		t.Errorf("cleanup schema: %v", err)
	}
}

func connectPostgresWithSchema(t *testing.T) (*sqlx.DB, string) {
	t.Helper()
	dsn := "postgres://mall:mall_dev@localhost:5432/mall?sslmode=disable"
	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	db.SetMaxOpenConns(4)

	schema := fmt.Sprintf("e2e_%08x", rand.Int63())[:16]
	if _, err := db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, schema)); err != nil {
		db.Close()
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec(fmt.Sprintf(`SET search_path TO "%s", public`, schema)); err != nil {
		db.Close()
		t.Fatalf("set search_path: %v", err)
	}
	if err := database.NewMigrator(db).Up(); err != nil {
		db.Close()
		t.Fatalf("run migrations: %v", err)
	}
	return db, schema
}

func connectNATS(t *testing.T) *nats.Conn {
	t.Helper()
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		t.Fatalf("connect nats: %v", err)
	}
	return nc
}

func setupJetStream(t *testing.T, nc *nats.Conn, _ string) jetstream.JetStream {
	t.Helper()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("create jetstream: %v", err)
	}
	for _, cfg := range []jetstream.StreamConfig{
		{Name: "checkout", Subjects: []string{"checkout.>"}, MaxAge: 30 * time.Minute, Storage: jetstream.MemoryStorage},
		{Name: "orders", Subjects: []string{"order.>"}, MaxAge: 30 * time.Minute, Storage: jetstream.MemoryStorage},
	} {
		if _, err := js.CreateOrUpdateStream(context.Background(), cfg); err != nil {
			t.Fatalf("create stream %s: %v", cfg.Name, err)
		}
	}
	return js
}

func startSagaConsumer(t *testing.T, js jetstream.JetStream, saga *appOrder.CheckoutCompletedSaga) jetstream.ConsumeContext {
	t.Helper()
	cons, err := js.CreateOrUpdateConsumer(context.Background(), "checkout", jetstream.ConsumerConfig{
		Name:          "order-saga",
		FilterSubject: "checkout.updated",
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}
	cc, err := cons.Consume(func(msg jetstream.Msg) {
		msg.Ack()
		if err := saga.Handle(context.Background(), msg.Data()); err != nil {
			t.Logf("saga: handle failed: %v", err)
		}
	})
	if err != nil {
		t.Fatalf("start consume: %v", err)
	}
	return cc
}
