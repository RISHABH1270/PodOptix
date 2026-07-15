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
ENCRYPTION_KEY=my-local-dev-encryption-key-32bytes
```

**Required environment variables:**

| Variable | Required | Description |
|----------|----------|-------------|
| `PORT` | No | HTTP port (default: `8080`) |
| `DATABASE_URL` | Yes | PostgreSQL connection string — app refuses to start without this |
| `REDIS_URL` | Yes | Redis connection string — app refuses to start without this |
| `JWT_SECRET` | Yes | Secret for signing JWT tokens — must be 32+ random characters in production |
| `ENCRYPTION_KEY` | Yes | 32-byte key for AES-256-GCM encryption of Prometheus tokens — must be exactly 32 bytes in production |

**`ENCRYPTION_KEY` detail:** Used to encrypt Prometheus auth tokens before storing them in PostgreSQL. In production, generate with:
```bash
openssl rand -hex 16   # produces 32 hex chars = 16 bytes
# or
openssl rand -base64 32
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

**7-step startup sequence** (happens automatically):

```
1. config.Load()           → read env vars — everything needs config
                             panics if DATABASE_URL, REDIS_URL, JWT_SECRET,
                             or ENCRYPTION_KEY are missing
2. printBanner()           → show startup info with version and port
3. store.EnsureDatabase()  → connect to default "postgres" DB,
                             CREATE DATABASE podoptix if it doesn't exist
4. store.SyncSchema()      → run *.up.sql files in numeric order (skips already applied)
                             auto-fixes dirty migration flag if app crashed mid-migration
5. store.New()             → open pgxpool connection pool
                             (max: 10, min: 2 warm, lifetime: 1hr, idle timeout: 30min)
6. defer db.Close()        → register cleanup — closes all connections on shutdown
7. api.NewServer(db)       → wire store, jwtSecret, encryptionKey into server
   server.Start()          → open TCP port, block forever
```

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

### Public (no auth required)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/healthz` | Kubernetes liveness probe |
| `POST` | `/auth/register` | Register a new user account |
| `POST` | `/auth/login` | Login and receive JWT token |

### Protected (JWT required)

All `/api/v1/*` endpoints require:
```
Authorization: Bearer <jwt_token>
```

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/clusters` | List all registered clusters |
| `POST` | `/api/v1/clusters` | Register a new cluster |
| `GET` | `/api/v1/clusters/:id` | Get a cluster by ID |
| `PUT` | `/api/v1/clusters/:id` | Update a cluster's configuration |
| `DELETE` | `/api/v1/clusters/:id` | Remove a cluster |
| `GET` | `/api/v1/clusters/:id/recommendations` | Get recommendations for a cluster |

### Cluster Status Values

Clusters can have one of three status values:

| Status | Meaning |
|--------|---------|
| `pending` | Registered but not yet queried by the scheduler |
| `healthy` | Last collection job succeeded |
| `unhealthy` | Last collection job failed (Prometheus unreachable, auth error, etc.) |

---

## Example API Calls

### Register a user

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email":    "user@example.com",
    "password": "securepassword"
  }'
# returns: { "token": "eyJhbGci...", "user_id": "...", "email": "..." }
```

### Login

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email":    "user@example.com",
    "password": "securepassword"
  }'
# returns: { "token": "eyJhbGci...", "user_id": "...", "email": "..." }
```

### Register a cluster

```bash
curl -X POST http://localhost:8080/api/v1/clusters \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-jwt-token>" \
  -d '{
    "name":            "production-cluster",
    "prometheus_url":  "https://prometheus.example.com",
    "token":           "your-prometheus-token",
    "lookback_window": "7d"
  }'
```

### Update a cluster

```bash
curl -X PUT http://localhost:8080/api/v1/clusters/<cluster-id> \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-jwt-token>" \
  -d '{
    "prometheus_url": "https://new-prometheus.example.com",
    "token":          "new-prometheus-token"
  }'
```

### Get recommendations for a cluster

```bash
curl http://localhost:8080/api/v1/clusters/<cluster-id>/recommendations \
  -H "Authorization: Bearer <your-jwt-token>"
```

---

## Running Tests

Tests auto-create and destroy their own isolated database — no manual setup needed.

```bash
# Run API tests
go test ./internal/api/ -v

# Run collector tests
go test ./internal/collector/ -v

# Run all tests
go test ./...
```

Tests use a separate `podoptix_test` database that is created fresh and dropped automatically for every test run.

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
│   ├── auth/           ← JWT token generation/validation + bcrypt password hashing
│   ├── cache/          ← Redis client
│   ├── collector/      ← queries Prometheus via PromQL HTTP API
│   ├── compute/        ← p99 computation engine
│   ├── config/         ← environment variable loading
│   ├── recommender/    ← p99 × 2 = recommended limit
│   ├── registry/       ← address book of registered clusters
│   ├── scheduler/      ← cron-based job runner (once/day per cluster)
│   └── store/          ← PostgreSQL layer (CRUD, migrations, connection pool)
├── pkg/models/         ← shared data models (Cluster, Recommendation, User)
├── migrations/         ← SQL schema files (*.up.sql), run in numeric order
├── docs/               ← architecture, design, trade-offs, setup guides
├── docker-compose.yml  ← local PostgreSQL + Redis
├── .env.example        ← environment variable template
└── go.mod              ← Go module definition
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

Then restart the app — `SyncSchema` will re-run migrations from the start.

**Environment variables not loaded:**
```bash
export $(cat .env | xargs) && go run ./cmd/hub
```

**App panics on startup with "ENCRYPTION_KEY is required":**

Add `ENCRYPTION_KEY` to your `.env` file. This is a required variable for AES-256-GCM encryption of Prometheus tokens. For local development any 32-character string works:
```
ENCRYPTION_KEY=my-local-dev-encryption-key-32bytes
```

**Missing token for protected routes — 401:**

1. Register or login to get a JWT token
2. Include it in every API request:
```bash
-H "Authorization: Bearer <token>"
```
JWT tokens expire after 24 hours — re-login to get a new one.
