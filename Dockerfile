# To build and test: docker build -t podoptix/hub:v0.1.0 .

# ── Stage 1: Build React frontend ────────────────────────────────────────────
FROM node:24-alpine AS frontend-builder

WORKDIR /app/frontend

# install dependencies first (cached if package.json unchanged)
COPY frontend/package*.json ./
RUN npm ci

# copy source and build
COPY frontend/ ./
RUN npm run build
# output: /app/frontend/dist/


# ── Stage 2: Build Go binary ──────────────────────────────────────────────────
FROM golang:1.26-alpine AS go-builder

WORKDIR /app

# download Go dependencies first (cached if go.mod unchanged)
COPY go.mod go.sum ./
RUN go mod download

# copy all source code
COPY . .

# copy compiled frontend from stage 1 — embedded into the binary
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# build the binary
# CGO_ENABLED=0 = static binary, no C dependencies
# -ldflags="-s -w" = strip debug info (smaller binary)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o hub ./cmd/hub


# ── Stage 3: Minimal runtime image ───────────────────────────────────────────
FROM alpine:3.20

# add CA certificates for HTTPS calls to Prometheus
RUN apk --no-cache add ca-certificates

WORKDIR /app

# copy only the compiled binary — nothing else needed
COPY --from=go-builder /app/hub .

# run as non-root user — security best practice
RUN adduser -D -u 1000 podoptix
USER podoptix

EXPOSE 8080

ENTRYPOINT ["./hub"]
