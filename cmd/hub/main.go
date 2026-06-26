package main

import (
	"fmt"
	"log"

	"github.com/RISHABH1270/PodOptix/internal/api"
	"github.com/RISHABH1270/PodOptix/internal/config"
	"github.com/RISHABH1270/PodOptix/internal/store"
)

const (
	cyan   = "\033[0;36m"
	green  = "\033[0;32m"
	yellow = "\033[0;33m"
	white  = "\033[1;37m"
	reset  = "\033[0m"
)

func main() {
	// load config first — everything depends on it
	var cfg *config.Config
	var err error
	cfg, err = config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// print banner
	printBanner(cfg.Port)

	// step 1 — ensure database exists
	if err = store.EnsureDatabase(cfg.DatabaseURL); err != nil {
		log.Fatalf("failed to ensure database: %v", err)
	}

	// step 2 — sync schema (run migrations)
	if err = store.SyncSchema(cfg.DatabaseURL); err != nil {
		log.Fatalf("schema sync failed: %v", err)
	}

	// step 3 — open permanent connection pool
	var db *store.Store
	db, err = store.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to initialize connection pool: %v", err)
	}
	defer db.Close()
	fmt.Println(green + "  Database : " + reset + "Schema synced · Connection pool ready")

	// TODO: connect to Redis
	// TODO: start scheduler

	// create and start HTTP server — inject store
	var server *api.Server
	server = api.NewServer(db)

	// start server — blocks here until stopped or error
	if err = server.Start(cfg.Port); err != nil {
		fmt.Println("\033[0;31m" + "  ERROR    : Server failed to start — " + err.Error() + reset)
		log.Fatalf("server failed: %v", err)
	}
}

func printBanner(port string) {
	bold := "\033[1m"
	fmt.Println()
	fmt.Println(bold + cyan + "  PodOptix" + reset + white + bold + "  —  Kubernetes Resource Right-Sizing  —  Powered by p99" + reset)
	fmt.Println(yellow + "  ──────────────────────────────────────────────────────────────" + reset)
	fmt.Println(green + "  Version  : " + reset + "v0.1.0")
	fmt.Println(green + "  Status   : " + reset + "Starting...")
	fmt.Println(green + "  Port     : " + reset + port)
	fmt.Println(yellow + "  ──────────────────────────────────────────────────────────────" + reset)
	fmt.Println()
}
