# PodOptix — Codebase Reference

## Table of Contents

1. [go.mod](#1-gomod)
2. [pkg/models/](#2-pkgmodels)
3. [internal/config/](#3-internalconfig)
4. [migrations/](#4-migrations)
5. [internal/store/](#5-internalstore)
6. [internal/auth/](#6-internalauth)
7. [internal/api/](#7-internalapi)
8. [internal/collector/](#8-internalcollector)
9. [internal/compute/](#9-internalcompute)
10. [internal/recommender/](#10-internalrecommender)
11. [internal/scheduler/](#11-internalscheduler)
12. [internal/cache/](#12-internalcache)
13. [cmd/hub/main.go](#13-cmdhubmaingo)
14. [frontend/](#14-frontend)
15. [Dockerfile](#15-dockerfile)
16. [deploy/helm/](#16-deployhelm)

---

## 1. `go.mod`

Project manifest — equivalent to `package.json` (Node), `requirements.txt` (Python), `build.gradle` (Java).

- `module github.com/RISHABH1270/PodOptix` — unique module name, matches GitHub URL exactly
- `go 1.26.4` — minimum Go version (lower bound, not exact — 1.27+ also works)
- Direct dependencies we explicitly chose:

| Library | Purpose |
|---------|---------|
| `gin` | HTTP web framework |
| `golang-migrate` | Auto-runs SQL migration files |
| `pgx/v5` | PostgreSQL driver + connection pool |
| `golang-jwt/jwt/v5` | JWT token generation and verification |
| `google/uuid` | UUID generation for IDs |
| `redis/go-redis/v9` | Redis client |
| `golang.org/x/crypto` | bcrypt password hashing |

`// indirect` = dependencies of our dependencies. Never touched manually. `go mod tidy` manages them.
`go.sum` = cryptographic fingerprints of every library. Prevents supply chain attacks.

---

## 2. `pkg/models/`

Data blueprints. Define what each entity looks like in memory, API responses, and the database. No business logic here — just shape.

### `cluster.go`

```
ClusterID      UUID string  ← primary key, unguessable, never sequential
Name           string       ← human-readable, UNIQUE in DB
PrometheusURL  string       ← HTTP endpoint queried by collector
Token          string       ← json:"-" never in API response · encrypted at rest in DB
LookbackWindow string       ← "7d" not int — preserves unit for PromQL queries
CreatedAt      time.Time
UpdatedAt      time.Time
```

### `recommendation.go`

```
RecommendationID  UUID    ← own primary key
ClusterID         UUID    ← foreign key → clusters (one cluster → many recommendations)
Namespace         string  ← "payments"
PodName           string  ← "payment-api-7d9f"
ContainerName     string  ← "api" ← per container, not per pod
Status            string  ← "new_service" | "ready"
CurrentCPULimit   int     ← millicores — normalized from any K8s format (1000m, 1, 0.5)
CurrentMemLimit   int     ← MiB
P99CPU            float64 ← raw p99 needs decimal precision (120.5m)
P99Mem            float64 ← raw p99
RecommendedCPULimit int   ← ceil(p99 × 2) — final integer for K8s manifests
RecommendedMemLimit int   ← ceil(p99 × 2)
LookbackWindow    string  ← "7d"
CreatedAt         time.Time
UpdatedAt         time.Time ← when last recalculated by scheduler
```

One row per container. UPSERTED daily — not a new row each run.

### `user.go`

```
UserID        UUID    ← primary key. Email can change, UUID never does
Email         string  ← UNIQUE, login identifier
PasswordHash  string  ← json:"-" never in API response · bcrypt hash, not plain text
CreatedAt     time.Time
UpdatedAt     time.Time
```

**Struct tags pattern:**
- `json:"x"` → key used in API JSON response
- `db:"x"` → PostgreSQL column name
- `json:"-"` → field completely excluded from all JSON output

---
