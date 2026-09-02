# PodOptix — Developer Setup Guide

Everything you need to go from zero to a running local development environment.

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.26.4+ | `brew install go` |
| Docker | 28+ | [docker.com/get-started](https://www.docker.com/get-started) |
| Git | Any | `brew install git` |

Verify:
```bash
go version
docker --version
git --version
```

---

## Step 1 — Clone the Repository

```bash
git clone https://github.com/RISHABH1270/PodOptix.git
cd PodOptix
git checkout development
```

---

## Step 2 — Install Go Dependencies

```bash
go mod download
```

---

## Step 3 — Set Up Environment Variables

```bash
cp .env.example .env
```

Default values already match the local Docker setup — no changes needed for local development.

`.env` contents (from `.env.example`):
```
PORT=8080
DATABASE_URL=postgres://postgres:password@localhost:5432/podoptix?sslmode=disable
REDIS_URL=redis://localhost:6379
JWT_SECRET=change-me-to-a-long-random-secret
ENCRYPTION_KEY=change-me-to-32-byte-random-key!
```

**Required environment variables:**

| Variable | Required | Description |
|----------|----------|-------------|
| `PORT` | No | HTTP port (default: `8080`) |
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `REDIS_URL` | Yes | Redis connection string |
| `JWT_SECRET` | Yes | Secret for signing JWT tokens — 32+ random characters in production |
| `ENCRYPTION_KEY` | Yes | Exactly 32 bytes — used for AES-256-GCM encryption of Prometheus tokens |

> `.env` is in `.gitignore` — never committed to Git.

---

## Step 4 — Start Local Database and Cache

```bash
docker compose up -d
```

Starts:
- **PostgreSQL 16** on port `5432` (container: `podoptix-db`)
- **Redis 7** on port `6379` (container: `podoptix-redis`)

Verify containers are healthy:
```bash
docker ps
```

---

## Step 5 — Run the App

```bash
export $(cat .env | xargs) && go run ./cmd/hub
```

**7-step startup sequence (automatic):**

```
1. config.Load()          → reads env vars — panics if any required var is missing
2. store.EnsureDatabase() → connects to default "postgres" DB, CREATE DATABASE podoptix if absent
3. store.SyncSchema()     → runs migration files in order (001, 002, 003) — skips already applied
                            auto-fixes dirty migration state on crash recovery
4. store.New()            → opens pgxpool connection pool (max 10, min 2, lifetime 1h, idle 30m)
5. cache.New()            → connects to Redis, verifies with PING
6. scheduler.Start()      → background goroutine — runs collect→recommend→upsert every 24h
7. server.Listen(:8080)   → binds TCP port and starts accepting requests
```

Expected output:
```
  PodOptix  —  Kubernetes Resource Right-Sizing  —  Powered by p99
  ──────────────────────────────────────────────────────────────
  Version  : v0.1.0
  Status   : Starting...
  Port     : 8080
  ──────────────────────────────────────────────────────────────

  Database : Database ready
  Schema   : Schema synced
  Pool     : Connection pool ready
  Redis    : Connected
  Scheduler: Started — runs every 24 hours
  Server   : Up and running on port 8080
  ──────────────────────────────────────────────────────────────
```

---

## Step 6 — Verify the Server

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}

curl http://localhost:8080/readyz
# {"status":"healthy","checks":{"database":"ok","redis":"ok"}}
```

---

## API Quick Reference

### Public Routes (no auth)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/healthz` | Liveness probe — always 200 |
| `GET` | `/readyz` | Readiness probe — checks DB + Redis |
| `POST` | `/auth/register` | Register a new user account |
| `POST` | `/auth/login` | Login and receive JWT token |

### Protected Routes (JWT required)

Add to every request:
```
Authorization: Bearer <jwt_token>
```

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/clusters` | List all clusters |
| `POST` | `/api/v1/clusters` | Register a new cluster |
| `GET` | `/api/v1/clusters/:id` | Get a cluster by ID |
| `PUT` | `/api/v1/clusters/:id` | Update cluster config (all fields optional) |
| `DELETE` | `/api/v1/clusters/:id` | Remove a cluster |
| `GET` | `/api/v1/clusters/:id/recommendations` | Get resource recommendations |
| `POST` | `/api/v1/clusters/:id/recalculate` | Trigger manual recalculation |

### Cluster Status Values

| Status | Meaning |
|--------|---------|
| `connected` | Prometheus reachable — last sync or ping succeeded |
| `disconnected` | Prometheus unreachable — last sync or ping failed |

> Status is set immediately on register (via 10s ping) — never stays in an unknown state.

---

## Example API Calls

### 1. Register a user

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email":    "user@example.com",
    "password": "securepassword"
  }'
```

Response:
```json
{ "token": "eyJhbGci...", "user_id": "...", "email": "user@example.com" }
```

### 2. Login

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email":    "user@example.com",
    "password": "securepassword"
  }'
```

### 3. Register a cluster

```bash
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "cluster_name":     "production-cluster",
    "prometheus_url":   "https://prometheus.example.com",
    "prometheus_token": "your-prometheus-token",
    "lookback_window":  "7d"
  }'
