package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
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
	"github.com/beeleelee/mall/domain/kernel"
	domainOrder "github.com/beeleelee/mall/domain/order"
	infraCart "github.com/beeleelee/mall/infrastructure/cart"
	infraCheckout "github.com/beeleelee/mall/infrastructure/checkout"
	"github.com/beeleelee/mall/infrastructure/database"
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
	cartPub := infraCart.NewNATSCartEventPublisher(js)
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
		{Name: "cart", Subjects: []string{"cart.>"}, MaxAge: 30 * time.Minute, Storage: jetstream.MemoryStorage},
		{Name: "checkout", Subjects: []string{"checkout.>"}, MaxAge: 30 * time.Minute, Storage: jetstream.MemoryStorage},
		{Name: "orders", Subjects: []string{"order.>"}, MaxAge: 30 * time.Minute, Storage: jetstream.MemoryStorage},
	} {
		if _, err := js.CreateOrUpdateStream(context.Background(), cfg); err != nil {
			t.Fatalf("create stream %s: %v", cfg.Name, err)
		}
	}
	return js
}

func TestE2E_PurchaseFlowHTTP(t *testing.T) {
	if !servicesUp() {
		t.Skip("e2e: need 'docker compose up postgres redis nats' running")
	}

	port := freePort()
	os.Setenv("PORT", port)
	os.Setenv("DATABASE_URL", "postgres://mall:mall_dev@localhost:5432/mall?sslmode=disable")
	os.Setenv("REDIS_ADDR", "localhost:6379")

	done := make(chan struct{}, 1)
	go func() {
		main()
		close(done)
	}()

	time.Sleep(1 * time.Second)

	baseURL := fmt.Sprintf("http://localhost:%s", port)
	email := fmt.Sprintf("e2e-http-%d@example.com", time.Now().UnixMilli())

	var token string

	t.Run("register", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    email,
			"password": "testpass123",
			"name":     "E2E HTTP User",
		})
		resp, err := http.Post(baseURL+"/api/v1/auth/register", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("register: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("register: expected 201, got %d", resp.StatusCode)
		}
	})

	t.Run("login", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{
			"email":    email,
			"password": "testpass123",
		})
		resp, err := http.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("login: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("login: expected 200, got %d", resp.StatusCode)
		}
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		tok, _ := result["token"].(string)
		if tok == "" {
			t.Fatal("login: no token in response")
		}
		token = tok
	})

	authReq := func(method, path string, body any) *http.Response {
		var data io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			data = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, baseURL+path, data)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		return resp
	}

	var cartID float64
	var checkoutID float64

	t.Run("create cart", func(t *testing.T) {
		resp := authReq("POST", "/api/v1/carts", nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("create cart: expected 200, got %d", resp.StatusCode)
		}
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		cartID = result["id"].(float64)
		if cartID == 0 {
			t.Fatal("create cart: no id")
		}
		t.Logf("created cart %.0f", cartID)
	})

	t.Run("add item", func(t *testing.T) {
		item := map[string]any{
			"product_id": 100,
			"sku":        "SKU-HTTP-001",
			"name":       "HTTP Test Product",
			"quantity":   2,
			"unit_price": 1500,
		}
		resp := authReq("POST", fmt.Sprintf("/api/v1/carts/%.0f/items", cartID), item)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("add item: expected 200, got %d: %s", resp.StatusCode, string(body))
		}
		t.Log("added item to cart")
	})

	t.Run("create checkout", func(t *testing.T) {
		body := map[string]any{
			"cart_id": cartID,
			"items": []map[string]any{
				{"product_id": 100, "sku": "SKU-HTTP-001", "name": "HTTP Test Product", "quantity": 2, "unit_price": 1500},
			},
		}
		resp := authReq("POST", "/api/v1/checkouts", body)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("create checkout: expected 201, got %d: %s", resp.StatusCode, string(b))
		}
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		checkoutID = result["id"].(float64)
		t.Logf("created checkout %.0f", checkoutID)
	})

	checkoutPath := func(p string) string {
		return fmt.Sprintf("/api/v1/checkouts/%.0f%s", checkoutID, p)
	}

	t.Run("set shipping address", func(t *testing.T) {
		addr := map[string]string{
			"line1": "123 Main St", "city": "Seattle", "state": "WA",
			"postal_code": "98101", "country": "US",
		}
		resp := authReq("POST", checkoutPath("/shipping-address"), addr)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("shipping address: expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("set billing address", func(t *testing.T) {
		addr := map[string]string{
			"line1": "456 Oak St", "city": "Seattle", "state": "WA",
			"postal_code": "98101", "country": "US",
		}
		resp := authReq("POST", checkoutPath("/billing-address"), addr)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("billing address: expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("select shipping option", func(t *testing.T) {
		opt := map[string]any{
			"id": "std", "name": "Standard", "cost": 500,
		}
		resp := authReq("POST", checkoutPath("/shipping-option"), opt)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("shipping option: expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("select payment handler", func(t *testing.T) {
		resp := authReq("POST", checkoutPath("/payment-handler"), map[string]string{"handler": "stripe"})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("payment handler: expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("complete checkout", func(t *testing.T) {
		resp := authReq("POST", checkoutPath("/complete"), nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("complete: expected 200, got %d: %s", resp.StatusCode, string(b))
		}
		t.Logf("checkout %.0f completed", checkoutID)
	})

	t.Run("get order by checkout", func(t *testing.T) {
		var orderID float64
		deadline := time.Now().Add(e2eTimeout)
		for time.Now().Before(deadline) {
			resp := authReq("GET", "/api/v1/orders", nil)
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				continue
			}
			var orders []map[string]any
			json.NewDecoder(resp.Body).Decode(&orders)
			resp.Body.Close()
			if len(orders) > 0 {
				// find the order matching our checkout
				for _, o := range orders {
					if o["checkout_id"].(float64) == checkoutID {
						orderID = o["id"].(float64)
						break
					}
				}
				if orderID > 0 {
					break
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
		if orderID == 0 {
			t.Fatalf("order not created within %v (saga may not have consumed the event)", e2eTimeout)
		}
		t.Logf("order %.0f created from checkout %.0f", orderID, checkoutID)

		resp := authReq("GET", fmt.Sprintf("/api/v1/orders/%.0f", orderID), nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("get order: expected 200, got %d", resp.StatusCode)
		}
		var order map[string]any
		json.NewDecoder(resp.Body).Decode(&order)
		if order["status"] != "confirmed" {
			t.Errorf("expected confirmed, got %v", order["status"])
		}
	})

	t.Cleanup(func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("REDIS_ADDR")
	})
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
