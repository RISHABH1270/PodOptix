package testkit

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/RISHABH1270/PodOptix/internal/api"
	"github.com/RISHABH1270/PodOptix/internal/auth"
	"github.com/RISHABH1270/PodOptix/internal/store"
	"github.com/jackc/pgx/v5"
)

var (
	ts     *httptest.Server // real TCP listener — tests make actual HTTP requests
	client = &http.Client{}
)

const (
	adminURL  = "postgres://postgres:password@localhost:5432/postgres?sslmode=disable"
	testDBURL = "postgres://postgres:password@localhost:5432/podoptix_test?sslmode=disable"
	jwtSecret = "test-jwt-secret-key-for-testing"
	encKey    = "test-32-byte-encryption-key!!!!1"
)

func init() {
	os.Chdir("../../..")
}

// testToken returns a valid JWT for use in tests.
func testToken() string {
	t, _ := auth.GenerateToken("test-user-id", "testauth@podoptix.io", jwtSecret)
	return t
}

// do fires a real HTTP request against the test server and returns the response.
func do(t *testing.T, method, path, body, token string) *http.Response {
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
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	return resp
}

// readBody reads and returns the response body as string.
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

	srv := api.NewServer(db, nil, jwtSecret, encKey)

	// real TCP listener on a random port — tests make actual HTTP requests
	ts = httptest.NewServer(srv)

	fmt.Println("\n  Running PodOptix API Tests...")
	fmt.Printf("  Server: %s\n", ts.URL)
	fmt.Println("  ──────────────────────────────────────")

	code := m.Run()

	ts.Close()
	db.Close()
	conn, _ = pgx.Connect(context.Background(), adminURL)
	conn.Exec(context.Background(), "DROP DATABASE IF EXISTS podoptix_test WITH (FORCE)")
	conn.Close(context.Background())

	fmt.Println("  ──────────────────────────────────────")
	fmt.Println()
	os.Exit(code)
}
