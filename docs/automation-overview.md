# PodOptix — Automation & Testing Overview

---

## Run Tests

```bash
go test ./internal/api/ -v
```

---

## Test Types

| Type | What it tests | Database |
|------|-------------|----------|
| Integration | Full stack — handler → store → PostgreSQL | Real PostgreSQL (`podoptix_test`) |
| E2E (planned) | Real HTTP calls against a running server | Staging environment |

---

## Test Lifecycle — `setup_test.go`

Every test run goes through this sequence automatically:

```
go test ./internal/api/ -v
        ↓
init()         → os.Chdir("../..") — move to project root so migrations/ is found
        ↓
TestMain(m)    → SETUP:
  1. adminURL  → connect to default "postgres" database
  2.            → DROP DATABASE podoptix_test WITH (FORCE)
  3.            → CREATE DATABASE podoptix_test  (clean slate every run)
  4. testDBURL → run SyncSchema (create tables via migrations)
  5.            → store.New() — open connection pool
  6.            → NewServer(db) — create test server
  7.            → print "Running PodOptix API Tests..."
        ↓
m.Run()        → Go scans all _test.go files, runs every func TestXxx(t *testing.T)
        ↓
TestMain(m)    → TEARDOWN:
  8.            → db.Close()
  9. adminURL  → DROP DATABASE podoptix_test WITH (FORCE)
  10.           → print Total / Passed / Failed
        ↓
os.Exit(code)  → 0 = all passed, 1 = some failed
```

---

## Why Two Database URLs?

```go
adminURL  = "postgres://...@localhost:5432/postgres"       // default system DB
testDBURL = "postgres://...@localhost:5432/podoptix_test"  // our test DB
```

PostgreSQL cannot drop a database while connected to it. `adminURL` connects to the neutral `postgres` database to CREATE and DROP `podoptix_test`. `testDBURL` connects to `podoptix_test` to run migrations and tests.

---

## How Go Finds Tests

`m.Run()` automatically finds every function in `_test.go` files that matches `func TestXxx(t *testing.T)`. No registration needed — naming convention is enough.

```
TestHealthz             ← found automatically
TestCreateCluster       ← found automatically
TestListClusters        ← found automatically
TestGetCluster          ← found automatically
TestGetCluster_NotFound ← found automatically
TestDeleteCluster       ← found automatically
```

---

## Test Counter — `trackTest(t *testing.T)`

Called at the start of every test. Tracks total, passed, failed using `sync.Mutex` (prevents race conditions when tests run in parallel).

```go
func TestCreateCluster(t *testing.T) {
    trackTest(t)   // register this test for counting
    ...
}
```

`t.Cleanup()` registers a function that runs after the test finishes — at that point `t.Failed()` tells us the result.

`sync.Mutex` = a lock. Only one test can update the counter at a time — prevents two tests from corrupting the count simultaneously.

---

## `httptest` — How Requests Are Simulated

No real TCP server. Requests are simulated in memory:

```go
req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
w   := httptest.NewRecorder()
testServer.router.ServeHTTP(w, req)

// check results
w.Code          // HTTP status code
w.Body.String() // response body
```

- `httptest.NewRequest` — creates a fake HTTP request
- `httptest.NewRecorder` — captures the response (like a fake `http.ResponseWriter`)
- `ServeHTTP` — passes the request through the full Gin pipeline (middleware → handler)

The full stack runs: RequestIDMiddleware → handler → store → real PostgreSQL.

---

## `assert` — Test Assertions

From `github.com/stretchr/testify/assert`. Cleaner than raw Go comparisons:

```go
assert.Equal(t, http.StatusOK, w.Code)           // must be 200
assert.Contains(t, w.Body.String(), "test-cluster") // body must contain this
assert.Equal(t, http.StatusNotFound, w.Code)     // must be 404
```

If assertion fails → test fails with a clear message showing expected vs actual.

---

## Current Test Coverage

| Test | What it verifies |
|------|----------------|
| `TestHealthz` | `/healthz` returns 200 and `{"status":"ok"}` |
| `TestCreateCluster` | POST creates cluster, returns 201 with cluster data |
| `TestCreateCluster_MissingFields` | POST without required fields returns 400 |
| `TestListClusters` | GET returns 200 |
| `TestGetCluster` | GET by ID returns 200 with correct cluster |
| `TestGetCluster_NotFound` | GET with fake ID returns 404 with "Cluster not found" |
| `TestDeleteCluster` | DELETE removes cluster, subsequent GET returns 404 |

---

## `health_test.go` — Walkthrough

Simplest test — one request, two assertions.

```
req  = fake GET /healthz (no body)
w    = response recorder (captures status + body)
ServeHTTP → runs full Gin pipeline: middleware → handleHealthz → w

assert status = 200
assert body contains "status":"ok"
```

---

## `clusters_test.go` — Walkthrough

Every test follows the **AAA pattern** — Arrange → Act → Assert.

**`TestCreateCluster`**
```
Arrange → build JSON body with name, prometheus_url, token
Act     → POST /api/v1/clusters with Content-Type: application/json
Assert  → status 201, body contains cluster name and URL
```

**`TestCreateCluster_MissingFields`**
```
Arrange → JSON with only name (missing prometheus_url and token)
Act     → POST /api/v1/clusters
Assert  → status 400 (binding:"required" validation caught it)
```

