# PodOptix — multi-platform container image.
# Produces one manifest for both linux/amd64 and linux/arm64 in a single buildx run.
#
# Build strategy: builder stages run on the NATIVE architecture ($BUILDPLATFORM),
# Go cross-compiles to $TARGETARCH — fast on Apple Silicon (no QEMU emulation).
#
#   Stage 1 (web):   build the React dashboard   → /out/dist            (arch-independent JS)
#   Stage 2 (go):    cross-compile the Go binary → /out/podoptix        (native cross, not QEMU)
#   Stage 3 (final): distroless runtime          → runs as UID 65532, exposes :8080

# ── Stage 1: build the React dashboard (arch-independent) ────────────
FROM --platform=$BUILDPLATFORM node:20-alpine AS web-builder
WORKDIR /app/web

# Cache npm install by copying lockfile first
COPY web/package.json web/package-lock.json ./
RUN npm ci --silent

# Copy source and build — Vite outputs to ../internal/dashboard/dist
COPY web/ ./
RUN mkdir -p /app/internal/dashboard/dist && npm run build

# ── Stage 2: cross-compile the Go binary for $TARGETARCH ─────────────
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS go-builder
WORKDIR /app

# buildx injects these automatically — used to cross-compile
ARG TARGETOS
ARG TARGETARCH

# Copy vendored deps + go.mod/go.sum — no network needed at build time
COPY go.mod go.sum ./
COPY vendor/ ./vendor/

# Copy Go source (only folders that go build needs)
COPY cmd/        ./cmd/
COPY internal/   ./internal/
COPY pkg/        ./pkg/
COPY migrations/ ./migrations/

# Pull in the freshly built dashboard from Stage 1 — go:embed picks it up
COPY --from=web-builder /app/internal/dashboard/dist ./internal/dashboard/dist

# CGO_ENABLED=0 → fully static binary (works on distroless/scratch)
# -ldflags='-s -w' → strip debug info, ~30% smaller binary
# -mod=vendor    → use vendored deps, no network needed
# GOOS/GOARCH from buildx → Go cross-compiles natively, no QEMU needed
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
      go build -mod=vendor -ldflags='-s -w' -o /out/podoptix ./cmd/hub

# ── Stage 3: minimal runtime image ───────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

# Binary + migrations (SyncSchema reads file://migrations at startup)
COPY --from=go-builder /out/podoptix   /app/podoptix
COPY --from=go-builder /app/migrations /app/migrations

ENV PORT=8080
EXPOSE 8080

# distroless :nonroot runs as UID 65532 with no shell — smallest attack surface
USER nonroot:nonroot
ENTRYPOINT ["/app/podoptix"]
