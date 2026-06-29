# PodOptix — Best Practices

Standards and conventions every contributor must follow.

---

## 1. Code Structure

Follow the [Standard Go Project Layout](https://github.com/golang-standards/project-layout).

```
PodOptix/
│
├── cmd/                    ← entry point — turns the app on
│   └── hub/
│       └── main.go         ← wires everything together, nothing else
│
├── internal/               ← all actual logic
│   ├── api/                ← HTTP server, routes, handlers, middleware
│   ├── collector/          ← fetches data from Prometheus via PromQL
│   ├── compute/            ← p99 computation engine
│   ├── recommender/        ← calculates p99 × 2 = new limit
│   ├── scheduler/          ← runs collection jobs once per day
│   ├── registry/           ← address book of all registered clusters
│   ├── auth/               ← JWT auth + token encryption
│   ├── cache/              ← Redis client
│   ├── config/             ← reads environment variables
│   └── store/              ← PostgreSQL layer (CRUD + migrations)
│
├── pkg/
│   └── models/             ← shared data models (Cluster, Recommendation)
│
├── api/
│   └── v1/                 ← OpenAPI spec (coming soon)
│
├── migrations/             ← SQL schema files (*.up.sql)
├── deploy/helm/            ← Helm chart (coming soon)
└── docs/                   ← architecture, decisions, best practices
```

**Rules:**
- `main.go` only wires dependencies and starts the server — no business logic
- Business logic lives in `internal/` — not importable by external packages by design
- Shared data models go in `pkg/models/`
- All secrets via environment variables — never hardcoded

---

## 2. API Design

- Version all endpoints from day one — `/api/v1/...`
- Use consistent HTTP methods: `GET` read · `POST` create · `DELETE` remove
- Always return a consistent error shape with `request_id` for tracing:

```json
{
  "error": "Cluster not found",
  "request_id": "a3f8c2d1-9b4e-4f1a-8c3d-2e5f7a9b1c4d"
}
```

- Never expose internal errors, stack traces, or raw DB errors to API consumers
- Use Input DTOs — separate request structs from database models (prevents customers from setting server-generated fields like `cluster_id`, `created_at`)

**Current endpoints:**
```
GET    /healthz                              Kubernetes liveness probe
GET    /api/v1/clusters                      list all clusters
POST   /api/v1/clusters                      register a cluster
GET    /api/v1/clusters/:id                  get a cluster by ID
DELETE /api/v1/clusters/:id                  remove a cluster
GET    /api/v1/clusters/:id/recommendations  get recommendations for a cluster
```

---

## 3. Security

| Rule | Detail |
|------|--------|
| **Never log secrets** | No auth tokens, Prometheus URLs with credentials, or JWT secrets in logs |
| **Encrypt at rest** | Cluster tokens encrypted before storing in PostgreSQL |
| **TLS always** | All outbound Prometheus connections must verify TLS — never `InsecureSkipVerify: true` |
| **Validate all input** | Use `binding:"required"` on request structs — Gin rejects missing fields automatically |
| **No hardcoded secrets** | All secrets via environment variables or Kubernetes Secrets |
| **Least privilege** | Hub's K8s ServiceAccount needs no cluster-wide RBAC — queries Prometheus over HTTP only |
| **Request tracing** | Every request gets a unique `X-Request-ID` header — traceable end to end |
| **Password hashing** | bcrypt only — never store plain text passwords. Cannot be reversed even if DB is stolen |
| **JWT expiry** | Tokens expire in 24 hours — limits blast radius if a token is stolen |

---

## 4. Logging

Current: Go standard `log` package (temporary). Planned: `zerolog` for structured JSON logs.

```go
// current — standard log with request ID for tracing
log.Printf("ERROR [%s] listClusters: %v", requestID, err)

// planned — structured JSON (zerolog)
log.Error().Str("request_id", requestID).Err(err).Msg("listClusters failed")
```

**Two separate outputs — always:**
- `log.Printf` → stderr → real error for developer
- `c.JSON(500, gin.H{"error": "friendly message"})` → HTTP response → customer sees this

**Never log:**
- Auth tokens or Prometheus credentials
- Full HTTP request bodies
- Stack traces in API responses

---

## 5. Error Handling

Go errors must be handled explicitly — never silently ignored.

```go
// correct — wrap with context
result, err := s.store.GetCluster(ctx, id)
if err != nil {
    return fmt.Errorf("get cluster %s: %w", id, err)
}

// wrong — swallowing the error
result, _ := s.store.GetCluster(ctx, id)
```

- Always wrap errors with `fmt.Errorf("context: %w", err)` — preserves full error chain
- Return errors up — don't log AND return (log only at top-level handler)
- `%w` wraps the original error so callers can inspect the full chain

---

## 6. Observability

The Hub must be observable itself.

| Endpoint | Purpose |
|----------|---------|
| `GET /healthz` | Liveness probe — is the process alive? |
| `GET /readyz` | Readiness probe — is DB + Redis connected? (coming soon) |
| `GET /metrics` | Prometheus metrics — scrape PodOptix itself (coming soon) |

**Metrics to expose (planned):**
- `podoptix_collection_jobs_total` — counter of daily collection jobs
- `podoptix_collection_duration_seconds` — histogram of job duration
- `podoptix_recommendations_total` — total recommendations generated
- `podoptix_prometheus_query_errors_total` — failed PromQL queries per cluster

---

## 7. Testing

**Current approach — integration tests against real PostgreSQL:**

```go
// TestMain auto-creates and drops the test database
func TestMain(m *testing.M) {
    // CREATE DATABASE podoptix_test
    // run migrations
    // run tests
    // DROP DATABASE podoptix_test
}
```

| Type | Tool | Status |
|------|------|--------|
| API integration | `httptest` + real PostgreSQL | Done |
| p99 computation unit tests | Go `testing` + `testify` | Planned |
| PromQL integration | `testcontainers-go` | Planned |

**Rules:**
- Test database is created and destroyed automatically — no manual setup
- Never mock the database — test against real PostgreSQL
- `trackTest(t)` in every test — enables pass/fail count at end
- p99 computation engine must have 100% unit test coverage

---

## 8. Git Workflow

```
main          ← production-ready only. No direct commits.
  └── development  ← all work merges here first
```

**Commit message format:**
```
feat: add PromQL collection engine
fix: handle empty time series in p99 computation
docs: update architecture diagram
refactor: rename window to lookback_window
test: add unit tests for p99 engine
chore: update Go dependencies
```

- Every PR goes `development → main` via pull request
- No force pushes to `main`

---

## 9. Docker

Use multi-stage builds — keep the final image minimal.

```dockerfile
# Stage 1 — build
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o hub ./cmd/hub

# Stage 2 — run
FROM alpine:3.20
COPY --from=builder /app/hub /hub
ENTRYPOINT ["/hub"]
```

**Rules:**
- Never run as root — use a non-root user
- Use `alpine` or `distroless` base — not `ubuntu` or `debian`
- Set resource limits in the Helm chart

---

## 10. Kubernetes Deployment Standards

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

- Hub's `ServiceAccount` needs **no** cluster-wide RBAC
- Store DB and Redis passwords as Kubernetes `Secret`, not `ConfigMap`
- `DATABASE_URL`, `REDIS_URL`, `JWT_SECRET` injected as environment variables from Secret
