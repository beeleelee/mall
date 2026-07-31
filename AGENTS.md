# AGENTS.md — mall

## What this is

A **UCP-native e-commerce platform** in Go with **A2A (Agent-to-Agent) Protocol** support. All phases complete — catalog, identity, OAuth, cart, checkout, order, inventory, payment/discount/fulfillment, reviews, wishlist, subscriptions, notifications, admin, and A2A agent capabilities.

## Architecture

DDD layering:
| Layer | Path | Status |
|-------|------|--------|
 | Domain | `domain/{catalog,identity,oauth,cart,checkout,order,kernel,a2a,inventory,payment,discount,fulfillment,ecp,review,wishlist,subscription,notification,analytics}/` | All domains done |
| Application | `application/{identity,oauth,order,payment,subscription}/` | Identity + OAuth app services, Checkout→Order saga, subscription billing service |
| Infrastructure | `infrastructure/{catalog,identity,oauth,cart,checkout,order,database,a2a,inventory,payment,discount,fulfillment,review,wishlist,subscription,notification,...}/` | All repos, migrator, NATS publishers + consumers done |
| Interfaces | `interfaces/{middleware,rest,mcp}/` | REST + MCP + A2A handlers done |

Web framework: **go-zero** (`github.com/zeromicro/go-zero`). Do not import gin, chi, or similar.
Persistence: `pgx`/`sqlx` + `go-redis`. Identity uses bcrypt via `golang.org/x/crypto`.

## Key developer commands

| Command | What |
|---------|------|
| `make lint vet build test` | Default workflow (or `make all`) |
| `make test` | `go test -count=1 -race ./...` |
| `make lint` | `golangci-lint run ./...` |
| `make tidy` | `go mod tidy && go mod verify` |
| `scripts/up-test-env.sh` | Start postgres+redis for integration tests |
| `make run` | Start server on `:8080` (reads `.env` if present; override with `PORT`, `DATABASE_URL`, `REDIS_ADDR`) |
| `go run .` | Equivalent to `make run` |
| `make test -run TestSmoke` | Run end-to-end smoke test (needs postgres+redis) |

## Testing patterns

- **Domain tests**: in-memory fake repos in same package (`domain/identity/fake_repository_test.go`). No external deps.
- **App tests**: in-memory fakes in same package (`application/identity/fake_repository_test.go`).
- **Integration tests**: in `infrastructure/identity/`, need postgres+redis. Use `scripts/up-test-env.sh` first, then `go test -count=1 -race ./infrastructure/identity/...`.
- **Interface tests**: `httptest.NewRecorder()`, wire up app service with fakes.
- **Smoke test**: `main_test.go` starts the full server, tests UCP profile + register flow. Needs postgres+redis. Use `go test -count=1 -race -run TestSmoke ./...` or `make test -run TestSmoke`.
- Always use `-count=1` to disable test caching.

## Project conventions

