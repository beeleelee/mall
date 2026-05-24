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
- [~] **1.8** Catalog MCP binding: JSON-RPC 2.0 tools `search_catalog`, `lookup_catalog`, `get_product` *(deferred — build after Phase 2 transport layer)*
- [~] **1.9** Catalog REST binding: OpenAPI 3.1.0 endpoints for search, lookup, detail *(deferred — build after Phase 2 transport layer)*
- [~] **1.10** Verification: Claude agent end-to-end discovery → search → lookup, `@omnixhq/ucp-client` integration test *(deferred)*

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
- [ ] **2.13** Order webhooks: Signed webhook delivery via NATS JetStream (at-least-once), detached JWS signature verification per UCP spec
- [x] **2.14** Interservice: NATS JetStream subjects (`checkout.>` and `order.>`), event schemas, `checkout.completed → order creation` saga
- [ ] **2.15** Interservice: DTM saga for order placement (checkout completed → reserve inventory → capture payment → confirm order)
- [ ] **2.16** Verification: full e2e test (create user → login → search products → add to cart → create checkout → complete purchase), webhook receiver test

**New dependencies**: `golang.org/x/crypto` (bcrypt) ✅ added, `github.com/zeromicro/go-zero` ✅ added, `github.com/golang-jwt/jwt/v4` ✅ added, `github.com/nats-io/nats.go` ✅ added, DTM (already in stack)

---

### Phase 3: ECP + Payments + Production Hardening (Next + 1)

**Goal**: Handle transactions that require human judgment, process real payments via AP2, and harden the system for production traffic.

**Deliverables**:

1. **Embedded Checkout Protocol (ECP)**
   - JSON-RPC 2.0 channel over WebSocket for bidirectional agent↔merchant communication
   - Checkout state `requires_escalation` handling
   - `continue_url` generation and session handoff
   - ECP message types: `state.update`, `credentials.submit`, `payment.authorize`, `address.select`
   - Agent-side: render embedded checkout iframe with postMessage bridge
   - Merchant-side: serve embedded checkout UI with delegated payment + address selection
   - Backward compatibility: agents that don't support ECP receive a standard redirect URL

2. **AP2 (Agent Payments Protocol)**
   - AP2 mandate lifecycle: `request → approve → execute → settle`
   - Mandate aggregate: `Mandate`, `MandateScope` (amount, merchant, expiry), `MandateSignature`
   - DTM Saga for mandate creation across identity service + payment service
   - Single-use, scoped payment tokens (not standing API credentials)
   - AP2 mandate extension: `dev.ucp.shopping.ap2_mandate`
   - Integration with checkout: on `complete_checkout`, agent presents AP2 mandate → merchant verifies → payment captured

3. **Payment handler negotiation**
   - Payment handler registry (declared in UCP profile)
   - Handler specifications per provider (Shop Pay, Google Pay, Stripe, etc.)
   - Dynamic negotiation per transaction based on cart contents, buyer region, amount
   - Payment Token Exchange capability for PSP ↔ Credential Provider communication
   - Mock payment handler for development + testing

4. **Extension capabilities**
   - **Fulfillment** (`dev.ucp.shopping.fulfillment`): shipping options, rate calculation, destination selection, fulfillment group management
   - **Discount** (`dev.ucp.shopping.discount`): discount code creation, validation, application, stacking rules
   - Extension negotiation: profile declares supported extensions, agent discovers and invokes
   - Domain services: `FulfillmentService`, `DiscountService` with pluggable providers

