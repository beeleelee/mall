# Next-Generation E-Commerce System for AI Agents — Roadmap

## Goal

We are building a **UCP-native, production-ready e-commerce platform** designed from the ground up for the **agentic commerce era** — where AI agents, not just humans, discover products, manage carts, complete purchases, and handle post-purchase workflows on behalf of users.

Our system bridges the gap between traditional high-concurrency e-commerce infrastructure (go-zero microservices, PostgreSQL, Redis, NATS, DTM) and the emerging **Universal Commerce Protocol (UCP)** standard co-developed by Google, Shopify, and leading retailers. It is a reference-quality open-source implementation that any business can deploy to make its catalog, checkout, and order management instantly accessible to any UCP-compliant AI agent or platform.

The platform serves three constituencies:
- **AI agents and platforms** (Gemini, Claude, shopping assistants) — via standard MCP, A2A, and REST transport bindings with dynamic capability discovery
- **Businesses and merchants** — via a clear capability declaration model, flexible payment handler negotiation, and full ownership of customer relationships
- **Human buyers** — via graceful handoff protocols (ECP) when transactions require human judgment

Built with **Go + go-zero**, the system inherits battle-tested patterns for distributed transactions (DTM), caching (Redis sorted sets, multi-layer cache-aside), concurrency (MapReduce, Singleflight, message queues), and observability (OpenTelemetry, Jaeger). We extend these with UCP-native constructs: capability profiles at `/.well-known/ucp`, server-selects negotiation, AP2 mandates for autonomous payments, signed webhooks for order lifecycle events, and embedded checkout for human-in-the-loop flows.

The outcome is a **single integration point** that collapses N×N complexity — any agent, any merchant, any payment provider, one protocol — making agentic commerce practical, secure, and scalable from day one.

---

## Phases

### Phase 1: Foundation — Skeleton + Catalog (Now)

**Goal**: A running system that an AI agent can discover via UCP profile and search/browse products through MCP.

**Deliverables**:

1. **Project scaffolding**
   - Monorepo with DDD layering: `domain/`, `application/`, `infrastructure/`, `interfaces/`
   - Docker Compose for PostgreSQL, Redis, NATS, etcd, DTM
   - CI pipeline (lint + build + test)
   - Shared kernel utilities (snowflake, error codes, interceptors)

2. **UCP Profile endpoint**
   - `GET /.well-known/ucp` returning machine-readable profile
   - Declare `dev.ucp.shopping.catalog` capability with MCP and REST bindings
   - UCP-Agent header parsing middleware
   - Capability negotiation (server-selects intersection)

3. **Catalog capability**
   - Product schema in PostgreSQL with JSONB for flexible attributes
   - go-zero sqlx models with cache-aside (Redis)
   - Product search (by name, category, price range)
   - Product lookup (by ID, SKU)
   - Cursor-based pagination 
   - REST binding (OpenAPI 3.1.0)
   - MCP binding (JSON-RPC 2.0 tools: `search_catalog`, `lookup_catalog`, `get_product`)

4. **Verification**
   - Claude/Gemini agent discovers profile and queries products via MCP
   - `@omnixhq/ucp-client` integration test
   - Automated test: profile → discover → search → lookup

**Tech stack**: Go + go-zero, PostgreSQL, Redis, NATS, etcd, Docker Compose  
**Design model**: Domain-Driven Design (Entity, Value Object, Aggregate, Repository, Domain Service, Application Service)

**Exit criteria**: An AI agent can discover our profile and search+lookup products without a single custom integration.

#### Todo

- [x] **1.1** Scaffold monorepo: `domain/`, `application/`, `infrastructure/`, `interfaces/` directories, `go.mod`, `Makefile`, `.golangci.yml`
- [x] **1.2** Docker Compose: PostgreSQL, Redis, NATS, etcd, DTM, Quickwit, MinIO, Vector, Grafana
- [x] **1.3** Shared kernel: DDD base types (Entity, ValueObject, AggregateRoot), error codes, `Logger` interface, Snowflake ID gen
- [x] **1.4** CI pipeline: lint, build, test steps in GitHub Actions
- [x] **1.5** UCP Profile: `GET /.well-known/ucp` endpoint, capability negotiation middleware, `UCP-Agent` header parsing
- [x] **1.6** Catalog domain: `Product` entity, `ProductRepository` interface, `CatalogService` domain service
- [x] **1.7** Catalog infra: PostgreSQL schema + migration, sqlx repository implementation with Redis cache-aside
- [x] **1.8** Catalog MCP binding: JSON-RPC 2.0 tools `search_catalog`, `lookup_catalog`, `get_product`
- [x] **1.9** Catalog REST binding: endpoints for search, lookup, detail
- [x] **1.10** Verification: UCP profile structure validation + MCP tools/list verification through automated tests

