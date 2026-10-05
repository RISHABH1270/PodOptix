# PodOptix — Code Map

```mermaid
classDiagram
    direction TB

    class main {
        <<entry point>>
        cmd/hub/main.go
        wires everything
        handles SIGTERM
    }

    class config {
        <<package>>
        internal/config
        env var loader
    }

    class store {
        <<package>>
        internal/store
        pgxpool + migrations
        CRUD on clusters / users / recommendations
    }

    class cache {
        <<package>>
        internal/cache
        Redis · cache-aside · distributed locks
    }

    class collector {
        <<package>>
        internal/collector
        Prometheus queries
        pod → workload owner resolution
        MAX across replicas
    }

    class recommender {
        <<package>>
        internal/recommender
        p99 → request = ceil(p99)
        limit = ceil(p99 × 2)
    }

    class scheduler {
        <<package>>
        internal/scheduler
        24h ticker
        collect → recommend → upsert → mark orphan
    }

    class api {
        <<package>>
        internal/api
        Gin HTTP server
        auth · clusters · recommendations · savings
    }

    main --> config : 1 · Load config
    main --> store : 2 · EnsureDatabase ·· 3 · SyncSchema ·· 4 · Open pool (New)
    main --> cache : 5 · Connect Redis (New)
    main --> scheduler : 6 · New + go Start (background 24h loop)
    main --> api : 7 · NewServer + Listen + Serve (blocks)

    scheduler --> store : 1 · ListClusters ·· 5 · UpsertRecommendation ·· 6 · MarkOrphaned
    scheduler --> collector : 2 · Collect(lookback)
    scheduler --> recommender : 3 · GenerateAll (p99 math) ·· 4 · back to store

    api --> cache : 1 · check cache first
    api --> store : 2 · DB on miss ·· 3 · writes invalidate cache
    api --> scheduler : 4 · POST /recalculate → RunForCluster
```
