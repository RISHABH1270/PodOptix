# PodOptix — Best Practices

Standards and conventions every contributor must follow.

---

## 1. Code Structure

Follow the [Standard Go Project Layout](https://github.com/golang-standards/project-layout).

```
podoptix/
├── cmd/
│   └── hub/
│       └── main.go          # entrypoint only — no business logic
├── internal/
│   ├── api/                 # REST API handlers
│   ├── auth/                # JWT + token handling
│   ├── cache/               # Redis client
│   ├── collector/           # PromQL query engine
│   ├── compute/             # p99 computation engine
│   ├── recommender/         # recommendation engine
│   ├── registry/            # cluster registry
│   ├── scheduler/           # cron job runner
│   └── store/               # database layer (PostgreSQL)
├── pkg/
│   └── models/              # shared data models
├── api/
│   └── v1/                  # OpenAPI / Swagger spec
├── deploy/
│   └── helm/                # Helm chart
├── docs/                    # architecture, decisions, best practices
└── Makefile
```

**Rules:**
- `main.go` wires dependencies and starts the server — nothing else
- Business logic lives in `internal/` — not importable by external packages by design
- Shared types and utilities go in `pkg/`

---

## 2. API Design

- Version all endpoints from day one — `/api/v1/...`
- Use consistent HTTP methods: `GET` read · `POST` create · `PUT` update · `DELETE` remove
- Always return a consistent error shape:

```json
{
  "error": "cluster not found",
  "code": 404
}
```

- Never expose internal errors, stack traces, or raw DB errors to API consumers
- Paginate list endpoints — never return unbounded arrays

**Endpoint naming convention:**
```
GET    /api/v1/clusters                  list all clusters
POST   /api/v1/clusters                  register a cluster
GET    /api/v1/clusters/:id              get a cluster
DELETE /api/v1/clusters/:id              remove a cluster
GET    /api/v1/clusters/:id/recommendations   get recommendations for cluster
```

---

## 3. Security

| Rule | Detail |
|------|--------|
| **Never log secrets** | No auth tokens, Prometheus URLs with credentials, or JWT secrets in logs |
| **Encrypt at rest** | Cluster tokens encrypted before storing in PostgreSQL |
| **TLS always** | All outbound Prometheus connections must verify TLS — never `InsecureSkipVerify: true` |
| **Validate all input** | Validate cluster names, namespace filters, URLs before use |
| **No hardcoded secrets** | All secrets via environment variables or Kubernetes Secrets |
| **Least privilege** | Hub's K8s ServiceAccount needs no cluster-wide permissions |

---

## 4. Logging

Use structured JSON logging — **never** `fmt.Println` in production code.

**Library:** `zerolog` or `zap`

```go
// correct
log.Info().
    Str("cluster_id", clusterID).
    Str("namespace", namespace).
    Msg("collection job started")

// wrong
fmt.Println("collection job started for " + clusterID)
```

**Log levels:**
- `DEBUG` — detailed flow, PromQL queries, cache hits/misses
- `INFO` — job started/completed, cluster registered, recommendations stored
- `WARN` — retries, slow queries, degraded state
- `ERROR` — failed collection, DB errors, auth failures

**Never log:**
- Auth tokens
- Prometheus credentials
- Full HTTP request bodies

---

## 5. Error Handling

Go errors must be handled explicitly — never silently ignored.

```go
// correct — wrap with context
result, err := collector.Query(ctx, clusterID, metric)
if err != nil {
    return fmt.Errorf("query cluster %s: %w", clusterID, err)
}

// wrong — swallowing the error
result, _ := collector.Query(ctx, clusterID, metric)
```

- Always wrap errors with context using `fmt.Errorf("context: %w", err)`
- Return errors up the call stack — don't log AND return
- Log errors only at the top-level handler

---

## 6. Observability

The Hub must be observable itself.

| Endpoint | Purpose |
|----------|---------|
| `GET /healthz` | Liveness probe — is the process alive? |
| `GET /readyz` | Readiness probe — is DB + Redis connected? |
| `GET /metrics` | Prometheus metrics — scrape PodOptix itself |

**Metrics to expose:**
- `podoptix_collection_jobs_total` — counter of collection jobs run
- `podoptix_collection_duration_seconds` — histogram of job duration
- `podoptix_recommendations_total` — total recommendations generated
- `podoptix_prometheus_query_errors_total` — failed PromQL queries per cluster

---

## 7. Testing

| Type | What to test | Tool |
|------|-------------|------|
| Unit | p99 computation engine · recommendation multiplier logic | Go `testing` |
| Integration | PromQL queries against real Prometheus | `testcontainers-go` |
| API | REST endpoint contracts | `httptest` |

**Rules:**
- p99 computation engine must have 100% unit test coverage — it is the core value of the product
- Never mock Prometheus in integration tests — use a real instance via `testcontainers-go`
- Table-driven tests for computation logic

```go
func TestP99Computation(t *testing.T) {
    tests := []struct {
        name     string
        series   []float64
        expected float64
    }{
        {"basic", []float64{100, 200, 300}, 297},
        {"single value", []float64{150}, 150},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := ComputeP99(tt.series)
            assert.Equal(t, tt.expected, got)
        })
    }
}
```

---

## 8. Git Workflow

```
main          ← production-ready only. No direct commits.
  └── development  ← all work merges here first
        └── feature/xxx  ← individual features (optional)
```

**Commit message format:**
```
feat: add PromQL collection engine
fix: handle empty time series in p99 computation
docs: update architecture diagram
refactor: extract recommendation logic into separate package
test: add unit tests for p99 engine
chore: update Go dependencies
```

- Every PR goes `development → main` via pull request
- No force pushes to `main`
- PRs must include a description of what changed and why

---

## 9. Docker

Use multi-stage builds — keep the final image minimal.

```dockerfile
# Stage 1 — build
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o hub ./cmd/hub

# Stage 2 — run
FROM alpine:3.20
COPY --from=builder /app/hub /hub
ENTRYPOINT ["/hub"]
```

**Rules:**
- Never run the container as root — use a non-root user
- Set resource limits in the Helm chart
- Use `distroless` or `alpine` base — not `ubuntu` or `debian`

---

## 10. Kubernetes Deployment Standards

The Hub's own Kubernetes deployment must follow:

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

- Hub's `ServiceAccount` needs **no** cluster-wide RBAC — it queries Prometheus over HTTP, not the K8s API
- Store DB password and Redis password as Kubernetes `Secret`, not `ConfigMap`
