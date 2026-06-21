# PodOptix — Technology Decisions & Trade-offs

Every decision here was made intentionally. This doc records what we chose, what we rejected, and why — so future contributors understand the reasoning.

---

## 1. Programming Language

### Decision: **Go**

| Option | Pros | Cons |
|--------|------|------|
| **Go** ✅ | Native K8s ecosystem · Single binary · Low memory · Strong concurrency · Official Prometheus client library | Steeper learning curve than Python |
| Python | Easy to write · Great data libs (pandas, numpy) | High memory · Slow startup · Not idiomatic in K8s tooling |
| Node.js | Fast API development | Weak typing · Poor K8s ecosystem · Not suited for systems tooling |
| Rust | Extremely fast · Low memory | Very steep learning curve · Small K8s ecosystem |

**Why Go:** Kubernetes, Prometheus, Grafana, and virtually every major infrastructure tool is written in Go. The official Prometheus HTTP client (`prometheus/client_golang`) and Kubernetes client (`client-go`) are both Go-native. Go compiles to a single binary — easy to ship in a container with zero runtime dependencies.

---

## 2. Database

### Decision: **PostgreSQL**

| Option | Pros | Cons |
|--------|------|------|
| **PostgreSQL** ✅ | ACID · Production-grade · Excellent Go libraries (pgx) · JSON support · Open source | Needs a running Postgres instance |
| SQLite | Zero config · Embedded | Not suited for multi-instance · No concurrent writes |
| MySQL | Widely adopted | Weaker JSON support · Less feature-rich than Postgres |
| MongoDB | Flexible schema · Document store | Overkill · Weaker consistency guarantees · Not ideal for structured recommendations |
| TimescaleDB | Built for time series | Extra complexity · Postgres extension · Harder to operate |

**Why PostgreSQL:** Recommendations and cluster metadata are structured, relational data. PostgreSQL handles this perfectly with strong consistency (ACID), great indexing, and native JSON columns for storing metric snapshots. `pgx` is the best-in-class Go driver for Postgres.

---

## 3. Cache

### Decision: **Redis**

| Option | Pros | Cons |
|--------|------|------|
| **Redis** ✅ | TTL support · Persistent · Industry standard · Pub/Sub capable | Extra infrastructure component |
| In-memory (Go map) | Zero config · Fast | Lost on restart · Not shareable across Hub instances · No TTL |
| Memcached | Simple · Fast | No persistence · No TTL per key · No data structures |

**Why Redis:** PromQL queries against large clusters can be expensive. Redis lets us cache results with a TTL (default: 1 hr) so repeated requests don't hammer Prometheus. Redis also supports future use cases like session storage and pub/sub for live dashboard updates.

---

## 4. REST API Framework

### Decision: **Gin (Go)**

| Option | Pros | Cons |
|--------|------|------|
| **Gin** ✅ | Fast · Minimal · Battle-tested · Great middleware ecosystem | Opinionated routing |
| Echo | Similar to Gin · Slightly cleaner API | Smaller community |
| net/http (stdlib) | Zero dependencies · Full control | Too verbose for REST APIs |
| Fiber | Very fast · Express-like | Newer · Smaller ecosystem |

**Why Gin:** Gin is the most widely adopted Go HTTP framework for REST APIs. It's fast, has great middleware support (CORS, auth, logging), and has extensive documentation. Most Go REST API examples and tutorials use Gin — easier onboarding for contributors.

---

## 5. Frontend Dashboard

### Decision: **React + TypeScript**

| Option | Pros | Cons |
|--------|------|------|
| **React + TypeScript** ✅ | Largest ecosystem · Rich charting libs · Type safety | Bundle size · Build complexity |
| Vue | Simpler · Lighter | Smaller ecosystem for dashboards |
| Go HTML Templates | No separate build · Zero JS deps | Poor interactivity for dashboards · Hard to build charts |
| Angular | Opinionated · Full framework | Heavy · Overkill for MVP |

**Why React + TypeScript:** Dashboard needs charts, tables, and live-ish updates. React has the best ecosystem for this — Recharts or Chart.js for visualizations, React Query for data fetching, Tailwind for styling. TypeScript prevents a class of runtime errors in a data-heavy UI.

---

## 6. Deployment

### Decision: **Helm Chart**

| Option | Pros | Cons |
|--------|------|------|
| **Helm** ✅ | Industry standard for K8s apps · Templating · Versioned releases · Easy upgrades | Requires Helm CLI |
| Kustomize | No templating engine · Pure YAML | Less flexible for external distribution |
| Raw YAML manifests | Simple | No templating · Hard to configure per environment |
| Operator | Most powerful | Way too complex for MVP |

**Why Helm:** Every platform engineer knows Helm. `helm install podoptix` is the simplest onboarding story. Helm also handles upgrades, rollbacks, and values overrides cleanly — essential for a tool sold to enterprises.

---

## 7. Prometheus Client

### Decision: **Official `prometheus/client_golang`**

| Option | Pros | Cons |
|--------|------|------|
| **prometheus/client_golang** ✅ | Official · Battle-tested · Full PromQL HTTP API support | None significant |
| Raw HTTP (`net/http`) | No extra dependency | Reinventing the wheel · Error-prone |
| Third-party wrappers | Simpler API sometimes | Less maintained · Hidden abstractions |

**Why official client:** It's maintained by the Prometheus team, supports the full `/api/v1/query_range` API, handles TLS, basic auth, and response parsing out of the box.

