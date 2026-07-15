# PodOptix — High Level Design

## Table of Contents

- [What is PodOptix](#what-is-podoptix)
- [System Overview](#system-overview)
- [Component Overview](#component-overview)
- [Multi-Cluster Architecture](#multi-cluster-architecture)
- [High-Level Data Flow](#high-level-data-flow)
- [Scheduler Flow](#scheduler-flow)
- [Metrics Collected](#metrics-collected)

---

## What is PodOptix

PodOptix is a Kubernetes resource right-sizing tool. It analyzes real CPU and memory usage across your workload clusters and generates actionable recommendations for resource limits — powered by p99 percentile computation.

**The problem it solves:**

Most Kubernetes teams set resource limits by guessing or copying from similar services. This leads to one of two failure modes:

1. **Over-provisioning** — limits set too high. You pay for CPU and memory that is never used. At scale (hundreds of pods), this waste compounds quickly.
2. **Under-provisioning** — limits set too low. Pods get OOMKilled or CPU-throttled under load. Incidents happen.

PodOptix queries historical usage data from Prometheus, computes the 99th percentile over a rolling 7-day window, and applies a 2× safety multiplier to produce a right-sized limit recommendation. No guesswork. No manual analysis.

**Key design principle:** No agents. No sidecars. Nothing to deploy inside workload clusters. PodOptix runs as a single Hub in your management cluster and queries each workload cluster's existing Prometheus directly.

---

## System Overview

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
│  │    metrics       │  │              │  │  kube-state-     │  │
│  └──────────────────┘  │              │  └──────────────────┘  │
│    Pods / Workloads    │              │    Pods / Workloads    │
└────────────────────────┘              └────────────────────────┘
```

---

## Component Overview

| Component | Layer | Description |
|-----------|-------|-------------|
| **Web Dashboard** | Presentation | UI to view recommendations per cluster, namespace, and pod |
| **REST API Server** | Presentation | HTTP server — CRUD for clusters, serve recommendations as JSON or YAML |
| **Auth Service** | Service | User registration + login · bcrypt password hashing · JWT token issuance · middleware verifies all `/api/v1/*` requests |
| **Cluster Registry** | Service | Stores Prometheus endpoint URLs and encrypted auth tokens |
| **Scheduler** | Service | Cron-based job runner — triggers data collection per cluster once per day |
| **PromQL Engine** | Processing | Queries Prometheus `/api/v1/query_range` with PromQL expressions |
| **p99 Computation Engine** | Processing | Computes 99th percentile from raw time series over a rolling 7-day window |
| **Recommendation Engine** | Processing | Applies 2× multiplier · Formats output as YAML resource patches |
| **Database (PostgreSQL)** | Storage | Persists cluster config and recommendations — one row per container, updated daily |
| **Cache (Redis)** | Storage | Caches PromQL query results per cluster — TTL: 1 hour |

---

## Multi-Cluster Architecture

PodOptix uses a **Hub and Spoke** model:

- **Hub** — single PodOptix instance deployed in a management or ops cluster. It holds all state, runs all computation, and serves the dashboard.
- **Spokes** — workload clusters, each with their own Prometheus. The Hub queries these remotely. No PodOptix components are installed in spoke clusters.

**How clusters are registered:**

A user registers a workload cluster by providing:
- A human-readable name
- The Prometheus HTTP endpoint URL
- An auth token for that Prometheus

The Hub stores the token encrypted at rest (AES-256-GCM) and begins scheduling daily collection jobs for that cluster.

**Cluster status values:**

| Status | Meaning |
|--------|---------|
| `pending` | Registered but not yet queried |
| `healthy` | Last collection job succeeded |
| `unhealthy` | Last collection job failed (Prometheus unreachable, auth error, etc.) |

---

## High-Level Data Flow

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

## Scheduler Flow

The Scheduler is a cron-based job runner. It runs once per day and iterates over every registered cluster:

```
Scheduler (cron: once/day)
        │
        ├── For each cluster in Cluster Registry
        │         │
        │         ├── Decrypt Prometheus token (AES-256-GCM)
        │         │
        │         ├── PromQL Engine → query_range (CPU + Memory, 7-day window)
        │         │         │
        │         │         └── Cache result in Redis (TTL: 1 hr)
        │         │
        │         ├── p99 Computation Engine → quantile(0.99, time_series)
        │         │
        │         ├── Recommendation Engine → p99 × 2
        │         │
        │         └── UPSERT into recommendations table
        │
        └── Done — next run in 24 hours
```

**Two triggers for recalculation:**
1. **Automatic** — scheduler runs once per day for all clusters
2. **Manual** — "Recalculate" button in the dashboard triggers on-demand refresh via a Redis job queue

**Job queue rationale:** 100 clusters × 1000 pods = 100,000 containers to process simultaneously would cause 200 concurrent Prometheus HTTP calls and 100,000 DB upserts — likely crashing the server. The job queue ensures one cluster is processed at a time. The user gets an immediate "queued" response.

---

## Metrics Collected

| Metric | Source | Used For |
|--------|--------|----------|
| `container_cpu_usage_seconds_total` | cAdvisor | CPU p99 computation |
| `container_memory_working_set_bytes` | cAdvisor | Memory p99 computation |
| `kube_pod_container_resource_limits` | kube-state-metrics | Current limit baseline comparison |
| `kube_pod_container_resource_requests` | kube-state-metrics | Current request baseline comparison |

**PromQL queries used:**

```promql
rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m]) * 1000
```
`rate()` computes per-second rate to smooth spikes. `* 1000` converts cores to millicores. Filters exclude infrastructure and pause containers.

```promql
container_memory_working_set_bytes{container!="",container!="POD"} / 1048576
```
`working_set` is actual memory in use (not cached). `/ 1048576` converts bytes to MiB.
