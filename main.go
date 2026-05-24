package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	gozerorest "github.com/zeromicro/go-zero/rest"

	appIdentity "github.com/beeleelee/mall/application/identity"
	appOAuth "github.com/beeleelee/mall/application/oauth"
	appOrder "github.com/beeleelee/mall/application/order"
	domainCart "github.com/beeleelee/mall/domain/cart"
	domainCheckout "github.com/beeleelee/mall/domain/checkout"
	domainIdentity "github.com/beeleelee/mall/domain/identity"
	"github.com/beeleelee/mall/domain/kernel"
	domainOAuth "github.com/beeleelee/mall/domain/oauth"
	domainOrder "github.com/beeleelee/mall/domain/order"
	infraCart "github.com/beeleelee/mall/infrastructure/cart"
	infraCheckout "github.com/beeleelee/mall/infrastructure/checkout"
	"github.com/beeleelee/mall/infrastructure/database"
	infraIdentity "github.com/beeleelee/mall/infrastructure/identity"
	infraOAuth "github.com/beeleelee/mall/infrastructure/oauth"
	infraOrder "github.com/beeleelee/mall/infrastructure/order"
	"github.com/beeleelee/mall/interfaces/middleware"
	"github.com/beeleelee/mall/interfaces/rest"
)

func main() {
	loadDotEnv()

	port := envOrDefault("PORT", "8080")
	pgDSN := envOrDefault("DATABASE_URL", "postgres://mall:mall_dev@localhost:5432/mall?sslmode=disable")
	redisAddr := envOrDefault("REDIS_ADDR", "localhost:6379")

	db, err := sqlx.Connect("pgx", pgDSN)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   0,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer rdb.Close()

	if err := database.NewMigrator(db).Up(); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	userRepo := infraIdentity.NewPostgresUserRepository(db, rdb)
	sf, err := kernel.NewSnowflake(1)
	if err != nil {
		log.Fatalf("new snowflake: %v", err)
	}

	jwtSecret := []byte(envOrDefault("JWT_SECRET", "dev-jwt-secret-change-in-production"))

	logger := stdLogger{}
	domainSvc := domainIdentity.NewIdentityService(userRepo, logger)
	appSvc := appIdentity.NewIdentityAppService(domainSvc, userRepo, logger, sf)

	identityHandler := rest.NewIdentityHandler(appSvc)
	ucpHandler := rest.NewUCPHandler(nil)

	oauthClientRepo := infraOAuth.NewPostgresOAuthClientRepository(db)
	oauthCodeRepo := infraOAuth.NewPostgresAuthorizationCodeRepository(db)
	oauthTokenRepo := infraOAuth.NewPostgresRefreshTokenRepository(db)
	oauthDomainSvc := domainOAuth.NewOAuthService(oauthClientRepo, oauthCodeRepo, oauthTokenRepo, logger, jwtSecret)
	oauthAppSvc := appOAuth.NewOAuthAppService(oauthDomainSvc, logger)
	oauthHandler := rest.NewOAuthHandler(oauthAppSvc)

	seedOAuthClient(oauthClientRepo, logger)

	natsURL := envOrDefault("NATS_URL", "nats://localhost:4222")
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("connect nats: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("create jetstream context: %v", err)
	}

	if _, err := js.CreateOrUpdateStream(context.Background(), jetstream.StreamConfig{
		Name:     "cart",
		Subjects: []string{"cart.>"},
		MaxAge:   72 * time.Hour,
		Storage:  jetstream.FileStorage,
	}); err != nil {
		log.Fatalf("create cart jetstream: %v", err)
	}

	if _, err := js.CreateOrUpdateStream(context.Background(), jetstream.StreamConfig{
		Name:     "checkout",
		Subjects: []string{"checkout.>"},
		MaxAge:   72 * time.Hour,
		Storage:  jetstream.FileStorage,
	}); err != nil {
		log.Fatalf("create checkout jetstream: %v", err)
	}

	if _, err := js.CreateOrUpdateStream(context.Background(), jetstream.StreamConfig{
		Name:     "orders",
		Subjects: []string{"order.>"},
		MaxAge:   72 * time.Hour,
		Storage:  jetstream.FileStorage,
	}); err != nil {
		log.Fatalf("create orders jetstream: %v", err)
	}

	cartRepo := infraCart.NewPostgresCartRepository(db, rdb)
	cartPub := infraCart.NewNATSCartEventPublisher(js)
	cartSvc := domainCart.NewCartService(cartRepo, cartPub, logger)
	cartHandler := rest.NewCartHandler(cartSvc, sf)

	defaultTaxSvc := domainCheckout.NewDefaultTaxService()
	defaultPriceCalc := domainCheckout.NewDefaultPriceCalculator()
	checkoutRepo := infraCheckout.NewPostgresCheckoutRepository(db, rdb)
	checkoutPub := infraCheckout.NewNATSCheckoutEventPublisher(js)
	checkoutSvc := domainCheckout.NewCheckoutService(checkoutRepo, defaultTaxSvc, defaultPriceCalc, checkoutPub, logger)
	checkoutHandler := rest.NewCheckoutHandler(checkoutSvc, sf)

	orderRepo := infraOrder.NewPostgresOrderRepository(db, rdb)
	orderPub := infraOrder.NewNATSOrderEventPublisher(js)
	orderSvc := domainOrder.NewOrderService(orderRepo, orderPub, logger)
	orderHandler := rest.NewOrderHandler(orderSvc)

	saga := appOrder.NewCheckoutCompletedSaga(orderSvc, sf, logger)

	go func() {
		cons, err := js.CreateOrUpdateConsumer(context.Background(), "checkout", jetstream.ConsumerConfig{
			Name:          "order-saga",
			FilterSubject: "checkout.updated",
			AckPolicy:     jetstream.AckExplicitPolicy,
		})
		if err != nil {
			log.Fatalf("create checkout consumer: %v", err)
		}

		cons.Consume(func(msg jetstream.Msg) {
			msg.Ack()
			if err := saga.Handle(context.Background(), msg.Data()); err != nil {
				log.Printf("saga: handle failed: %v", err)
			}
		})
	}()

	srv := gozerorest.MustNewServer(gozerorest.RestConf{
		Host:    "0.0.0.0",
		Port:    mustParsePort(port),
		Timeout: 30000,
	})

	supportedCaps := []string{"dev.ucp.shopping.catalog", "dev.ucp.shopping.cart", "dev.ucp.shopping.checkout", "dev.ucp.shopping.order"}
	srv.Use(gozerorest.ToMiddleware(middleware.UCPAgentMiddleware))
	srv.Use(gozerorest.ToMiddleware(middleware.NegotiationMiddleware(supportedCaps)))

	auth := middleware.AuthMiddleware(jwtSecret)

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/.well-known/ucp",
		Handler: ucpHandler.ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/auth/register",
		Handler: identityHandler.Register,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/auth/login",
		Handler: identityHandler.Login,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/users/:id",
		Handler: identityHandler.GetUser,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/users/:id/suspend",
		Handler: identityHandler.SuspendUser,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/oauth/authorize",
		Handler: oauthHandler.Authorize,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/oauth/token",
		Handler: oauthHandler.Token,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/oauth/revoke",
		Handler: oauthHandler.Revoke,
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/carts",
		Handler: auth(http.HandlerFunc(cartHandler.CreateOrGet)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/carts/:id",
		Handler: auth(http.HandlerFunc(cartHandler.GetCart)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/carts/:id/items",
		Handler: auth(http.HandlerFunc(cartHandler.AddItem)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPut,
		Path:    "/api/v1/carts/:id/items/:productId",
		Handler: auth(http.HandlerFunc(cartHandler.UpdateQuantity)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodDelete,
		Path:    "/api/v1/carts/:id/items/:productId",
		Handler: auth(http.HandlerFunc(cartHandler.RemoveItem)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodDelete,
		Path:    "/api/v1/carts/:id",
		Handler: auth(http.HandlerFunc(cartHandler.ClearCart)).ServeHTTP,
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts",
		Handler: auth(http.HandlerFunc(checkoutHandler.Create)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/checkouts/:id",
		Handler: auth(http.HandlerFunc(checkoutHandler.GetCheckout)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts/:id/shipping-address",
		Handler: auth(http.HandlerFunc(checkoutHandler.SetShippingAddress)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts/:id/billing-address",
		Handler: auth(http.HandlerFunc(checkoutHandler.SetBillingAddress)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts/:id/shipping-option",
		Handler: auth(http.HandlerFunc(checkoutHandler.SelectShippingOption)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts/:id/payment-handler",
		Handler: auth(http.HandlerFunc(checkoutHandler.SelectPaymentHandler)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts/:id/complete",
		Handler: auth(http.HandlerFunc(checkoutHandler.Complete)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts/:id/cancel",
		Handler: auth(http.HandlerFunc(checkoutHandler.Cancel)).ServeHTTP,
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/orders",
		Handler: auth(http.HandlerFunc(orderHandler.ListByUser)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/orders/:id",
		Handler: auth(http.HandlerFunc(orderHandler.GetOrder)).ServeHTTP,
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Printf("Starting server on :%s\n", port)
		srv.Start()
	}()

	<-quit
	fmt.Println("\nShutting down...")
	srv.Stop()
}

func seedOAuthClient(repo domainOAuth.OAuthClientRepository, logger kernel.Logger) {
	ctx := context.Background()
	if _, err := repo.FindByClientID(ctx, "web"); err == nil {
		return
	}

	client, err := domainOAuth.NewClient(1, "web", "web-secret", []string{"/oauth/callback"}, []string{"openid", "profile", "email", "read", "write"})
	if err != nil {
		logger.Error(ctx, "failed to create default OAuth client", err)
		return
	}
	if err := repo.Save(ctx, client); err != nil {
		logger.Error(ctx, "failed to save default OAuth client", err)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" {
			continue
		}
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func mustParsePort(port string) int {
	var p int
	if _, err := fmt.Sscanf(port, "%d", &p); err != nil || p <= 0 || p > 65535 {
		log.Fatalf("invalid port: %s", port)
	}
	return p
}

type stdLogger struct{}

func (stdLogger) Debug(_ context.Context, msg string, fields ...kernel.LogField) {
	log.Println("DEBUG", msg, fields)
}

func (stdLogger) Info(_ context.Context, msg string, fields ...kernel.LogField) {
	log.Println("INFO", msg, fields)
}

func (stdLogger) Warn(_ context.Context, msg string, fields ...kernel.LogField) {
	log.Println("WARN", msg, fields)
}

func (stdLogger) Error(_ context.Context, msg string, err error, fields ...kernel.LogField) {
	log.Println("ERROR", msg, err, fields)
}
