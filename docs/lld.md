# PodOptix — Low Level Design

## Table of Contents

- [Hub Internals Diagram](#hub-internals-diagram)
- [Detailed Step-by-Step Data Flow](#detailed-step-by-step-data-flow)
- [Database Schema](#database-schema)
- [API Contract](#api-contract)
- [Redis Key Design](#redis-key-design)
- [Security Model](#security-model)
- [Collection Pipeline](#collection-pipeline)
- [Testing Approach](#testing-approach)

---

## Hub Internals Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                          PODOPTIX HUB                               │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                    PRESENTATION LAYER                         │  │
│  │                                                               │  │
│  │   ┌────────────────────────┐     ┌────────────────────────┐   │  │
│  │   │     Web Dashboard      │     │    REST API Server     │   │  │
│  │   │  · Recommendations     │     │  GET  /recommendations │   │  │
│  │   │  · Cluster management  │     │  POST /clusters        │   │  │
│  │   │  · Savings summary     │     │  GET  /clusters        │   │  │
│  │   └────────────────────────┘     └────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                     SERVICE LAYER                             │  │
│  │                                                               │  │
│  │   ┌───────────────────────────────────────────────────────┐   │  │
│  │   │  Auth Service      │  JWT-based user auth · API keys  │   │  │
│  │   │  Cluster Registry  │  Stores Prometheus URL + token   │   │  │
│  │   │  Scheduler         │  Cron-based · runs per cluster   │   │  │
│  │   └───────────────────────────────────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                   PROCESSING PIPELINE                         │  │
│  │                                                               │  │
│  │   ┌───────────────────────────────────────────────────────┐   │  │
│  │   │  PromQL Engine  ·  queries /api/v1/query_range        │   │  │
│  │   │  Metrics: container_cpu  ·  container_memory          │   │  │
│  │   └────────────────────────┬──────────────────────────────┘   │  │
│  │                            │                                  │  │
│  │   ┌────────────────────────▼──────────────────────────────┐   │  │
│  │   │  p99 Computation Engine                               │   │  │
│  │   │  quantile(0.99, time_series)  ·  window: 7d           │   │  │
│  │   └────────────────────────┬──────────────────────────────┘   │  │
│  │                            │                                  │  │
│  │   ┌────────────────────────▼──────────────────────────────┐   │  │
│  │   │  Recommendation Engine                                │   │  │
│  │   │  CPU = p99_cpu × 2   ·   Mem = p99_mem × 2            │   │  │
│  │   │  Output → YAML patch                                  │   │  │
│  │   └───────────────────────────────────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                      STORAGE LAYER                            │  │
│  │                                                               │  │
│  │   ┌────────────────────────┐     ┌────────────────────────┐   │  │
│  │   │        Database        │     │         Cache          │   │  │
│  │   │  · clusters            │     │  · PromQL results      │   │  │
│  │   │  · recommendations     │     │  · TTL: 3 hr           │   │  │
│  │   │  · users               │     │                        │   │  │
│  │   └────────────────────────┘     └────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Detailed Step-by-Step Data Flow

### Registration Flow

```
User → POST /auth/register { email, password }
          │
          ├── 1. Validate JSON body (binding:"required")
          ├── 2. HashPassword(password) → bcrypt hash (cost=10, random salt auto-added)
          ├── 3. Build User { UUID, email, hash, timestamps }
          ├── 4. CreateUser in PostgreSQL → 409 if email already exists
          ├── 5. GenerateToken(userID, email, JWT_SECRET) → JWT (24hr expiry)
          └── 6. Return { token, user_id, email }
```

### Login Flow

```
User → POST /auth/login { email, password }
          │
          ├── 1. Validate JSON body
          ├── 2. GetUserByEmail — if not found → 401 "Invalid email or password"
          ├── 3. CheckPassword(input, storedHash) — if wrong → 401 same message
          │        (same error prevents user enumeration attacks)
          ├── 4. GenerateToken(userID, email, JWT_SECRET) → JWT
          └── 5. Return { token, user_id, email }
```

### JWT Middleware Flow (every protected request)

```
Authorization: Bearer eyJhbGci...
      │
      ├── SplitN by " " → ["Bearer", "eyJhbGci..."]
      ├── ValidateToken(parts[1], JWT_SECRET)
      │         │
      │         ├── ParseWithClaims → verify HMAC signature
      │         ├── Check t.Method.(*jwt.SigningMethodHMAC) → prevent alg:none attacks
      │         └── Check expiry (ExpiresAt in claims)
      │
      ├── valid  → c.Set("user_id", claims.UserID) → c.Next() → handler runs
      └── invalid → 401 → c.Abort() → handler never runs
```

### Cluster Registration Flow

```
User → POST /clusters { cluster_name, prometheus_url, prometheus_token, lookback_window }
          │
          ├── 1. JWTMiddleware validates token
          ├── 2. ShouldBindJSON → validate required fields
          ├── 3. Generate UUID for cluster_id
          ├── 4. EncryptToken(prometheus_token, ENCRYPTION_KEY) → AES-256-GCM ciphertext
          ├── 5. INSERT into clusters table (encrypted token stored, status = 'disconnected')
          ├── 6. Ping Prometheus immediately (10s timeout) → UPDATE status = 'connected'/'disconnected'
          ├── 7. Return 201 with Cluster object (last_synced_at = "not yet synced")
          └── 8. If connected: background goroutine runs collect → recommend → upsert
```

### Recommendation Collection Flow (Scheduler-triggered)

```
Scheduler (cron: daily)
      │
      └── For each cluster:
               │
               ├── 1. GetCluster → decrypt token (AES-256-GCM)
               ├── 2. PromQL Engine: queryRange(CPU query, start, end, step=3600)
               │         · start = now - lookback_window
               │         · end   = now
               │         · step  = 3600 (one data point per hour)
               │
               ├── 3. PromQL Engine: queryRange(Memory query, start, end, step=3600)
               │
               ├── 4. mergeMetrics(cpuData, memData) → []*ContainerMetrics
               │         · indexed by (namespace, pod, container)
               │
               ├── 5. p99 Engine: quantile(0.99, values) per container
               │
               ├── 6. Recommendation Engine: ceil(p99 × 2) per container
               │
               ├── 7. UpsertRecommendation per container
               │         ON CONFLICT (cluster_id, namespace, pod_name, container_name)
               │         DO UPDATE SET p99_cpu=..., updated_at=NOW()
               │
               └── 8. Invalidate Redis key: cluster:{id}:recommendations
```

### Dashboard Read Flow (recommendations)

```
GET /clusters/:id/recommendations
      │
      ├── 1. JWTMiddleware validates token
      ├── 2. Check Redis: cluster:{id}:recommendations
      │         ├── HIT  → return cached JSON (< 1ms)
      │         └── MISS → query PostgreSQL → cache TTL 3hr → return
      └── 3. Return []Recommendation ordered by created_at DESC
```

---

## Database Schema

### Table: `clusters`

```sql
CREATE TABLE IF NOT EXISTS clusters (
    cluster_id      VARCHAR(36)   PRIMARY KEY,
    cluster_name    VARCHAR(100)  NOT NULL UNIQUE,
    prometheus_url  TEXT          NOT NULL,
    prometheus_token TEXT         NOT NULL,
    lookback_window VARCHAR(10)   NOT NULL DEFAULT '7d',
    status          VARCHAR(20)   NOT NULL DEFAULT 'disconnected',
    created_by      VARCHAR(255)  NOT NULL,
    last_synced_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CHECK (status IN ('connected', 'disconnected')),
    CHECK (lookback_window IN ('7d', '10d', '30d'))
);
```

| Column | Type | Constraints | Notes |
|--------|------|-------------|-------|
| `cluster_id` | VARCHAR(36) | PRIMARY KEY | UUID v4 — always 36 chars |
| `cluster_name` | VARCHAR(100) | NOT NULL, UNIQUE | Human-readable cluster name |
| `prometheus_url` | TEXT | NOT NULL | Full HTTP endpoint URL |
| `prometheus_token` | TEXT | NOT NULL | AES-256-GCM encrypted at rest |
| `lookback_window` | VARCHAR(10) | NOT NULL, DEFAULT '7d' | `7d`, `10d`, or `30d` only |
| `status` | VARCHAR(20) | NOT NULL, DEFAULT 'disconnected' | `connected` / `disconnected` |
| `created_by` | VARCHAR(255) | NOT NULL | Email of registering user — audit trail |
| `last_synced_at` | TIMESTAMPTZ | nullable | NULL until first sync completes |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | UTC timestamp |
| `updated_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | UTC timestamp |

### Table: `recommendations`

```sql
CREATE TABLE IF NOT EXISTS recommendations (
    recommendation_id   VARCHAR(36)  PRIMARY KEY,
    cluster_id          VARCHAR(36)  NOT NULL REFERENCES clusters(cluster_id),
    namespace           VARCHAR(255) NOT NULL,
    pod_name            VARCHAR(255) NOT NULL,
    container_name      VARCHAR(255) NOT NULL,
    status              VARCHAR(20)  NOT NULL DEFAULT 'new_service',
    current_cpu_limit   INTEGER      NOT NULL DEFAULT 0,
    current_mem_limit   INTEGER      NOT NULL DEFAULT 0,
    p99_cpu             FLOAT        NOT NULL DEFAULT 0,
    p99_mem             FLOAT        NOT NULL DEFAULT 0,
    recommended_cpu_limit INTEGER    NOT NULL DEFAULT 0,
    recommended_mem_limit INTEGER    NOT NULL DEFAULT 0,
    applied             BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    UNIQUE (cluster_id, namespace, pod_name, container_name)
);

CREATE INDEX idx_recommendations_cluster_id ON recommendations(cluster_id);
```

| Column | Type | Notes |
|--------|------|-------|
| `recommendation_id` | VARCHAR(36) | UUID v4 primary key |
| `cluster_id` | VARCHAR(36) | FK → clusters.cluster_id |
| `namespace` | VARCHAR(255) | Kubernetes namespace |
| `pod_name` | VARCHAR(255) | Pod name (may include hash suffix) |
| `container_name` | VARCHAR(255) | Container within the pod |
| `status` | VARCHAR(20) | `new_service` or `ready` |
| `current_cpu_limit` | INTEGER | Millicores — 0 if unset |
| `current_mem_limit` | INTEGER | MiB — 0 if unset |
| `p99_cpu` | FLOAT | Raw p99 value in millicores |
| `p99_mem` | FLOAT | Raw p99 value in MiB |
| `recommended_cpu_limit` | INTEGER | `ceil(p99_cpu × 2)` millicores |
| `recommended_mem_limit` | INTEGER | `ceil(p99_mem × 2)` MiB |
| `applied` | BOOLEAN | TRUE if recommendation was applied to cluster — tracks cost savings |
| `created_at` | TIMESTAMPTZ | First generated |
| `updated_at` | TIMESTAMPTZ | Last recalculated |

**Composite UNIQUE constraint** on `(cluster_id, namespace, pod_name, container_name)` is what enables UPSERT — same container = UPDATE existing row, not INSERT new row.

**B-Tree index** on `cluster_id` enables O(log n) lookup for dashboard queries instead of full table scan.

### Table: `users`

```sql
CREATE TABLE IF NOT EXISTS users (
    user_id        VARCHAR(36)  PRIMARY KEY,
    email          VARCHAR(255) NOT NULL UNIQUE,
    password_hash  TEXT         NOT NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```

| Column | Type | Notes |
|--------|------|-------|
| `user_id` | VARCHAR(36) | UUID v4 primary key |
| `email` | VARCHAR(255) | UNIQUE — one account per email |
| `password_hash` | TEXT | bcrypt hash — TEXT future-proofs hash length |
| `created_at` | TIMESTAMPTZ | UTC timestamp |
| `updated_at` | TIMESTAMPTZ | UTC timestamp |

`TEXT` (not VARCHAR) for `password_hash` — bcrypt produces ~60 chars today but TEXT future-proofs against longer algorithm outputs.

---

## API Contract

### Authentication

All cluster endpoints require:
```
Authorization: Bearer <jwt_token>
```

Public endpoints (no auth required): `GET /healthz`, `GET /readyz`, `POST /auth/register`, `POST /auth/login`

### Error Response Format

Every error response includes a `request_id` for tracing:
```json
{
  "error": "Cluster not found",
  "request_id": "a3f8c2d1-9b4e-4f1a-8c3d-2e5f7a9b1c4d"
}
```

### Endpoints

---

#### `GET /healthz`

Kubernetes liveness probe.

**Auth:** None

**Response 200:**
```json
{ "status": "ok" }
```

---

#### `GET /readyz`

Kubernetes readiness probe. Checks PostgreSQL and Redis connectivity.

**Auth:** None

**Response 200 (all healthy):**
```json
{ "status": "ok", "checks": { "postgres": "ok", "redis": "ok" } }
```

**Response 503 (degraded):**
```json
{ "status": "degraded", "checks": { "postgres": "ok", "redis": "error" } }
```

---

#### `POST /auth/register`

Register a new user account.

**Auth:** None

**Request body:**
```json
{
  "email":    "user@example.com",
  "password": "securepassword"
}
```

**Response 201:**
```json
{
  "token":   "eyJhbGci...",
  "user_id": "a3f8c2d1-9b4e-4f1a-8c3d-2e5f7a9b1c4d",
  "email":   "user@example.com"
}
```

**Errors:**
- `400` — missing required fields
- `409` — email already registered

---

#### `POST /auth/login`

Authenticate and receive a JWT token.

**Auth:** None

**Request body:**
```json
{
  "email":    "user@example.com",
  "password": "securepassword"
}
```

**Response 200:**
```json
{
  "token":   "eyJhbGci...",
  "user_id": "a3f8c2d1-9b4e-4f1a-8c3d-2e5f7a9b1c4d",
  "email":   "user@example.com"
}
```

**Errors:**
- `400` — missing required fields
- `401` — "Invalid email or password" (same message for wrong email OR wrong password — prevents user enumeration)

---

#### `GET /clusters`

List all registered clusters.

**Auth:** JWT required

**Response 200:**
```json
[
  {
    "cluster_id":      "a3f8c2d1-9b4e-4f1a-8c3d-2e5f7a9b1c4d",
    "cluster_name":    "production-cluster",
    "prometheus_url":  "https://prometheus.example.com",
    "lookback_window": "7d",
    "status":          "connected",
    "created_by":      "user@example.com",
    "last_synced_at":  "not yet synced",
    "created_at":      "2026-06-24T00:00:00Z",
    "updated_at":      "2026-06-24T00:00:00Z"
  }
]
```

Note: `prometheus_token` field is always omitted from responses (`json:"-"`).

Returns `[]` (empty array) when no clusters exist — never `null`.

---

#### `POST /clusters`

Register a new workload cluster.

**Auth:** JWT required

**Request body:**
```json
{
  "cluster_name":      "production-cluster",
  "prometheus_url":    "https://prometheus.example.com",
  "prometheus_token":  "your-prometheus-bearer-token",
  "lookback_window":   "7d"
}
```

All fields required. `lookback_window` must be one of: `"7d"`, `"10d"`, `"30d"`.

**Response 201:**
```json
{
  "cluster_id":      "a3f8c2d1-9b4e-4f1a-8c3d-2e5f7a9b1c4d",
  "cluster_name":    "production-cluster",
  "prometheus_url":  "https://prometheus.example.com",
  "lookback_window": "7d",
  "status":          "connected",
  "created_by":      "user@example.com",
  "last_synced_at":  "not yet synced",
  "created_at":      "2026-06-24T00:00:00Z",
  "updated_at":      "2026-06-24T00:00:00Z"
}
```

Status is `connected` or `disconnected` — never `pending`. Prometheus is pinged during registration (10s timeout) to set the status immediately.

**Errors:**
- `400` — missing or invalid fields

---

#### `GET /clusters/:id`

Get a single cluster by ID.

**Auth:** JWT required

**Response 200:** Same shape as POST response.

**Errors:**
- `404` — "Cluster not found"

---

#### `PUT /clusters/:id`

Update a cluster's configuration.

**Auth:** JWT required

**Request body:** (any subset of fields)
```json
{
  "cluster_name":     "production-cluster-updated",
  "prometheus_url":   "https://new-prometheus.example.com",
  "prometheus_token": "new-prometheus-token",
  "lookback_window":  "30d"
}
```

**Response 200:** Updated cluster object.

**Errors:**
- `400` — invalid fields
- `404` — "Cluster not found"

---

#### `DELETE /clusters/:id`

Remove a cluster and all its recommendations.

**Auth:** JWT required

**Response 204:** No body.

**Errors:**
- `404` — "Cluster not found"

---

#### `GET /clusters/:id/recommendations`

Get all recommendations for a cluster.

**Auth:** JWT required

**Response 200:**
```json
[
  {
    "recommendation_id":    "x7f3c2d1-9b4e-4f1a-8c3d-2e5f7a9b1c4d",
    "cluster_id":           "a3f8c2d1-9b4e-4f1a-8c3d-2e5f7a9b1c4d",
    "namespace":            "payments",
    "pod_name":             "payment-api-7d9f",
    "container_name":       "payment-api",
    "status":               "ready",
    "current_cpu_limit":    1000,
    "current_mem_limit":    1024,
    "p99_cpu":              120.5,
    "p99_mem":              180.2,
    "recommended_cpu_limit": 241,
    "recommended_mem_limit": 361,
    "applied":              false,
    "created_at":           "2026-06-24T00:00:00Z",
    "updated_at":           "2026-06-24T00:00:00Z"
  }
]
```

CPU values in millicores. Memory values in MiB. Ordered by `created_at DESC`.

**Errors:**
- `404` — "Cluster not found"

---

#### `POST /clusters/:id/recalculate`

Trigger a manual recommendation recalculation for a cluster.

**Auth:** JWT required

**Response 202:** No body — recalculation started in background.

**Errors:**
- `404` — "Cluster not found"
- `429` — recalculation already in progress (distributed Redis lock held)

---

## Redis Key Design

| Role | Key Pattern | TTL | Operation | Why |
|------|-------------|-----|-----------|-----|
| Recommendations cache | `cluster:{id}:recommendations` | 3 hours | GET/SET/DEL | Dashboard reads on every page load — serve from Redis, not PostgreSQL |
| Distributed lock | `lock:cluster:{id}:recalculate` | 10 minutes | SetNX | Prevent duplicate recalculate jobs for same cluster |

**Cache-aside pattern for recommendations:**

```
GET /clusters/:id/recommendations
        ↓
Redis GET cluster:{id}:recommendations
        ↓
HIT  → unmarshal JSON → return (< 1ms)
MISS → PostgreSQL SELECT WHERE cluster_id = :id
        ↓
     marshal to JSON → Redis SET TTL 3hr → return
```

After scheduler completes or manual recalculate finishes:
```
Redis DEL cluster:{id}:recommendations   ← invalidate cache
```
Next dashboard request fetches fresh data from PostgreSQL and repopulates cache.

**Distributed lock with SetNX (rate limiting):**

```go
// SetNX = SET if Not eXists — atomic, prevents race conditions
acquired := redis.SetNX("lock:cluster:{id}:recalculate", "1", 10*time.Minute)
if !acquired {
    return "already running"
}
defer redis.Del("lock:cluster:{id}:recalculate")
// ... do work
```

SetNX is atomic — even if two requests arrive simultaneously, exactly one acquires the lock.

---

## Security Model

### Authentication and Authorization

| Mechanism | Implementation | Detail |
|-----------|---------------|--------|
| **User passwords** | bcrypt (cost=10) | One-way hash. Random salt auto-added — same password hashed twice gives different results. Deliberately slow — brute force impractical |
| **JWT tokens** | HMAC-SHA256 | `Signature = HMAC-SHA256(header + payload, JWT_SECRET)`. 24-hour expiry. Stateless — no database hit per request |
| **JWT algorithm check** | Explicit HMAC type assertion | `t.Method.(*jwt.SigningMethodHMAC)` — prevents algorithm confusion attacks where attacker sends `"alg":"none"` |
| **Prometheus tokens** | AES-256-GCM | Encrypted before storing in PostgreSQL. `ENCRYPTION_KEY` never stored in DB |

### bcrypt Salt Detail

```
bcrypt.GenerateFromPassword(password, cost=10)
```

bcrypt automatically generates a random 128-bit salt and embeds it in the output hash. The hash output format is:
```
$2a$10$<22-char-salt><31-char-hash>
```

When verifying: `bcrypt.CompareHashAndPassword` extracts the salt from the stored hash, re-hashes the input with that same salt, and compares. This is why the same password hashed twice produces different output — each has a different random salt — but both verify correctly against their own stored hash.

### AES-256-GCM Nonce Detail

```
AES-256-GCM encryption:
  1. Generate random 12-byte nonce (96 bits)
  2. ciphertext = AES-GCM-Encrypt(plaintext, key, nonce)
  3. Store: nonce + ciphertext (prepended)

AES-256-GCM decryption:
  1. Extract first 12 bytes = nonce
  2. remainder = ciphertext
  3. plaintext = AES-GCM-Decrypt(ciphertext, key, nonce)
```

GCM (Galois/Counter Mode) provides **authenticated encryption** — it detects if the ciphertext was tampered with. If anyone modifies the stored encrypted token, decryption fails with an authentication error rather than returning garbled data.

### JWT Algorithm Confusion Attack Prevention

```go
// In jwt.go ParseWithClaims callback:
func(t *jwt.Token) (interface{}, error) {
    if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
    }
    return []byte(secret), nil
}
```

Without this check, an attacker could craft a token with `"alg":"none"` (no signature). Libraries that blindly trust the `alg` field would skip verification and accept the forged token. The explicit type assertion enforces HS256 and rejects everything else.

### Security Properties Summary

| Concern | Approach |
|---------|----------|
| Prometheus token at rest | AES-256-GCM encrypted — stolen DB = useless tokens |
| Transit encryption | TLS enforced on all Hub → Prometheus connections |
| Dashboard auth | JWT — stateless, 24hr expiry |
| Password storage | bcrypt — one-way, salted, slow |
| Input validation | `binding:"required"` — Gin rejects missing fields |
| Secrets in logs | Never — no tokens, credentials, or JWT secrets logged |
| API access | JWT middleware on all cluster routes |
| No plaintext credentials | All secrets via environment variables or Kubernetes Secrets |
| Request tracing | Every request gets a unique `X-Request-ID` header |

---

## Collection Pipeline

The collection pipeline runs per cluster per scheduler tick:

```
internal/collector/prometheus.go
        │
        ├── Collect(ctx, clusterURL, token, lookbackWindow)
        │         │
        │         ├── parseDuration("7d") → 7 * 24 * time.Hour
        │         ├── start = now - 7d, end = now
        │         │
        │         ├── queryRange(cpuQuery, start, end)
        │         │     · builds /api/v1/query_range URL
        │         │     · step=3600 → 168 data points (one per hour)
        │         │     · Authorization: Bearer <token>
        │         │     · response: [[timestamp, "0.120"], ...]
        │         │     · extractValues() → [0.120, 0.115, ...]
        │         │
        │         ├── queryRange(memQuery, start, end)
        │         │
        │         └── mergeMetrics(cpuData, memData)
        │               · map by (namespace, pod, container)
        │               → []*ContainerMetrics
        │
internal/compute/p99.go
        │
        └── ComputeP99(containerMetrics) → p99_cpu, p99_mem per container
              · sort values
              · index = ceil(0.99 * len) - 1
              · return values[index]

internal/recommender/recommender.go
        │
        └── Recommend(containerMetrics, p99Results)
              · RecommendedCPULimit = ceil(p99_cpu × 2)
              · RecommendedMemLimit = ceil(p99_mem × 2)
              · status = "ready" if data > 7d, else "new_service"
              → []*models.Recommendation

internal/store/recommendation.go
        │
        └── UpsertRecommendation(ctx, rec)
              · ON CONFLICT (cluster_id, namespace, pod_name, container_name)
              · DO UPDATE SET p99_cpu=..., recommended_cpu_limit=..., updated_at=NOW()
```

**ContainerMetrics struct (intermediate):**

```go
type ContainerMetrics struct {
    Namespace     string    // "payments"
    PodName       string    // "payment-api"
    ContainerName string    // "api"
    CPUValues     []float64 // 168 values over 7d, millicores
    MemValues     []float64 // 168 values over 7d, MiB
}
```

One `ContainerMetrics` per container. One pod with 3 containers = 3 `ContainerMetrics` objects.

**Unit normalization:**

| Resource | Prometheus raw | Internal unit | Conversion |
|----------|---------------|---------------|------------|
| CPU | cores/second | millicores | `rate(...) * 1000` in PromQL |
| Memory | bytes | MiB | `/ 1048576` in PromQL |
| Limits | Kubernetes string | millicores / MiB | Normalized at ingest |

---

## Testing Approach

### Test Architecture

Tests live in `tests/` at the project root. Run with:
```
go test ./tests/... -count=1 -p 1
```

```
go test ./tests/... -count=1 -p 1
        ↓
TestMain(m)    → SETUP:
  1. adminURL  → connect to default "postgres" database
  2.            → DROP DATABASE podoptix_test WITH (FORCE)
  3.            → CREATE DATABASE podoptix_test  (clean slate every run)
  4. testDBURL → run SyncSchema (create tables via migrations)
  5.            → store.New() — open connection pool
  6. Redis     → connects to Redis DB index 1 (isolated from dev data)
  7.            → Start real TCP server on port 9090
  8.            → print "Running PodOptix API Tests..."
        ↓
m.Run()        → Go scans all _test.go files, runs every func TestXxx(t *testing.T)
        ↓
TestMain(m)    → TEARDOWN:
  9.            → server.Shutdown()
  10.           → db.Close()
  11. adminURL → DROP DATABASE podoptix_test WITH (FORCE)
  12.           → print Total / Passed / Failed
        ↓
os.Exit(code)  → 0 = all passed, 1 = some failed
```

### Why Two Database URLs

```go
adminURL  = "postgres://...@localhost:5432/postgres"       // default system DB
testDBURL = "postgres://...@localhost:5432/podoptix_test"  // our test DB
```

PostgreSQL cannot drop a database while connected to it. `adminURL` connects to the neutral `postgres` database to CREATE and DROP `podoptix_test`. `testDBURL` connects to `podoptix_test` to run migrations and tests.

### Real TCP Server on Port 9090

Tests use a real TCP server started on `localhost:9090` — not `httptest.NewRecorder`. All requests are made over a real HTTP connection:

```go
resp, err := http.Get("http://localhost:9090/healthz")

// check results
resp.StatusCode         // HTTP status code
body, _ := io.ReadAll(resp.Body)  // response body
```

The full middleware stack runs: `RequestIDMiddleware → JWTMiddleware → handler → store → real PostgreSQL + Redis`.

### Fake Prometheus Server (collector tests)

```go
// httptest.NewServer creates a real HTTP server on a random port
fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // return fake Prometheus JSON response
    w.WriteHeader(200)
    w.Write(fakePrometheusResponse(...))
}))
defer fakeServer.Close()

// Collector makes real HTTP calls to fakeServer — we test OUR code, not Prometheus
collector := New(fakeServer.URL, "test-token", 30*time.Second)
metrics, err := collector.Collect(ctx, "7d")
```

This tests our request building, response parsing, auth header attachment, and error handling — without needing a real Prometheus instance.

### Test File Organization

```
tests/
  health_test.go    → TestHealthz, TestReadyz
  auth_test.go      → TestRegister, TestLogin, TestProtectedRoute_*
  clusters_test.go  → TestCreateCluster, TestListClusters, TestGetCluster, TestDeleteCluster
```

Tests are run with `-p 1` (sequential) to prevent parallel database conflicts. The `-count=1` flag disables test result caching.

### Current Test Coverage

| Test | What it verifies |
|------|-----------------|
| `TestHealthz` | `/healthz` returns 200 and `{"status":"ok"}` |
| `TestRegister` | POST creates user, returns 201 with token |
| `TestRegister_DuplicateEmail` | same email twice → 409 |
| `TestRegister_MissingFields` | missing password → 400 |
| `TestLogin` | register then login → 200 + token |
| `TestLogin_WrongPassword` | wrong password → 401 |
| `TestLogin_UnknownEmail` | unknown email → 401 same message |
| `TestProtectedRoute_NoToken` | no Authorization header → 401 |
| `TestProtectedRoute_WrongFormat` | malformed header → 401 |
| `TestProtectedRoute_InvalidToken` | `Bearer fakejwt` → 401 |
| `TestProtectedRoute_WithToken` | valid token → 200 |
| `TestCreateCluster` | POST creates cluster, returns 201 with data |
| `TestCreateCluster_MissingFields` | POST without required fields → 400 |
| `TestListClusters` | GET returns 200 |
| `TestGetCluster` | GET by ID returns 200 with correct cluster |
| `TestGetCluster_NotFound` | GET with fake ID returns 404 |
| `TestDeleteCluster` | DELETE removes cluster, subsequent GET returns 404 |

### Integration vs E2E

| | Integration tests (current) | E2E tests (planned) |
|--|---|---|
| Server | Real TCP server on port 9090 | Real running server |
| Database | `podoptix_test` (auto-created) | Staging/dev cluster |
| Redis | DB index 1 (isolated) | Staging Redis |
| Speed | Fast — milliseconds | Slow — seconds |
| When | Every commit | Before deployment |
| Catches | Code bugs | Deployment and config bugs |
