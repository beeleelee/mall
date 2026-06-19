package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	db    *sqlx.DB
	rdb   *redis.Client
	nc    *nats.Conn
	start time.Time
}

func NewHealthHandler(db *sqlx.DB, rdb *redis.Client, nc *nats.Conn) *HealthHandler {
	return &HealthHandler{db: db, rdb: rdb, nc: nc, start: time.Now()}
}

func (h *HealthHandler) Livez(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"uptime_ms": time.Since(h.start).Milliseconds(),
	})
}

func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	status := http.StatusOK
	checks := map[string]string{}

	if err := h.db.PingContext(ctx); err != nil {
		checks["postgres"] = "unreachable"
		status = http.StatusServiceUnavailable
	} else {
		checks["postgres"] = "ok"
	}

	if err := h.rdb.Ping(ctx).Err(); err != nil {
		checks["redis"] = "unreachable"
		status = http.StatusServiceUnavailable
	} else {
		checks["redis"] = "ok"
	}

	if !h.nc.IsConnected() {
		checks["nats"] = "disconnected"
		status = http.StatusServiceUnavailable
	} else {
		checks["nats"] = "ok"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"checks": checks,
	})
}
