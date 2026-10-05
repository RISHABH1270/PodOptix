package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RISHABH1270/PodOptix/internal/api"       // Gin HTTP server
	"github.com/RISHABH1270/PodOptix/internal/cache"     // Redis wrapper
	"github.com/RISHABH1270/PodOptix/internal/config"    // env var loader
	"github.com/RISHABH1270/PodOptix/internal/scheduler" // 24h ticker
	"github.com/RISHABH1270/PodOptix/internal/store"     // PostgreSQL
)

const (
	cyan   = "\033[0;36m"
	green  = "\033[0;32m"
	yellow = "\033[0;33m"
	white  = "\033[1;37m"
	red    = "\033[0;31m"
	reset  = "\033[0m"
)

// main is the entry point — wires every package together and controls startup + shutdown.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	printBanner(cfg.Port)

	must("Database ", store.EnsureDatabase(cfg.DatabaseURL))
	must("Schema   ", store.SyncSchema(cfg.DatabaseURL))

	db, err := store.New(cfg.DatabaseURL)
	must("Pool     ", err)
	defer db.Close()

	redisCache, err := cache.New(cfg.RedisURL)
	must("Redis    ", err)
	defer redisCache.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sched := scheduler.New(db, 24*time.Hour, cfg.EncryptionKey)
	go sched.Start(ctx)
	info("Scheduler", "Started — 24h interval")

	server := api.NewServer(db, redisCache, sched, cfg.JWTSecret, cfg.EncryptionKey)
	listener, err := server.Listen(cfg.Port)
	must("Server   ", err)
	fmt.Println(yellow + "  ──────────────────────────────────────────────────────────────" + reset)
	fmt.Println()

	// run server in background — block until SIGTERM/SIGINT
	go server.Serve(listener) //nolint

	<-ctx.Done()
	log.Println("INFO  received shutdown signal — draining in-flight requests (10s deadline)")

	// Stop accepting new connections and wait for in-flight handlers to finish.
	// http.Server.Shutdown closes the listener too — no listener.Close() needed.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("WARN  shutdown deadline hit before drain completed: %v", err)
	}
	log.Println("INFO  shutdown complete")
}

// must prints a green success line or a red failure and exits.
// Writes the failure line once to stderr (where ops tooling expects errors) and exits non-zero.
func must(label string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s  %s: failed - %s%s\n", red, label, err.Error(), reset)
		os.Exit(1)
	}
	fmt.Printf("%s  %s:%s OK\n", green, label, reset)
}

// info prints a labelled status line.
func info(label, msg string) {
	fmt.Printf("%s  %s:%s %s\n", green, label, reset, msg)
}

func printBanner(port string) {
	bold := "\033[1m"
	fmt.Println()
	fmt.Println(bold + cyan + "  PodOptix" + reset + white + bold + "  —  Kubernetes Resource Right-Sizing  —  Powered by p99" + reset)
	fmt.Println(yellow + "  ──────────────────────────────────────────────────────────────" + reset)
	fmt.Printf("%s  Version  :%s v0.1.0\n", green, reset)
	fmt.Printf("%s  Port     :%s %s\n", green, reset, port)
	fmt.Println(yellow + "  ──────────────────────────────────────────────────────────────" + reset)
	fmt.Println()
}
