# PodOptix — API Testing Guide

Everything you need to understand, run, and extend the PodOptix backend test suite.

---

## Tooling

| Piece | What we use | Why |
|-------|-------------|-----|
| Test runner       | Go's built-in `testing` package                              | Ships with Go, zero extra deps |
| Assertions        | [`stretchr/testify/assert`](https://github.com/stretchr/testify) | Readable one-line assertions vs verbose `if` blocks |
| HTTP server       | `httptest.NewServer` (Go stdlib)                             | Spins up a real TCP listener on a random port — no mocks |
| DB driver         | `pgx/v5`                                                     | Same driver as production — tests exercise the real code path |
| Migrations        | `golang-migrate/migrate/v4`                                  | Same tool as production |
| Isolated DB       | Separate `podoptix_test` database in the same PostgreSQL container | Full isolation, zero infra cost |
| Isolated Redis    | Redis logical index `1` (production uses `0`)                | Same container, no key collisions |

**No mocking.** Every test hits real PostgreSQL, real Redis, real Gin router, real middleware. If a test passes here, it works in production.

---

## 1. Two types of tests we run

**A. Integration tests** — hit real DB + Redis + real HTTP server
- Tests full request flow: HTTP → Gin middleware → handler → PostgreSQL → response
- Files: `health_test.go`, `auth_test.go`, `clusters_test.go`, `recommendations_test.go`

**B. Unit tests** — pure Go functions, no I/O
- Tests business logic in isolation
- Files: `compute_p99_test.go`, `recommender_test.go`, `collector_test.go`, `auth_encrypt_test.go`

Both types live in the same `tests/` package and share the same setup.

---

## 2. What `TestMain` does (runs ONCE before any test)

`TestMain` is a special Go function — it runs before Go executes any `TestXxx` function. Our version does:

```
1. Connect to postgres (default admin DB)
   ↓
2. DROP DATABASE podoptix_test    ← wipe any leftover data from previous run
   ↓
3. CREATE DATABASE podoptix_test  ← clean slate
   ↓
4. store.SyncSchema()             ← run migrations (creates 3 tables)
   ↓
5. store.New()                    ← open connection pool to podoptix_test
   ↓
6. cache.New(redis://localhost:6379/1)  ← Redis index 1 (production uses 0)
   ↓
7. redisCache.FlushDB()           ← wipe Redis index 1
   ↓
8. api.NewServer(db, cache, nil, jwtSecret, encKey)  ← nil scheduler (not needed in tests)
   ↓
9. srv.Listen(:9090) → go srv.Serve()  ← real TCP server on port 9090
   ↓
10. m.Run()                       ← NOW Go runs all TestXxx functions
    ↓
11. TEARDOWN:
    - close DB pool, close Redis
    - DROP DATABASE podoptix_test  ← clean up
    - print summary line
```

### Isolation — production and tests never collide

| | Production | Tests |
|--|-----------|-------|
| DB | `podoptix` | `podoptix_test` (dropped after run) |
| Redis | index `0` | index `1` |
| Server | port `8080` | port `9090` |

You can run the app AND tests simultaneously — they never touch each other's data.

---

## 3. The helpers in `setup_test.go`

**`do(t, method, path, body, auth)`** — the workhorse. Fires a real HTTP request against `http://localhost:9090`:

```go
resp := do(t, "POST", "/api/v1/clusters", `{"cluster_name":"prod"}`, bearer(testToken()))
```

This is EXACTLY what `curl` does — full HTTP round trip through the real Gin stack.

**`testToken()`** — generates a valid JWT signed with the test secret. Every test that hits a protected route uses this.

**`bearer(token)`** — wraps `"Bearer " + token` for the Authorization header.

**`readBody(t, resp)`** — reads response body into a string for assertions.

**`track(t)`** — increments a counter, prints `[ 12 ] ✓ test_name` when the test finishes. Used for the pretty output you see.

**`log(...)`** — writes to `/dev/tty` directly, bypassing Go's stdout capture (that's why output shows even without `-v` flag).

---

## 4. What each test file verifies

### `health_test.go` (1 test)
- `GET /healthz` returns 200 + `{"status":"ok"}`

