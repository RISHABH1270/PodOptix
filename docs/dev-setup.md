# PodOptix — Developer Guide

A linear walkthrough for a new developer. Start with a fresh laptop, end with the ability to ship a change. Every command in this guide is meant to be run exactly as written.

> Just want to install and use PodOptix? See [../USER_MANUAL.md](../USER_MANUAL.md).

---

## Table of Contents

1. [Prerequisites — install this first](#1-prerequisites)
2. [Clone + first boot (~15 min to running)](#2-clone--first-boot)
3. [Verify it works — click through the dashboard](#3-verify-it-works)
4. [Repo tour — what every folder does](#4-repo-tour)
5. [Backend deep dive — every package explained](#5-backend-deep-dive)
6. [Dashboard deep dive — React app walkthrough](#6-dashboard-deep-dive)
7. [Testing framework — API tests + UI tests](#7-testing-framework)
8. [Build pipeline — Vite → go:embed → single binary](#8-build-pipeline)
9. [Container + Helm chart — how we ship](#9-container--helm-chart)
10. [Make your first change — worked example](#10-make-your-first-change)
11. [Command cheat sheet](#11-command-cheat-sheet)

---

# 1. Prerequisites

Install these once. Each check-mark verifies it worked.

| Tool | Version | macOS | Linux | Windows |
|------|---------|-------|-------|---------|
| **Go** | 1.26+ | `brew install go` | `sudo apt install golang-go` | `winget install GoLang.Go` |
| **Node.js** | 20+ | `brew install node` | `sudo apt install nodejs npm` | `winget install OpenJS.NodeJS` |
| **Docker** | 28+ | [Docker Desktop](https://docker.com/products/docker-desktop) | Docker Engine + Compose plugin | Docker Desktop (WSL2 backend) |
| **Git** | Any | `brew install git` | pre-installed | `winget install Git.Git` |
| **Make** | Any | pre-installed | pre-installed | via Git Bash / WSL |
| **helm** (optional — for K8s work) | 3.8+ | `brew install helm` | [helm.sh/docs/intro/install](https://helm.sh/docs/intro/install/) | `winget install Helm.Helm` |
| **kubectl** (optional — for K8s work) | Any | `brew install kubectl` | [kubernetes.io/docs/tasks/tools](https://kubernetes.io/docs/tasks/tools/) | `winget install Kubernetes.kubectl` |

**Verify:**
```bash
go version         # go version go1.26.x
node --version     # v20.x.x or newer
docker --version   # Docker version 28.x
git --version
make --version
```

**Recommended IDE:** VS Code with these extensions:
- `golang.go` — Go language server
- `dbaeumer.vscode-eslint` — TypeScript linting
- `bradlc.vscode-tailwindcss` — Tailwind autocomplete
- `esbenp.prettier-vscode` — auto-format

---

# 2. Clone + First Boot

### 2.1 Clone

```bash
git clone https://github.com/RISHABH1270/PodOptix.git
cd PodOptix
git checkout development       # active branch
```

### 2.2 Install Go dependencies

```bash
go mod download
```

This reads `go.mod` and pulls every Go library the project depends on into your local module cache (`~/go/pkg/mod`). One-time. Takes ~30s on first run.

### 2.3 Set up environment variables

```bash
cp .env.example .env
```

Open `.env` — the defaults already point at the local Docker services you're about to start. Nothing to change for local dev.

```bash
PORT=8080
DATABASE_URL=postgres://postgres:password@localhost:5432/podoptix?sslmode=disable
REDIS_URL=redis://localhost:6379
JWT_SECRET=change-me-to-a-long-random-secret
ENCRYPTION_KEY=change-me-to-32-byte-random-key!
```

| Variable | Required | Purpose |
|----------|----------|---------|
| `PORT` | No (default 8080) | HTTP port the backend binds |
| `DATABASE_URL` | **Yes** | Postgres connection string |
| `REDIS_URL` | **Yes** | Redis connection string |
| `JWT_SECRET` | **Yes** | Signs JWT tokens — 32+ random chars in prod |
| `ENCRYPTION_KEY` | **Yes** | AES-256-GCM key for Prometheus tokens — exactly 32 bytes |

`.env` is in `.gitignore` — real secrets never get committed.

### 2.4 Start Postgres + Redis + local Prometheus

```bash
docker compose up -d
```

Brings up three containers:

| Container | Image | Port | Purpose |
|-----------|-------|------|---------|
| `podoptix-db` | postgres:16 | 5432 | Database |
| `podoptix-redis` | redis:7-alpine | 6379 | Cache + distributed lock |
| `podoptix-prometheus` | prom/prometheus:v2.54.1 | 9090 | Local Prometheus so you can register a "test cluster" pointing at `http://localhost:9090` |

**Verify all three are healthy:**
```bash
docker ps
# STATUS column should show "Up X seconds (healthy)" for all three
```

### 2.5 Pick your run mode

You now have two ways to run PodOptix locally:

#### Option A — Development mode (hot reload) — recommended while coding

**Two terminals.** Backend on one, Vite dev server on the other:

```bash
# Terminal 1 — Go backend on :8080
export $(cat .env | xargs) && go run ./cmd/hub
```

You should see the startup banner + 9 green OK lines. If any line is red, check the troubleshooting section at the end.

```bash
# Terminal 2 — React dashboard on :5173 (proxies /api/* to :8080)
cd web
npm install     # one-time — pulls React, Vite, Tailwind, ~185 packages
npm run dev
```

Vite prints: `Local: http://localhost:5173/`. Open that.

**Edit any `.tsx` file and save — the browser updates instantly (hot module reload).**

#### Option B — Single binary mode (what production runs)

```bash
make build
# builds the React dashboard, embeds it into the Go binary via go:embed,
# outputs bin/podoptix (~41 MB)

export $(cat .env | xargs) && ./bin/podoptix
```

Open <http://localhost:8080> — same dashboard, same API, one process, one port. This is exactly what customers deploy.

Use Option B to verify a change works end-to-end; use Option A while iterating.

### 2.6 The 9-step startup sequence — what's actually happening

When `main.go` runs, this is what you're watching happen (see [cmd/hub/main.go](../cmd/hub/main.go)):

```
1. config.Load()              → reads env vars, panics if any required var is missing
2. store.EnsureDatabase()     → connects to default "postgres" DB, CREATE DATABASE podoptix if absent
3. store.SyncSchema()         → runs migration files 000001..000003 in order, skips already applied
4. store.New()                → opens pgxpool connection pool (max 10, min 2, lifetime 1h)
5. cache.New()                → connects to Redis, verifies with PING
6. signal.NotifyContext()     → registers SIGTERM/SIGINT for graceful shutdown
7. scheduler.Start()          → background goroutine — runs collect→recommend→upsert every 24h
8. server.Listen(:8080)       → binds TCP port
9. server.Serve()             → accepts requests (blocks until shutdown signal)
```

You'll see this exact output in your terminal — bookmark the pattern; you'll recognize each step in the code later.

---

# 3. Verify It Works

Sanity checks in order.

### 3.1 Health probes

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}

curl http://localhost:8080/readyz
# {"status":"ok","checks":{"postgres":"ok","redis":"ok"}}
```

Both should return 200. `readyz` also confirms Postgres + Redis are reachable.

### 3.2 Register a user via the UI

1. Open the dashboard (5173 in dev mode, 8080 in binary mode)
2. Click **"Create one"** at the bottom of the login screen
3. Email: `dev@localhost.test`, Password: `password123` (min 8)
4. On submit, you land on `/clusters` — the empty state

### 3.3 Register a test cluster

The local Prometheus in `docker-compose.yml` gives you a real endpoint to point at.

1. Click **Register cluster**
2. Fill in:
   - Cluster name: `local-test`
   - Prometheus URL: `http://localhost:9090`
   - Prometheus token: `no-token-needed` (any string — local Prometheus doesn't auth)
   - Lookback: `7d`
3. Click **Register cluster**

You should land on the cluster detail page with status **connected**. In the background, the scheduler is now running the collect → compute → upsert pipeline. Watch your Terminal 1 backend logs — you'll see:

```
INFO  cluster registered name=local-test id=... status=connected by=dev@localhost.test
INFO  scheduler collecting cluster=...
INFO  collector fetching prometheus=http://localhost:9090 lookback=7d
INFO  collector done containers=0 took=45ms
```

`containers=0` because local Prometheus isn't scraping cAdvisor or kube-state-metrics — just itself. That's fine; the pipeline still ran.

### 3.4 Everything works

If you got here, your dev environment is fully functional. Now let's understand what you're looking at.

---

# 4. Repo Tour

```
PodOptix/
├── cmd/hub/                     entry point — main.go and only main.go
│
├── internal/                    Go code the outside world can't import (Go convention)
│   ├── api/                     HTTP layer — Gin server, routes, handlers, middleware
│   ├── auth/                    JWT (HMAC-SHA256), bcrypt, AES-256-GCM
│   ├── cache/                   Redis client (recommendation cache + distributed lock)
│   ├── collector/               Prometheus HTTP client (PromQL)
│   ├── compute/                 the p99 algorithm
│   ├── config/                  env var loading
│   ├── dashboard/               //go:embed of the built React app
│   ├── metrics/                 Prometheus /metrics registrations
│   ├── recommender/             p99 × 2 = recommended limit
│   ├── scheduler/               24h ticker that drives the pipeline
│   └── store/                   PostgreSQL — pgxpool + migrations + CRUD
│
├── pkg/models/                  shared data models — importable from anywhere
│   ├── cluster.go
│   ├── recommendation.go
│   └── user.go
│
├── migrations/                  SQL migration files (numbered, run in order)
│   ├── 000001_create_clusters.up.sql
│   ├── 000002_create_recommendations.up.sql
│   └── 000003_create_users.up.sql
│
├── tests/                       backend integration tests (63 tests)
│   ├── setup_test.go
│   ├── auth_test.go, clusters_test.go, etc.
│   └── TESTING.md               how the test framework works
│
├── web/                         React 18 + TypeScript + Vite + Tailwind dashboard
│   ├── src/
│   │   ├── pages/               one file per URL
│   │   ├── components/          reusable UI pieces
│   │   ├── lib/                 api.ts + auth.ts
│   │   ├── App.tsx              routes
│   │   └── main.tsx             React entry point
│   ├── tests-e2e/               Playwright end-to-end tests (9 tests)
│   │   └── UI_TESTING.md
│   ├── vite.config.ts
│   ├── tailwind.config.js
│   ├── package.json
│   └── DASHBOARD.md
│
├── deploy/helm/podoptix/        Helm chart — customers install this
│   ├── Chart.yaml, values.yaml, README.md
│   └── templates/
│
├── docs/                        design + reference docs
│   ├── dev-setup.md             ← this file
│   ├── hld.md                   High-Level Design
│   ├── lld.md                   Low-Level Design (schema, API contract, Redis, security)
│   ├── engineering-trade-offs.md every decision + why
│   └── architecture.html        interactive architecture diagram (open in browser)
│
├── assets/                      logo.svg, banner.svg, brand tokens
│
├── Dockerfile                   multi-stage build (React → Go → distroless)
├── docker-compose.yml           local Postgres + Redis + Prometheus
├── Makefile                     command runner — make dev / build / test / docker-push / helm-push
├── .env.example                 env var template
├── go.mod / go.sum              Go module manifest + checksums
├── vendor/                      committed vendored deps (works behind corporate SSL proxies)
├── README.md                    project overview + roadmap
├── USER_MANUAL.md               for end users installing PodOptix
└── LICENSE
```

**Go convention:** anything under `internal/` cannot be imported by other Go projects — it's compiler-enforced privacy. Public API surface goes in `pkg/`.

---

# 5. Backend Deep Dive

Every Go package explained. Read in this order — dependencies flow from top to bottom.

## 5.1 `pkg/models/` — the data shape

Plain Go structs shared across every layer. Zero logic, zero dependencies.

**[pkg/models/cluster.go](../pkg/models/cluster.go)** — the Cluster struct, one field per DB column. Tags map to JSON (API) and to DB columns. Constants `ClusterStatusConnected` / `Disconnected` and `LookbackWindow7d/10d/30d`.

**[pkg/models/recommendation.go](../pkg/models/recommendation.go)** — same pattern. `Applied bool` tracks whether the user has applied the recommendation (for the Savings dashboard).

**[pkg/models/user.go](../pkg/models/user.go)** — email + bcrypt hash. `PasswordHash` has `json:"-"` — it's NEVER serialized to an API response.

Read the source first. Under 100 lines total. Everything downstream references these types.

## 5.2 `internal/config/` — env var loading

**[internal/config/config.go](../internal/config/config.go)** — one file, ~60 lines.

```go
func Load() (*Config, error) {
    databaseURL, err := mustGetEnv("DATABASE_URL")
    // ... same for REDIS_URL, JWT_SECRET, ENCRYPTION_KEY
    return &Config{Port: getEnv("PORT", "8080"), DatabaseURL: ..., ...}, nil
}
```

`mustGetEnv` → error if missing. `getEnv` → fallback value. `main.go` calls `config.Load()` first thing — if a required var is missing, the app refuses to start with a clear error message.

## 5.3 `internal/store/` — PostgreSQL

Owns the connection pool, migrations, and every SQL query.

**[internal/store/store.go](../internal/store/store.go)** — the two most important functions:

- `EnsureDatabase(url)` — parses the URL, connects to the default `postgres` DB, `CREATE DATABASE podoptix` if it doesn't exist. Handles the chicken-and-egg (pool can't open a DB that doesn't exist yet).
- `SyncSchema(url)` — uses `golang-migrate` to apply every SQL file in `migrations/`. Idempotent, handles crashed-mid-migration ("dirty") state automatically.
- `New(url)` — opens the pgxpool: max 10, min 2, lifetime 1h, idle 30m.

**[internal/store/cluster.go](../internal/store/cluster.go)** — CRUD for the `clusters` table. `SaveCluster`, `GetCluster`, `ListClusters`, `UpdateCluster`, `DeleteCluster`, `UpdateClusterHealth` (the ONLY function that touches `status` + `last_synced_at` — a discipline that prevents race conditions).

**[internal/store/recommendation.go](../internal/store/recommendation.go)** — `UpsertRecommendation` (INSERT ON CONFLICT), `ListByCluster`, `ListAllWithClusterName` (the cross-cluster view — JOIN with clusters, order by biggest CPU delta first).

**[internal/store/user.go](../internal/store/user.go)** — `CreateUser`, `GetUserByEmail`. That's it.

**[migrations/](../migrations/)** — three files, run in numeric order:
1. `000001_create_clusters` — CHECK constraint on status + lookback_window
2. `000002_create_recommendations` — UNIQUE(cluster_id, namespace, pod, container) — enables upsert
3. `000003_create_users` — email UNIQUE

## 5.4 `internal/cache/` — Redis

**[internal/cache/redis.go](../internal/cache/redis.go)** — one file, three responsibilities:

- **Cache-aside for recommendations** — 3-hour TTL. Key: `cluster:{id}:recommendations`. Serialized as JSON.
- **Distributed lock** — `SETNX` with 10-min TTL. Key: `lock:cluster:{id}:recalculate`. Prevents two recalculates for the same cluster running simultaneously.
- **Ping** — used by the `/readyz` handler.

## 5.5 `internal/auth/` — crypto primitives

Three files, each ~60 lines. Pure functions — no state, easy to test.

**[internal/auth/password.go](../internal/auth/password.go)** — `HashPassword` (bcrypt cost 10 → ~100ms per hash, deliberately slow), `CheckPassword`.

**[internal/auth/jwt.go](../internal/auth/jwt.go)** — `GenerateToken(userID, email, secret)` returns a signed JWT with 24h expiry. `ValidateToken` explicitly checks `t.Method.(*jwt.SigningMethodHMAC)` — prevents the `alg:none` bypass attack.

**[internal/auth/encrypt.go](../internal/auth/encrypt.go)** — AES-256-GCM. Random 12-byte nonce prepended to ciphertext, base64 encoded for DB storage. Used ONLY for Prometheus tokens.

## 5.6 `internal/collector/` — Prometheus HTTP client

**[internal/collector/prometheus.go](../internal/collector/prometheus.go)** — 4 PromQL queries wrapped in Go:

1. `rate(container_cpu_usage_seconds_total[5m]) * 1000` — CPU usage in millicores, over the lookback window
2. `container_memory_working_set_bytes / 1048576` — memory in MiB
3. `kube_pod_container_resource_limits{resource="cpu"} * 1000` — current CPU limit (instant query)
4. `kube_pod_container_resource_limits{resource="memory"} / 1048576` — current memory limit (instant query)

`Collect()` runs all four, merges results by (namespace, pod, container) key, returns `[]*ContainerMetrics`. `Ping()` runs an `up` query — used by cluster registration.

## 5.7 `internal/compute/` — the p99 algorithm

**[internal/compute/p99.go](../internal/compute/p99.go)** — 30 lines.

```go
func ComputeP99(values []float64) (float64, error) {
    sorted := make([]float64, len(values))
    copy(sorted, values)                 // never modify the caller's slice
    sort.Float64s(sorted)
    index := int(math.Ceil(0.99 * float64(len(sorted)))) - 1
    return sorted[index], nil
}
```

That's it. Doesn't know or care about clusters, containers, HTTP. Perfectly testable.

## 5.8 `internal/recommender/` — p99 × 2

**[internal/recommender/recommender.go](../internal/recommender/recommender.go)** — pipes ContainerMetrics through ComputeP99, applies `× 2`, wraps in a Recommendation struct.

Two entry points:
- `Generate(clusterID, *ContainerMetrics)` — one container
- `GenerateAll(clusterID, []*ContainerMetrics)` — the whole cluster (containers with no data get status `new_service`)

## 5.9 `internal/scheduler/` — the pipeline driver

**[internal/scheduler/scheduler.go](../internal/scheduler/scheduler.go)** — a `time.Ticker` running every 24h + one immediate run on startup.

`RunForCluster(ctx, clusterID, url, token, lookbackWindow)`:
1. `collector.Collect()` → `[]*ContainerMetrics`
2. `recommender.GenerateAll()` → `[]*Recommendation`
3. `store.UpsertRecommendation()` for each (loop)
4. `store.UpdateClusterHealth("connected", now)` on success, `"disconnected"` on collect failure
5. Emits Prometheus metrics via `internal/metrics/`

Called from two places: the 24h ticker (`Start()`) and directly from the cluster registration handler (immediate first sync).

## 5.10 `internal/api/` — the HTTP layer

The biggest package. Broken into files by concern.

**[server.go](../internal/api/server.go)** — `Server` struct holds pointers to store, cache, scheduler, config. `NewServer()` builds the Gin engine, adds middleware, calls `registerRoutes()`. `Listen()` + `Serve()` split so `main.go` can bind the port before starting to serve (used for the port-check test).

**[routes.go](../internal/api/routes.go)** — all URL registrations in one place. The order matters:
1. Public routes (`/healthz`, `/readyz`, `/metrics`, `/auth/*`)
2. Protected group `/api/v1` with `JWTMiddleware`
3. `router.NoRoute(...)` — falls through to the embedded React dashboard

**[middleware.go](../internal/api/middleware.go)** — two middlewares:
- `RequestIDMiddleware` — assigns a UUID to every request, stored in Gin context as `"request_id"`, returned in the `X-Request-ID` response header. Every log line and error response includes it — trace an error end-to-end.
- `JWTMiddleware(secret)` — validates the `Authorization: Bearer <token>` header. Sets `"user_id"` and `"email"` in Gin context for downstream handlers.

**[metrics.go](../internal/api/metrics.go)** — Gin middleware that records HTTP latency + count into Prometheus metrics (defined in `internal/metrics/`). Uses `c.FullPath()` so the metric label is the ROUTE TEMPLATE (`/api/v1/clusters/:id`) not the URL — keeps label cardinality bounded.

**[auth.go](../internal/api/auth.go)** — `register` and `login` handlers. See `internal/auth/*` for the crypto.

**[clusters.go](../internal/api/clusters.go)** — CRUD handlers. `createCluster`:
1. `ShouldBindJSON` → 400 if fields missing
2. Validate URL scheme + host
3. `collector.Ping()` — 10-second timeout — sets initial status
4. `auth.Encrypt(token)` — never store plaintext
5. `store.SaveCluster()`
6. `scheduler.RunForCluster()` in a goroutine — immediate first sync
7. Return 201 with `last_synced_at: "not yet synced"`

**[recommendation.go](../internal/api/recommendation.go)** — `listRecommendations` (cache-aside), `listAllRecommendations` (cross-cluster JOIN), `recalculate` (SETNX lock → fire-and-forget background goroutine → 202).

**[health.go](../internal/api/health.go)** — `/healthz` (always 200), `/readyz` (pings DB + Redis).

## 5.11 `internal/dashboard/` — go:embed

**[internal/dashboard/dashboard.go](../internal/dashboard/dashboard.go)** — the `//go:embed all:dist` directive tells Go's compiler to read every file under `dist/` at build time and embed it as bytes in the binary.

`Handler()` returns an `http.Handler` that:
- Serves static files (JS, CSS, favicon) with correct MIME types
- Falls back to `index.html` for any unknown path — enables React Router's client-side routing
- Returns a helpful 503 message if the dashboard hasn't been built yet

Registered in `routes.go` via `router.NoRoute(...)`.

## 5.12 `internal/metrics/` — Prometheus counters + histograms

**[internal/metrics/metrics.go](../internal/metrics/metrics.go)** — defined in its own package (not `api`) so the scheduler can import metrics without a cycle.

Metric families:
- `podoptix_http_requests_total{method, path, status}` (counter)
- `podoptix_http_request_duration_seconds{method, path}` (histogram)
- `podoptix_scheduler_runs_total{outcome}` (counter)
- `podoptix_scheduler_run_duration_seconds` (histogram)
- `podoptix_scheduler_containers_scanned_total` (counter)
- `podoptix_cache_hits_total{kind}` / `misses_total{kind}` (counter)

Exposed at `/metrics` — public, standard Prometheus scrape target.

## 5.13 `cmd/hub/main.go` — the ties-it-all-together

**[cmd/hub/main.go](../cmd/hub/main.go)** — under 100 lines.

```go
func main() {
    cfg, err := config.Load(); ...
    must("Database ", store.EnsureDatabase(cfg.DatabaseURL))
    must("Schema   ", store.SyncSchema(cfg.DatabaseURL))
    db, err := store.New(cfg.DatabaseURL); must("Pool     ", err); defer db.Close()
    redisCache, err := cache.New(cfg.RedisURL); must("Redis    ", err); defer redisCache.Close()
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM); defer stop()
    sched := scheduler.New(db, 24*time.Hour, cfg.EncryptionKey)
    go sched.Start(ctx)
    server := api.NewServer(db, redisCache, sched, cfg.JWTSecret, cfg.EncryptionKey)
    listener, err := server.Listen(cfg.Port); must("Server   ", err)
    go server.Serve(listener)
    <-ctx.Done()          // block here
    listener.Close()      // unblocks Serve — graceful shutdown
}
```

`must(label, err)` prints a green OK or a red failure line, then exits on error. This is what produces the pretty startup banner.

The `<-ctx.Done()` + `listener.Close()` pattern is how a single Ctrl+C fully shuts everything down (see the graceful shutdown section in [engineering-trade-offs.md](engineering-trade-offs.md)).

---

# 6. Dashboard Deep Dive

React 18 + TypeScript + Vite + Tailwind + React Router v7. Grafana-style dark theme.

## 6.1 Config files

**[web/package.json](../web/package.json)** — dependencies + scripts (`dev`, `build`, `test:e2e`).

**[web/vite.config.ts](../web/vite.config.ts)** — Vite is the dev server + build tool. Two things it does:
- Dev proxy: forwards `/api`, `/auth`, `/healthz`, `/readyz` to the Go backend so the browser sees a single origin (no CORS).
- Build output: `outDir: '../internal/dashboard/dist'` — writes straight into the Go embed target, so `go build` picks it up.

**[web/tailwind.config.js](../web/tailwind.config.js)** — defines the color palette (`bg`, `surface`, `accent`, `ok`, `warn`, etc.) used as `className="bg-surface"` everywhere.

**[web/tsconfig.json](../web/tsconfig.json)** — TypeScript compiler settings. Strict mode on.

**[web/index.html](../web/index.html)** — the shell. Just `<div id="root"></div>` + `<script src="/src/main.tsx">` + fonts.

## 6.2 Boot layer

**[web/src/main.tsx](../web/src/main.tsx)** — 8 lines. Wraps `<App />` in `<BrowserRouter>` and renders into `#root`.

**[web/src/App.tsx](../web/src/App.tsx)** — every route defined:
- Public: `/login`, `/register`
- Protected (via `<ProtectedRoute>`): `/clusters`, `/clusters/new`, `/clusters/:id`, `/clusters/:id/edit`, `/recommendations`, `/savings`
- All protected routes wrap in `<Layout>` — sidebar + main content

**[web/src/index.css](../web/src/index.css)** — Tailwind imports + subtle dot-grid background.

## 6.3 The library layer

**[web/src/lib/auth.ts](../web/src/lib/auth.ts)** — 15 lines. localStorage wrapper. `getToken`, `save`, `clear`, `isLoggedIn`, `getEmail`.

**[web/src/lib/api.ts](../web/src/lib/api.ts)** — the entire backend surface in one file. `request<T>()` is the workhorse:
- Attaches JWT
- Serializes/parses JSON
- On 401 → clears auth + redirects to /login (session expired)
- Throws `ApiError` on non-2xx

Public methods: `register`, `login`, `listClusters`, `getCluster`, `createCluster`, `updateCluster`, `deleteCluster`, `listRecommendations`, `listAllRecommendations`, `recalculate`.

## 6.4 Components (reusable pieces)

- **[Logo.tsx](../web/src/components/Logo.tsx)** — inline SVG of the PodOptix gauge
- **[StatusPill.tsx](../web/src/components/StatusPill.tsx)** — colored pill with a dot: `ok | warn | danger | info | muted`
- **[ProtectedRoute.tsx](../web/src/components/ProtectedRoute.tsx)** — 3 lines. If `!auth.isLoggedIn()` → `<Navigate to="/login" />` else `<Outlet />`
- **[Layout.tsx](../web/src/components/Layout.tsx)** — sidebar + `<Outlet />`. Handles logout button.

## 6.5 Pages (screens)

Each page follows the same pattern:
```tsx
const [data, setData]       = useState([])
const [loading, setLoading] = useState(true)
const [error, setError]     = useState(null)
useEffect(() => { api.method().then(setData).finally(() => setLoading(false)) }, [])
```

Files:
- **Login.tsx** / **Register.tsx** — split-screen auth pages
- **Clusters.tsx** — the main dashboard (list + register button)
- **RegisterCluster.tsx** — form
- **EditCluster.tsx** — same form, pre-filled, token field optional
- **ClusterDetail.tsx** — status cards + recommendations table + recalculate button + poll-after-recalculate
- **Recommendations.tsx** — cross-cluster view, sort/filter
- **Savings.tsx** — potential + realized savings, top waste, per-cluster + per-namespace breakdown

For a full walkthrough see [web/DASHBOARD.md](../web/DASHBOARD.md).

---

# 7. Testing Framework

Two independent suites. Both isolated from your dev database.

| Suite | Location | Framework | Tests | DB used | Port |
|-------|----------|-----------|-------|---------|------|
| Backend API + unit | `tests/` | Go `testing` + `testify` + `httptest` + `pgx` | 63 | `podoptix_test` (dropped/created each run) | 9090 |
| UI end-to-end | `web/tests-e2e/` | Playwright + Chromium | 9 | `podoptix_ui_test` (dropped/created each run) | 9091 backend / 5174 vite |

**Isolation matrix — all three can coexist:**

| | Production | API tests | UI tests |
|---|-----------|-----------|----------|
| Server port | 8080 | 9090 | 9091 |
| Vite port | — | — | 5174 |
| Postgres DB | podoptix | podoptix_test | podoptix_ui_test |
| Redis index | 0 | 1 | 2 |

You can run `go run ./cmd/hub` + `go test ./tests/...` + `npm run test:e2e` simultaneously. Nothing collides.

## 7.1 Run everything

```bash
# Backend
go test ./tests/... -count=1 -p 1

# UI (from web/)
cd web && npm run test:e2e
```

Or via Make:
```bash
make test         # both
make test-api     # backend only
make test-ui      # UI only
```

## 7.2 How backend tests work

`tests/setup_test.go` has a `TestMain` that runs ONCE before any test:
1. Drop + recreate `podoptix_test`
2. Run migrations
3. Open connection pool
4. Connect to Redis (index 1)
5. Wipe Redis
6. Start a real TCP server on 9090
7. `m.Run()` → all tests execute
8. Teardown: close DB pool, drop DB, print summary

Every test hits the real Gin router, real middleware, real Postgres, real Redis. No mocks (except the collector uses `httptest.NewServer` to fake Prometheus).

Full details: [tests/TESTING.md](../tests/TESTING.md).

## 7.3 How UI tests work

Playwright automates a real Chromium browser. The `playwright.config.ts`:
1. Starts the Go backend on :9091 (with UI-test env vars)
2. Starts Vite on :5174 (proxying to :9091)
3. Launches headless Chromium
4. Each test: clicks buttons, fills forms, asserts what's on screen
5. Kills both servers when done

Full details: [web/tests-e2e/UI_TESTING.md](../web/tests-e2e/UI_TESTING.md).

## 7.4 Add a test

Backend — drop a file `tests/mything_test.go`:
```go
func TestMyThing(t *testing.T) {
    t.Run("does the thing", func(t *testing.T) {
        track(t)   // increments the run counter for pretty output
        resp := do(t, "GET", "/healthz", "", "")
        assert.Equal(t, 200, resp.StatusCode)
    })
}
```

UI — drop a file `web/tests-e2e/mything.spec.ts`:
```typescript
test('does the thing', async ({ page }) => {
    await registerAndLogin(page, uniqueEmail('mytest'))
    await page.getByRole('button', { name: /register cluster/i }).click()
    await expect(page).toHaveURL(/\/clusters\/new$/)
})
```

---

# 8. Build Pipeline

Three build stages, orchestrated by `make build`.

## 8.1 Vite bundles the React app

```bash
cd web && npm run build
```

Runs `tsc` (TypeScript type-check) then `vite build`. Output:

```
../internal/dashboard/dist/
  index.html                   ~0.8 kB
  favicon.svg
  assets/
    index-<hash>.js           ~72 kB gzipped   (all your React + libs)
    index-<hash>.css          ~4 kB gzipped    (all Tailwind you use)
```

Note the output path: `../internal/dashboard/dist` — this is Vite's `outDir` config. It writes straight into the Go embed target.

## 8.2 Go embeds the dist folder + compiles

```bash
go build -o bin/podoptix ./cmd/hub
```

`internal/dashboard/dashboard.go` has:
```go
//go:embed all:dist
var distFS embed.FS
```

At compile time, Go's toolchain reads every file under `internal/dashboard/dist/` and inlines it as bytes into the binary. Runtime doesn't touch disk for these files — they're already in RAM.

## 8.3 The Makefile orchestrates

```bash
make build
```

Runs:
1. `cd web && npm install --silent && npm run build`
2. `go build -o bin/podoptix ./cmd/hub`

Result: `bin/podoptix` — a single ~41 MB static binary that serves both the API and the dashboard on the same port. This is what production runs.

## 8.4 The Docker image

`Dockerfile` — 3 stages:

1. **web-builder** (node:20-alpine) — `npm ci && npm run build` — outputs the dist
2. **go-builder** (golang:1.26-alpine) — copies vendored deps + source + the dist from stage 1, cross-compiles the binary via `GOOS=$TARGETOS GOARCH=$TARGETARCH`
3. **final** (gcr.io/distroless/static-debian12:nonroot) — just the binary + migrations. ~44 MB, no shell, no package manager, runs as UID 65532.

Vendored deps (`vendor/`) let the container build offline — critical behind corporate SSL-intercept proxies.

## 8.5 Multi-arch push (buildx)

```bash
make docker-push TAG=0.1.0
```

Runs `docker buildx build --platform linux/amd64,linux/arm64 -t $IMAGE:$TAG --push .` — pushes ONE manifest that works on both amd64 (most K8s) and arm64 (Apple Silicon, AWS Graviton, Raspberry Pi).

Go's built-in cross-compiler + `--platform=$BUILDPLATFORM` on builder stages means Go compiles natively for each target instead of running through QEMU — fast on Apple Silicon.

---

# 9. Container + Helm Chart

## 9.1 The chart

`deploy/helm/podoptix/`:
- **Chart.yaml** — metadata (name, version, appVersion)
- **values.yaml** — user-configurable defaults (image, resources, service.type, autoscaling, networkPolicy, securityContext, ...)
- **templates/**
  - **_helpers.tpl** — reusable template functions (fullname, labels)
  - **secret.yaml** — auto-generates POSTGRES_PASSWORD, JWT_SECRET, ENCRYPTION_KEY on first install, preserved via `lookup` on upgrades
  - **postgres-statefulset.yaml** — 1 replica, PVC 10Gi, `pg_isready` probes
  - **postgres-service.yaml** — ClusterIP, internal only
  - **redis-deployment.yaml** — 1 replica, emptyDir (cache only)
  - **redis-service.yaml** — ClusterIP, internal only
  - **podoptix-deployment.yaml** — stateless, initContainers wait for postgres/redis
  - **podoptix-service.yaml** — type driven by `values.service.type` (default ClusterIP)
  - **hpa.yaml** — opt-in HorizontalPodAutoscaler (CPU + memory targets)
  - **networkpolicy.yaml** — opt-in NetworkPolicies restricting pod-to-pod traffic
  - **NOTES.txt** — post-install message showing customer how to access

## 9.2 Verify chart

```bash
helm lint deploy/helm/podoptix
helm template test deploy/helm/podoptix                          # render defaults
helm template test deploy/helm/podoptix --set service.type=LoadBalancer   # test with a flag
```

## 9.3 Push chart to GHCR

Helm 3.8+ pushes to OCI registries — same host + auth as Docker.

```bash
# One-time: login (or reuse the docker login from earlier)
echo <YOUR_GITHUB_TOKEN> | docker login ghcr.io -u RISHABH1270 --password-stdin

# Package + push
make helm-push
```

Chart is now at `oci://ghcr.io/rishabh1270/charts/podoptix:0.1.0`. Customer install:
```bash
helm install podoptix oci://ghcr.io/rishabh1270/charts/podoptix --version 0.1.0
```

Full customer walkthrough: [../USER_MANUAL.md](../USER_MANUAL.md).

---

# 10. Make Your First Change

A realistic worked example: **add a new field to Cluster called `region` (a text label, optional).**

You'll touch every layer.

### 10.1 Create a migration

```bash
touch migrations/000004_add_cluster_region.up.sql
```

Content:
```sql
ALTER TABLE clusters ADD COLUMN IF NOT EXISTS region VARCHAR(50) NOT NULL DEFAULT '';
```

Also add a `.down.sql` for symmetry (though we don't run downs automatically):
```sql
ALTER TABLE clusters DROP COLUMN IF EXISTS region;
```

Restart the backend — `SyncSchema` applies the new migration.

### 10.2 Update the model

`pkg/models/cluster.go` — add the field:
```go
Region string `json:"region" db:"region"`
```

### 10.3 Update the store

`internal/store/cluster.go` — every SELECT and INSERT/UPDATE query needs the new column. Add `region` to:
- `SaveCluster` INSERT
- `GetCluster` SELECT
- `ListClusters` SELECT
- `UpdateCluster` UPDATE
- The Scan target list

### 10.4 Update the API request/response

`internal/api/clusters.go`:
- Add `Region string \`json:"region"\`` to `CreateClusterRequest` and `UpdateClusterRequest`
- Add `Region string \`json:"region"\`` to `ClusterResponse`
- Copy the field through in `createCluster`, `updateCluster`, `toClusterResponse`

### 10.5 Add a backend test

`tests/clusters_test.go` — extend the "success returns 201" test to verify `region` roundtrips:
```go
assert.Contains(t, body, `"region":"us-east"`)
```

Run: `go test ./tests/... -run TestClusters -count=1 -p 1`

### 10.6 Update the frontend types + form

`web/src/lib/api.ts` — add `region: string` to the `Cluster` interface and to the `createCluster` body type.

`web/src/pages/RegisterCluster.tsx` and `EditCluster.tsx` — add a Region text input.

`web/src/pages/Clusters.tsx` — optionally add a Region column to the table.

### 10.7 Add a UI test

`web/tests-e2e/clusters.spec.ts` — extend the register-cluster test to fill Region and assert it shows on the detail page.

Run: `cd web && npm run test:e2e -- --grep "register a cluster"`

### 10.8 Ship

```bash
git add .
git commit -m "feat: add region field to Cluster"
git push origin development
```

If you want to also push a new container/chart:
```bash
make docker-push TAG=0.2.0
make helm-push
```

That's the entire loop. Every feature follows this pattern.

---

# 11. Command Cheat Sheet

### Development

| Command | What it does |
|---------|-------------|
| `docker compose up -d` | Start Postgres + Redis + Prometheus |
| `docker compose down` | Stop containers |
| `docker compose down -v` | Stop + wipe all data volumes |
| `export $(cat .env \| xargs) && go run ./cmd/hub` | Run backend from source |
| `cd web && npm run dev` | Run Vite dev server (hot reload) |
| `make dev` | Same as `go run ./cmd/hub` |
| `make build` | Build the dashboard + Go binary → `bin/podoptix` |
| `./bin/podoptix` | Run the built binary (single-process) |

### Testing

| Command | What it does |
|---------|-------------|
| `make test` | All backend + UI tests |
| `make test-api` | Backend tests only |
| `make test-ui` | UI tests only |
| `go test ./tests/... -count=1 -p 1` | Backend directly |
| `cd web && npm run test:e2e` | UI directly |
| `cd web && npm run test:e2e:ui` | Playwright interactive UI |
| `cd web && npm run test:e2e:report` | Open the last HTML report |

### Docker

| Command | What it does |
|---------|-------------|
| `make docker-build` | Build local image `podoptix:local` (single-arch) |
| `make docker-run` | Run `podoptix:local` against docker compose services |
| `make docker-push TAG=0.1.0` | Multi-arch build + push to ghcr.io |

### Helm

| Command | What it does |
|---------|-------------|
| `helm lint deploy/helm/podoptix` | Chart syntax check |
| `helm template test deploy/helm/podoptix` | Render templates locally |
| `make helm-package` | Produce `bin/podoptix-<version>.tgz` |
| `make helm-push` | Package + push to ghcr.io OCI registry |
| `helm install podoptix ./deploy/helm/podoptix` | Install from local chart |
| `helm upgrade podoptix ./deploy/helm/podoptix` | Upgrade existing install |

### Debug + inspect

| Command | What it does |
|---------|-------------|
| `docker exec -it podoptix-db psql -U postgres -d podoptix` | Interactive psql shell |
| `docker exec -it podoptix-db psql -U postgres -d podoptix -c "\dt"` | List tables |
| `docker exec -it podoptix-redis redis-cli` | Interactive Redis shell |
| `docker exec -it podoptix-redis redis-cli KEYS '*'` | See all keys |
| `curl http://localhost:8080/healthz` | Liveness |
| `curl http://localhost:8080/readyz` | Readiness (DB + Redis) |
| `curl http://localhost:8080/metrics \| grep podoptix_` | See Prometheus metrics |

### Cleanup

| Command | What it does |
|---------|-------------|
| `make clean` | Remove bin/, node_modules/, built dashboard |
| `docker compose down -v` | Kill containers + wipe data volumes |
| `docker system prune -af` | Nuclear — free all unused Docker space |

---

## Troubleshooting

**Port 8080 already in use:**
```bash
lsof -ti:8080 | xargs kill -9
```

**Postgres connection refused:**
```bash
docker compose up -d
docker ps               # verify all three containers are healthy
docker logs podoptix-db # check for OOM or config errors
```

**"database does not exist" error at startup:**
The backend's `EnsureDatabase` should create it. If it fails, check your `DATABASE_URL` in `.env`. Try recreating from scratch:
```bash
docker compose down -v && docker compose up -d
```

**Schema migration stuck in "dirty" state:**
`SyncSchema` auto-fixes this on the next start. Just restart the backend.

**Vite dev server won't start:**
```bash
cd web
rm -rf node_modules package-lock.json
npm install
npm run dev
```

**Playwright: "Chromium not installed"**:
```bash
cd web
npx playwright install chromium
# If corporate SSL intercept blocks it:
NODE_TLS_REJECT_UNAUTHORIZED=0 npx playwright install chromium
```

**Docker build fails on `go mod download` (SSL error behind corporate proxy):**
Refresh vendored deps then rebuild:
```bash
make vendor
make docker-build
```

**Prometheus token encryption fails:**
`ENCRYPTION_KEY` must be EXACTLY 32 bytes. Count: `echo -n "your-key" | wc -c`.

**401 on protected routes:**
1. Register or log in first
2. JWT expires after 24h — log in again
3. Header format is exactly: `Authorization: Bearer <token>`

**Backend runs but dashboard shows blank page:**
You're in Option B (single binary) but the dashboard wasn't built. Run `make dashboard` then restart.

---

## Where to go next

- **Backend contributor?** → deep-read `internal/api/` + `internal/scheduler/`
- **Frontend contributor?** → deep-read `web/src/pages/` + `web/src/lib/api.ts`
- **Testing?** → [tests/TESTING.md](../tests/TESTING.md) + [web/tests-e2e/UI_TESTING.md](../web/tests-e2e/UI_TESTING.md)
- **Design decisions?** → [engineering-trade-offs.md](engineering-trade-offs.md)
- **System overview?** → [hld.md](hld.md), [lld.md](lld.md), [architecture.html](architecture.html)
- **Just want to install PodOptix?** → [../USER_MANUAL.md](../USER_MANUAL.md)
