# AGENTS.md — mall

## What this is

A **UCP-native e-commerce platform** in Go. Phase 2 (identity, OAuth, cart, checkout, order) in progress — order domain + infra just completed.

## Architecture

DDD layering:
| Layer | Path | Status |
|-------|------|--------|
 | Domain | `domain/{catalog,identity,oauth,cart,checkout,order,kernel}/` | Catalog + identity + OAuth + cart + checkout + order done |
| Application | `application/{identity,oauth}/` | Identity + OAuth app services done |
| Infrastructure | `infrastructure/{catalog,identity,oauth,cart,checkout,order,database,...}/` | Catalog + identity + OAuth + cart + checkout + order repos, custom migrator done |
| Interfaces | `interfaces/{middleware,rest}/` | UCP profile, middleware, identity + OAuth handlers done |

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

## Key files

- `main.go` — go-zero server, DI wiring, route registration
- `domain/kernel/` — DDD base types
- `infrastructure/database/migrator.go` — custom SQL migration runner
- `infrastructure/cart/publisher.go` — reference for NATS publisher (JetStream publisher)
- `infrastructure/order/webhook.go` — PostgresWebhookRepository, HMAC signer, HTTP delivery with retries
- `interfaces/mcp/catalog.go` — MCP JSON-RPC 2.0 handler (tools/list, tools/call)
- `roadmap.md` — detailed project plan

## Avoid

- Do not add frameworks besides go-zero
- Do not use UUIDs — use `kernel.ID` (Snowflake int64)
- Do not implement `Down()` on migrator unless rollback is needed — it's stubbed but not wired
- Do not bypass the application service layer — HTTP handlers should call app services, not domain directly
