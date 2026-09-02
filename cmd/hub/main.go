package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RISHABH1270/PodOptix/internal/api"
	"github.com/RISHABH1270/PodOptix/internal/cache"
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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	printBanner(cfg.Port)

	if err = store.EnsureDatabase(cfg.DatabaseURL); err != nil {
		fmt.Println(red + "  Database : failed — " + err.Error() + reset)
		log.Fatalf("failed to ensure database: %v", err)
	}
	fmt.Println(green + "  Database : " + reset + "Database ready")

	if err = store.SyncSchema(cfg.DatabaseURL); err != nil {
		fmt.Println(red + "  Schema   : failed — " + err.Error() + reset)
		log.Fatalf("schema sync failed: %v", err)
	}
	fmt.Println(green + "  Schema   : " + reset + "Schema synced")

	db, err := store.New(cfg.DatabaseURL)
	if err != nil {
		fmt.Println(red + "  Pool     : failed — " + err.Error() + reset)
		log.Fatalf("failed to initialize connection pool: %v", err)
	}
	defer db.Close()
	fmt.Println(green + "  Pool     : " + reset + "Connection pool ready")

	redisCache, err := cache.New(cfg.RedisURL)
	if err != nil {
		fmt.Println(red + "  Redis    : failed — " + err.Error() + reset)
		log.Fatalf("failed to connect to redis: %v", err)
	}
	defer redisCache.Close()
	fmt.Println(green + "  Redis    : " + reset + "Connected")

	// context cancelled on SIGTERM/SIGINT — propagates to scheduler and in-flight jobs
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sched := scheduler.New(db, 24*time.Hour, cfg.EncryptionKey)
	go sched.Start(ctx)
	fmt.Println(green + "  Scheduler: " + reset + "Started — runs every 24 hours")

	server := api.NewServer(db, redisCache, sched, cfg.JWTSecret, cfg.EncryptionKey)

	listener, err := server.Listen(cfg.Port)
	if err != nil {
		fmt.Println(red + "  Server   : failed to bind port " + cfg.Port + " — " + err.Error() + reset)
		log.Fatalf("server failed: %v", err)
	}

	fmt.Println(green + "  Server   : " + reset + "Up and running on port " + cfg.Port)
	fmt.Println(yellow + "  ──────────────────────────────────────────────────────────────" + reset)
	fmt.Println()

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