---

## 8. Authentication

### Decision: **JWT for users · API tokens per cluster**

| Concern | Decision | Reasoning |
|---------|----------|-----------|
| Dashboard user auth | JWT (JSON Web Tokens) | Stateless · Industry standard · Easy to implement in Go with `golang-jwt` |
| Cluster Prometheus access | Static API tokens per cluster | Simple · Customer controls their Prometheus auth · No OAuth complexity needed at MVP |
| Token storage | Encrypted at rest in PostgreSQL | Never stored in plaintext |

---

## Summary

| Component | Choice | Key Reason |
|-----------|--------|------------|
| Language | Go | K8s ecosystem · single binary |
| Database | PostgreSQL | ACID · structured data · pgx driver |
| Cache | Redis | TTL · PromQL result caching |
| API Framework | Gin | Fast · widely adopted in Go |
| Frontend | React + TypeScript | Best dashboard ecosystem |
| Deployment | Helm | Industry standard K8s distribution |
| Prometheus client | prometheus/client_golang | Official · battle-tested |
| Auth | JWT + API tokens | Simple · stateless |
| ID Strategy | UUID v4 (string) | Globally unique · secure · no collision risk |

---

## 9. ID Strategy

### Decision: **UUID v4 as string**

| Option | Pros | Cons |
|--------|------|------|
| **UUID v4 (string)** ✅ | Globally unique · Secure · No central counter needed · Industry standard | Slightly larger storage than int |
| Auto-increment int | Simple · Small storage | Guessable · Clashes across distributed systems · Security risk |

**Why not integer IDs:**
- Integer IDs are sequential (`1, 2, 3...`) — an attacker can guess other cluster IDs and attempt unauthorized access
- In distributed systems, two services can independently generate the same integer ID causing collisions

**Why UUID v4:**
- 122 bits of randomness → 2^122 possible values → practically impossible to collide
- Cannot be guessed — protects against enumeration attacks
- Industry standard used by AWS, Stripe, GitHub, Google

**Collision safety:**
UUID is not 100% mathematically guaranteed to be unique, so we add a second layer of protection — a `PRIMARY KEY` constraint in PostgreSQL. If a duplicate UUID ever occurs (probability near zero), the database rejects the insert and a new UUID is generated.

**Why `Window` is also a string:**
The collection window (how far back to look in Prometheus) is stored as `"7d"`, `"24h"`, `"30d"` — not as a plain integer. A plain `7` loses the unit (days? hours?). The string format is self-describing and maps directly to PromQL range syntax: `container_cpu_usage_seconds_total[7d]`.

---

## 10. Data Model — Foreign Key Design

### Decision: **Separate tables linked by ClusterID (Foreign Key)**

Every table has its own `ID`. The `Recommendation` model has two ID fields:
- `ID` — the recommendation's own unique identity
- `ClusterID` — a pointer back to which cluster this recommendation belongs to

| Option | Pros | Cons |
|--------|------|------|
| **Separate tables + Foreign Key** ✅ | No data duplication · Easy to update cluster info in one place · Industry standard relational design | Requires JOIN queries |
| Single flat table | Simple queries | Cluster URL/name repeated thousands of times · Wasteful · Hard to update |

**How it works:**

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

`ClusterID` in Recommendation points to `ID` in Cluster. This is called a **Foreign Key** — the standard way relational databases model relationships. Cluster info is stored once and referenced many times instead of being repeated per recommendation.

---

## 11. Why p99 and not p100

### Decision: **p99 × 2 as the recommended limit**

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

**Why p99 × 2 is the right formula:**

```
p99  = 120m   →  limit = 120m  × 2 = 240m
```

The ×2 multiplier IS the safety buffer for that top 1%. You are not ignoring spikes — you are budgeting for them intelligently instead of over-provisioning for the worst freak event ever recorded.

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

## 12. Cold Start Problem — New Services with No Historical Data

### Decision: **`new_service` status + namespace average as bootstrap**

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
          Recommendations updated every N hours as usage evolves.
```

**Why only two statuses — `new_service` and `ready`:**

We intentionally kept it simple. Both "brand new service" and "sparse data" tell the customer the same thing — not ready yet. There is no value in showing the customer a third status. Two is enough.

| Status | Meaning |
|--------|---------|
| `new_service` | Not enough data yet — recommendation available after 7 days |
| `ready` | p99 computed — recommendation is available |

**Why `new_service` and not `collecting` or `pending`:**

| Status | Problem |
|--------|---------|
| `collecting` | Makes it sound like the system is slow or broken |
| `pending` | Implies something is wrong or stuck |
| `warming_up` | Sounds like a system issue |
| `new_service` ✅ | Clear — the service is new, no history exists yet. Not a system problem. |

**Bootstrap strategy for Phase 1:**
While waiting for 7 days of data, PodOptix uses the **namespace average** of existing services as a starting point — giving the team a reasonable initial estimate instead of nothing.

```
payment-api  → p99 cpu: 120m
auth-api     → p99 cpu: 80m
order-api    → p99 cpu: 100m
─────────────────────────────────────
new-service  → initial estimate: 100m  (namespace average × 2 = 200m limit)
```

---

## 13. Resource Unit Standardization

### Decision: **Millicores for CPU · Mebibytes for Memory — stored as integers**

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

The customer always sees human-readable values. Internally everything is a clean integer.

This is clearly labeled as an estimate, not a real recommendation.
