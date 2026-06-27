# PodOptix — Architecture

## Table of Contents

- [System Overview](#system-overview)
- [Hub Internals](#hub-internals)
- [Data Flow](#data-flow)
- [Component Reference](#component-reference)
- [Metrics Collected](#metrics-collected)
- [Security Model](#security-model)

---

## System Overview

PodOptix runs as a single **Master Hub** deployed in your management or ops cluster. It directly queries each workload cluster's Prometheus via the PromQL HTTP API. No agents. No sidecars. Nothing to deploy inside workload clusters.

```
                           ┌──────────────┐
                           │     User     │
                           └──────┬───────┘
                                  │ HTTPS
┌─────────────────────────────────────────────────────────────────────┐
│                    MANAGEMENT / OPS CLUSTER                         │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                        PODOPTIX HUB                           │  │
│  │                                                               │  │
│  │   ┌────────────────────────┐     ┌────────────────────────┐   │  │
│  │   │       Dashboard        │     │        REST API        │   │  │
│  │   └────────────────────────┘     └────────────────────────┘   │  │
│  │   ┌───────────────────────────────────────────────────────┐   │  │
│  │   │               Auth  ·  Cluster Registry               │   │  │
│  │   └───────────────────────────────────────────────────────┘   │  │
│  │   ┌───────────────────────────────────────────────────────┐   │  │
│  │   │                       Scheduler                       │   │  │
│  │   └───────────────────────────────────────────────────────┘   │  │
│  │   ┌───────────────────────────────────────────────────────┐   │  │
│  │   │  PromQL Engine · p99 Engine · Recommendation Engine   │   │  │
│  │   └───────────────────────────────────────────────────────┘   │  │
│  │   ┌────────────────────────┐     ┌────────────────────────┐   │  │
│  │   │        Database        │     │         Cache          │   │  │
│  │   └────────────────────────┘     └────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────────┘  │
└──────────────────┬──────────────────────────┬───────────────────────┘
        TLS+Token  │                          │  TLS+Token
          ┌────────┘                          └────────┐
          │                                            │
┌─────────┴──────────────┐              ┌─────────────┴──────────┐
│   WORKLOAD CLUSTER 1   │              │   WORKLOAD CLUSTER 2   │
│                        │              │                        │
│  ┌──────────────────┐  │              │  ┌──────────────────┐  │
│  │    Prometheus    │  │              │  │    Prometheus    │  │
│  └────────┬─────────┘  │              │  └────────┬─────────┘  │
│           │ scrapes    │              │           │ scrapes    │
│  ┌────────┴─────────┐  │              │  ┌────────┴─────────┐  │
│  │    cAdvisor      │  │              │  │    cAdvisor      │  │
│  │  kube-state-     │  │              │  │  kube-state-     │  │
│  │    metrics       │  │              │  │    metrics       │  │
│  └──────────────────┘  │              │  └──────────────────┘  │
│    Pods / Workloads    │              │    Pods / Workloads    │
└────────────────────────┘              └────────────────────────┘
```

---

## Hub Internals

```
┌─────────────────────────────────────────────────────────────────────┐
│                          PODOPTIX HUB                               │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                    PRESENTATION LAYER                         │  │
│  │                                                               │  │
│  │   ┌────────────────────────┐     ┌────────────────────────┐   │  │
│  │   │     Web Dashboard      │     │    REST API Server     │   │  │
│  │   │  · Recommendations     │     │  GET  /recommendations │   │  │
│  │   │  · Cluster management  │     │  POST /clusters        │   │  │
│  │   │  · Savings summary     │     │  GET  /clusters        │   │  │
│  │   └────────────────────────┘     └────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                     SERVICE LAYER                             │  │
│  │                                                               │  │
│  │   ┌───────────────────────────────────────────────────────┐   │  │
│  │   │  Auth Service      │  JWT-based user auth · API keys  │   │  │
│  │   │  Cluster Registry  │  Stores Prometheus URL + token   │   │  │
│  │   │  Scheduler         │  Cron-based · runs per cluster   │   │  │
│  │   └───────────────────────────────────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                   PROCESSING PIPELINE                         │  │
│  │                                                               │  │
│  │   ┌───────────────────────────────────────────────────────┐   │  │
│  │   │  PromQL Engine  ·  queries /api/v1/query_range        │   │  │
│  │   │  Metrics: container_cpu  ·  container_memory          │   │  │
│  │   └────────────────────────────┬──────────────────────────┘   │  │
│  │                                │                              │  │
│  │   ┌────────────────────────────▼──────────────────────────┐   │  │
│  │   │  p99 Computation Engine                               │   │  │
│  │   │  quantile(0.99, time_series)  ·  window: 7d           │   │  │
│  │   └────────────────────────────┬──────────────────────────┘   │  │
│  │                                │                              │  │
│  │   ┌────────────────────────────▼──────────────────────────┐   │  │
│  │   │  Recommendation Engine                                │   │  │
│  │   │  CPU = p99_cpu × 2   ·   Mem = p99_mem × 2            │   │  │
│  │   │  Output → YAML patch                                  │   │  │
│  │   └───────────────────────────────────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                      STORAGE LAYER                            │  │
│  │                                                               │  │
│  │   ┌────────────────────────┐     ┌────────────────────────┐   │  │
│  │   │        Database        │     │         Cache          │   │  │
│  │   │  · clusters            │     │  · PromQL results      │   │  │
│  │   │  · recommendations     │     │  · TTL: 1 hr           │   │  │
│  │   │                        │     │                        │   │  │
│  │   └────────────────────────┘     └────────────────────────┘   │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Data Flow

```
  Step 1   User registers a cluster
           Dashboard → POST /clusters { name, prometheus_url, token }
           Cluster Registry stores encrypted credentials in Database

  Step 2   Scheduler triggers a collection job
           Runs once per day per registered cluster

  Step 3   PromQL Engine queries Prometheus HTTP API
           GET /api/v1/query_range with the following metrics:
           · container_cpu_usage_seconds_total     (source: cAdvisor)
           · container_memory_working_set_bytes    (source: cAdvisor)
           · kube_pod_container_resource_limits    (source: kube-state-metrics)

  Step 4   p99 Computation Engine processes raw time series
           Computes 99th percentile over a rolling window (default: 7 days)

  Step 5   Recommendation Engine calculates new limits
           CPU  limit = p99_cpu × 2   (unit: millicores)
           Mem  limit = p99_mem × 2   (unit: MiB)

  Step 6   Recommendations UPSERTed — one row per container, updated in place
           Unique key: (cluster_id, namespace, pod_name, container_name)

  Step 7   Dashboard displays per namespace / per pod recommendations
           REST API serves YAML patches ready for kubectl apply
```

---

## Component Reference

| Component | Description |
|-----------|-------------|
| **Web Dashboard** | UI to view recommendations per cluster, namespace, and pod |
| **REST API Server** | HTTP server — CRUD for clusters, serve recommendations as JSON or YAML |
| **Auth Service** | User registration + login · bcrypt password hashing · JWT token issuance · middleware verifies all `/api/v1/*` requests |
| **Cluster Registry** | Stores Prometheus endpoint URLs and encrypted auth tokens |
| **Scheduler** | Cron-based job runner — triggers data collection per cluster on a schedule |
| **PromQL Engine** | Queries Prometheus `/api/v1/query_range` with PromQL expressions |
| **p99 Computation Engine** | Computes 99th percentile from raw time series over a rolling window |
| **Recommendation Engine** | Applies 2× multiplier · Formats output as YAML resource patches |
| **Database** | Persists cluster config and recommendations (one row per container, updated daily) |
| **Cache** | Caches PromQL query results per cluster to reduce Prometheus load |

---

## Metrics Collected

| Metric | Source | Used For |
|--------|--------|----------|
| `container_cpu_usage_seconds_total` | cAdvisor | CPU p99 computation |
| `container_memory_working_set_bytes` | cAdvisor | Memory p99 computation |
| `kube_pod_container_resource_limits` | kube-state-metrics | Current limit baseline comparison |
| `kube_pod_container_resource_requests` | kube-state-metrics | Current request baseline comparison |

---

## Security Model

| Concern | Approach |
|---------|----------|
| **Prometheus access** | Per-cluster auth token stored encrypted in Database |
| **Transit encryption** | TLS enforced on all Hub → Prometheus connections |
| **Dashboard auth** | JWT-based authentication for all dashboard users |
| **API access** | API key required for all REST API calls |
| **Secrets** | No credentials stored in plaintext — encrypted at rest |
