package main

import "fmt"

const (
	cyan   = "\033[0;36m"
	green  = "\033[0;32m"
	yellow = "\033[0;33m"
	white  = "\033[1;37m"
	reset  = "\033[0m"
)

func main() {
	printBanner()

	// TODO: load config
	// TODO: connect to PostgreSQL
	// TODO: connect to Redis
	// TODO: start scheduler
	// TODO: start HTTP server
}

func printBanner() {
	bold := "\033[1m"
	fmt.Println()
	fmt.Println(bold + cyan + "  PodOptix" + reset + white + bold + "  —  Kubernetes Resource Right-Sizing  —  Powered by p99" + reset)
	fmt.Println(yellow + "  ──────────────────────────────────────────────────────────────" + reset)
	fmt.Println(green  + "  Version  : " + reset + "v0.1.0-MVP")
	fmt.Println(green  + "  Status   : " + reset + "Starting...")
	fmt.Println(green  + "  Port     : " + reset + "8080")
	fmt.Println(yellow + "  ──────────────────────────────────────────────────────────────" + reset)
	fmt.Println()
}
