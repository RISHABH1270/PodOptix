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

    main --> config
    main --> store
    main --> cache
    main --> scheduler
    main --> api

    scheduler --> store
    scheduler --> collector
    scheduler --> recommender

    api --> store
    api --> cache
    api --> scheduler
```
