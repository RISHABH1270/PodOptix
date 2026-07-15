# PodOptix — Engineering Trade-offs

Every decision here was made intentionally. This doc records what we chose, what we rejected, and why — so future contributors understand the reasoning.

---

## Summary Table

| Component | Choice | Key Reason |
|-----------|--------|------------|
| Language | Go | K8s ecosystem · single binary |
| Database | PostgreSQL | ACID · structured data · pgx driver |
| Cache | Redis | TTL · PromQL result caching |
| API Framework | Gin | Fast · widely adopted in Go |
| Deployment | Helm | Industry standard K8s distribution |
| Prometheus client | prometheus/client_golang | Official · battle-tested |
| Auth | JWT + API tokens | Simple · stateless |
| ID Strategy | UUID v4 (string) | Globally unique · secure · no collision risk |
| Recommendation storage | UPSERT | One row per container — clean dashboard |
| Resource percentile | p99 × 2 | Real usage + smart buffer — not freak spikes |

---

## 1. Programming Language

### Decision: Go

| Option | Pros | Cons |
|--------|------|------|
| **Go** ✅ | Native K8s ecosystem · Single binary · Low memory · Strong concurrency · Official Prometheus client library | Steeper learning curve than Python |
| Python | Easy to write · Great data libs (pandas, numpy) | High memory · Slow startup · Not idiomatic in K8s tooling |
| Node.js | Fast API development | Weak typing · Poor K8s ecosystem · Not suited for systems tooling |
| Rust | Extremely fast · Low memory | Very steep learning curve · Small K8s ecosystem |

**Why Go:** Kubernetes, Prometheus, Grafana, and virtually every major infrastructure tool is written in Go. The official Prometheus HTTP client (`prometheus/client_golang`) and Kubernetes client (`client-go`) are both Go-native. Go compiles to a single binary — easy to ship in a container with zero runtime dependencies.

---

## 2. Database

### Decision: PostgreSQL

| Option | Pros | Cons |
|--------|------|------|
| **PostgreSQL** ✅ | ACID · Production-grade · Excellent Go libraries (pgx) · JSON support · Open source | Needs a running Postgres instance |
| SQLite | Zero config · Embedded | Not suited for multi-instance · No concurrent writes |
| MySQL | Widely adopted | Weaker JSON support · Less feature-rich than Postgres |
| MongoDB | Flexible schema · Document store | Overkill · Weaker consistency guarantees · Not ideal for structured recommendations |
| TimescaleDB | Built for time series | Extra complexity · Postgres extension · Harder to operate |

**Why PostgreSQL:** Recommendations and cluster metadata are structured, relational data. PostgreSQL handles this perfectly with strong consistency (ACID), great indexing, and native JSON columns for storing metric snapshots. `pgx` is the best-in-class Go driver for Postgres.

### Connection Pool Settings

```go
pool settings:
  max connections:    10
  min connections:    2 (always warm)
  max conn lifetime:  1 hour (refresh prevents stale connections)
  idle timeout:       30 minutes (close idle connections to save resources)
```

**Rationale:** `min: 2` ensures two connections are always ready — no latency on the first request after idle. `max: 10` prevents overwhelming the database under burst load. `1hr lifetime` refreshes connections to catch credential rotations and network changes. `30min idle timeout` releases connections that are not being used during off-peak hours.

---

## 3. Cache

### Decision: Redis

| Option | Pros | Cons |
|--------|------|------|
| **Redis** ✅ | TTL support · Persistent · Industry standard · Pub/Sub capable | Extra infrastructure component |
| In-memory (Go map) | Zero config · Fast | Lost on restart · Not shareable across Hub instances · No TTL |
| Memcached | Simple · Fast | No persistence · No TTL per key · No data structures |

**Why Redis:** PromQL queries against large clusters can be expensive. Redis lets us cache results with a TTL (default: 1 hr) so repeated requests don't hammer Prometheus. Redis also supports future use cases like session storage and pub/sub for live dashboard updates.

### Cache-Aside Pattern

PodOptix uses the cache-aside (lazy-loading) pattern — not write-through:

```
Read path:
  1. Check Redis
  2. HIT  → return cached data
  3. MISS → read from PostgreSQL → write to Redis with TTL → return

Write path (scheduler completes):
  1. Write to PostgreSQL
  2. Invalidate Redis key (DEL)
  3. Next read repopulates the cache
```

