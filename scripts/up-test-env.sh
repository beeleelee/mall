#!/usr/bin/env bash
set -euo pipefail

echo "==> Starting PostgreSQL and Redis for integration tests..."

cd "$(dirname "$0")/.."

docker compose up -d postgres redis

echo "==> Waiting for PostgreSQL..."
until docker compose exec postgres pg_isready -U mall -d mall > /dev/null 2>&1; do
  sleep 1
done

echo "==> Waiting for Redis..."
until docker compose exec redis redis-cli ping > /dev/null 2>&1; do
  sleep 1
done

echo "==> Ready! Run: go test -count=1 -race ./infrastructure/catalog/..."
