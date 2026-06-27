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

`.env` contents:
```
PORT=8080
DATABASE_URL=postgres://postgres:password@localhost:5432/podoptix?sslmode=disable
REDIS_URL=redis://localhost:6379
JWT_SECRET=my-local-dev-secret-key
```

`.env` is in `.gitignore` — never committed to Git.

---

## Step 4 — Start Local Database and Cache

```bash
docker compose up -d
```

Starts:
- **PostgreSQL** on port `5432` (container: `podoptix-db`)
- **Redis** on port `6379` (container: `podoptix-redis`)

Verify:
```bash
docker ps
```

---

## Step 5 — Run the App

```bash
export $(cat .env | xargs) && go run ./cmd/hub
```

The app automatically:
1. Creates the `podoptix` database if it does not exist
2. Syncs database schema (runs migrations)
3. Initializes connection pool
4. Starts HTTP server on port `8080`

Expected output:
```
  PodOptix  —  Kubernetes Resource Right-Sizing  —  Powered by p99
  ──────────────────────────────────────────────────────────────
  Version  : v0.1.0
  Status   : Starting...
  Port     : 8080
  ──────────────────────────────────────────────────────────────

  Database : Schema synced · Connection pool ready
  ──────────────────────────────────────────────────────────────
  Status   : Server Running
  Listening: port 8080
  ──────────────────────────────────────────────────────────────
```

---

## Step 6 — Verify

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

---

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/healthz` | Health check |
| `GET` | `/api/v1/clusters` | List all clusters |
| `POST` | `/api/v1/clusters` | Register a new cluster |
| `GET` | `/api/v1/clusters/:id` | Get a cluster by ID |
| `DELETE` | `/api/v1/clusters/:id` | Delete a cluster |
| `GET` | `/api/v1/clusters/:id/recommendations` | Get recommendations for a cluster |

### Example — Register a cluster

```bash
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -d '{
    "name":            "production-cluster",
    "prometheus_url":  "https://prometheus.example.com",
    "token":           "your-prometheus-token",
    "lookback_window": "7d"
  }'
```

---

## Running Tests

Tests auto-create and destroy their own isolated database — no manual setup needed.

```bash
go test ./internal/api/ -v
```

Run all tests:
```bash
go test ./...
```

---

## Common Commands

| Command | Description |
|---------|-------------|
| `go run ./cmd/hub` | Run the app |
| `go build ./...` | Build all packages |
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
├── cmd/hub/            ← entry point (main.go)
├── internal/
│   ├── api/            ← HTTP server, routes, handlers, middleware, tests
│   ├── config/         ← environment variable loading
│   └── store/          ← PostgreSQL layer (CRUD, migrations, connection pool)
├── pkg/models/         ← shared data models (Cluster, Recommendation)
├── migrations/         ← SQL schema files (*.up.sql)
├── docs/               ← architecture, decisions, best practices, dev setup
├── docker-compose.yml  ← local PostgreSQL + Redis
├── .env.example        ← environment variable template
└── go.mod              ← Go module (equivalent of package.json)
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
docker ps
docker compose up -d
```

**Schema sync failed — dirty database:**
```bash
docker exec -it podoptix-db psql -U postgres -d podoptix \
  -c "DROP TABLE IF EXISTS schema_migrations;"
```

**Environment variables not loaded:**
```bash
export $(cat .env | xargs) && go run ./cmd/hub
```