**Why cache-aside over write-through:** The scheduler writes infrequently (once/day). Write-through would add Redis write overhead on every UPSERT for minimal benefit. Cache-aside is simpler and correct: the cache is invalidated when data changes, and lazily repopulated on next read.

### Rate Limiting with Redis SetNX

```go
// Prevent duplicate recalculate jobs for the same cluster
acquired := redis.SetNX("lock:cluster:{id}:recalculate", "1", 10*time.Minute)
if !acquired {
    return errors.New("recalculation already in progress for this cluster")
}
defer redis.Del("lock:cluster:{id}:recalculate")
```

`SetNX` (SET if Not eXists) is atomic in Redis — even if two requests arrive simultaneously, exactly one acquires the lock. This prevents:
- Two scheduler ticks running simultaneously for the same cluster
- Manual recalculate triggered while scheduler is already running
- 200 concurrent Prometheus calls for a 100-cluster deployment

---

## 4. REST API Framework

### Decision: Gin (Go)

| Option | Pros | Cons |
|--------|------|------|
| **Gin** ✅ | Fast · Minimal · Battle-tested · Great middleware ecosystem | Opinionated routing |
| Echo | Similar to Gin · Slightly cleaner API | Smaller community |
| net/http (stdlib) | Zero dependencies · Full control | Too verbose for REST APIs |
| Fiber | Very fast · Express-like | Newer · Smaller ecosystem |

**Why Gin:** Gin is the most widely adopted Go HTTP framework for REST APIs. It's fast, has great middleware support (CORS, auth, logging), and has extensive documentation. Most Go REST API examples and tutorials use Gin — easier onboarding for contributors.

---

## 5. Deployment

### Decision: Helm Chart

| Option | Pros | Cons |
|--------|------|------|
| **Helm** ✅ | Industry standard for K8s apps · Templating · Versioned releases · Easy upgrades | Requires Helm CLI |
| Kustomize | No templating engine · Pure YAML | Less flexible for external distribution |
| Raw YAML manifests | Simple | No templating · Hard to configure per environment |
| Operator | Most powerful | Way too complex for MVP |

**Why Helm:** Every platform engineer knows Helm. `helm install podoptix` is the simplest onboarding story. Helm also handles upgrades, rollbacks, and values overrides cleanly — essential for a tool sold to enterprises.

---

## 6. Prometheus Client

### Decision: Official `prometheus/client_golang`

| Option | Pros | Cons |
|--------|------|------|
| **prometheus/client_golang** ✅ | Official · Battle-tested · Full PromQL HTTP API support | None significant |
| Raw HTTP (`net/http`) | No extra dependency | Reinventing the wheel · Error-prone |
| Third-party wrappers | Simpler API sometimes | Less maintained · Hidden abstractions |

**Why official client:** It's maintained by the Prometheus team, supports the full `/api/v1/query_range` API, handles TLS, basic auth, and response parsing out of the box.

---

## 7. Authentication

### Decision: JWT for users · API tokens per cluster

| Concern | Decision | Reasoning |
|---------|----------|-----------|
| Dashboard user auth | JWT (JSON Web Tokens) | Stateless · Industry standard · Easy to implement in Go with `golang-jwt` |
| Cluster Prometheus access | Static API tokens per cluster | Simple · Customer controls their Prometheus auth · No OAuth complexity needed at MVP |
| Token storage | Encrypted at rest in PostgreSQL | Never stored in plaintext |

---

## 8. ID Strategy

### Decision: UUID v4 as string

| Option | Pros | Cons |
|--------|------|------|
| **UUID v4 (string)** ✅ | Globally unique · Secure · No central counter needed · Industry standard | Slightly larger storage than int |
| Auto-increment int | Simple · Small storage | Guessable · Clashes across distributed systems · Security risk |

**UUID v4 details:**
- 36 characters: `a3f8c2d1-9b4e-4f1a-8c3d-2e5f7a9b1c4d` (32 hex + 4 dashes)
- 128 bits total — **122 bits random**, 6 bits fixed (4 = version, 2 = variant)
- 122 bits of randomness → 340,282,366,920,938,463,463,374,607,431,768,211,456 possible values
- Collision probability: generate 1 billion UUIDs/second for 100 years → `0.0000000000000000006%`
- Sequential IDs (`1, 2, 3...`) are guessable — attacker tries `/clusters/1`, `/clusters/2`. UUID prevents enumeration attacks

