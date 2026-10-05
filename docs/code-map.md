# PodOptix — Code Map

A living diagram of the codebase. Grows as we walk through each file.

Rendered natively by GitHub (Mermaid). Zero dependencies.

**House rules:**
- Each diagram tells ONE story. Many small diagrams beat one giant one.
- Arrows go top-to-bottom or left-to-right only. **No cross arrows.**
- If an arrow would cross, split the diagram instead.

---

## Legend

| Arrow | Meaning |
|-------|---------|
| `A --> B` | A calls B |
| `A ..> B` | A creates / returns B |
| `A o-- B` | A holds a pointer to B (B lives independently) |
| `A *-- B` | A owns B (B dies with A) |

---

## 1 · Startup sequence (`cmd/hub/main.go`)

What `main()` does, in order:

```mermaid
flowchart TB
    s1["1 · config.Load() — read env vars"]
    s2["2 · store.EnsureDatabase() — CREATE DATABASE if missing"]
    s3["3 · store.SyncSchema() — run migrations"]
    s4["4 · store.New() — open pgxpool"]
    s5["5 · cache.New() — connect Redis"]
    s6["6 · signal.NotifyContext() — wire SIGTERM to ctx"]
    s7["7 · scheduler.New() + go sched.Start(ctx)"]
    s8["8 · api.NewServer() + server.Listen()"]
    s9["9 · go server.Serve(listener) — block"]
    s10["10 · ctx.Done() → server.Shutdown(10s) — drain in-flight"]

    s1 --> s2 --> s3 --> s4 --> s5 --> s6 --> s7 --> s8 --> s9 --> s10
```

---

## 2 · `internal/store/store.go` — DB bootstrap

Three steps, three different libraries, strict chronological chain:

```mermaid
flowchart TB
    e["EnsureDatabase(url)"]
    s["SyncSchema(url)"]
    n["New(url) → *Store"]

    e -->|"1 · pgx.Connect to 'postgres' admin DB<br/>then CREATE DATABASE podoptix"| pgx[("pgx<br/>jackc/pgx/v5")]
    s -->|"2 · migrate.New(file://, postgres://)<br/>then m.Up() (fails loud on dirty)"| mig[("golang-migrate/v4")]
    n -->|"3 · pgxpool.ParseConfig + NewWithConfig<br/>MaxConns 10 · MinConns 2 · 1h lifetime"| pool[("pgxpool<br/>jackc/pgx/v5/pgxpool")]
```

---

## 3 · `*Store` and its CRUD method files

`*Store` is just a wrapper around `*pgxpool.Pool`. The CRUD methods live in sibling files that attach to `*Store` via Go's method receiver syntax:

```mermaid
classDiagram
    direction TB

    class Store {
        <<struct>>
        -pool *pgxpool.Pool
        +Ping(ctx) error
        +Close()
    }

    class cluster_go {
        <<methods on *Store>>
        internal/store/cluster.go
        +SaveCluster · GetCluster · ListClusters
        +UpdateCluster · UpdateClusterHealth · DeleteCluster
    }

    class recommendation_go {
        <<methods on *Store>>
        internal/store/recommendation.go
        +UpsertRecommendation (clears first_missed_at)
        +ListByCluster · ListAllWithClusterName
        +MarkOrphaned (safety-gated set-diff)
        +DeleteOrphaned · DeleteRecommendation
    }

    class user_go {
        <<methods on *Store>>
        internal/store/user.go
        +CreateUser · GetUserByEmail
    }

    Store *-- cluster_go
    Store *-- recommendation_go
    Store *-- user_go
```

Three repeating patterns across all three files:
- `pool.Exec` + check err → INSERT with no match check
- `pool.Exec` + `tag.RowsAffected()==0` → UPDATE/DELETE where no-match = not-found error
- `pool.QueryRow+Scan` (one row) or `pool.Query + rows.Next + Scan` (many rows) → SELECT

---

## 4 · Who holds what at runtime

After `main()` wires everything together:

```mermaid
classDiagram
    direction TB

    class Store {
        <<struct>>
        -pool *pgxpool.Pool
    }
    class Cache {
        <<struct>>
        -client *redis.Client
    }
    class Scheduler {
        <<struct>>
        -store *Store
        -interval 24h
        -encryptionKey string
    }
    class Server {
        <<struct>>
        -router *gin.Engine
        -httpServer *http.Server
        -store *Store
        -cache *Cache
        -scheduler *Scheduler
    }

    Scheduler o-- Store
    Server o-- Store
    Server o-- Cache
    Server o-- Scheduler
```

`o--` means "holds a pointer to" — the Store/Cache are created once by `main` and shared. `defer db.Close()` in `main` cleans them up at shutdown.

---

## 5 · Recommendation lifecycle (scheduler run)

End-to-end flow of one scheduler tick for one cluster:

```mermaid
flowchart TB
    a["Scheduler.RunForCluster(cluster)"]
    b["Collector.Collect(lookback)<br/>6 usage/limit queries<br/>+ kube_pod_owner<br/>+ kube_replicaset_owner"]
    c["buildOwnerMap()<br/>pod → (kind, name)<br/>RS collapses to Deployment"]
    d["mergeMetrics()<br/>MAX across replicas per timestamp<br/>→ []ContainerMetrics (one per workload-container)"]
    e["Recommender.GenerateAll()<br/>p99 → request=ceil(p99), limit=ceil(p99×2)<br/>→ []Recommendation"]
    f["Store.UpsertRecommendation (loop)<br/>ON CONFLICT clears first_missed_at = NULL<br/>→ builds seenKeys"]
    g["Store.MarkOrphaned(cluster, seenKeys)<br/>UPDATE first_missed_at=NOW()<br/>WHERE NOT IN seen AND first_missed_at IS NULL<br/>no-op if seenKeys empty"]
    h["Store.UpdateClusterHealth(connected, now)"]

    a --> b --> c --> d --> e --> f --> g --> h
```

---

## 6 · Delete / orphan review path

Operator reviews orphans in the dashboard and clicks delete:

```mermaid
flowchart TB
    ui["Dashboard<br/>ClusterDetail · orphaned section"]
    r1["DELETE /clusters/:id/recommendations/:recId<br/>per-row trash button"]
    r2["DELETE /clusters/:id/recommendations?orphaned=true<br/>bulk 'delete all orphaned' button<br/>?orphaned=true REQUIRED (footgun guard)"]
    s1["Store.DeleteRecommendation(recID)<br/>404 if not found"]
    s2["Store.DeleteOrphaned(cluster)<br/>→ rows deleted count"]
    cache["cache.InvalidateRecommendations(cluster)"]

    ui --> r1 --> s1 --> cache
    ui --> r2 --> s2 --> cache
```

---

## What's coming next

Each session adds a new focused diagram. Up next: `internal/config/config.go` on a second review pass.