```

Response:
```json
{
  "cluster_id":      "...",
  "cluster_name":    "production-cluster",
  "prometheus_url":  "https://prometheus.example.com",
  "lookback_window": "7d",
  "status":          "connected",
  "created_by":      "user@example.com",
  "last_synced_at":  "not yet synced",
  "created_at":      "2026-08-26T00:00:00Z",
  "updated_at":      "2026-08-26T00:00:00Z"
}
```

> `lookback_window` allowed values: `7d`, `10d`, `30d` — defaults to `7d` if omitted.

### 4. Update a cluster

```bash
curl -X PUT http://localhost:8080/api/v1/clusters/<cluster-id> \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "prometheus_url":   "https://new-prometheus.example.com",
    "prometheus_token": "new-token"
  }'
```

> All fields are optional — only provided fields are updated.

### 5. Delete a cluster

```bash
curl -X DELETE http://localhost:8080/api/v1/clusters/<cluster-id> \
  -H "Authorization: Bearer <token>"
```

### 6. Get recommendations

```bash
curl http://localhost:8080/api/v1/clusters/<cluster-id>/recommendations \
  -H "Authorization: Bearer <token>"
```

Response:
```json
[
  {
    "recommendation_id":     "...",
    "cluster_id":            "...",
    "namespace":             "production",
    "pod_name":              "api-server-7d9f",
    "container_name":        "api",
    "status":                "ready",
    "current_cpu_limit":     2000,
    "current_mem_limit":     512,
    "p99_cpu":               312.4,
    "p99_mem":               198.1,
    "recommended_cpu_limit": 625,
    "recommended_mem_limit": 397,
    "applied":               false,
    "created_at":            "2026-08-26T00:00:00Z",
    "updated_at":            "2026-08-26T06:00:00Z"
  }
]
```

> `status` is `ready` when data is available, `new_service` when the container has no historical data yet.
> `applied` — toggle to `true` once you apply the recommendation to your cluster (used for cost savings tracking).

### 7. Trigger manual recalculation

```bash
curl -X POST http://localhost:8080/api/v1/clusters/<cluster-id>/recalculate \
  -H "Authorization: Bearer <token>"
```

Response: `202 Accepted` — recalculation runs in background, check recommendations in a few minutes.

> Returns `429 Too Many Requests` if a recalculation is already in progress for this cluster.

---

## Running Tests

> Requires docker-compose to be running (PostgreSQL on 5432, Redis on 6379).

Tests use an isolated environment:
- PostgreSQL database: `podoptix_test` (created and dropped automatically)
- Redis index: `1` (production uses `0` — no key collisions)
- Server port: `9090` (production uses `8080`)

```bash
# Run everything (requires docker compose up -d)
# -p 1 runs packages sequentially — prevents interleaved output from parallel packages
go test ./... -count=1 -p 1

# Run only API integration tests
go test ./tests/... -count=1

# Run a specific test group
go test ./tests/... -run TestClusters -count=1
go test ./tests/... -run TestRecommendations -count=1
go test ./tests/... -run TestAuth -count=1
go test ./tests/... -run TestHealth -count=1

# Run a specific subtest
go test ./tests/... -run TestClusters/POST -count=1

# Run unit tests only (no docker needed)
go test ./internal/compute/... -count=1
go test ./internal/collector/... -count=1
go test ./internal/recommender/... -count=1
```

---

## Common Commands

| Command | Description |
|---------|-------------|
| `go run ./cmd/hub` | Run the app |
| `go build ./...` | Build all packages |
| `go test ./tests/... -v` | Run API tests |
| `go test ./...` | Run all tests |
| `go fmt ./...` | Format all Go files |
| `docker compose up -d` | Start PostgreSQL + Redis |
| `docker compose down` | Stop containers |
| `docker compose down -v` | Stop containers and wipe all data |
| `docker ps` | Check running containers |

---

## Project Structure

```
PodOptix/
├── cmd/hub/                ← entry point (main.go)
├── internal/
│   ├── api/                ← HTTP server, routes, handlers, middleware
│   │   └── testkit/        ← integration tests (table-driven, real TCP)
│   ├── auth/               ← JWT + bcrypt + AES-256-GCM
│   ├── cache/              ← Redis client
│   ├── collector/          ← Prometheus HTTP client (PromQL)
│   ├── compute/            ← p99 algorithm
│   ├── config/             ← environment variable loading
│   ├── recommender/        ← p99 × 2 = recommended limit
│   ├── scheduler/          ← cron pipeline (24h interval)
│   └── store/              ← PostgreSQL CRUD + migrations + connection pool
├── pkg/models/             ← shared data models (Cluster, Recommendation, User)
├── migrations/             ← SQL migration files (run in numeric order)
├── docs/                   ← architecture.html, design docs
├── assets/                 ← banner.svg, logo.svg
├── docker-compose.yml      ← local PostgreSQL 16 + Redis 7
├── .env.example            ← environment variable template
└── go.mod                  ← Go module definition
```

---

## Troubleshooting

**Port 8080 already in use:**
```bash
lsof -i :8080
kill -9 <PID>
```

**Database connection refused:**
```bash
docker compose up -d
docker ps   # verify containers are running and healthy
```

**Schema dirty state (app crashed mid-migration):**

`SyncSchema` auto-fixes dirty state on restart — just re-run the app.

**Environment variables not loaded:**
```bash
export $(cat .env | xargs) && go run ./cmd/hub
```

**401 on protected routes:**

1. Register or login to get a JWT token
2. Token expires after 24 hours — re-login if expired
3. Include in every request: `Authorization: Bearer <token>`

**Prometheus token encryption error:**

`ENCRYPTION_KEY` must be exactly 32 bytes. For local dev:
```
ENCRYPTION_KEY=change-me-to-32-byte-random-key!
```
