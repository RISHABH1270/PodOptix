# PodOptix — Codebase Overview

---

## 1. `go.mod`

Project manifest — same as `build.gradle` (Java), `requirements.txt` (Python), `package.json` (Node.js).

- `module` — unique project name, used as base for all internal imports
- `go 1.26.4` — minimum Go version (lower bound)
- `require` — 3 libraries we directly chose: `gin` (HTTP server framework), `golang-migrate` (SQL migrations), `pgx` (PostgreSQL driver converts your Go function calls into PostgreSQL's wire protocol)
- `// indirect` — dependencies of our dependencies, managed automatically by Go

`go.sum` — companion file that stores cryptographic fingerprints of every library to prevent tampering.

---

## 2. `pkg/models/cluster.go`

Blueprint for a Kubernetes cluster. Defines what data it holds.

```
Cluster struct (in memory)
┌─────────────────────────────────────────────────┐
│ ClusterID      "a3f8c2d1-9b4e-4f1a-8c3d..."    │  string
│ Name           "production-cluster"             │  string
│ PrometheusURL  "https://prometheus.example.com" │  string
│ Token          "eyJhbGci..."  (hidden in API)   │  string
│ LookbackWindow "7d"                             │  string
│ CreatedAt      2026-06-24 00:00:00              │  time.Time
│ UpdatedAt      2026-06-24 00:00:00              │  time.Time
└─────────────────────────────────────────────────┘

*Cluster = pointer (8 bytes) → points to the above struct in heap
```

Struct tags control how fields are named in JSON responses (`json:"x"`) and PostgreSQL columns (`db:"x"`). `json:"-"` on Token means it is never sent in API responses.

---