---

### Phase 2: Identity + Cart + Checkout (Now)

**Goal**: An AI agent can link a user identity, build a cart, and complete a purchase checkout end-to-end.

**Exit criteria**: An AI agent can complete a full purchase — from discovery to order confirmation — without human intervention.

#### Todo

- [x] **2.1** Identity domain: `User` aggregate (ID, Email, Password bcrypt hash, Name, Status, Roles), `Password` value object, `IdentityService` (Register, Login, GetUser, SuspendUser), `UserRepository` interface
- [x] **2.2** Identity infra: PostgreSQL schema + migration for `users` table, `PostgresUserRepository` with cache-aside, integration tests
- [x] **2.3** OAuth 2.0 domain: `OAuthClient`, `AuthorizationCode`, `RefreshToken` aggregates, OAuth service (authorize, token exchange, refresh, revoke), auth middleware, app service, 30+ tests
- [x] **2.4** OAuth 2.0 infra: PostgreSQL migrations for OAuth tables (`oauth_clients`, `oauth_authorization_codes`, `oauth_refresh_tokens`), `Postgres` repos with JSON arrays, JWT signing (HS256), integration tests
- [x] **2.5** Cart domain: `Cart` aggregate (`Cart`, `CartItem`, `CartTotal`), `CartRepository` interface, `CartService` (create, add item, update quantity, remove item, get cart, merge), domain events, 30+ tests
- [x] **2.6** Cart infra: PostgreSQL schema + migration `000004` for carts table (JSONB items), `PostgresCartRepository` with Redis cache-aside, integration tests
- [x] **2.7** Cart NATS event: `cart.updated` event publishing on mutations, `CartEventPublisher` interface + NATS implementation via `NATSCartEventPublisher`
- [x] **2.8** Checkout domain: `CheckoutSession` aggregate, UCP state machine (`incomplete → ready_for_complete → completed | cancelled`), `CheckoutService`, `CheckoutRepository` interface, 30+ tests
- [x] **2.9** Checkout domain: `TaxService` domain service (pluggable providers, default passthrough), `PriceCalculator` service with discount extension hook (default sum calculator)
- [x] **2.10** Checkout infra: PostgreSQL schema + migration `000005` for checkout sessions (JSONB payload), `PostgresCheckoutRepository` with Redis cache-aside, `NATSCheckoutEventPublisher`, integration tests
- [x] **2.11** Order domain: `Order` aggregate (`Order`, `OrderLineItem`, `OrderStatus`), state machine (`confirmed → processing → shipped → delivered | returned | cancelled`), `OrderRepository` interface, `OrderService` with 6 domain events, 30+ tests
- [x] **2.12** Order infra: PostgreSQL schema + migration `000006` for orders table, `PostgresOrderRepository` with Redis cache-aside, NATS JetStream publisher, integration tests
- [x] **2.13** Order webhooks: Signed webhook delivery via NATS JetStream, HMAC-SHA256 signatures, HTTP delivery with retries
- [x] **2.14** Interservice: NATS JetStream subjects (`checkout.>` and `order.>`), event schemas, `checkout.completed → order creation` saga
- [x] **2.15** Interservice: DTM saga for order placement (3-step saga: reserve inventory → verify payment → create order, with automatic compensation on failure)
- [x] **2.16** Verification: full e2e test (create user → login → search products → add to cart → create checkout → complete purchase), webhook receiver test

**New dependencies**: `golang.org/x/crypto` (bcrypt) ✅ added, `github.com/zeromicro/go-zero` ✅ added, `github.com/golang-jwt/jwt/v4` ✅ added, `github.com/nats-io/nats.go` ✅ added, DTM (already in stack)

---

### Phase 3: ECP + Payments + Production Hardening ✅

**Goal**: Handle transactions that require human judgment, process real payments via AP2, and harden the system for production traffic.