- **IDs**: `kernel.ID` (int64), not UUID. Snowflake generator in `domain/kernel/snowflake.go`.
- **Errors**: `kernel.DomainError` with typed codes. Use `kernel.IsNotFound()`, `kernel.IsAlreadyExists()`, etc. HTTP handlers use `writeDomainError()` to map codes to status codes.
- **Events**: aggregates emit domain events via `AddEvent()` / `Events()` / `ClearEvents()`.
- **Migrations**: custom embed-based system in `infrastructure/database/migrator.go`. Files at `infrastructure/database/migrations/`. Run via code in `main.go`.
- **Logging**: domain-layer `Logger` interface in `kernel`. `main.go` wires a std logger; zerolog planned for Phase 3.
- **Cart**: `Cart` aggregate with `CartItem` value objects, JSONB persistence, domain events (`cart.created`, `cart.updated`, `cart.cleared`, `cart.merged`), `CartService` for mutations, NATS `cart.updated` events via `NATSCartEventPublisher`.
- **Checkout**: `CheckoutSession` aggregate, UCP state machine (`incomplete → ready_for_complete → completed | cancelled`), `CheckoutService`, `TaxService`/`PriceCalculator` interfaces + defaults, JSONB persistence, NATS `checkout.updated` events via `NATSCheckoutEventPublisher`.
- **Order**: `Order` aggregate with `OrderLineItem` value objects, state machine (`confirmed → processing → shipped → delivered | returned | cancelled`), `OrderService` with 6 domain events, JSONB persistence, NATS JetStream publisher (order.> subject). Address/ShippingOption imported from `domain/checkout`.
- **Checkout → Order Saga**: `CheckoutCompletedSaga` in `application/order/` subscribes to `checkout.updated` via JetStream, filters completed events, generates new Snowflake ID, creates order. Idempotent via `FindByCheckoutID`.
- **Identity**: `Password` value object wraps bcrypt hashing. `User` aggregate has status (`active`/`suspended`), roles (`customer`/`admin`), and domain events.
- **OAuth 2.0**: `OAuthClient` aggregate (bcrypt secret hash), `AuthorizationCode` entity (single-use, TTL), `RefreshToken` entity (opaque SHA-256 hash). JWT access tokens via `golang-jwt/jwt/v4`, signed with HS256. `OAuthService` handles authorize, exchange, refresh, revoke flows.
- **Auth middleware**: `interfaces/middleware/auth.go` extracts Bearer JWT, validates signature, injects `UserInfo{UserID, ClientID, Scope}` into request context.
- **Seed client**: `main.go` seeds a default OAuth client (`client_id: "web"`, `client_secret: "web-secret"`) on startup.
- **Application service**: `IdentityAppService` in `application/identity/` generates IDs via Snowflake, delegates to domain. New features should follow this pattern.
- **Webhooks**: `Webhook` aggregate in `domain/order/`, `WebhookRepository` interface, `WebhookService` with Register/ListByUser/Delete. HMAC-SHA256 signatures via `infra/webhook.go`. Delivery consumer subscribes to `order.>` JetStream subject, 3 retries with 1s backoff.
- **Subscription**: `Subscription` aggregate (`pending → trialing/active → past_due → cancelled | expired`), `SubscriptionPlan` aggregate, `SubscriptionService`, `BillingCharger` interface + `MockBillingCharger`. Payment tokens enable recurring billing; `HandleBillingCycle` marks past_due on first charge failure, expired on second. `SubscriptionBillingWorker` (ticker) + `SubscriptionBillingService.ProcessDueBilling` in `application/subscription/billing.go`. NATS `subscription.>` events via `NATSPaymentEventPublisher`-style publisher. Migration `000025`/`000026`.
- **Notification**: `Notification` aggregate + `NotificationPreferences` (channel toggles, `*[]NotificationType` where nil = allow all). `NotificationService` with `WithInAppWriter`, `WithPreferenceRepository`, `WithNotificationRepository`, `WithSnowflake`. `NotificationConsumer` in `infrastructure/notification/` subscribes to `order.>` and `subscription.>` for preference-gated email + in-app. Migration `000027`. REST `/api/v1/notifications*` + MCP tools.

## Admin (requires admin role)

| Method | Path | Handler |
|--------|------|---------|
| POST | `/api/v1/admin/products` | CreateProduct |
| PUT | `/api/v1/admin/products/:id` | UpdateProduct |
| DELETE | `/api/v1/admin/products/:id` | DeleteProduct |
| GET | `/api/v1/admin/orders` | ListOrders |
| GET | `/api/v1/admin/users` | ListUsers |
| POST | `/api/v1/admin/users/:id/activate` | ActivateUser |
| POST | `/api/v1/admin/inventory` | SetStock |
| GET | `/api/v1/admin/inventory/:productId` | GetStock |
| GET | `/api/v1/admin/inventory/low-stock` | ListLowStock |

- **Admin auth**: `AdminMiddleware` wraps the auth middleware and checks `UserRoleAdmin` via `UserRepository`
- **Seed admin**: Manually assign `admin` role to a user via DB or use the existing identity register + direct DB update

## Routes

