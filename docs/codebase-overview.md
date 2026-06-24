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
