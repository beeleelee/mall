package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/trace"
	gozerorest "github.com/zeromicro/go-zero/rest"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	appIdentity "github.com/beeleelee/mall/application/identity"
	appOAuth "github.com/beeleelee/mall/application/oauth"
	appOrder "github.com/beeleelee/mall/application/order"
	appPayment "github.com/beeleelee/mall/application/payment"
	domainA2A "github.com/beeleelee/mall/domain/a2a"
	domainCart "github.com/beeleelee/mall/domain/cart"
	domainCatalog "github.com/beeleelee/mall/domain/catalog"
	domainAnalytics "github.com/beeleelee/mall/domain/analytics"
	domainCheckout "github.com/beeleelee/mall/domain/checkout"
	domainDiscount "github.com/beeleelee/mall/domain/discount"
	domainIdentity "github.com/beeleelee/mall/domain/identity"
	domainInventory "github.com/beeleelee/mall/domain/inventory"
	"github.com/beeleelee/mall/domain/kernel"
	domainNotification "github.com/beeleelee/mall/domain/notification"
	domainOAuth "github.com/beeleelee/mall/domain/oauth"
	domainOrder "github.com/beeleelee/mall/domain/order"
	domainPayment "github.com/beeleelee/mall/domain/payment"
	infraA2A "github.com/beeleelee/mall/infrastructure/a2a"
	infraAnalytics "github.com/beeleelee/mall/infrastructure/analytics"
	infraCart "github.com/beeleelee/mall/infrastructure/cart"
	infraCatalog "github.com/beeleelee/mall/infrastructure/catalog"
	infraCheckout "github.com/beeleelee/mall/infrastructure/checkout"
	"github.com/beeleelee/mall/infrastructure/database"
	infraDiscount "github.com/beeleelee/mall/infrastructure/discount"
	infraFulfillment "github.com/beeleelee/mall/infrastructure/fulfillment"
	infraIdentity "github.com/beeleelee/mall/infrastructure/identity"
	infraInventory "github.com/beeleelee/mall/infrastructure/inventory"
	"github.com/beeleelee/mall/infrastructure/logging"
	"github.com/beeleelee/mall/infrastructure/metrics"
	notificationInfra "github.com/beeleelee/mall/infrastructure/notification"
	infraOAuth "github.com/beeleelee/mall/infrastructure/oauth"
	infraOrder "github.com/beeleelee/mall/infrastructure/order"
	infraPayment "github.com/beeleelee/mall/infrastructure/payment"
	infraStorage "github.com/beeleelee/mall/infrastructure/storage"
	"github.com/beeleelee/mall/infrastructure/tracing"
	"github.com/beeleelee/mall/interfaces/mcp"
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

	logger := logging.NewZerologLogger("mall")
	domainSvc := domainIdentity.NewIdentityService(userRepo, logger)
	passwordResetTokenRepo := infraIdentity.NewPostgresPasswordResetTokenRepository(db)
	appSvc := appIdentity.NewIdentityAppService(domainSvc, userRepo, passwordResetTokenRepo, logger, sf)

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

	healthHandler := rest.NewHealthHandler(db, rdb, nc)

	metricsRecorder := metrics.NewPrometheusRecorder("mall")
	metricsMW := middleware.NewMetricsMiddleware(metricsRecorder)

	catalogRepo := infraCatalog.NewPostgresProductRepository(db, rdb)
	catalogSvc := domainCatalog.NewCatalogService(catalogRepo, logger)
	catalogHandler := rest.NewCatalogHandler(catalogSvc)
	mcpRouter := mcp.NewMCPRouter()

	cartRepo := infraCart.NewPostgresCartRepository(db, rdb)
	cartPub := infraCart.NewNATSCartEventPublisher(js)
	cartSvc := domainCart.NewCartService(cartRepo, cartPub, logger)
	cartHandler := rest.NewCartHandler(cartSvc, sf)

	defaultTaxSvc := domainCheckout.NewDefaultTaxService()
	defaultPriceCalc := domainCheckout.NewDefaultPriceCalculator()
	checkoutRepo := infraCheckout.NewPostgresCheckoutRepository(db, rdb)
	checkoutPub := infraCheckout.NewNATSCheckoutEventPublisher(js)

	mandateRepo := infraPayment.NewPostgresMandateRepository(db)
	tokenValidator := infraPayment.NewMockWalletTokenValidator()
	paymentSvc := domainPayment.NewPaymentService(mandateRepo, logger, domainPayment.WithWalletTokenValidator(tokenValidator))
	paymentHandler := rest.NewPaymentHandler(paymentSvc, sf)

	dtmServer := envOrDefault("DTM_SERVER", "http://localhost:36789/api/dtmsvr")
	callbackURL := envOrDefault("SAGA_CALLBACK_URL", "http://localhost:8080")
	sagaSecret := envOrDefault("SAGA_SECRET", "")
	mandateSaga := appPayment.NewDTMMandateSaga(dtmServer, callbackURL, logger, sagaSecret)
	mandateVerifier := infraCheckout.NewCheckoutMandateVerifier(paymentSvc, mandateSaga, tokenValidator)
	checkoutSvc := domainCheckout.NewCheckoutService(checkoutRepo, defaultTaxSvc, defaultPriceCalc, checkoutPub, logger, mandateVerifier)
	checkoutHandler := rest.NewCheckoutHandler(checkoutSvc, sf)
	checkoutWSHandler := rest.NewCheckoutWSHandler(checkoutSvc, logger)

	orderRepo := infraOrder.NewPostgresOrderRepository(db, rdb)
	orderPub := infraOrder.NewNATSOrderEventPublisher(js)
	orderSvc := domainOrder.NewOrderService(orderRepo, orderPub, logger)
	orderHandler := rest.NewOrderHandler(orderSvc)

	webhookRepo := infraOrder.NewPostgresWebhookRepository(db)
	webhookSvc := domainOrder.NewWebhookService(webhookRepo, sf)
	webhookHandler := rest.NewWebhookHandler(webhookSvc)

	discountRepo := infraDiscount.NewPostgresDiscountRepository(db)
	discountSvc := domainDiscount.NewDiscountService(discountRepo, logger)
	discountHandler := rest.NewDiscountHandler(discountSvc, sf)

	fulfillmentSvc := infraFulfillment.NewDefaultFulfillmentService()
	fulfillmentHandler := rest.NewFulfillmentHandler(fulfillmentSvc)

	inventoryRepo := infraInventory.NewPostgresInventoryRepository(db, rdb)
	inventoryLogger := logger.WithCapability("inventory")
	inventorySvc := domainInventory.NewInventoryService(inventoryRepo, inventoryLogger)

	mcpRouter.Register(mcp.NewCatalogMCPHandler(catalogSvc))
	mcpRouter.Register(mcp.NewCartMCPHandler(cartSvc, sf))
	mcpRouter.Register(mcp.NewCheckoutMCPHandler(checkoutSvc, sf))
	mcpRouter.Register(mcp.NewOrderMCPHandler(orderSvc))
	mcpRouter.Register(mcp.NewDiscountMCPHandler(discountSvc, sf))
	mcpRouter.Register(mcp.NewInventoryMCPHandler(inventorySvc, sf))
	mcpRouter.Register(mcp.NewPaymentMCPHandler(paymentSvc, sf))
	mcpRouter.Register(mcp.NewIdentityMCPHandler(domainSvc, sf))
	mcpRouter.Register(mcp.NewWebhookMCPHandler(webhookSvc))
	mcpRouter.Register(mcp.NewFulfillmentMCPHandler(fulfillmentSvc))
	mcpRouter.Register(mcp.NewOAuthMCPHandler(oauthDomainSvc))
	mcpRouter.Register(mcp.NewAdminMCPHandler(catalogSvc, orderSvc, domainSvc, userRepo, inventorySvc, sf))

	sagaHandler := rest.NewSagaHandler(inventorySvc, paymentSvc, checkoutSvc, orderSvc)

	minioEndpoint := envOrDefault("MINIO_ENDPOINT", "localhost:9000")
	minioAccessKey := envOrDefault("MINIO_ACCESS_KEY", "minioadmin")
	minioSecretKey := envOrDefault("MINIO_SECRET_KEY", "minioadmin")
	minioBucket := envOrDefault("MINIO_BUCKET", "mall")
	storageSvc, storErr := infraStorage.NewMinIOStorage(minioEndpoint, minioAccessKey, minioSecretKey, minioBucket, "", false)
	if storErr != nil {
		log.Printf("warning: minio not available, images disabled: %v", storErr)
	}

	webhookDLQ := infraOrder.NewPostgresWebhookDeliveryLogRepository(db, sf)
	categoryRepo := infraCatalog.NewPostgresCategoryRepository(db)
	analyticsRepo := infraAnalytics.NewPostgresAnalyticsRepository(db)
	analyticsSvc := domainAnalytics.NewAnalyticsService(analyticsRepo)
	adminHandler := rest.NewAdminHandler(catalogSvc, orderSvc, appSvc, inventorySvc, storageSvc, categoryRepo, analyticsSvc, sf, db, webhookDLQ)
	adminMW := middleware.AdminMiddleware(userRepo)

	a2aTaskRepo := infraA2A.NewPostgresTaskRepository(db)
	a2aPushRepo := infraA2A.NewPostgresPushNotificationConfigRepository(db)
	a2aSvc := domainA2A.NewAgentService(a2aTaskRepo, a2aPushRepo, logger, sf)

	a2aSvc.RegisterSkill("catalog", &catalogSkillHandler{svc: catalogSvc})
	a2aSvc.RegisterSkill("cart", &cartSkillHandler{svc: cartSvc, sf: sf})
	a2aSvc.RegisterSkill("checkout", &checkoutSkillHandler{svc: checkoutSvc, sf: sf})
	a2aSvc.RegisterSkill("order", &orderSkillHandler{svc: orderSvc})
	a2aSvc.RegisterSkill("identity", &identitySkillHandler{svc: domainSvc, sf: sf})

	a2aHandler := rest.NewA2AHandler(a2aSvc, fmt.Sprintf("http://localhost:%s", port))

	dtmSaga := appOrder.NewDTMCheckoutSaga(orderSvc, dtmServer, callbackURL, sf, logger, sagaSecret)

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
			ctx := tracing.ExtractFromJetStream(msg)
			if err := dtmSaga.Handle(ctx, msg.Data()); err != nil {
				log.Printf("dtm-saga: handle failed: %v", err)
				msg.Nak()
				return
			}
			msg.Ack()
		})
	}()

	webhookDeliverer := infraOrder.NewWebhookDeliverer(infraOrder.WithDeliveryLogRepo(webhookDLQ))

	go func() {
		cons, err := js.CreateOrUpdateConsumer(context.Background(), "orders", jetstream.ConsumerConfig{
			Name:          "webhook-delivery",
			FilterSubject: "order.>",
			AckPolicy:     jetstream.AckExplicitPolicy,
		})
		if err != nil {
			log.Fatalf("create webhook consumer: %v", err)
		}

		cons.Consume(func(msg jetstream.Msg) {
			ctx := tracing.ExtractFromJetStream(msg)

			subject := msg.Subject()
			webhooks, err := webhookRepo.FindByEvent(ctx, subject)
			if err != nil || len(webhooks) == 0 {
				msg.Ack()
				return
			}

			for _, wh := range webhooks {
				if !wh.Active {
					continue
				}
				if err := webhookDeliverer.Deliver(ctx, wh, subject, msg.Data()); err != nil {
					log.Printf("webhook delivery failed for %s (url=%s): %v", subject, wh.URL, err)
				}
			}
			msg.Ack()
		})
	}()

	retryCtx, retryCancel := context.WithCancel(context.Background())
	defer retryCancel()

	retryWorker := infraOrder.NewWebhookRetryWorker(webhookDLQ, webhookRepo, webhookDeliverer, 30*time.Second, logger)
	go retryWorker.Start(retryCtx)

	if smtpHost := envOrDefault("SMTP_HOST", ""); smtpHost != "" {
		smtpPort, _ := strconv.Atoi(envOrDefault("SMTP_PORT", "587"))
		smtpSender := notificationInfra.NewSMTPEmailSender(notificationInfra.SMTPConfig{
			Host:     smtpHost,
			Port:     smtpPort,
			Username: envOrDefault("SMTP_USERNAME", ""),
			Password: envOrDefault("SMTP_PASSWORD", ""),
			From:     envOrDefault("SMTP_FROM", "noreply@mall.example.com"),
		})
		notifSvc := domainNotification.NewNotificationService(smtpSender, logger)
		notificationInfra.StartEmailConsumer(js, notifSvc, userRepo)
	}

	if otelEndpoint := envOrDefault("OTEL_ENDPOINT", ""); otelEndpoint != "" {
		trace.StartAgent(trace.Config{
			Name:     "mall",
			Endpoint: otelEndpoint,
			Sampler:  1.0,
			Batcher:  envOrDefault("OTEL_BATCHER", "otlpgrpc"),
		})
		logger.Info(context.Background(), "telemetry started", kernel.Field("endpoint", otelEndpoint))
	}

	srv := gozerorest.MustNewServer(gozerorest.RestConf{
		Host:    "0.0.0.0",
		Port:    mustParsePort(port),
		Timeout: 30000,
	})

	supportedCaps := []string{"dev.ucp.shopping.catalog", "dev.ucp.shopping.cart", "dev.ucp.shopping.checkout", "dev.ucp.shopping.order", "dev.ucp.shopping.ecp", "dev.ucp.shopping.ap2_mandate", "dev.ucp.shopping.payment_token_exchange", "dev.ucp.shopping.fulfillment", "dev.ucp.shopping.discount", "dev.ucp.shopping.identity", "dev.ucp.shopping.webhook", "dev.ucp.shopping.oauth", "dev.ucp.shopping.inventory", "dev.ucp.shopping.admin", "dev.ucp.shopping.admin.dashboard", "dev.a2a.agent"}
	srv.Use(gozerorest.ToMiddleware(middleware.RequestIDMiddleware))
	srv.Use(gozerorest.ToMiddleware(middleware.CORSMiddleware))
	srv.Use(gozerorest.ToMiddleware(middleware.RecoveryMiddleware))
	srv.Use(gozerorest.ToMiddleware(middleware.UCPAgentMiddleware))
	srv.Use(gozerorest.ToMiddleware(middleware.NegotiationMiddleware(supportedCaps)))
	srv.Use(gozerorest.ToMiddleware(metricsMW.Wrap))

	rateLimiter := middleware.NewRateLimiter(100, 200)
	defer rateLimiter.Stop()
	srv.Use(gozerorest.ToMiddleware(middleware.RateLimitMiddleware(rateLimiter)))

	auth := middleware.AuthMiddleware(jwtSecret)
	sagaAuth := middleware.SagaAuthMiddleware(sagaSecret)

	catalogCB := middleware.NewCircuitBreaker(5, 2, 30*time.Second)
	cartCB := middleware.NewCircuitBreaker(5, 2, 30*time.Second)
	checkoutCB := middleware.NewCircuitBreaker(5, 2, 30*time.Second)
	cb := func(cb *middleware.CircuitBreaker, h http.HandlerFunc) http.HandlerFunc {
		return middleware.CircuitBreakerMiddleware(cb, http.HandlerFunc(h)).ServeHTTP
	}

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/healthz",
		Handler: healthHandler.Livez,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/readyz",
		Handler: healthHandler.Readyz,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/metrics",
		Handler: promhttp.Handler().ServeHTTP,
	})
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
		Path:    "/api/v1/auth/password-reset-request",
		Handler: identityHandler.RequestPasswordReset,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/auth/password-reset",
		Handler: identityHandler.ResetPassword,
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
		Method:  http.MethodGet,
		Path:    "/api/v1/catalog/search",
		Handler: cb(catalogCB, catalogHandler.Search),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/catalog/lookup",
		Handler: cb(catalogCB, catalogHandler.Lookup),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/catalog/products/:id",
		Handler: cb(catalogCB, catalogHandler.GetProduct),
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/mcp",
		Handler: mcpRouter.ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/mcp",
		Handler: mcpRouter.ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/mcp/:sessionId",
		Handler: mcpRouter.ServeHTTP,
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/carts",
		Handler: cb(cartCB, auth(http.HandlerFunc(cartHandler.CreateOrGet)).ServeHTTP),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/carts/:id",
		Handler: cb(cartCB, auth(http.HandlerFunc(cartHandler.GetCart)).ServeHTTP),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/carts/:id/items",
		Handler: cb(cartCB, auth(http.HandlerFunc(cartHandler.AddItem)).ServeHTTP),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPut,
		Path:    "/api/v1/carts/:id/items/:productId",
		Handler: cb(cartCB, auth(http.HandlerFunc(cartHandler.UpdateQuantity)).ServeHTTP),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodDelete,
		Path:    "/api/v1/carts/:id/items/:productId",
		Handler: cb(cartCB, auth(http.HandlerFunc(cartHandler.RemoveItem)).ServeHTTP),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodDelete,
		Path:    "/api/v1/carts/:id",
		Handler: cb(cartCB, auth(http.HandlerFunc(cartHandler.ClearCart)).ServeHTTP),
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts",
		Handler: cb(checkoutCB, auth(http.HandlerFunc(checkoutHandler.Create)).ServeHTTP),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/checkouts/:id",
		Handler: cb(checkoutCB, auth(http.HandlerFunc(checkoutHandler.GetCheckout)).ServeHTTP),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts/:id/shipping-address",
		Handler: cb(checkoutCB, auth(http.HandlerFunc(checkoutHandler.SetShippingAddress)).ServeHTTP),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts/:id/billing-address",
		Handler: cb(checkoutCB, auth(http.HandlerFunc(checkoutHandler.SetBillingAddress)).ServeHTTP),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts/:id/shipping-option",
		Handler: cb(checkoutCB, auth(http.HandlerFunc(checkoutHandler.SelectShippingOption)).ServeHTTP),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts/:id/payment-handler",
		Handler: cb(checkoutCB, auth(http.HandlerFunc(checkoutHandler.SelectPaymentHandler)).ServeHTTP),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts/:id/payment-token",
		Handler: cb(checkoutCB, auth(http.HandlerFunc(checkoutHandler.SubmitPaymentToken)).ServeHTTP),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts/:id/mandate",
		Handler: cb(checkoutCB, auth(http.HandlerFunc(checkoutHandler.SelectMandate)).ServeHTTP),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts/:id/complete",
		Handler: cb(checkoutCB, auth(http.HandlerFunc(checkoutHandler.Complete)).ServeHTTP),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/checkouts/:id/cancel",
		Handler: cb(checkoutCB, auth(http.HandlerFunc(checkoutHandler.Cancel)).ServeHTTP),
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/ws/checkout/:id",
		Handler: cb(checkoutCB, auth(http.HandlerFunc(checkoutWSHandler.ServeWS)).ServeHTTP),
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
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/orders/:id/process",
		Handler: auth(http.HandlerFunc(orderHandler.StartProcessing)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/orders/:id/ship",
		Handler: auth(http.HandlerFunc(orderHandler.Ship)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/orders/:id/deliver",
		Handler: auth(http.HandlerFunc(orderHandler.MarkDelivered)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/orders/:id/return",
		Handler: auth(http.HandlerFunc(orderHandler.ReturnOrder)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/orders/:id/cancel",
		Handler: auth(http.HandlerFunc(orderHandler.CancelOrder)).ServeHTTP,
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/webhooks",
		Handler: auth(http.HandlerFunc(webhookHandler.Register)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/webhooks",
		Handler: auth(http.HandlerFunc(webhookHandler.ListByUser)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodDelete,
		Path:    "/api/v1/webhooks/:id",
		Handler: auth(http.HandlerFunc(webhookHandler.Delete)).ServeHTTP,
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/payments/mandates",
		Handler: auth(http.HandlerFunc(paymentHandler.CreateMandate)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/payments/mandates",
		Handler: auth(http.HandlerFunc(paymentHandler.ListMandates)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/payments/mandates/:id",
		Handler: auth(http.HandlerFunc(paymentHandler.GetMandate)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/payments/mandates/:id/approve",
		Handler: auth(http.HandlerFunc(paymentHandler.ApproveMandate)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/payments/mandates/:id/execute",
		Handler: auth(http.HandlerFunc(paymentHandler.ExecuteMandate)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/payments/mandates/:id/settle",
		Handler: auth(http.HandlerFunc(paymentHandler.SettleMandate)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/payments/mandates/:id/cancel",
		Handler: auth(http.HandlerFunc(paymentHandler.CancelMandate)).ServeHTTP,
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/discounts",
		Handler: auth(http.HandlerFunc(discountHandler.Create)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/discounts/validate",
		Handler: auth(http.HandlerFunc(discountHandler.Validate)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/discounts/apply",
		Handler: auth(http.HandlerFunc(discountHandler.Apply)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/discounts/deactivate",
		Handler: auth(http.HandlerFunc(discountHandler.Deactivate)).ServeHTTP,
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/fulfillment/rates",
		Handler: auth(http.HandlerFunc(fulfillmentHandler.CalculateRates)).ServeHTTP,
	})

	adminAuth := func(handler http.HandlerFunc) http.HandlerFunc {
		return adminMW(auth(http.HandlerFunc(handler))).ServeHTTP
	}

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/admin/products",
		Handler: adminAuth(adminHandler.CreateProduct),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPut,
		Path:    "/api/v1/admin/products/:id",
		Handler: adminAuth(adminHandler.UpdateProduct),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodDelete,
		Path:    "/api/v1/admin/products/:id",
		Handler: adminAuth(adminHandler.DeleteProduct),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/admin/orders",
		Handler: adminAuth(adminHandler.ListOrders),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/admin/users",
		Handler: adminAuth(adminHandler.ListUsers),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/admin/users/:id/activate",
		Handler: adminAuth(adminHandler.ActivateUser),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/admin/inventory",
		Handler: adminAuth(adminHandler.SetStock),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/admin/inventory/:productId",
		Handler: adminAuth(adminHandler.GetStock),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/admin/inventory/low-stock",
		Handler: adminAuth(adminHandler.ListLowStock),
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/admin/products/:id/images",
		Handler: adminAuth(adminHandler.UploadImage),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodDelete,
		Path:    "/api/v1/admin/products/:id/images/:imageId",
		Handler: adminAuth(adminHandler.DeleteImage),
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/admin/categories",
		Handler: adminAuth(adminHandler.ListCategories),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/admin/categories",
		Handler: adminAuth(adminHandler.CreateCategory),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/admin/categories/:id",
		Handler: adminAuth(adminHandler.GetCategory),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPut,
		Path:    "/api/v1/admin/categories/:id",
		Handler: adminAuth(adminHandler.UpdateCategory),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodDelete,
		Path:    "/api/v1/admin/categories/:id",
		Handler: adminAuth(adminHandler.DeleteCategory),
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/admin/webhooks/failed",
		Handler: adminAuth(adminHandler.ListFailedDeliveries),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/api/v1/admin/webhooks/retry/:id",
		Handler: adminAuth(adminHandler.RetryDelivery),
	})

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/admin/dashboard",
		Handler: adminAuth(adminHandler.Dashboard),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/admin/analytics/revenue",
		Handler: adminAuth(adminHandler.RevenueAnalytics),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/admin/analytics/orders",
		Handler: adminAuth(adminHandler.OrderAnalytics),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/admin/analytics/users",
		Handler: adminAuth(adminHandler.UserAnalytics),
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/admin/analytics/products",
		Handler: adminAuth(adminHandler.ProductAnalytics),
	})

	type sagaRoute struct {
		path    string
		handler http.HandlerFunc
	}
	for _, r := range []sagaRoute{
		{"/api/v1/saga/inventory/reserve", sagaHandler.ReserveInventory},
		{"/api/v1/saga/inventory/release", sagaHandler.ReleaseInventory},
		{"/api/v1/saga/inventory/confirm", sagaHandler.ConfirmInventory},
		{"/api/v1/saga/payment/verify", sagaHandler.VerifyPayment},
		{"/api/v1/saga/payment/cancel", sagaHandler.CancelPayment},
		{"/api/v1/saga/order/create", sagaHandler.CreateOrder},
		{"/api/v1/saga/order/cancel", sagaHandler.CancelOrder},
		{"/api/v1/saga/mandate/execute", sagaHandler.ExecuteMandate},
		{"/api/v1/saga/mandate/settle", sagaHandler.SettleMandate},
		{"/api/v1/saga/mandate/rollback", sagaHandler.RollbackMandateSettle},
	} {
		srv.AddRoute(gozerorest.Route{
			Method:  http.MethodPost,
			Path:    r.path,
			Handler: sagaAuth(r.handler).ServeHTTP,
		})
	}

	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/.well-known/a2a/agent-card",
		Handler: a2aHandler.AgentCard,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodGet,
		Path:    "/.well-known/a2a/agent-card/extended",
		Handler: auth(http.HandlerFunc(a2aHandler.ExtendedAgentCard)).ServeHTTP,
	})
	srv.AddRoute(gozerorest.Route{
		Method:  http.MethodPost,
		Path:    "/a2a",
		Handler: auth(http.HandlerFunc(a2aHandler.ServeJSONRPC)).ServeHTTP,
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Printf("Starting server on :%s\n", port)
		srv.Start()
	}()

	<-quit
	fmt.Println("\nShutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	done := make(chan struct{}, 1)
	go func() {
		if err := nc.Drain(); err != nil {
			logger.Error(shutdownCtx, "nats drain failed", err)
		}
		srv.Stop()
		rdb.Close()
		db.Close()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("Shutdown complete")
	case <-shutdownCtx.Done():
		fmt.Println("Shutdown timed out, forcing exit")
	}
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

type catalogSkillHandler struct {
	svc *domainCatalog.CatalogService
}

func (h *catalogSkillHandler) Handle(ctx context.Context, task *domainA2A.Task, msg domainA2A.Message) error {
	var query string
	for _, p := range msg.Parts {
		if p.Type == domainA2A.PartTypeText {
			query = p.Text
			break
		}
	}
	if query == "" {
		return nil
	}

	opts := domainCatalog.SearchOptions{Limit: 10}
	result, err := h.svc.Search(ctx, query, opts)
	if err != nil {
		return err
	}

	summary := fmt.Sprintf("Found %d products", len(result.Products))
	task.AddArtifact(domainA2A.Artifact{
		ID:    "search-results",
		Name:  "Catalog Search Results",
		Parts: []domainA2A.Part{domainA2A.TextPart(summary)},
	})
	return nil
}

type cartSkillHandler struct {
	svc *domainCart.CartService
	sf  *kernel.Snowflake
}

func (h *cartSkillHandler) Handle(ctx context.Context, task *domainA2A.Task, msg domainA2A.Message) error {
	task.AddArtifact(domainA2A.Artifact{
		ID:    "cart-result",
		Name:  "Cart Response",
		Parts: []domainA2A.Part{domainA2A.TextPart("cart operation received for user " + task.UserID.String())},
	})
	return nil
}

type checkoutSkillHandler struct {
	svc *domainCheckout.CheckoutService
	sf  *kernel.Snowflake
}

func (h *checkoutSkillHandler) Handle(ctx context.Context, task *domainA2A.Task, msg domainA2A.Message) error {
	task.AddArtifact(domainA2A.Artifact{
		ID:    "checkout-result",
		Name:  "Checkout Response",
		Parts: []domainA2A.Part{domainA2A.TextPart("checkout operation received")},
	})
	return nil
}

type orderSkillHandler struct {
	svc *domainOrder.OrderService
}

func (h *orderSkillHandler) Handle(ctx context.Context, task *domainA2A.Task, msg domainA2A.Message) error {
	orders, err := h.svc.GetOrdersByUser(ctx, task.UserID)
	if err != nil {
		return err
	}

	orderStr := fmt.Sprintf("Found %d orders", len(orders))
	task.AddArtifact(domainA2A.Artifact{
		ID:    "orders-result",
		Name:  "Orders List",
		Parts: []domainA2A.Part{domainA2A.TextPart(orderStr)},
	})
	return nil
}

type identitySkillHandler struct {
	svc *domainIdentity.IdentityService
	sf  *kernel.Snowflake
}

func (h *identitySkillHandler) Handle(ctx context.Context, task *domainA2A.Task, msg domainA2A.Message) error {
	task.AddArtifact(domainA2A.Artifact{
		ID:    "identity-result",
		Name:  "Identity Response",
		Parts: []domainA2A.Part{domainA2A.TextPart("identity operation received")},
	})
	return nil
}

func mustParsePort(port string) int {
	var p int
	if _, err := fmt.Sscanf(port, "%d", &p); err != nil || p <= 0 || p > 65535 {
		log.Fatalf("invalid port: %s", port)
	}
	return p
}