### `metrics_test.go` (~3 tests)
- `GET /metrics` returns 200 with Prometheus text exposition format (no auth required)
- Response body contains the `podoptix_*` metric families (HTTP, scheduler, cache)
- HTTP counter increments after a real request through the router

### `auth_test.go` (~10 tests)
- Register creates user, returns JWT
- Register with duplicate email → 409
- Register without password → 400
- Login with correct password → 200 + JWT
- Login with wrong password → 401
- Login with unknown email → 401 (same message — anti-enumeration)
- Protected route without token → 401
- Protected route with wrong-format header → 401
- Protected route with invalid token → 401
- Protected route with valid token → 200

### `clusters_test.go` (~13 tests)
- POST creates cluster, returns 201 with `last_synced_at: "not yet synced"`
- POST without required fields → 400
- POST with invalid lookback_window (e.g. `99d`) → 400
- POST without auth → 401
- GET list returns array (empty if none)
- GET by ID returns cluster
- GET unknown ID → 404
- PUT updates name
- PUT with invalid lookback → 400
- PUT unknown ID → 404
- DELETE returns 204
- DELETE then GET → 404
- DELETE unknown ID → 404

### `recommendations_test.go` (~7 tests)
- GET returns empty array for new cluster
- GET unknown cluster → empty array
- GET without auth → 401
- POST recalculate → 202 accepted
- POST recalculate twice → 429 (distributed lock)
- POST recalculate unknown cluster → 404
- POST without auth → 401

### `auth_encrypt_test.go` (5 tests — unit)
- AES round-trip works
- Same input → different ciphertext (random nonce)
- Wrong key length → error
- Wrong decryption key → error
- Tampered ciphertext → error (GCM auth tag)

### `compute_p99_test.go` (7 tests — unit)
- Empty dataset → error
- Single value → returns that value
- Two values → returns highest
- Correct index position for larger sets
- Top 1% spike is IGNORED (that's the whole point of p99)
- Original slice is NOT modified
- Typical 7d workload — 168 values + 1 spike → spike ignored

### `recommender_test.go` (7 tests — unit)
- p99 × 2 = recommended limit (main formula)
- Nil metrics → error
- Empty CPU values → error
- Empty memory values → error
- Single value → recommended is double
- GenerateAll with mixed containers → some ready, some new_service
- GenerateAll with no containers → empty slice

### `collector_test.go` (13 tests — unit)
- ParseDuration handles days/hours/minutes
- ParseDuration with invalid input → error
- ExtractValues parses Prometheus JSON format
- ExtractValues skips invalid values
- Collect (with mock Prometheus via `httptest.NewServer`) — HTTP calls, merging, auth header attachment

---

## 5. Running them

```bash
docker compose up -d              # ensure DB + Redis running
go test ./tests/... -count=1 -p 1
```

- `-count=1` disables Go's test cache — every run is fresh
- `-p 1` runs packages sequentially — output isn't interleaved

### Run a specific test group

```bash
go test ./tests/... -run TestClusters -count=1 -p 1
go test ./tests/... -run TestRecommendations -count=1 -p 1
go test ./tests/... -run TestAuth -count=1 -p 1
```

### Run a specific subtest

```bash
go test ./tests/... -run TestClusters/POST -count=1 -p 1
```

### Expected output

```
Running PodOptix API Tests...
Server: http://localhost:9090
──────────────────────────────────────
[ 1]  ✓  success_returns_201_with_cluster_id_and_not_yet_synced
[ 2]  ✓  missing_required_fields_returns_400
...
Total: 63  |  Passed: 63  |  Failed: 0
✓ All tests passed
──────────────────────────────────────
```

---

## 6. Writing a new test

Every leaf subtest should call `track(t)` for the counter, and use `do()` for HTTP:

```go
func TestSomething(t *testing.T) {
    t.Run("does the thing", func(t *testing.T) {
        track(t)
        resp := do(t, "POST", "/api/v1/clusters", `{"cluster_name":"x"}`, bearer(testToken()))
        assert.Equal(t, 201, resp.StatusCode)
    })
}
```

---

## 7. Key principle

Every integration test goes through the **real** Gin router, **real** middleware, **real** PostgreSQL, **real** Redis. If it passes here, it works in production.

No mocks except for the Prometheus fake server (`httptest.NewServer` in `collector_test.go`) — we can't run 40 real Prometheuses in CI.
