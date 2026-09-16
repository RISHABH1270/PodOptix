.PHONY: help dev dashboard build test test-api test-ui clean

help:
	@echo "PodOptix — common commands"
	@echo ""
	@echo "  make dev         Run the Go backend (needs docker compose up -d first)"
	@echo "  make dashboard   Build the React dashboard into internal/dashboard/dist/"
	@echo "  make build       Build the dashboard + Go binary at bin/podoptix"
	@echo "  make test        Run all tests (backend API + UI e2e)"
	@echo "  make test-api    Run backend Go tests only"
	@echo "  make test-ui     Run Playwright UI tests only"
	@echo "  make clean       Remove bin/, node_modules/, and built dashboard"

dev:
	go run ./cmd/hub

dashboard:
	cd web && npm install --silent && npm run build

build: dashboard
	mkdir -p bin
	go build -o bin/podoptix ./cmd/hub
	@echo ""
	@echo "  ✓ Single-binary build ready at bin/podoptix"
	@echo "    → serves API on the configured PORT (default 8080)"
	@echo "    → serves embedded dashboard at the same origin"

test: test-api test-ui

test-api:
	go test ./tests/... -count=1 -p 1

test-ui:
	cd web && npm run test:e2e

clean:
	rm -rf bin web/node_modules web/dist
	find internal/dashboard/dist -mindepth 1 ! -name '.gitkeep' -exec rm -rf {} +
