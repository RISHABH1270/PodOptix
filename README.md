<div align="center">

<img src="./assets/banner.svg" alt="PodOptix" width="100%"/>

<br/>
<br/>

[![License: MIT](https://img.shields.io/badge/License-MIT-F59E0B?style=for-the-badge)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26.4-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golang.org)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.24%2B-326CE5?style=for-the-badge&logo=kubernetes&logoColor=white)](https://kubernetes.io)
[![Prometheus](https://img.shields.io/badge/Prometheus-Compatible-E6522C?style=for-the-badge&logo=prometheus&logoColor=white)](https://prometheus.io)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?style=for-the-badge&logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D?style=for-the-badge&logo=redis&logoColor=white)](https://redis.io)
[![JWT](https://img.shields.io/badge/Auth-JWT-F59E0B?style=for-the-badge&logo=jsonwebtokens&logoColor=white)](https://jwt.io)

</div>

---

## The Problem

**3:12 AM. PagerDuty fires. The on-call engineer gets paged. `payment-api` is OOMKilled in production.**

The root cause? Someone copied resource limits from an unrelated service six months ago. The fix takes 2 minutes. Finding it took 40. It will happen again.

This is what happens when 150 containers across 50 microservices all have limits set by guesswork:

| Symptom | Reality |
|---------|---------|
| Pods OOMKilled at midnight | Limits set too low with no data |
| Cloud bill up 40-60% | Limits set too high — paying for unused capacity |
| Engineers afraid to reduce limits | Nobody knows actual usage |
| Cascading failures | Limits copied from unrelated workloads |
| Finance blind to cost drivers | No per-service visibility across clusters |

**Every new microservice makes it worse. The problem compounds.**

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

PodOptix runs as a single **Master Hub** deployed in your management or ops Kubernetes cluster. No agents. No sidecars. Nothing to deploy inside your workload clusters.

The Hub connects directly to each cluster's Prometheus HTTP API, queries p99 metrics, and generates recommendations — all from one place.

```
┌─────────────────────────────────────────────────────────────┐
│                         HUB                                 │
│       Master Control Plane · Dashboard · REST API           │
│        Queries p99 · Generates Recommendations              │
└──────────┬──────────────────┬──────────────────┬────────────┘
           │ PromQL API       │ PromQL API       │ PromQL API
    ┌──────┴──────┐    ┌──────┴──────┐    ┌──────┴──────┐
    │ Prometheus  │    │ Prometheus  │    │ Prometheus  │
    │  Cluster 1  │    │  Cluster 2  │    │  Cluster 3  │
    └─────────────┘    └─────────────┘    └─────────────┘
```

Register a cluster with its Prometheus URL + auth token. Recommendations are generated on startup and refreshed every 24 hours.

---

## Quick Start

Deploy PodOptix Hub in your management or ops Kubernetes cluster:

```bash
helm repo add podoptix https://charts.podoptix.io
helm repo update

helm install podoptix podoptix/hub \
  --namespace podoptix \
  --create-namespace \
  --set secrets.databaseURL="postgres://..." \
  --set secrets.redisURL="redis://..." \
  --set secrets.jwtSecret="your-secret" \
  --set secrets.encryptionKey="your-32-byte-key"
```

Once deployed, open the PodOptix dashboard at `http://<your-hub-ip>:8080` and register your first cluster.

---

## API

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| `POST` | `/auth/register` | — | Create a user account |
| `POST` | `/auth/login` | — | Login and receive JWT token |
| `GET` | `/healthz` | — | Liveness probe |
| `GET` | `/readyz` | — | Readiness probe (checks DB + Redis) |
| `GET` | `/api/v1/clusters` | JWT | List all clusters |
| `POST` | `/api/v1/clusters` | JWT | Register a cluster |
| `GET` | `/api/v1/clusters/:id` | JWT | Get cluster by ID |
| `PUT` | `/api/v1/clusters/:id` | JWT | Update cluster details |
| `DELETE` | `/api/v1/clusters/:id` | JWT | Remove a cluster |
| `GET` | `/api/v1/clusters/:id/recommendations` | JWT | Get recommendations (cached) |
| `POST` | `/api/v1/clusters/:id/recalculate` | JWT | Trigger manual recalculation |

---

## Documentation

| Doc | Description |
|-----|-------------|
| [HLD](docs/hld.md) | High Level Design — system overview, architecture, data flow |
| [LLD](docs/lld.md) | Low Level Design — DB schema, API contract, Redis design, security model |
| [Engineering Trade-offs](docs/engineering-trade-offs.md) | Every technical decision with full reasoning |
| [Dev Setup](docs/dev-setup.md) | How to run locally in 5 minutes |

---

## Roadmap

- [x] Architecture design and documentation
- [x] Data models (Cluster, Recommendation, User)
- [x] Config loader (environment variables)
- [x] PostgreSQL — migrations, store layer, connection pool
- [x] HTTP server (Gin) with middleware
- [x] REST API — full CRUD for clusters
- [x] Auth — JWT + bcrypt password hashing
- [x] Token encryption at rest (AES-256-GCM)
- [x] Prometheus metrics collector (PromQL API)
- [x] p99 computation engine (100% test coverage)
- [x] Recommendation engine
- [x] Scheduler — daily collection pipeline
- [x] Redis — recommendations cache + distributed lock
- [x] Integration tests — automated, real PostgreSQL
- [x] Readiness probe (/readyz)
- [ ] Web Dashboard
- [ ] Docker image
- [ ] Helm chart

---

## Contributing

PodOptix is in active development. PRs, issues, and ideas are welcome.

---

<div align="center">
<b>For every on-call platform engineer who spent their night debugging a crisis that a correct memory limit would have prevented.</b>
</div>
