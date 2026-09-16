# Container image config — override with `make docker-push IMAGE=my/name TAG=v1`
IMAGE      ?= ghcr.io/rishabh1270/podoptix
TAG        ?= dev
HELM_CHART := ./deploy/helm/podoptix
# Helm 3.8+ pushes charts to OCI registries — same host + auth as docker.
HELM_REPO  ?= oci://ghcr.io/rishabh1270/charts

.PHONY: help dev dashboard build test test-api test-ui clean vendor docker-build docker-push docker-run helm-lint helm-package helm-push

help:
	@echo "PodOptix — common commands"
	@echo ""
	@echo "  make dev            Run the Go backend (needs docker compose up -d first)"
	@echo "  make dashboard      Build the React dashboard into internal/dashboard/dist/"
	@echo "  make build          Build the dashboard + Go binary at bin/podoptix"
	@echo "  make test           Run all tests (backend API + UI e2e)"
	@echo "  make test-api       Run backend Go tests only"
	@echo "  make test-ui        Run Playwright UI tests only"
	@echo "  make docker-build   Build local single-arch Docker image (podoptix:local)"
	@echo "  make docker-run     Run the local image against docker compose services"
	@echo "  make docker-push    Multi-arch build + push (amd64 + arm64) — needs registry login"
	@echo "  make helm-lint      Lint the Helm chart"
	@echo "  make helm-package   Package the Helm chart into a .tgz"
	@echo "  make helm-push      Push chart to OCI registry (same as docker: ghcr.io)"
	@echo "  make clean          Remove bin/, node_modules/, and built dashboard"

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

# ── Docker ────────────────────────────────────────────────────────────

# Refresh vendored Go deps — required before any docker build so the container
# doesn't need network access to proxy.golang.org (bypasses corporate SSL intercept).
vendor:
	go mod vendor

# Build a single-arch image for the CURRENT machine (fast, loads into local docker).
# Use this to test the container locally before pushing.
docker-build: vendor
	docker buildx build --load -t podoptix:local .
	@echo ""
	@echo "  ✓ Built podoptix:local — try: make docker-run"

# Run the local image, connecting to docker compose Postgres/Redis via host.docker.internal.
docker-run:
	docker run --rm -p 8080:8080 \
	  -e DATABASE_URL='postgres://postgres:password@host.docker.internal:5432/podoptix?sslmode=disable' \
	  -e REDIS_URL='redis://host.docker.internal:6379' \
	  -e JWT_SECRET='dev-jwt-secret' \
	  -e ENCRYPTION_KEY='dev-32-byte-encryption-key!!!!!!' \
	  podoptix:local

# Multi-arch build + push to registry. Requires `docker login` to the registry first.
# Example: make docker-push IMAGE=ghcr.io/your-user/podoptix TAG=v0.1.0
docker-push: vendor
	docker buildx build \
	  --platform linux/amd64,linux/arm64 \
	  -t $(IMAGE):$(TAG) \
	  --push \
	  .
	@echo ""
	@echo "  ✓ Pushed $(IMAGE):$(TAG) — amd64 + arm64"

# ── Helm ──────────────────────────────────────────────────────────────

helm-lint:
	helm lint $(HELM_CHART)

# Package the chart into a versioned .tgz (reads version from Chart.yaml).
helm-package:
	mkdir -p bin
	helm package $(HELM_CHART) -d bin/
	@echo ""
	@echo "  ✓ Chart packaged into bin/"

# Push to OCI registry — Helm 3.8+ uses same auth as docker login.
# Example: make helm-push HELM_REPO=oci://ghcr.io/rishabh1270/charts
helm-push: helm-package
	@CHART_TGZ=$$(ls -t bin/podoptix-*.tgz | head -1); \
	  echo "  Pushing $$CHART_TGZ to $(HELM_REPO)..."; \
	  helm push $$CHART_TGZ $(HELM_REPO)
	@echo ""
	@echo "  ✓ Chart pushed — customers can now install with:"
	@echo "      helm install podoptix $(HELM_REPO)/podoptix --version <ver>"
