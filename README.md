# mall — UCP-native e-commerce platform for AI agents

A **production-ready e-commerce platform** built for the **agentic commerce era**, where AI agents discover products, manage carts, complete purchases, and handle post-purchase workflows on behalf of users. Implements the [Universal Commerce Protocol (UCP)](https://ucp.shopping) and [Agent-to-Agent (A2A) Protocol](https://a2a.org).

## Quick start

```bash
docker compose up -d postgres redis nats
go run .
# Server starts on :8080
```

## Architecture

Domain-Driven Design with four layers:

| Layer | Path | Purpose |
|-------|------|---------|
| Domain | `domain/` | Business logic, aggregates, value objects, events |
| Application | `application/` | Orchestration, sagas, app services |
| Infrastructure | `infrastructure/` | PostgreSQL, Redis, NATS, monitoring |
| Interfaces | `interfaces/` | REST, MCP, A2A, middleware |

## Key capabilities

- **UCP Profile** at `/.well-known/ucp` — 15 declared capabilities
- **MCP** at `POST /mcp` — 52+ JSON-RPC tools across 12 domains
- **A2A Protocol** at `POST /a2a` — task management with streaming
- **REST API** — 80+ endpoints for full commerce flow
- **ECP** — Embedded Checkout Protocol via WebSocket
- **AP2** — Agent Payments Protocol with mandate lifecycle
- **DTM** — Distributed transaction manager for sagas

## Development

```bash
make all        # lint + vet + build + test
make test       # go test -count=1 -race ./...
make lint       # golangci-lint
make run        # start server
```

Integration tests need postgres, redis, and nats:
```bash
scripts/up-test-env.sh
make test -run TestSmoke
```

## Tech stack

Go + go-zero, PostgreSQL 16, Redis 7, NATS JetStream, DTM, OpenTelemetry, Prometheus, Grafana

## License

MIT
