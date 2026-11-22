# PodOptix — REST API Reference

Every HTTP endpoint PodOptix exposes, organised by concern. This page is the one-screen lookup table; for full JSON request/response shapes, auth header details, and error codes see [LLD](lld.md#rest-api-contract).

## Base URL

Local dev (Option A — Vite proxy): `http://localhost:5173` → proxied to `:8080`
Local dev (Option B — single binary): `http://localhost:8080`
Production (Helm): whatever `Service` / `Ingress` fronts the Hub deployment

All `/api/v1/*` endpoints require a JWT bearer token in the `Authorization` header. Obtain one via `POST /auth/login`.

---

## Endpoints

### Auth · unauthenticated

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/auth/register` | Create a user account. Body: `{"email", "password"}`. Password must be 8–128 chars; email validated for shape. |
| `POST` | `/auth/login` | Exchange email + password for a JWT token (24h expiry, HS256). |

### Platform health · unauthenticated

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/healthz` | Liveness probe — always 200 if the process is up. |
| `GET` | `/readyz` | Readiness probe — 200 only if Postgres AND Redis are reachable. Returns 503 with per-dep breakdown otherwise. |
| `GET` | `/metrics` | Prometheus scrape target. Exposes `podoptix_*` metrics (HTTP, scheduler, cache). |

### Clusters · JWT required

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/clusters` | List every registered cluster. |
| `POST` | `/api/v1/clusters` | Register a cluster. Body: `{"cluster_name", "prometheus_url", "prometheus_token", "lookback_window"?}`. Prometheus token is AES-256-GCM encrypted before storage. Returns 409 if `cluster_name` is taken. |
| `GET` | `/api/v1/clusters/:id` | Get one cluster by ID. |
| `PUT` | `/api/v1/clusters/:id` | Partial update — only provided fields are changed. Empty `prometheus_token` field is a no-op (existing token preserved). |
| `DELETE` | `/api/v1/clusters/:id` | Remove the cluster. **Cascades**: all recommendations for the cluster are deleted atomically via the FK `ON DELETE CASCADE`. |

### Recommendations · JWT required

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/clusters/:id/recommendations` | Recommendations for one cluster, cache-aside served (3h TTL). |
| `PATCH` | `/api/v1/clusters/:id/recommendations/:recId` | Toggle the `applied` flag. Body: `{"applied": true \| false}`. This is the **ONLY** write path for the flag — the scheduler never touches it. Drives the Savings page's realized-vs-pending math. |
| `DELETE` | `/api/v1/clusters/:id/recommendations/:recId` | Delete one recommendation row (operator cleanup — typically used on orphaned workloads). |
| `DELETE` | `/api/v1/clusters/:id/recommendations?orphaned=true` | Bulk-delete every orphaned row for the cluster. The `?orphaned=true` query parameter is **REQUIRED** (footgun guard — without it the request 400s). |
| `POST` | `/api/v1/clusters/:id/recalculate` | Trigger a manual pipeline run. Returns 202 immediately; actual work happens in a background goroutine. Returns 429 if a scheduler tick or another recalculate is already running (shared fencing-token Redis lock). |
| `GET` | `/api/v1/recommendations` | Cross-cluster view — every recommendation joined with its `cluster_name`, sorted by biggest `current_cpu_request - recommended_cpu_request` delta (biggest waste first). |

---

## Response envelope on errors

Every non-2xx response returns:

```json
{
  "error":      "A human-readable message",
  "request_id": "f9527dc3-d8eb-4309-8ca5-393531c6c788"
}
```

The `request_id` matches the `X-Request-ID` response header and the `req=` field in server logs — quote it when filing a bug or asking for support.

---

## Status-code quick reference

| Code | When |
|---|---|
| **200** | Success, with body |
| **201** | Resource created (register user, create cluster) |
| **202** | Accepted — work is running asynchronously (recalculate) |
| **204** | Success, no body (delete) |
| **400** | Malformed body / missing required field / invalid query param |
| **401** | Missing / invalid / expired JWT |
| **404** | Unknown ID (cluster / recommendation / user) |
| **409** | Duplicate (cluster name already taken, email already registered) |
| **429** | Rate-limited (recalculate lock held by another run) |
| **500** | Internal error — check logs with the `request_id` |
| **503** | `/readyz` only — a dependency (Postgres or Redis) is down |

---

## See also

- **[LLD](lld.md#rest-api-contract)** — full JSON request/response examples for every endpoint, header details, auth flow
- **[Engineering Trade-offs](engineering-trade-offs.md)** — why we picked JWT over sessions, why the fencing-token lock, why `?orphaned=true` is required for bulk delete
- **[USER_MANUAL](../USER_MANUAL.md)** — operator-facing workflows (register cluster, apply recommendation, review orphans)
