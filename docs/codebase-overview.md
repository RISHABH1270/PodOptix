# PodOptix — Codebase Overview

---

## 1. `go.mod`

Project manifest — same as `build.gradle` (Java), `requirements.txt` (Python), `package.json` (Node.js).

- `module` — unique project name, used as base for all internal imports
- `go 1.26.4` — minimum Go version (lower bound)
- `require` — 3 libraries we directly chose: `gin` (HTTP server framework), `golang-migrate` (SQL migrations), `pgx` (PostgreSQL driver converts your Go function calls into PostgreSQL's wire protocol)
- `// indirect` — dependencies of our dependencies, managed automatically by Go

`go.sum` — companion file that stores cryptographic fingerprints of every library to prevent tampering.

---

## 2. `pkg/models/cluster.go`

Blueprint for a Kubernetes cluster. Defines what data it holds.

```
Cluster struct (in memory)
┌─────────────────────────────────────────────────┐
│ ClusterID      "a3f8c2d1-9b4e-4f1a-8c3d..."     │  string
│ Name           "production-cluster"             │  string
│ PrometheusURL  "https://prometheus.example.com" │  string
│ Token          "eyJhbGci..."  (hidden in API)   │  string
│ LookbackWindow "7d"                             │  string
│ CreatedAt      2026-06-24 00:00:00              │  time.Time
│ UpdatedAt      2026-06-24 00:00:00              │  time.Time
└─────────────────────────────────────────────────┘

*Cluster = pointer (8 bytes) → points to the above struct in heap
```

Struct tags control how fields are named in JSON responses (`json:"x"`) and PostgreSQL columns (`db:"x"`). `json:"-"` on Token means it is never sent in API responses.

---

## 3. `pkg/models/recommendation.go`

One recommendation per container. One pod with 3 containers = 3 Recommendation objects.

```
Recommendation struct (in memory)
┌─────────────────────────────────────────────────────────┐
│ RecommendationID  "x7f3-..."          string            │
│ ClusterID         "a3f8-..."          string  (FK)      │
│ Namespace         "payments"          string            │
│ PodName           "payment-api-7d9f"  string            │
│ ContainerName     "payment-api"       string            │
│ Status            "new_service"       string            │
│ CurrentCPULimit   1000                int (millicores)  │
│ CurrentMemLimit   1024                int (MiB)         │
│ P99CPU            120.5               float64           │
│ P99Mem            180.2               float64           │
│ RecommendedCPULimit 241               int (p99 × 2)     │
│ RecommendedMemLimit 360               int (p99 × 2)     │
│ LookbackWindow    "7d"                string            │
│ CreatedAt         2026-06-24...       time.Time         │
└─────────────────────────────────────────────────────────┘
```

- `int` for limits — normalized to millicores/MiB for clean math (`p99 × 2`)
- `float64` for p99 — decimal precision needed for raw Prometheus values
- Two constants: `StatusNewService = "new_service"` · `StatusReady = "ready"`

---

## 4. `internal/config/config.go`

Reads environment variables on startup. First thing `main.go` calls.

```
Config struct (in memory)
┌────────────────────────────────────────────┐
│ Port        "8080"                         │  string
│ DatabaseURL "postgres://postgres:pass@..." │  string
│ RedisURL    "redis://localhost:6379"       │  string
│ JWTSecret   "my-local-dev-secret-key"      │  string
└────────────────────────────────────────────┘
```

- `getEnv(key, fallback)` — optional, returns fallback if not set (`PORT` defaults to `"8080"`)
- `mustGetEnv(key)` — required, panics if missing. App refuses to start without `DATABASE_URL`, `REDIS_URL`, `JWT_SECRET`
- `func Load() (*Config, error)` — same as `Config* Load()` in C++. Returns pointer + error

---

## 5. `internal/store/store.go`

Manages the PostgreSQL connection pool and database lifecycle.

```
Store struct (in memory)
┌──────────────────┐
│ pool → 0x0042 ───┼──► pgxpool.Pool { conn1, conn2, ... conn10 }
└──────────────────┘
```

- `pool *pgxpool.Pool` — pointer to the pool object from the `pgx` library. `pgxpool` = package, `Pool` = type (same pattern as `time.Time`)
- `New()` — creates the pool with settings: max 10 conns, min 2 always warm, refresh after 1hr, close idle after 30min. Pings DB to verify connection before returning
- `Close()` — `(s *Store)` means this method belongs to Store (`s` = `this` in C++). Called via `defer` in main — closes all connections on shutdown
- `EnsureDatabase()` — extracts database name from `DATABASE_URL`, connects to default `postgres` DB, runs `CREATE DATABASE podoptix`. Name comes from the URL — never hardcoded
- `SyncSchema()` — runs `.up.sql` files in order. Skips already applied ones. If database is dirty (app crashed mid-migration), auto-fixes by resetting the dirty flag and retrying

**Dirty database:** golang-migrate marks a migration dirty the moment it starts. If the app crashes halfway through — flag stays dirty. On next startup our code detects it, forces the version clean, and retries. Safe because SQL uses `IF NOT EXISTS`.

---

## 6. `internal/store/cluster.go`

CRUD operations for clusters. All methods belong to `Store` — `(s *Store)` means `s` is `this` in C++.

- `SaveCluster` — `pool.Exec()` runs INSERT. `$1, $2...` are safe placeholders — prevents SQL injection. `_` discards rows-affected count
- `GetCluster` — `pool.QueryRow()` returns exactly one row. `row.Scan(&c.Field)` writes each column into the struct. `&` = pass by reference, same as C++
- `ListClusters` — `pool.Query()` returns multiple rows. `rows.Next()` iterates like a C++ iterator. `defer rows.Close()` always frees the connection back to pool. `append(clusters, c)` = `push_back` in C++ vector. Returns `[]*models.Cluster` = `vector<Cluster*>` in C++
- `DeleteCluster` — `pool.Exec()` runs DELETE
- `UpdateCluster` — `pool.Exec()` runs UPDATE. `time.Now()` sets `updated_at` automatically
- `fmt.Errorf("save cluster: %w", err)` — `%w` wraps the original error with context so the full chain is visible when it reaches the API handler

---

## 7. `internal/store/recommendation.go`

Same patterns as `cluster.go`. Two operations only — Save and List.

- `SaveRecommendation` — 13 fields, `$1` through `$13` must match values in exact order
- `ListByCluster` — fetches only recommendations for one cluster. `ORDER BY created_at DESC` = newest first. Uses the `idx_recommendations_cluster_id` index — fast even with millions of rows
- No Update or Delete — recommendations are immutable. Once generated they are historical records. New run = new row. Old rows stay for history
- `UpsertRecommendation` — one row per container, updated in place daily using `ON CONFLICT ... DO UPDATE`

---

## 21. `internal/api/auth.go`

Handles user registration and login. Same Input DTO pattern as clusters.

**`register` handler flow:**
```
1. Read + validate JSON body (email, password)
2. HashPassword(password) → bcrypt hash — never store plain text
3. Build User { UUID, email, hash, timestamps }
4. CreateUser in database → 409 if email already exists
5. GenerateToken immediately → user is logged in right after registering
6. Return { token, user_id, email }
```

**`login` handler flow:**
```
1. Read + validate JSON body
2. GetUserByEmail — if not found → 401 "Invalid email or password"
3. CheckPassword(input, storedHash) — if wrong → 401 same message
   (same error for wrong email OR wrong password — prevents user enumeration)
4. GenerateToken
5. Return { token, user_id, email }
```

- `409 Conflict` — specific HTTP status for duplicate resource (email already exists)
- User enumeration prevention — attacker cannot tell if an email exists by trying login

---

## 22. `internal/api/middleware.go` — JWTMiddleware

Protects all `/api/v1/*` routes. Runs before every handler in the group.

```
Authorization: Bearer eyJhbGci...
      ↓
SplitN by " " → ["Bearer", "eyJhbGci..."]
      ↓
ValidateToken(parts[1], secret)
      ↓
valid  → c.Set("user_id", ...) → c.Next() → handler runs
invalid → 401 → c.Abort() → handler never runs
```

- `c.GetHeader("Authorization")` — reads the Authorization header
- `strings.SplitN(header, " ", 2)` — splits into exactly 2 parts, enforces `Bearer <token>` format
- `c.Abort()` — stops the middleware chain. Without it Gin would continue to the handler even after sending 401
- `c.Set("user_id", claims.UserID)` — stores identity in context, available to all downstream handlers via `c.GetString("user_id")`

---

## 23. `internal/api/routes.go` (updated)

```
Public (no auth):
  GET  /healthz
  POST /auth/register
  POST /auth/login

Protected (JWT required):
  v1 group with JWTMiddleware
  GET    /api/v1/clusters
  POST   /api/v1/clusters
  GET    /api/v1/clusters/:id
  DELETE /api/v1/clusters/:id
  GET    /api/v1/clusters/:id/recommendations
```

`v1.Use(JWTMiddleware(...))` attaches JWT check to every route in the group automatically. Routes outside the group remain public.

---

## 24. `internal/api/server.go` (updated)

Added `jwtSecret string` field — injected from `main.go` which reads it from `JWT_SECRET` env var. Passed to `JWTMiddleware` and `GenerateToken`. Never hardcoded.

---

---

## 8. `migrations/`

SQL files run automatically on startup by `SyncSchema()`. Named `000001_...up.sql`, `000002_...up.sql` — golang-migrate runs them in numeric order.

**`000001_create_clusters.up.sql`**
- `IF NOT EXISTS` — safe to run every startup, skips if already created
- `VARCHAR(36)` — UUID is always 36 chars. `TEXT` = unlimited length (for token)
- `NOT NULL UNIQUE` on name — no two clusters can share a name
- `TIMESTAMPTZ` — timestamp with timezone, always stored as UTC
- `DEFAULT NOW()` — auto-set to current time on insert

**`000002_create_recommendations.up.sql`**
- `REFERENCES clusters(cluster_id)` — foreign key. PostgreSQL rejects any recommendation whose cluster_id doesn't exist in clusters table
- `UNIQUE (cluster_id, namespace, pod_name, container_name)` — composite key ensuring one recommendation per container. This is what makes UPSERT work — same container = UPDATE not INSERT
- `CREATE INDEX` — B-Tree index on `cluster_id` for fast dashboard queries

---

## 9. `internal/api/server.go`

Builds the HTTP server and wires everything together.

```
User → Server → Middleware 1 (Logger) → Middleware 2 (Recovery) → Middleware 3 (RequestID) → Handler
```

Middleware sits between the user and the handler. Runs before every request regardless of endpoint. Handler is the final destination — does the actual work.

```
Server struct (in memory)
┌──────────────────────────────────┐
│ router → 0x0100 ───► gin.Engine  │
│ store  → 0x0200 ───► Store       │
└──────────────────────────────────┘
```

- `gin.Default()` — creates router with Logger (prints every request) and Recovery (catches panics, returns 500 instead of crashing) middlewares built in
- `router.Use(RequestIDMiddleware())` — attaches our custom middleware. Runs before every handler
- `NewServer(st *store.Store)` — dependency injection. Server receives the store from `main.go`, does not create its own database connection
- `Start(port string)` — `router.Run(":8080")` opens TCP socket. Blocking — app lives here until stopped

---

## 10. `internal/api/routes.go`

Registers all URL → handler mappings. Called once at startup.

| Method | URL | Handler |
|--------|-----|---------|
| GET | `/healthz` | `handleHealthz` |
| GET | `/api/v1/clusters` | `listClusters` |
| POST | `/api/v1/clusters` | `createCluster` |
| GET | `/api/v1/clusters/:id` | `getCluster` |
| DELETE | `/api/v1/clusters/:id` | `deleteCluster` |
| GET | `/api/v1/clusters/:id/recommendations` | `listRecommendations` |

- `router.Group("/api/v1")` — shared prefix, avoids repeating `/api/v1` on every route
- `:id` — URL parameter. `/clusters/abc-123` → `:id = "abc-123"`. Read in handler with `c.Param("id")`
- `{}` curly braces — visual grouping only, no effect on behaviour

---

## 11. `internal/api/middleware.go`

Assigns a unique request ID to every incoming request.

- `gin.HandlerFunc` — function type Gin understands: `func(*gin.Context)`
- `RequestIDMiddleware()` returns a function (function factory pattern) — allows future config to be passed in
- `uuid.New().String()` — unique ID per request, never repeats
- `c.Set("request_id", id)` — stores in Gin context (key-value store for one request lifetime). Handlers read it with `c.GetString("request_id")`
- `c.Header("X-Request-ID", id)` — sends ID back in response header so customer can share it when reporting errors
- `c.Next()` — critical. Tells Gin to continue to next middleware/handler. Without it the request stops here
- `X-` prefix = custom non-standard header. `X-Request-ID` is industry standard name used by AWS, Stripe, GitHub — logging and APM tools automatically recognize it

---

## 13. `internal/api/clusters.go`

HTTP handlers for cluster CRUD operations.

**`CreateClusterRequest` vs `Cluster` model:**
Two separate structs intentionally — Input DTO pattern:
- `Cluster` model has `ClusterID`, `CreatedAt`, `UpdatedAt` — server-generated, customer must NOT set these
- `CreateClusterRequest` has only fields the customer is ALLOWED to send. If customer sends `cluster_id` in JSON — Gin ignores it completely
- After reading `CreateClusterRequest`, server builds the full `Cluster` adding UUID, timestamps itself

**Key patterns in every handler:**
- `c.GetString("request_id")` — reads request ID set by middleware for tracing
- `c.ShouldBindJSON(&req)` — reads + validates JSON body. `&req` = pass by reference so Gin writes into it. Missing required fields → 400
- `c.Request.Context()` — carries request deadline. If customer disconnects, database query cancels automatically
- `clusters == nil → []` — replace nil with empty slice so API returns `[]` not `null`
- `c.Status(204)` — DELETE returns no body, just status code
- Error pattern: `log.Printf` (real error for developer) + `c.JSON` (friendly message for customer) + `return`

---

## 14. `internal/api/recommendation.go`

Currently a stub — returns "coming soon". Will be fully built when the p99 computation engine is ready. Reads `cluster_id` from URL with `c.Param("id")` and will call `s.store.ListByCluster(ctx, id)`.

---

## 15. `cmd/hub/main.go`

The entry point — `package main` is the only package allowed to have a `main()` function. Wires every package together. Startup order matters — each step depends on the previous:

```
1. config.Load()          → read env vars — everything needs config
2. printBanner()          → show startup info
3. store.EnsureDatabase() → create DB if first time
4. store.SyncSchema()     → create tables if needed
5. store.New()            → open connection pool
6. defer db.Close()       → register cleanup — runs when app exits
7. api.NewServer(db)      → create server, inject store
8. server.Start()         → open port, block forever
```

- ANSI color constants defined at package level — shared by `main()` and `printBanner()`
- `fmt.Println` after `NewServer()` so `[GIN-debug]` output appears before our status lines
- `server.Start()` only returns on error — red message printed, then `log.Fatalf` kills app

---

---

## 12. `internal/api/health.go`

Responds to Kubernetes liveness probes. Simplest possible handler.

- `net/http` — built-in Go package used only for status code constants: `http.StatusOK` = 200, `http.StatusNotFound` = 404 etc.
- `c *gin.Context` — Gin passes this to every handler. Contains the request and methods to send a response
- `c.JSON(200, gin.H{"status":"ok"})` — sends `{"status":"ok"}` with HTTP 200
- Kubernetes calls `/healthz` every few seconds. 200 = keep running. 500 or timeout = restart the pod automatically (liveness probe)

---

## 16. `pkg/models/user.go`

Blueprint for a dashboard user.

```
User struct (in memory)
┌─────────────────────────────────────────┐
│ UserID       "a3f8-..."    string       │
│ Email        "user@x.com" string       │
│ PasswordHash "$2a$10$..." string (hidden)│
│ CreatedAt    2026-06-28   time.Time    │
│ UpdatedAt    2026-06-28   time.Time    │
└─────────────────────────────────────────┘
```

- `json:"-"` on PasswordHash — never included in any API response even accidentally
- Email has `UNIQUE` constraint in DB — one account per email

---

## 17. `migrations/000003_create_users.up.sql`

- `TEXT` for password_hash — bcrypt produces ~60 chars but TEXT future-proofs it
- `UNIQUE` on email — one account per email address
- Same patterns as clusters/recommendations migrations

---

## 18. `internal/auth/password.go`

Two functions — hash and verify. Never store plain text passwords.

- `bcrypt.GenerateFromPassword(password, DefaultCost)` — hashes with cost=10 rounds. More rounds = slower = harder to brute force. Automatically adds random salt — same password hashed twice gives different results
- `bcrypt.CompareHashAndPassword(hash, password)` — extracts salt from stored hash, re-hashes input with same salt, compares. Returns `nil` = match, error = wrong password. One-way — impossible to reverse

---

## 19. `internal/auth/jwt.go`

Generates and validates JWT tokens.

```
type Claims struct {
    UserID string            ← our custom payload
    Email  string            ← our custom payload
    jwt.RegisteredClaims     ← embedded (like C++ inheritance) — gives ExpiresAt, IssuedAt
}
```

- `jwt.NewWithClaims(HS256, claims)` → `SignedString(secret)` — creates `header.payload.signature`
- Token expires in 24 hours — after that ValidateToken returns error automatically
- `ParseWithClaims` callback checks `t.Method.(*jwt.SigningMethodHMAC)` — prevents algorithm confusion attacks where attacker sends `"alg":"none"` to bypass verification. We explicitly require HMAC before returning the secret

---

## 20. `internal/store/user.go`

Same patterns as `cluster.go`.

- `CreateUser` — INSERT with all fields
- `GetUserByEmail` — SELECT by email (used during login: fetch user → CheckPassword against stored hash)
- `UpdateUserPassword` — UPDATE hash + updated_at (for future password change feature)

---
