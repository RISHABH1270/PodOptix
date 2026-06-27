package api

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/RISHABH1270/PodOptix/internal/store"
	"github.com/jackc/pgx/v5"
)

var testServer *Server

// counter tracks test results across all tests
var counter = &testCounter{}

type testCounter struct {
	mu     sync.Mutex
	total  int
	passed int
	failed int
}

// trackTest registers a test for counting and prints its name when it starts.
func trackTest(t *testing.T) {
	counter.mu.Lock()
	counter.total++
	counter.mu.Unlock()

	t.Cleanup(func() {
		counter.mu.Lock()
		defer counter.mu.Unlock()
		if t.Failed() {
			counter.failed++
		} else {
			counter.passed++
		}
	})
}

func init() {
	os.Chdir("../..")
}

func TestMain(m *testing.M) {

	// connects to PostgreSQL itself - the default "postgres" database
	// You can't drop a database while you're connected to it
	adminURL := "postgres://postgres:password@localhost:5432/postgres?sslmode=disable"

	// connects to our test database - podoptix_test
	testDBURL := "postgres://postgres:password@localhost:5432/podoptix_test?sslmode=disable"

	// drop and recreate test DB — clean slate every run
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

	testServer = NewServer(db)

	fmt.Println("\n  Running PodOptix API Tests...")
	fmt.Println("  ──────────────────────────────────────")

	code := m.Run()

	db.Close()
	conn, _ = pgx.Connect(context.Background(), adminURL)
	conn.Exec(context.Background(), "DROP DATABASE IF EXISTS podoptix_test WITH (FORCE)")
	conn.Close(context.Background())

	fmt.Println("  ──────────────────────────────────────")
	fmt.Printf("  Total: %d  |  Passed: %d  |  Failed: %d\n",
		counter.total, counter.passed, counter.failed)
	if code == 0 {
		fmt.Println("  All tests passed")
	} else {
		fmt.Println("  Some tests failed")
	}
	fmt.Println()

	os.Exit(code)
}
