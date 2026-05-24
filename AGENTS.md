# AGENTS.md — mall

## What this is

A **UCP-native e-commerce platform** in Go. Phase 1 (skeleton + catalog) is done; Phase 2 (identity, cart, checkout) in progress. `main.go` now has a running go-zero server with identity routes wired.

## Architecture

DDD layering:
| Layer | Path | Status |
|-------|------|--------|
 | Domain | `domain/{catalog,identity,oauth,kernel}/` | Catalog + identity + OAuth done, cart/checkout planned |
| Application | `application/{identity,oauth}/` | Identity + OAuth app services done |
| Infrastructure | `infrastructure/{catalog,identity,oauth,database,...}/` | Catalog + identity + OAuth repos, custom migrator done |
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
- **Identity**: `Password` value object wraps bcrypt hashing. `User` aggregate has status (`active`/`suspended`), roles (`customer`/`admin`), and domain events.
- **OAuth 2.0**: `OAuthClient` aggregate (bcrypt secret hash), `AuthorizationCode` entity (single-use, TTL), `RefreshToken` entity (opaque SHA-256 hash). JWT access tokens via `golang-jwt/jwt/v4`, signed with HS256. `OAuthService` handles authorize, exchange, refresh, revoke flows.
- **Auth middleware**: `interfaces/middleware/auth.go` extracts Bearer JWT, validates signature, injects `UserInfo{UserID, ClientID, Scope}` into request context.
- **Seed client**: `main.go` seeds a default OAuth client (`client_id: "web"`, `client_secret: "web-secret"`) on startup.
- **Application service**: `IdentityAppService` in `application/identity/` generates IDs via Snowflake, delegates to domain. New features should follow this pattern.

## Routes

| Method | Path | Handler |
|--------|------|---------|
| GET | `/.well-known/ucp` | UCP profile |
| POST | `/api/v1/auth/register` | Register user |
| POST | `/api/v1/auth/login` | Login |
| GET | `/api/v1/users/:id` | Get user |
| POST | `/api/v1/users/:id/suspend` | Suspend user |
| POST | `/oauth/authorize` | OAuth authorization code |
| POST | `/oauth/token` | OAuth token exchange / refresh |
| POST | `/oauth/revoke` | OAuth token revocation |

## Key files

- `main.go` — go-zero server, DI wiring, route registration
- `domain/kernel/` — DDD base types
- `infrastructure/database/migrator.go` — custom SQL migration runner
- `roadmap.md` — detailed project plan

## Avoid

- Do not add frameworks besides go-zero
- Do not use UUIDs — use `kernel.ID` (Snowflake int64)
- Do not implement `Down()` on migrator unless rollback is needed — it's stubbed but not wired
- Do not bypass the application service layer — HTTP handlers should call app services, not domain directly