| Method | Path | Handler | Auth |
|--------|------|---------|------|
| GET | `/.well-known/ucp` | UCP profile | No |
| POST | `/api/v1/auth/register` | Register user | No |
| POST | `/api/v1/auth/login` | Login | No |
| GET | `/api/v1/users/:id` | Get user | No |
| POST | `/api/v1/users/:id/suspend` | Suspend user | No |
| POST | `/oauth/authorize` | OAuth authorization code | No |
| POST | `/oauth/token` | OAuth token exchange / refresh | No |
| POST | `/oauth/revoke` | OAuth token revocation | No |
| GET | `/api/v1/catalog/search` | Search products | No |
| GET | `/api/v1/catalog/lookup` | Lookup by SKU | No |
| GET | `/api/v1/catalog/products/:id` | Get product | No |
| POST | `/mcp` | MCP tools (JSON-RPC 2.0) | No |
| POST | `/api/v1/carts` | Create/get cart | Yes |
| GET | `/api/v1/carts/:id` | Get cart | Yes |
| POST | `/api/v1/carts/:id/items` | Add item | Yes |
| PUT | `/api/v1/carts/:id/items/:productId` | Update qty | Yes |
| DELETE | `/api/v1/carts/:id/items/:productId` | Remove item | Yes |
| DELETE | `/api/v1/carts/:id` | Clear cart | Yes |
| POST | `/api/v1/checkouts` | Create checkout | Yes |
| GET | `/api/v1/checkouts/:id` | Get checkout | Yes |
| POST | `/api/v1/checkouts/:id/shipping-address` | Set shipping address | Yes |
| POST | `/api/v1/checkouts/:id/billing-address` | Set billing address | Yes |
| POST | `/api/v1/checkouts/:id/shipping-option` | Select shipping | Yes |
| POST | `/api/v1/checkouts/:id/payment-handler` | Select payment | Yes |
| POST | `/api/v1/checkouts/:id/complete` | Complete checkout | Yes |
| POST | `/api/v1/checkouts/:id/cancel` | Cancel checkout | Yes |
| GET | `/api/v1/orders` | List user orders | Yes |
| GET | `/api/v1/orders/:id` | Get order | Yes |
| POST | `/api/v1/webhooks` | Register webhook | Yes |
| GET | `/api/v1/webhooks` | List webhooks | Yes |
| DELETE | `/api/v1/webhooks/:id` | Unregister webhook | Yes |
| GET | `/.well-known/a2a/agent-card` | A2A Agent Card | No |
| GET | `/.well-known/a2a/agent-card/extended` | A2A Extended Agent Card | Yes |
| POST | `/a2a` | A2A JSON-RPC (tasks/send, tasks/get, tasks/list, tasks/cancel, pushConfig/create, etc.) | Yes |
| GET | `/api/v1/products/:id/reviews` | List product reviews | No |
| POST | `/api/v1/products/:id/reviews` | Create review | Yes |
| GET | `/api/v1/reviews/:id` | Get review | No |
| DELETE | `/api/v1/reviews/:id` | Delete review | Yes |
| GET | `/api/v1/reviews/user/:id` | List user reviews | No |
| POST | `/api/v1/discounts` | Create discount code | Admin |
| GET | `/api/v1/discounts/validate` | Validate discount code | No |
| POST | `/api/v1/discounts/apply` | Apply discount code | Yes |
| POST | `/api/v1/discounts/deactivate` | Deactivate discount code | Admin |
| GET | `/api/v1/wishlist` | Get wishlist | Yes |
| POST | `/api/v1/wishlist/items` | Add item | Yes |
| DELETE | `/api/v1/wishlist/items/:productId` | Remove item | Yes |
| DELETE | `/api/v1/wishlist` | Clear wishlist | Yes |
| GET | `/api/v1/notifications` | List + unread count | Yes |
| GET | `/api/v1/notifications/unread-count` | Unread count | Yes |
| POST | `/api/v1/notifications/:id/read` | Mark read | Yes |
| POST | `/api/v1/notifications/mark-all-read` | Mark all read | Yes |
| GET | `/api/v1/notifications/preferences` | Get preferences | Yes |
| PUT | `/api/v1/notifications/preferences` | Update preferences | Yes |
| POST | `/api/v1/fulfillment/rates` | Calculate rates | Yes |
| GET | `/api/v1/subscriptions/plans` | List plans | No |
| GET | `/api/v1/subscriptions/plans/:id` | Get plan | No |
| POST | `/api/v1/subscriptions/plans` | Create plan | Admin |
| PUT | `/api/v1/subscriptions/plans/:id` | Update plan | Admin |
| POST | `/api/v1/subscriptions` | Subscribe | Yes |
| GET | `/api/v1/subscriptions` | List user subscriptions | Yes |
| GET | `/api/v1/subscriptions/:id` | Get subscription | Yes |
| POST | `/api/v1/subscriptions/:id/cancel` | Cancel subscription | Yes |
| POST | `/api/v1/subscriptions/:id/change-plan` | Change plan | Yes |
| POST | `/api/v1/subscriptions/:id/activate` | Activate subscription | Yes |
| POST | `/api/v1/subscriptions/:id/payment-token` | Attach payment token | Yes |
| POST | `/api/v1/subscriptions/plans/:id/activate` | Activate plan | Admin |

## Key files

- `main.go` — go-zero server, DI wiring, route registration
- `domain/kernel/` — DDD base types
- `infrastructure/database/migrator.go` — custom SQL migration runner
- `infrastructure/cart/publisher.go` — reference for NATS publisher (JetStream publisher)
- `infrastructure/order/webhook.go` — PostgresWebhookRepository, HMAC signer, HTTP delivery with retries
- `infrastructure/a2a/repository.go` — PostgresTaskRepository with JSONB, cursor pagination
- `infrastructure/notification/consumer.go` — NotificationConsumer (order.> + subscription.> → email + in-app)
- `infrastructure/subscription/billing_worker.go` — ticker-based billing worker pattern
- `interfaces/mcp/catalog.go` — MCP JSON-RPC 2.0 handler (tools/list, tools/call)
- `interfaces/rest/a2a.go` — A2A JSON-RPC router, Agent Card endpoint, SSE streaming
- `domain/a2a/` — A2A data model (Task, Message, Part, Artifact, AgentCard) + AgentService
- `roadmap.md` — detailed project plan

## Avoid

- Do not add frameworks besides go-zero
- Do not use UUIDs — use `kernel.ID` (Snowflake int64)
- Do not implement `Down()` on migrator unless rollback is needed — it's stubbed but not wired
- Do not bypass the application service layer — HTTP handlers should call app services, not domain directly