**`TestGetCluster`** — two-step test
```
Step 1 → CREATE a cluster, extract the real UUID from response
         json.Unmarshal parses JSON → created["id"].(string) reads id field
         t.Fatalf stops test immediately if create failed

Step 2 → GET /api/v1/clusters/{real-id}
         Assert status 200, body contains cluster name
```

**`TestGetCluster_NotFound`**
```
Act    → GET /api/v1/clusters/non-existent-id
Assert → status 404, body contains "Cluster not found"
```

**`TestDeleteCluster`** — three-step test
```
Step 1 → CREATE a cluster, extract real UUID
Step 2 → DELETE /api/v1/clusters/{id}
         Assert status 204 (no content — success, nothing to return)
Step 3 → GET /api/v1/clusters/{id}
         Assert status 404 — confirms data is actually gone from DB
         (204 only means the query ran, GET after confirms it really deleted)
```

**Key patterns:**
- `bytes.NewBufferString(body)` — converts string to readable stream for request body
- `req.Header.Set("Content-Type", "application/json")` — tells Gin to parse body as JSON
- `json.Unmarshal(w.Body.Bytes(), &map)` — parses JSON response into Go map
- `.(string)` — type assertion: reads map value as string
- `t.Fatalf` — fails test immediately, stops execution (unlike `assert` which continues)

---

## `01_auth_test.go` — Walkthrough

Auth tests run after health, before clusters. Order matters — register must work before login can be tested.

```
TestRegister               → POST /auth/register valid → 201 + token in response
TestRegister_DuplicateEmail → register twice same email → 409 "already exists"
TestRegister_MissingFields  → register without password → 400
TestLogin                  → register then login → 200 + token
TestLogin_WrongPassword    → login with wrong password → 401 "Invalid email or password"
TestLogin_UnknownEmail     → login unknown email → 401 same message (prevents user enumeration)
TestProtectedRoute_NoToken      → GET /api/v1/clusters no header → 401
TestProtectedRoute_WrongFormat  → Authorization: wrongformat → 401
TestProtectedRoute_InvalidToken → Authorization: Bearer fakejwt → 401
TestProtectedRoute_WithToken    → register → extract token → use on protected route → 200
```

**Key patterns:**
- Same error for wrong email AND wrong password — `"Invalid email or password"`. Prevents attacker from knowing if email exists
- `TestProtectedRoute_WithToken` is end-to-end: register → parse token from JSON → attach to request → verify 200

---

## `02_clusters_test.go` — What changed after auth

All cluster tests now call `getTestToken()` and attach the token to every request:

```go
token := getTestToken()
req.Header.Set("Authorization", "Bearer "+token)
```

**`getTestToken()` in `setup_test.go`:**
```go
func getTestToken() string {
    token, _ := auth.GenerateToken("test-user-id", "testauth@podoptix.io", "test-jwt-secret-key-for-testing")
    return token
}
```
Generates JWT directly — no HTTP call, no database. Avoids duplicate email issue (if it registered via HTTP, second call → 409 → no token → all cluster tests fail).

---

## Test File Order

```
00_health_test.go   → TestHealthz
01_auth_test.go     → TestRegister, TestLogin, TestProtectedRoute_*
02_clusters_test.go → TestCreateCluster, TestListClusters, TestGetCluster, TestDeleteCluster
```

Files prefixed with numbers ensure alphabetical = execution order.

---

## `internal/collector/01_prometheus_test.go` — Walkthrough

Tests for the Prometheus collector. No real Prometheus needed — uses `httptest.NewServer` (a fake HTTP server) to simulate Prometheus responses.

**What IS tested with real logic (no mock):**
```
TestParseDuration_Days/Hours/Minutes  → real parseDuration function
TestParseDuration_Invalid/UnknownUnit → error handling in parseDuration
TestExtractValues_Valid               → parses [[ts, "0.120"]] → [0.120]
TestExtractValues_Empty               → returns empty slice
TestExtractValues_InvalidValue        → skips bad values, keeps valid ones
```

**What is tested with fake Prometheus server:**
```
TestCollect_Success       → fake server returns valid CPU + memory data
                            verifies ContainerMetrics fields, values, count

TestCollect_PrometheusError → fake server returns 500
                              verifies error contains "prometheus returned status 500"

TestCollect_EmptyResponse → fake server returns success but no containers
                            verifies empty slice returned, no error

TestCollect_WithToken     → fake server captures the Authorization header
                            verifies "Bearer my-secret-token" was sent

TestCollect_InvalidDuration → no server needed
                              verifies "7w" returns parse error
```

**`httptest.NewServer`** — creates a real HTTP server in memory on a random port. Our collector makes real HTTP calls to it. We're testing OUR code (request building, parsing, error handling) — not Prometheus itself.

**`callCount`** in `TestCollect_Success` — tracks which call is CPU vs Memory. First call returns CPU data, second returns Memory data. Collector always queries CPU first then Memory.

**`fakePrometheusResponse()`** — helper that builds a valid Prometheus JSON response structure with given container labels and values.

**Why mocking is correct here:**
- Unit/integration tests → mock external services (industry standard)
- We test OUR parsing, error handling, auth — not Prometheus internals
- E2E tests against real Prometheus → future, runs in CI/CD with Docker

---

## Integration vs E2E

| | Integration tests (current) | E2E tests (planned) |
|--|---|---|
| Server | In-memory via `httptest` | Real running server |
| Database | `podoptix_test` (auto-created) | Staging/dev cluster |
| Speed | Fast — milliseconds | Slow — seconds |
| When | Every commit | Before deployment |
| Catches | Code bugs | Deployment and config bugs |
