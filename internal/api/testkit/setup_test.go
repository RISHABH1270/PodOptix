package testkit

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/RISHABH1270/PodOptix/internal/api"
	"github.com/RISHABH1270/PodOptix/internal/auth"
	"github.com/RISHABH1270/PodOptix/internal/cache"
	"github.com/RISHABH1270/PodOptix/internal/store"
	"github.com/jackc/pgx/v5"
)

var (
	ts      *httptest.Server
	client  = &http.Client{}
	passed  int
	failed  int
	counter int
	mu      sync.Mutex
)

const (
	adminURL  = "postgres://postgres:password@localhost:5432/postgres?sslmode=disable"
	testDBURL = "postgres://postgres:password@localhost:5432/podoptix_test?sslmode=disable"
	redisURL  = "redis://localhost:6379/1" // Redis index 1 — production uses 0, tests use 1 to avoid key collisions
	jwtSecret = "test-jwt-secret-key-for-testing"
	encKey    = "test-32-byte-encryption-key!!!!1"
	testPort  = "9090"
)

// tty writes directly to the terminal — bypasses go test's stdout/stderr capture.
var tty *os.File

func log(format string, args ...any) {
	fmt.Fprintf(tty, format, args...)
}

func init() {
	os.Chdir("../../..")
	var err error
	tty, err = os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		tty = os.Stderr // fallback for CI environments without a TTY
	}
}

// shortName extracts the leaf test name from a full subtest path.
func shortName(fullName string) string {
	for i := len(fullName) - 1; i >= 0; i-- {
		if fullName[i] == '/' {
			return fullName[i+1:]
		}
	}
	return fullName
}

// track increments the running counter and prints a single clean result line on completion.
func track(t *testing.T) {
	t.Helper()
	mu.Lock()
	counter++
	n := counter
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		name := shortName(t.Name())
		if t.Failed() {
			failed++
			log("  [%2d]  ✗  %s\n", n, name)
		} else {
			passed++
			log("  [%2d]  ✓  %s\n", n, name)
		}
	})
}

// testToken returns a valid JWT for use in tests.
func testToken() string {
	t, _ := auth.GenerateToken("test-user-id", "testauth@podoptix.io", jwtSecret)
	return t
}

// bearer returns a raw Bearer Authorization header value.
func bearer(token string) string {
	return "Bearer " + token
}

// do fires a real HTTP request against the test server.
// auth is the raw Authorization header value.
func do(t *testing.T, method, path, body, auth string) *http.Response {
	t.Helper()

	var reqBody io.Reader
	if body != "" {
		reqBody = bytes.NewBufferString(body)
	}

	req, err := http.NewRequest(method, ts.URL+path, reqBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	return resp
}

// readBody reads and returns the response body as a string.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func TestMain(m *testing.M) {
	conn, err := pgx.Connect(context.Background(), adminURL)
	if err != nil {
		panic("failed to connect to postgres: " + err.Error())
	}
	conn.Exec(context.Background(), "DROP DATABASE IF EXISTS podoptix_test WITH (FORCE)")
	conn.Exec(context.Background(), "CREATE DATABASE podoptix_test")
	conn.Close(context.Background())

	if err := store.SyncSchema(testDBURL); err != nil {
		panic("failed to sync test schema: " + err.Error())
	}

	db, err := store.New(testDBURL)
	if err != nil {
		panic("failed to connect to test database: " + err.Error())
	}

	redisCache, err := cache.New(redisURL)
	if err != nil {
		panic("failed to connect to redis: " + err.Error())
	}
	if err := redisCache.FlushDB(context.Background()); err != nil {
		panic("failed to flush redis test db: " + err.Error())
	}

	srv := api.NewServer(db, redisCache, nil, jwtSecret, encKey) // nil scheduler — tests don't need background sync

	listener, err := srv.Listen(testPort)
	if err != nil {
		panic("failed to bind port " + testPort + ": " + err.Error())
	}
	ts = &httptest.Server{URL: "http://localhost:" + testPort}
	go srv.Serve(listener)

	log("\n  Running PodOptix API Tests...\n")
	log("  Server: %s\n", ts.URL)
	log("  ──────────────────────────────────────\n")

	code := m.Run()

	redisCache.Close()
	db.Close()
	conn, _ = pgx.Connect(context.Background(), adminURL)
	conn.Exec(context.Background(), "DROP DATABASE IF EXISTS podoptix_test WITH (FORCE)")
	conn.Close(context.Background())

	total := passed + failed
	log("  Total: %d  |  Passed: %d  |  Failed: %d\n", total, passed, failed)
	if code == 0 {
		log("  ✓ All tests passed\n")
	} else {
		log("  ✗ Some tests failed\n")
	}
	log("  ──────────────────────────────────────\n\n")
	os.Exit(code)
}
