package testkit

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/RISHABH1270/PodOptix/internal/api"
	"github.com/RISHABH1270/PodOptix/internal/auth"
	"github.com/RISHABH1270/PodOptix/internal/store"
	"github.com/jackc/pgx/v5"
)

var srv *api.Server

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

// do fires a request against the test server and returns the recorder.
func do(t *testing.T, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)
	return w
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

	srv = api.NewServer(db, nil, jwtSecret, encKey)

	fmt.Println("\n  Running PodOptix API Tests...")
	fmt.Println("  ──────────────────────────────────────")

	code := m.Run()

	db.Close()
	conn, _ = pgx.Connect(context.Background(), adminURL)
	conn.Exec(context.Background(), "DROP DATABASE IF EXISTS podoptix_test WITH (FORCE)")
	conn.Close(context.Background())

	fmt.Println("  ──────────────────────────────────────")
	fmt.Println()
	os.Exit(code)
}