5. **Production observability**

   **Logs — Structured + Aggregated**
   - Structured JSON logging via zerolog adapter (replaces logx console output)
   - Every log entry includes: `service`, `capability`, `trace_id`, `span_id`, `severity`, `timestamp`
   - Domain layer emits logs through a `Logger` interface (hexagonal), not directly to zerolog:
     ```go
     type Logger interface {
         Info(ctx context.Context, msg string, fields ...LogField)
         Error(ctx context.Context, msg string, err error, fields ...LogField)
         // ...
     }
     ```
   - Implementation in `infrastructure/logging/` bridges the interface to zerolog
   - All services ship JSON to stdout (container-friendly) → collected by **Vector** (daemon set) →
     routed to **Quickwit** (indexed storage) → queried via **Grafana**
   - Vector handles: parsing, filtering, enrichment (k8s metadata), and routing
   - Log retention: 7 days hot (Loki), 30 days cold (object storage)
   - Alert rules on error rate spikes: `rate({service="checkout"} |= "error" [5m]) > 0.05`

   **Metrics — Per Capability + Per Service**
   - go-zero exports basic HTTP/gRPC metrics (request count, duration, error code) to Prometheus automatically
   - Custom metrics via `infrastructure/metrics/` package with standardized labels:
     ```go
     type MetricsRecorder interface {
         IncCounter(name string, labels ...Label)
         ObserveHistogram(name string, value float64, labels ...Label)
         SetGauge(name string, value float64, labels ...Label)
     }
     ```
   - Recorded at capability boundaries (application layer), not sprinkled through domain logic
   - **Per-capability metrics** (tracked for every UCP capability):

     | Metric | Type | Labels |
     |--------|------|--------|
     | `ucp_capability_requests_total` | Counter | `capability`, `transport`, `status` |
     | `ucp_capability_duration_seconds` | Histogram | `capability`, `transport` |
     | `ucp_capability_active_sessions` | Gauge | `capability` |
     | `ucp_capability_error_total` | Counter | `capability`, `error_code` |

   - **Per-infrastructure metrics**:

     | Metric | Type | Source |
     |--------|------|--------|
     | `nats_jetstream_consumer_lag` | Gauge | NATS exporter |
     | `pg_connection_pool_usage` | Gauge | PgBouncer exporter |
     | `redis_hit_ratio` | Gauge | Redis exporter |
     | `go_routine_count` | Gauge | go-zero built-in |

   - Prometheus scrapes all services + exporters → alertmanager for paging
   - **Dashboards** in Grafana per domain:
     - Catalog: search latency p50/p95/p99, cache hit ratio, query throughput
     - Checkout: session duration, completion rate, escalation rate, payment handler distribution
     - Orders: throughput, webhook delivery latency, webhook failure rate
     - NATS: consumer lag per subject, delivery retries

   **Health check endpoints**: `/healthz` (liveness) + `/readyz` (readiness) per service
   - `readyz` checks: PostgreSQL reachable, Redis reachable, NATS connected, DTM reachable
   - Structured error codes per domain 

   **OpenTelemetry tracing with Jaeger exporter across three layers**:

     **Layer 1 — Automatic (transport)**: go-zero's built-in instrumentation auto-creates spans for every HTTP/gRPC request. Configured per service YAML:
     ```yaml
     Telemetry:
       Name: catalog-service
       Endpoint: http://jaeger:14268/api/traces
       Sampler: 1.0
     ```
     This gives HTTP method + path, gRPC method, status code, and duration — zero code.

     **Layer 2 — Business (domain)**: Manual spans in domain services with semantic attributes:
     ```go
     import (
         "go.opentelemetry.io/otel"
         "go.opentelemetry.io/otel/attribute"
     )
     var tracer = otel.Tracer("catalog.domain")

     func (s *CatalogService) Search(ctx context.Context, query string) ([]*Product, error) {
         ctx, span := tracer.Start(ctx, "catalog.search",
             trace.WithAttributes(attribute.String("query", query)))
         defer span.End()
         // ... logic
         span.SetAttributes(attribute.Int("results.count", len(results)))
         return results, nil
     }
     ```

     **Layer 3 — Async (NATS)**: Trace context propagation across message boundaries:
     ```go
     // Publisher: inject context into NATS headers
     otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(msg.Header))

     // Consumer: extract and create child span
     ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(msg.Header))
     ctx, span := tracer.Start(ctx, "nats.consume."+msg.Subject)
     defer span.End()
     ```

     Shared implementation in `infrastructure/tracing/`:
     ```
     infrastructure/tracing/
     ├── tracer.go       # TracerProvider init (Jaeger exporter)
     ├── nats.go         # NATS context propagation helpers
     ├── middleware.go   # go-zero HTTP/gRPC middleware (if defaults insufficient)
     └── domain.go       # tracer helpers for domain layer
     ```

6. **Production infrastructure**
   - Dockerfiles for each service (multi-stage builds)
   - Kubernetes manifests: Deployments, Services, ConfigMaps, HPAs
   - PostgreSQL connection pooling with PgBouncer
   - Redis Sentinel for high availability
   - NATS JetStream cluster (3 nodes)
   - Rate limiting middleware per endpoint (token bucket)
   - Circuit breakers for inter-service RPC calls
   - Graceful shutdown handling (SIGTERM → drain → stop)

7. **Verification**
   - E2E test: agent discovers escalation → hands off to ECP → human completes checkout
   - AP2 mandate flow: agent requests mandate → policy approves → checkout completes
   - Load test: sustained 1000 concurrent checkout sessions
   - Failure test: Redis failover, NATS node loss, PostgreSQL replica switch

**New dependencies**: WebSocket library, AP2 library/crypto, PgBouncer, Prometheus client, K8s manifests

**Exit criteria**: The system handles mixed autonomy — fully agentic purchases via AP2 and human-in-the-loop via ECP — with production-grade observability, resilience, and deployment automation.
