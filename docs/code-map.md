# PodOptix — Code Map

```mermaid
flowchart TB
    main["<b>main</b><br/>cmd/hub/main.go<br/>wires everything<br/>graceful shutdown (10s drain)"]

    config["<b>config</b><br/>internal/config<br/>env var loader<br/>validates URL + key lengths at startup"]
    store["<b>store</b><br/>internal/store<br/>pgxpool + migrations<br/>CRUD on clusters / users / recommendations<br/>cluster delete cascades to recs"]
    cache["<b>cache</b><br/>internal/cache<br/>Redis · 3h cache-aside for rec list<br/>fencing-token lock for recalculate"]
    collector["<b>collector</b><br/>internal/collector<br/>Prometheus queries<br/>pod → workload owner resolution<br/>MAX across replicas per timestamp"]
    recommender["<b>recommender</b><br/>internal/recommender<br/>p99 → req=ceil(p99)<br/>limit=ceil(p99 × 2)"]
    scheduler["<b>scheduler</b><br/>internal/scheduler<br/>24h ticker · max 5 clusters parallel<br/>writes every rec field EXCEPT applied"]
    api["<b>api</b><br/>internal/api<br/>Gin HTTP server · 10s graceful drain<br/>sole writer of applied flag (PATCH)"]

    pg[("PostgreSQL<br/>external")]
    rd[("Redis<br/>external")]
    prom[("Prometheus<br/>per-cluster, external")]

    main -->|"1 · Load env"| config
    main -->|"2 · bootstrap DB"| store
    main -->|"3 · Redis"| cache
    main -->|"4 · go Start"| scheduler
    main -->|"5 · Serve"| api

    scheduler -->|"6 · ListClusters"| store
    scheduler -->|"7 · acquire lock (shared w/ recalc)"| cache
    scheduler -->|"8 · Collect(lookback)"| collector
    scheduler -->|"9 · GenerateAll (p99)"| recommender

    api -->|"10 · cache-aside"| cache
    api -->|"11 · DB on miss / writes"| store
    api -->|"12 · recalculate → RunForCluster"| scheduler

    store -.->|SQL| pg
    cache -.->|RESP| rd
    collector -.->|HTTP /api/v1/query| prom

    classDef ext fill:#1f2937,stroke:#6b7280,color:#e5e7eb
    class pg,rd,prom ext
```
