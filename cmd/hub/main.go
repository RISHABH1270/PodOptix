package main

import (
	"fmt"
	"log"

	"github.com/RISHABH1270/podoptix/internal/config"
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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// now print banner with real values from config
	printBanner(cfg.Port)

	// TODO: connect to PostgreSQL
	// TODO: connect to Redis
	// TODO: start scheduler
	// TODO: start HTTP server
}

func printBanner(port string) {
	bold := "\033[1m"
	fmt.Println()
	fmt.Println(bold + cyan + "  PodOptix" + reset + white + bold + "  —  Kubernetes Resource Right-Sizing  —  Powered by p99" + reset)
	fmt.Println(yellow + "  ──────────────────────────────────────────────────────────────" + reset)
	fmt.Println(green  + "  Version  : " + reset + "v0.1.0")
	fmt.Println(green  + "  Status   : " + reset + "Starting...")
	fmt.Println(green  + "  Port     : " + reset + port)
	fmt.Println(yellow + "  ──────────────────────────────────────────────────────────────" + reset)
	fmt.Println()
}