**Deliverables**:

1. **Embedded Checkout Protocol (ECP)** ✅
   - ✅ JSON-RPC 2.0 channel over WebSocket for bidirectional agent↔merchant communication
   - ✅ Checkout state `requires_escalation` handling
   - ✅ `continue_url` generation and session handoff
   - ✅ ECP message types: `state.update`, `credentials.submit`, `payment.authorize`, `address.select`
   - ~ Agent-side: render embedded checkout iframe with postMessage bridge *(frontend — deferred)*
   - ~ Merchant-side: serve embedded checkout UI *(frontend — deferred)*
   - ✅ Backward compatibility: `continue_url` serves as standard redirect fallback

2. **AP2 (Agent Payments Protocol)** ✅
   - ✅ AP2 mandate lifecycle: `request → approve → execute → settle`
   - ✅ Mandate aggregate: `Mandate`, `MandateScope`, `MandateSignature`
   - [x] DTM Saga for mandate creation (2-step saga: execute → settle, with cancel compensation via existing /api/v1/saga/payment/cancel)
   - ✅ Single-use, scoped payment tokens
   - ✅ AP2 mandate extension: `dev.ucp.shopping.ap2_mandate`
   - ✅ Integration with checkout: mandate verification + execution on `complete_checkout`

3. **Payment handler negotiation** ✅
   - ✅ Payment handler registry (declared in UCP profile)
   - ✅ Handler specifications: Stripe, Shop Pay, Google Pay, Apple Pay, AP2 mandate, Mock
   - ✅ Dynamic negotiation per transaction based on amount, region, requested handler
   - ~ Payment Token Exchange capability *(deferred)*
   - ✅ Mock payment handler for development + testing

4. **Extension capabilities** ✅
   - ✅ **Fulfillment**: `RateCalculator` interface, `DefaultFulfillmentService`, REST endpoint `POST /api/v1/fulfillment/rates`
   - ✅ **Discount**: `DiscountCode` aggregate, `DiscountService`, `PostgresDiscountRepository`, migration `000011`, REST endpoints (create, validate, apply, deactivate)
   - ✅ Extension negotiation: profile declares supported extensions, agent discovers via UCP

5. **Production observability** ✅
   - ✅ Structured JSON logging via zerolog with `service`, `capability`, `trace_id`, `span_id`, `severity`, `timestamp`
   - ✅ Domain layer `Logger` interface (hexagonal)
   - ✅ `infrastructure/logging/zerolog.go` with `WithCapability` method
   - ✅ All services ship JSON to stdout → Vector → Quickwit → Grafana (configs in `infrastructure/`)
   - ✅ Custom `MetricsRecorder` interface + `PrometheusRecorder` implementation
   - ✅ Metrics middleware recording `ucp_capability_requests_total`, `_duration_seconds`, `_error_total`
   - ✅ `/metrics` endpoint for Prometheus scraping
   - ✅ Health check endpoints: `/healthz` (liveness) + `/readyz` (readiness)
   - ✅ OpenTelemetry tracing (3 layers):
     - Layer 1: go-zero automatic transport instrumentation
     - Layer 2: Manual spans in domain services (checkout, order, cart, catalog)
     - Layer 3: NATS trace propagation (`InjectTrace`/`ExtractTrace` in publishers + consumers)
   - ✅ `infrastructure/tracing/tracer.go`, `nats.go`, `middleware.go`, `domain.go`

6. **Production infrastructure** ✅
   - ✅ Multi-stage Dockerfile (`Dockerfile`)
   - ✅ Kubernetes manifests: `deploy/k8s/` (Deployments, Services, ConfigMaps, HPAs, Secrets)
   - ✅ PgBouncer connection pooling in docker-compose
   - ~ Redis Sentinel *(single instance sufficient for dev — upgrade for HA in production)*
   - ✅ NATS JetStream cluster (3 nodes) in docker-compose
   - ✅ Rate limiting middleware per endpoint (token bucket, 100 req/s burst 200)
   - ✅ Circuit breaker middleware for inter-service calls
   - ✅ Graceful shutdown handling (SIGTERM → drain → stop)