**Collision safety — two layers:**
1. UUID randomness (122 bits) makes collision practically impossible
2. `PRIMARY KEY` constraint in PostgreSQL rejects any duplicate — if a collision ever occurs, the insert fails and a new UUID is generated

**Why `lookback_window` is also a string:**
Stored as `"7d"`, `"24h"`, `"30d"` — not as a plain integer. A plain `7` loses the unit (days? hours?). The string format is self-describing and maps directly to PromQL range syntax.

---

## 9. Data Model — Foreign Key Design

### Decision: Separate tables linked by ClusterID (Foreign Key)

| Option | Pros | Cons |
|--------|------|------|
| **Separate tables + Foreign Key** ✅ | No data duplication · Easy to update cluster info in one place · Industry standard relational design | Requires JOIN queries |
| Single flat table | Simple queries | Cluster URL/name repeated thousands of times · Wasteful · Hard to update |

```
CLUSTER TABLE
─────────────────────────────────────
ID          │ Name
────────────┼────────────────────────
"abc-123"   │ production-cluster
"def-456"   │ staging-cluster

RECOMMENDATION TABLE
──────────────────────────────────────────────
ID          │ ClusterID   │ PodName
────────────┼─────────────┼──────────────────
"xyz-789"   │ "abc-123"   │ payment-api
"xyz-790"   │ "abc-123"   │ auth-service
"xyz-791"   │ "def-456"   │ payment-api
```

Cluster info is stored once and referenced many times — not repeated per recommendation.

---

## 10. Why p99 and not p100

### Decision: p99 × 2 as the recommended limit

| Option | Pros | Cons |
|--------|------|------|
| **p99 × 2** ✅ | Based on real sustained usage · Smart buffer included · Cost-effective | Ignores the top 1% of spikes |
| p100 × 2 | Covers every spike ever | One freak spike ruins the limit — massive overprovisioning |
| p95 × 2 | Even more cost savings | Too aggressive — ignores too many real usage peaks |

**The problem with p100:**

p100 is the absolute maximum ever recorded. If a pod normally uses 80m–120m CPU but once spiked to 2000m for 30 seconds due to a bug:

```
p100 = 2000m  →  limit = 2000m × 2 = 4000m
```

You pay for 4000m every second — because of one 30-second incident. That is pure waste.

**Why p99 × 2 is correct:**

```
p99  = 120m   →  limit = 120m  × 2 = 240m
```

The ×2 multiplier IS the safety buffer for that top 1%. You are not ignoring spikes — you are budgeting for them intelligently.

**What lives in that top 1%:**
- One-time bugs
- Deployment restart spikes
- Abnormal batch jobs
- Once-in-a-week events

These should not define your permanent resource limits.

| | p100 | p99 × 2 |
|--|------|---------|
| Based on | Worst freak spike ever | Real sustained usage + smart buffer |
| Result | Massive overprovisioning | Right-sized with safe headroom |
| Optimizes for | Paranoia | Reality |

---

## 11. Cold Start Problem — New Services with No Historical Data

### Decision: `new_service` status + namespace average as bootstrap

When a service is newly deployed it has zero historical data in Prometheus. No data = no p99 = no recommendation.

**Three phases for new services:**

```
Phase 1 — Day 0 to Day 7   → Status: new_service
          Service is new. PodOptix shows this status so the customer
          knows it is not a system issue — the service simply has no history yet.
          Dashboard shows: "Recommendation available after 7 days."

Phase 2 — After 7 days     → Status: ready
          7 days of real usage data exists.
          p99 computed. Recommendation generated.

Phase 3 — Ongoing          → Status: ready
          Recommendations updated every 24 hours as usage evolves.
```

**Why only two statuses — `new_service` and `ready`:**

We intentionally kept it simple. Both "brand new service" and "sparse data" tell the customer the same thing — not ready yet. There is no value in showing the customer a third status.

| Status | Meaning |
|--------|---------|
| `new_service` | Not enough data yet — recommendation available after 7 days |
| `ready` | p99 computed — recommendation is available |

**Why `new_service` and not other names:**

| Status | Problem |
|--------|---------|
| `collecting` | Makes it sound like the system is slow or broken |
| `pending` | Implies something is wrong or stuck |
| `warming_up` | Sounds like a system issue |
| `new_service` ✅ | Clear — the service is new, no history exists yet. Not a system problem. |

**Bootstrap strategy for Phase 1:**

