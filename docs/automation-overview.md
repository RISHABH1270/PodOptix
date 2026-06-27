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

## Integration vs E2E

| | Integration tests (current) | E2E tests (planned) |
|--|---|---|
| Server | In-memory via `httptest` | Real running server |
| Database | `podoptix_test` (auto-created) | Staging/dev cluster |
| Speed | Fast — milliseconds | Slow — seconds |
| When | Every commit | Before deployment |
| Catches | Code bugs | Deployment and config bugs |