7. **Verification** ✅
   - ✅ E2E smoke test (`TestSmoke` in `e2e_test.go`) covering full purchase flow
   - ✅ AP2 mandate flow tests: domain (7 tests), service (5 tests), REST handler (3 tests)
   - ✅ Payment service tests, rate limiter tests, circuit breaker tests, metrics tests
   - ~ Load test: sustained 1000 concurrent checkout sessions *(perf testing — deferred)*
   - ~ Failure test: Redis failover, NATS node loss *(chaos engineering — deferred)*

**New dependencies**: WebSocket library (gorilla/websocket ✅), AP2 mandate domain ✅, PgBouncer ✅, Prometheus client ✅, K8s manifests ✅

**Exit criteria**: ✅ The system handles mixed autonomy — fully agentic purchases via AP2 and human-in-the-loop via ECP — with production-grade observability, resilience, and deployment automation.

---

### Phase 4: Admin & Management APIs (Now)

**Goal**: Provide full platform management capabilities — product CRUD, inventory tracking, order management, user management — through admin-only API endpoints.

**Exit criteria**: A platform admin can manage products, inventory, orders, and users via REST API without direct database access.

#### Todo

- [x] **4.1** Product CRUD: `CreateProduct`, `UpdateProduct`, `DeleteProduct` in `CatalogService` + admin REST handlers
- [x] **4.2** Inventory domain: `InventoryItem` aggregate, `InventoryService` (SetStock, Reserve, Release, Confirm), `InventoryRepository`, domain events, 23 unit tests
- [x] **4.3** Inventory infrastructure: PostgreSQL migration `000013`, `PostgresInventoryRepository` with Redis cache-aside, integration tests
- [x] **4.4** Admin order management: `ListAllOrders` on `OrderService` + admin REST handler
- [x] **4.5** Admin user management: `ListUsers`, `ActivateUser` on `IdentityService` + admin REST handler
- [x] **4.6** Admin middleware: role-based access with `AdminMiddleware` checking `UserRoleAdmin`
- [x] **4.7** Wire everything: admin routes registered in `main.go`, inventory service wired into admin handler

**Routes added**:

| Method | Path | Handler | Auth |
|--------|------|---------|------|
| POST | `/api/v1/admin/products` | CreateProduct | Admin |
| PUT | `/api/v1/admin/products/:id` | UpdateProduct | Admin |
| DELETE | `/api/v1/admin/products/:id` | DeleteProduct | Admin |
| GET | `/api/v1/admin/orders` | ListOrders | Admin |
| GET | `/api/v1/admin/users` | ListUsers | Admin |
| POST | `/api/v1/admin/users/:id/activate` | ActivateUser | Admin |
| POST | `/api/v1/admin/inventory` | SetStock | Admin |
| GET | `/api/v1/admin/inventory/:productId` | GetStock | Admin |
| GET | `/api/v1/admin/inventory/low-stock` | ListLowStock | Admin |

---

### Phase 5: A2A (Agent-to-Agent) Protocol ✅

**Goal**: Enable AI agents from different platforms to discover and delegate tasks to the mall's e-commerce agent via the standard A2A Protocol v1.0.

**Exit criteria**: An A2A-compliant agent can discover the mall's Agent Card, send tasks via JSON-RPC, track task progress, and receive results.

- [x] **5.1** A2A Domain Data Model: `Task`, `Message`, `Part`, `Artifact`, `AgentCard`, `PushNotificationConfig`, `AgentService` — 18 unit tests
- [x] **5.2** A2A Infrastructure: Migration `000014` for `a2a_tasks` + `a2a_push_notification_configs`, `PostgresTaskRepository` with JSONB and cursor pagination, `PostgresPushNotificationConfigRepository` — 11 integration tests
- [x] **5.3** Agent Card: `GET /.well-known/a2a/agent-card` public, `GET /.well-known/a2a/agent-card/extended` authenticated, UCP profile update
- [x] **5.4** A2A JSON-RPC endpoint (`POST /a2a`): tasks/send, tasks/get, tasks/list, tasks/cancel, tasks/sendStream (SSE), tasks/subscribe (SSE), pushConfig CRUD, agent/getCard
- [x] **5.5** Skill handlers bridging A2A tasks to catalog, cart, checkout, order, identity domain services
- [x] **5.6** Wiring in `main.go`: route registration, auth middleware, AgentService with 5 skill handlers

**Key files**: `domain/a2a/`, `infrastructure/a2a/`, `interfaces/rest/a2a.go`