While waiting for 7 days of data, PodOptix uses the **namespace average** of existing services as a starting point:

```
payment-api  → p99 cpu: 120m
auth-api     → p99 cpu: 80m
order-api    → p99 cpu: 100m
─────────────────────────────────────
new-service  → initial estimate: 100m  (namespace average × 2 = 200m limit)
```

This is clearly labeled as an estimate, not a real recommendation.

---

## 12. Cluster Status Values

### Decision: pending / healthy / unhealthy

When a cluster is registered, its status goes through:

| Status | When | Meaning |
|--------|------|---------|
| `pending` | Immediately after registration | Registered but not yet queried by scheduler |
| `healthy` | After successful collection job | Last Prometheus query succeeded |
| `unhealthy` | After failed collection job | Prometheus unreachable, auth error, timeout, etc. |

**Why not just healthy/unhealthy:**

A newly registered cluster has never been queried. Calling it "unhealthy" would be wrong — the scheduler hasn't tried yet. `pending` correctly communicates "registered, waiting for first collection run."

**Why not `ok` / `error`:**

`healthy` and `unhealthy` are the standard Kubernetes health terminology. Platform engineers immediately understand these values without needing to look them up.

---

## 13. Resource Unit Standardization

### Decision: Millicores for CPU · Mebibytes for Memory — stored as integers

Kubernetes allows many formats for the same value:

```
"1000m" = "1" = "1.0"       ← all mean 1 CPU core
"1Gi"   = "1024Mi"          ← all mean same memory
"1.2Gi" = "1228.8Mi"        ← decimal Gi
```

Storing raw strings makes comparison and math impossible.

| Resource | Internal Unit | Type | Why |
|----------|-------------|------|-----|
| CPU | millicores | `int` | Easy math — p99 × 2 = whole number. 1 core = 1000m |
| Memory | Mebibytes (MiB) | `int` | Most precise. 1Gi = 1024Mi. No floating point errors |

**Why not store in cores and GB directly?**

Storing as cores and GB would mean floats (`0.12 cores`, `0.18 GB`) — introduces floating point precision issues. `120 × 2 = 240` is exact. `0.12 × 2 = 0.24` has potential precision loss at scale. Kubernetes itself uses millicores internally. Prometheus returns millicores. We stay aligned with the ecosystem.

**Conversion rules:**
- `1` CPU → `1000` millicores
- `0.5` CPU → `500` millicores
- `1Gi` memory → `1024` MiB
- `1.2Gi` memory → `1229` MiB (rounded up)

**Display layer converts back to human-readable:**
```
2000 millicores → "2 cores"
1229 MiB        → "1.2Gi"
512  MiB        → "512Mi"
```

**Rule:** millicores/MiB in storage → human-readable only in the display layer (`fmtCPU()`, `fmtMem()` in frontend).

---

## 14. Database Index on `cluster_id`

### Decision: Index `recommendations.cluster_id` for fast cluster-based lookups

The most common query in PodOptix:

```sql
SELECT * FROM recommendations WHERE cluster_id = 'abc-123'
```

Runs every time a customer opens their dashboard.

**Without index (1 million rows):**
```
Row 1       → not a match, skip
Row 2       → not a match, skip
...
Row 1000000 → done
```
Time: proportional to total rows — gets slower as data grows.

**With B-Tree index:**
```
"abc-123" found in 3 steps regardless of table size
```

| | Without Index | With Index |
|--|--------------|-----------|
| Read speed | Slow — scans all rows | Fast — direct jump |
| Write speed | Fast | Slightly slower (index updates on insert) |
| Storage | Less | Slightly more |

We index `cluster_id` because it is the primary filter in every dashboard query. The read performance gain far outweighs the minor write overhead.

---

## 15. Recommendation Storage Strategy — UPSERT not INSERT

### Decision: One row per container, updated in place daily

| Option | Pros | Cons |
|--------|------|------|
| **UPSERT — one row per container** ✅ | Always one current recommendation per container · Clean dashboard · No confusion | No history |
| INSERT new row every run | Full history of every recommendation | After 360 days — 360 rows per container. User can't tell which to apply |

**The problem with INSERT every run:**

A pod running for 360 days with a daily scheduler = 360 recommendation rows per container. The user opens the dashboard and sees hundreds of rows with no indication which is the latest.

**The fix — UPSERT:**

