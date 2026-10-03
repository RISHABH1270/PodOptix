# PodOptix — Code Map

A living diagram of the codebase. Grows as we walk through each file.

Rendered natively by GitHub (Mermaid syntax). Zero dependencies.

---

## Legend

| Arrow | Meaning |
|-------|---------|
| `A --> B : foo()` | A calls `B.foo()` directly |
| `A ..> B : creates` | A instantiates B |
| `A --|> B` | A inherits/implements B |
| `A o-- B` | A holds B (aggregation — B lives independently) |
| `A *-- B` | A owns B (composition — B dies with A) |

**Stereotypes used:**

| Stereotype | What it marks |
|------------|---------------|
| `<<entry point>>` | Where the program starts (`func main`) |
| `<<package>>` | A Go package (collection of files under one `internal/x/`) |
| `<<struct>>` | A single Go struct type |
| `<<helper>>` | Internal utility function |

---

## Current scope: `cmd/hub/main.go`

So far the diagram covers the startup orchestration — the entry point and every package `main` directly calls.

```mermaid
classDiagram
    direction LR

    class main {
        <<entry point>>
        cmd/hub/main.go
        +main()
        -must(label, err) helper
        -info(label, msg) helper
        -printBanner(port) helper
        ─── ANSI color consts ───
        cyan green yellow red white reset
    }

    class config {
        <<package>>
        internal/config
        +Load() Config, error
        -mustGetEnv(key) string, error
        -getEnv(key, fallback) string
    }

    class Config {
        <<struct>>
        +Port string
        +DatabaseURL string
        +RedisURL string
        +JWTSecret string
        +EncryptionKey string
    }

    class osStdlib {
        <<stdlib>>
        os
        +Getenv(key) string
    }

    config --> osStdlib : reads env

    class store {
        <<package>>
        internal/store
        +EnsureDatabase(url) error
        +SyncSchema(url) error
        +New(url) Store
    }

    class Store {
        <<struct>>
        -pool *pgxpool.Pool
        +Ping(ctx) error
        +Close()
    }

    class cache {
        <<package>>
        internal/cache
        +New(url) Cache
    }

    class Cache {
        <<struct>>
        -client *redis.Client
        +Close() error
    }

    class scheduler {
        <<package>>
        internal/scheduler
        +New(db, interval, key) Scheduler
    }

    class Scheduler {
        <<struct>>
        -store *Store
        -interval Duration
        -encryptionKey string
        +Start(ctx)
    }

    class api {
        <<package>>
        internal/api
        +NewServer(db, cache, sched, jwt, enc) Server
    }

    class Server {
        <<struct>>
        -router *gin.Engine
        -store *Store
        -cache *Cache
        -scheduler *Scheduler
        +Listen(port) net.Listener
        +Serve(listener) error
    }

    class signal {
        <<stdlib>>
        os/signal
        +NotifyContext(parent, signals) ctx, stop
    }

    main --> config : 1. Load()
    config ..> Config : returns

    main --> store : 2. EnsureDatabase, SyncSchema
    main --> store : 3. New()
    store ..> Store : returns

    main --> cache : 4. New()
    cache ..> Cache : returns

    main --> signal : 5. NotifyContext()

    main --> scheduler : 6. New()
    scheduler ..> Scheduler : returns
    Scheduler o-- Store : uses

    main --> api : 7. NewServer()
    api ..> Server : returns
    Server o-- Store : uses
    Server o-- Cache : uses
    Server o-- Scheduler : uses

    main --> Server : 8. Listen() then 9. Serve()
```

### What this says in English

1. **main** is the orchestrator. It knows about 6 external packages: `config`, `store`, `cache`, `scheduler`, `api`, and stdlib `signal`.
2. **main → config.Load()** returns a `Config` struct with env vars.
3. **main → store.EnsureDatabase() + SyncSchema() + New()** — DB bootstrap, then open the pool, returns a `*Store`.
4. **main → cache.New()** — Redis connection, returns a `*Cache`.
5. **main → signal.NotifyContext()** — wires OS signals (Ctrl+C, SIGTERM) to cancel a context.
6. **main → scheduler.New()** — constructs the `*Scheduler`, which **holds** a pointer to `*Store`.
7. **main → api.NewServer()** — constructs the `*Server`, which **holds** pointers to `*Store`, `*Cache`, `*Scheduler`.
8. **main → Server.Listen() → Serve()** — bind port, run until the signal context cancels.

The `o--` (aggregation) arrows show that the Scheduler and Server don't own their dependencies — they share pointers to the Store/Cache that `main` created and will clean up via `defer`.

---

## What's coming next

Each session adds boxes + arrows. By the end we'll have the complete call graph: HTTP handlers → store methods → SQL, scheduler → collector → PromQL, etc.

**Next add:** `internal/store/store.go` (adds the `EnsureDatabase` chicken-and-egg dance, `SyncSchema` migrations, and the `pgxpool` connection pool).
