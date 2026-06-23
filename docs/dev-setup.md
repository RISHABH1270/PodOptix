# PodOptix — Developer Setup Guide

Everything you need to go from zero to a running local development environment.

---

## Prerequisites

Make sure you have the following installed before starting:

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.23+ | `brew install go` |
| Docker | 28+ | [docker.com/get-started](https://www.docker.com/get-started) |
| Git | Any | `brew install git` |

Verify installations:
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

The default values in `.env` are already configured to work with the local Docker setup.
No changes needed for local development.

---

## Step 4 — Start Local Database and Cache

```bash
docker compose up -d
```

This starts:
- **PostgreSQL** on port `5432`
- **Redis** on port `6379`

Verify both are running:
```bash
docker ps
```

---

## Step 5 — Run the App

```bash
export $(cat .env | xargs) && go run ./cmd/hub
```

The app will automatically:
1. Create the database if it does not exist
2. Sync the database schema (run migrations)
3. Initialize the connection pool
4. Start the HTTP server on port `8080`

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

## Step 6 — Verify the Server

```bash
curl http://localhost:8080/healthz
```

Expected response:
```json
{"status":"ok"}
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
    "name":           "production-cluster",
    "prometheus_url": "https://prometheus.example.com",
    "token":          "your-prometheus-token",
    "lookback_window": "7d"
  }'
```

---

## Running Tests

Tests create and destroy their own isolated database automatically — no manual setup needed.

```bash
go test ./internal/api/ -v
```

Run all tests across the project:
```bash
go test ./...
```

---

## Common Commands

| Command | Description |
|---------|-------------|
| `go run ./cmd/hub` | Run the app locally |
| `go build ./...` | Build all packages |
| `go test ./...` | Run all tests |
| `go fmt ./...` | Format all Go files |
| `docker compose up -d` | Start PostgreSQL + Redis |
| `docker compose down` | Stop all containers |
| `docker compose down -v` | Stop containers and delete all data |
| `docker ps` | Check running containers |

---

## Project Structure

```
PodOptix/
├── cmd/hub/            ← app entry point (main.go)
├── internal/
│   ├── api/            ← HTTP server, routes, handlers
│   ├── config/         ← environment variable loading
│   ├── store/          ← database layer (PostgreSQL)
│   ├── collector/      ← Prometheus PromQL queries (coming soon)
│   ├── compute/        ← p99 computation engine (coming soon)
│   └── recommender/    ← recommendation engine (coming soon)
├── pkg/models/         ← shared data models
├── migrations/         ← SQL schema files
├── deploy/helm/        ← Helm chart (coming soon)
├── docs/               ← architecture, decisions, best practices
├── docker-compose.yml  ← local PostgreSQL + Redis
├── .env.example        ← environment variable template
└── go.mod              ← Go module dependencies
```

---

## Troubleshooting

**Port 8080 already in use:**
```bash
lsof -i :8080          # find what is using the port
kill -9 <PID>          # kill it
```

**Database connection refused:**
```bash
docker ps              # check if PostgreSQL container is running
docker compose up -d   # start it if not running
```

**Schema sync failed — dirty database:**
```bash
docker exec -it podoptix-db psql -U postgres -d podoptix \
  -c "DROP TABLE IF EXISTS schema_migrations;"
# then restart the app
```

**Environment variables not loaded:**
```bash
export $(cat .env | xargs) && go run ./cmd/hub
```