```sql
INSERT INTO recommendations (...) VALUES (...)
ON CONFLICT (cluster_id, namespace, pod_name, container_name)
DO UPDATE SET
    p99_cpu = EXCLUDED.p99_cpu,
    p99_mem = EXCLUDED.p99_mem,
    recommended_cpu_limit = EXCLUDED.recommended_cpu_limit,
    recommended_mem_limit = EXCLUDED.recommended_mem_limit,
    updated_at = EXCLUDED.updated_at;
```

One row per container, always showing the latest values. `updated_at` shows when it was last recalculated.

**Two triggers for recalculation:**
1. **Automatic** — scheduler runs once per day for all clusters
2. **Manual** — "Recalculate" button in dashboard triggers on-demand refresh via Redis job queue

---

## 16. Database Migration Strategy

### Decision: Never edit existing migration files after production deployment

| Situation | Action | Why |
|-----------|--------|-----|
| Before first production deployment | Edit migration files freely, drop and recreate local DB | No real data exists — nothing is lost |
| After first production deployment | Add a new `ALTER TABLE` migration file | Real customer data exists — cannot drop or re-run |

**Before production (development phase):**

Migration files can be edited and the local database dropped and recreated safely:

```bash
docker exec -it podoptix-db psql -U postgres \
  -c "DROP DATABASE podoptix WITH (FORCE); CREATE DATABASE podoptix;"
```

**After production deployment:**

Never touch existing migration files. Every schema change requires a new numbered migration file:

```sql
-- 000004_rename_columns.up.sql
ALTER TABLE clusters RENAME COLUMN id TO cluster_id;
```

`SyncSchema` tracks which files already ran in the `schema_migrations` table. It skips already-applied migrations — editing an old file has no effect on an existing database.

**The real cost:** `ALTER TABLE` in production runs against a live table with real data. For large tables this can be slow. The cost is unavoidable — even AWS, Stripe, and Google pay this cost for every schema change. This is why getting the schema right before first deployment matters.

**Dirty database recovery:**

golang-migrate marks a migration dirty the moment it starts. If the app crashes halfway — the flag stays dirty. On next startup, PodOptix detects this, forces the version clean, and retries. Safe because migrations use `IF NOT EXISTS`.

---

## 17. Authentication — JWT (JSON Web Token)

### Decision: JWT for user authentication

**Why not username + password on every request:**
- Hits the database on every API call — slow
- Sending passwords repeatedly over the wire — risky
- Does not scale across multiple services

**How JWT works:**

```
Step 1  POST /auth/login { email, password }
        Server verifies credentials → issues JWT token (24hr expiry)

Step 2  GET /api/v1/clusters
        Authorization: Bearer eyJhbGci...
        Server verifies signature (no database hit) → allows request

Step 3  GET /api/v1/clusters (no token or expired)
        → 401 Unauthorized
```

**Why no database hit on every request:**

```
Signature = HMAC-SHA256(header + payload, JWT_SECRET)
```

The signature is mathematically tied to the payload and the secret. Server verifies by recomputing — if they match, token is valid. No database needed.

**JWT algorithm confusion attack prevention:**

```go
// In ParseWithClaims callback — explicitly require HMAC
if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
    return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
}
```

Without this check, an attacker could set `"alg":"none"` to bypass signature verification. The explicit type assertion enforces HS256.

**Risks and mitigations:**

| Risk | Mitigation |
|------|-----------|
| Token stolen (network) | Always use HTTPS — token safe over TLS |
| Token stolen from client | Short expiry (24 hrs) — expires automatically |
| `JWT_SECRET` leaked | Rotate secret immediately — all tokens invalidated |

**`JWT_SECRET` rules:** 32+ random characters · never logged · never committed to Git · stored as Kubernetes Secret in production

---

## 18. Redis — Purpose and Design

### Decision: Redis as cache + distributed lock + job queue

Same Redis instance serves three roles:

| Role | Key Pattern | TTL | Why |
|------|------------|-----|-----|
| **Recommendations cache** | `cluster:{id}:recommendations` | 1 hour | Dashboard reads frequently — serve from Redis not PostgreSQL every request |
| **Distributed lock** | `lock:cluster:{id}:recalculate` | 10 min | Prevent duplicate recalculate jobs for same cluster |
| **Job queue** | `recalculate-jobs` | no TTL | Sequential processing — one cluster at a time, prevents server overload |

**Why cache recommendations (not raw metrics):**

