package main

import (
	"fmt"
	"log"

	"context"
	"time"

	"github.com/RISHABH1270/PodOptix/internal/api"
	"github.com/RISHABH1270/PodOptix/internal/config"
	"github.com/RISHABH1270/PodOptix/internal/scheduler"
	"github.com/RISHABH1270/PodOptix/internal/store"
)

const (
	cyan   = "\033[0;36m"
	green  = "\033[0;32m"
	yellow = "\033[0;33m"
	white  = "\033[1;37m"
	red    = "\033[0;31m"
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

	printBanner(cfg.Port)

	// ensure database exists
	if err = store.EnsureDatabase(cfg.DatabaseURL); err != nil {
		fmt.Println(red + "  Database : failed — " + err.Error() + reset)
		log.Fatalf("failed to ensure database: %v", err)
	}
	fmt.Println(green + "  Database : " + reset + "Database ready")

	// sync schema
	if err = store.SyncSchema(cfg.DatabaseURL); err != nil {
		fmt.Println(red + "  Schema   : failed — " + err.Error() + reset)
		log.Fatalf("schema sync failed: %v", err)
	}
	fmt.Println(green + "  Schema   : " + reset + "Schema synced")

	// step 3 — open connection pool
	var db *store.Store
	db, err = store.New(cfg.DatabaseURL)
	if err != nil {
		fmt.Println(red + "  Pool     : failed — " + err.Error() + reset)
		log.Fatalf("failed to initialize connection pool: %v", err)
	}
	defer db.Close()
	fmt.Println(green + "  Pool     : " + reset + "Connection pool ready")

	// TODO: connect to Redis

	// start scheduler in background — runs once per day
	sched := scheduler.New(db, 24*time.Hour)
	go sched.Start(context.Background())
	fmt.Println(green + "  Scheduler: " + reset + "Started — runs every 24 hours")

	// step 4 — start HTTP server
	var server *api.Server
	server = api.NewServer(db, cfg.JWTSecret)

	// Bind the port first — if this succeeds the server is guaranteed to be up
	listener, err := server.Listen(cfg.Port)
	if err != nil {
		fmt.Println(red + "  Server   : failed to bind port " + cfg.Port + " — " + err.Error() + reset)
		log.Fatalf("server failed: %v", err)
	}

	// Port is bound — safe to confirm server is up
	fmt.Println(green + "  Server   : " + reset + "Up and running on port " + cfg.Port)
	fmt.Println(yellow + "  ──────────────────────────────────────────────────────────────" + reset)
	fmt.Println()

	// Start accepting requests — blocks here
	if err = server.Serve(listener); err != nil {
		fmt.Println(red + "  ERROR    : Server stopped — " + err.Error() + reset)
		log.Fatalf("server stopped: %v", err)
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
