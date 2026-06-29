<div align="center">

<img src="./assets/banner.svg" alt="PodOptix" width="100%"/>

<br/>
<br/>
<br/>

[![License: MIT](https://img.shields.io/badge/License-MIT-FF6B00?style=for-the-badge)](LICENSE)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.24%2B-326CE5?style=for-the-badge&logo=kubernetes&logoColor=white)](https://kubernetes.io)
[![Prometheus](https://img.shields.io/badge/Prometheus-Compatible-E6522C?style=for-the-badge&logo=prometheus&logoColor=white)](https://prometheus.io)
[![Go](https://img.shields.io/badge/Built%20with-Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golang.org)

</div>

---

## The Problem

Most Kubernetes and infra teams set pod resource limits by **guesswork or copy-paste** — either picking numbers that "feel right" with no data, or copying limits from another workload that has nothing to do with theirs.

The result?

- Pods get OOMKilled at midnight
- Clusters are 40-60% over-provisioned
- Cloud bills keep growing with no visibility
- Engineers waste hours manually tuning limits with no real data
- Limits copied from one service cause cascading failures in another
- Teams are afraid to reduce limits because they don't know actual usage
- Finance has no insight into which team or service is burning the most cost
- No single view across multiple clusters to understand total waste
- The problem gets worse as the number of microservices grows

---

## The Solution

PodOptix connects to your Prometheus, analyzes **real usage patterns**, and recommends limits at **2× the p99 percentile** — the engineering sweet spot between reliability and cost.

```
Actual Usage (p99)  →  × 2  →  Recommended Limit
      120m CPU                      240m CPU
      180Mi RAM                     360Mi RAM
```

No more guessing. No more waste.

---

## Architecture

PodOptix runs as a single **Master Hub** — deployed once in your management or ops cluster. No agents. No sidecars. Nothing to deploy inside your workload clusters.

From there, the Hub connects directly to each cluster's Prometheus HTTP API, runs PromQL queries to fetch real usage data, computes p99 percentiles, and generates recommendations — all from one place.

```
┌─────────────────────────────────────────────────────────────┐
│                         HUB                                 │
│       Master Control Plane · Dashboard · REST API           │
│        Queries p99 · Generates Recommendations              │
└──────────┬──────────────────┬──────────────────┬────────────┘
           │                  │                  │
      (PromQL API)       (PromQL API)       (PromQL API)
           │                  │                  │
    ┌──────┴──────┐    ┌──────┴──────┐    ┌──────┴──────┐
    │ Prometheus  │    │ Prometheus  │    │ Prometheus  │
    │  Cluster 1  │    │  Cluster 2  │    │  Cluster 3  │
    └─────────────┘    └─────────────┘    └─────────────┘
```

| Component | Role |
|-----------|------|
| **Hub** | Connects to each cluster's Prometheus · Runs PromQL queries · Computes p99 · Generates recommendations · Serves the dashboard |

**Onboarding a cluster takes 30 seconds** — just register the Prometheus endpoint and an auth token. That's it.

> For a deep dive into Hub internals, data flow, component reference, metrics collected, and the security model — see [docs/architecture.md](docs/architecture.md).

---

## Quick Start

```bash
# Deploy PodOptix Hub in your management / ops cluster
helm repo add podoptix https://charts.podoptix.io
helm repo update

helm install podoptix podoptix/hub \
  --namespace podoptix \
  --create-namespace \
  --set db.url=<postgresql-url> \
  --set redis.url=<redis-url>
```

That's it. Register your workload clusters via the dashboard and recommendations start appearing within 24 hours.

---

## Roadmap

- [x] Architecture design
- [x] Technology decisions and trade-offs
- [x] Project structure and scaffold
- [x] Data models (Cluster, Recommendation)
- [x] Config loader (environment variables)
- [x] Database schema sync (PostgreSQL)
- [x] Database store layer with connection pool
- [x] HTTP server (Gin)
- [x] REST API endpoints
- [x] Automated integration tests
- [x] Auth (JWT)
- [ ] Prometheus metrics collector (Hub → PromQL API)
- [ ] p99 computation engine
- [ ] Recommendation engine
- [ ] Scheduler (cron-based collection jobs)
- [ ] Cache layer (Redis)
- [ ] Token encryption at rest (AES-256)
- [ ] Central Hub with multi-cluster support
- [ ] Web Dashboard
- [ ] Docker image
- [ ] Helm chart

---

## Contributing

PodOptix is in early development. PRs, issues, and ideas are welcome.

---

<div align="center">
<b>For every platform engineer who got paged at midnight because someone set a memory limit by guesswork.</b>
</div>