Recommendations are already computed and stored in PostgreSQL by the scheduler. The dashboard reads them on every page load. Caching the final result in Redis means the dashboard never hits PostgreSQL directly — just Redis (sub-millisecond).

**Why job queue for recalculate:**

100 clusters × 1000 pods = 100,000 containers to process. If all triggered simultaneously:
- 200 concurrent Prometheus HTTP calls
- 100,000 p99 computations
- 100,000 DB upserts → server crash

Queue ensures ONE cluster is processed at a time. User gets immediate "queued" response.

---

## 19. Token Encryption at Rest (AES-256-GCM)

### Decision: Encrypt Prometheus tokens with AES-256-GCM before storing in PostgreSQL

**Problem:** Prometheus auth tokens stored as plain text in the database. If the database is compromised, attackers get every cluster's token and can query any customer's Prometheus directly.

**Solution:**

```
Register cluster:
  plain token → AES-256-GCM encrypt(token, ENCRYPTION_KEY) → store in DB

Use cluster (scheduler/collector):
  fetch from DB → AES-256-GCM decrypt(token, ENCRYPTION_KEY) → use for Prometheus
```

**Why AES-256-GCM:**
- AES-256 = industry standard, used by banks and governments
- GCM mode = authenticated encryption — detects if ciphertext was tampered with (integrity check built in)
- Symmetric — same key encrypts and decrypts
- Key comes from `ENCRYPTION_KEY` env variable — never stored in database

**Nonce detail:**

```
encrypt:
  1. Generate random 12-byte nonce
  2. ciphertext = AES-GCM-Encrypt(plaintext, key, nonce)
  3. store: nonce + ciphertext (nonce prepended)

decrypt:
  1. Extract first 12 bytes = nonce
  2. plaintext = AES-GCM-Decrypt(ciphertext[12:], key, nonce)
```

A new random nonce is generated for each encryption operation — even encrypting the same token twice produces different ciphertext.

**Security guarantee:** Stolen database + no `ENCRYPTION_KEY` = useless encrypted tokens.

**Key rules:** 32 bytes (256 bits) · never logged · stored as Kubernetes Secret in production · rotating the key invalidates all stored tokens (requires re-encryption of existing tokens)

---

## 20. Password Storage — bcrypt

### Decision: bcrypt with cost=10

**Why bcrypt for passwords:**
- One-way hash — impossible to reverse
- Built-in random salt — same password produces different hashes each time
- Deliberately slow (cost factor) — makes brute force attacks impractical
- Industry standard used by every major platform

**Salt detail:**

```
bcrypt.GenerateFromPassword(password, cost=10)
```

bcrypt automatically generates a random 128-bit salt and embeds it in the output. The stored hash format is:
```
$2a$10$<22-char-salt><31-char-hash>
```

`bcrypt.CompareHashAndPassword` extracts the salt from the stored hash, re-hashes the input with that same salt, and compares. The attacker cannot precompute a rainbow table because every hash has a unique random salt.

**Cost factor 10:** 2^10 = 1024 rounds of hashing. Takes ~100ms on modern hardware — acceptable for login UX, but makes brute-forcing 1 billion passwords take ~3 years instead of seconds.

---

## 21. User Enumeration Prevention

### Decision: Same error message for wrong email AND wrong password

```go
// Wrong email:
return 401, "Invalid email or password"

// Wrong password:
return 401, "Invalid email or password"
```

**Why:** If we returned "Email not found" for unknown emails, an attacker could enumerate valid accounts by trying email addresses. The identical error message prevents an attacker from learning whether a given email is registered.

---

## 22. Input DTO Pattern

### Decision: Separate request structs from database models

```go
// What the customer sends:
type CreateClusterRequest struct {
    Name           string `json:"name"            binding:"required"`
    PrometheusURL  string `json:"prometheus_url"  binding:"required"`
    Token          string `json:"token"           binding:"required"`
    LookbackWindow string `json:"lookback_window" binding:"required"`
}

// What lives in the database (server-generated fields added by handler):
type Cluster struct {
    ClusterID      string    `json:"cluster_id"`
    Name           string    `json:"name"`
    PrometheusURL  string    `json:"prometheus_url"`
    Token          string    `json:"-"`          // never in API response
    LookbackWindow string    `json:"lookback_window"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}
```

**Why:** If the customer sends `cluster_id` in their request JSON, Gin ignores it completely — it is not in the request struct. Server-generated fields (`cluster_id`, `created_at`, `updated_at`) can never be overridden by the customer.
