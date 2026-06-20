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

Most Kubernetes and infra teams set pod resource limits by **guesswork or copy-paste**.

The result?

- Pods get OOMKilled at 3AM
- Clusters are 40-60% over-provisioned
- Cloud bills keep growing with no visibility

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
└──────────┬──────────────────┬──────────────────┬───────────┘
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

---

## Quick Start

```bash
# Install the PodOptix agent in your cluster
helm repo add podoptix https://charts.podoptix.io
helm repo update

helm install podoptix podoptix/agent \
  --namespace podoptix \
  --create-namespace \
  --set hub.url=<your-hub-url> \
  --set hub.token=<your-token>
```

That's it. Your cluster starts sending recommendations within minutes.

---

## Roadmap

- [x] Architecture design
- [ ] Prometheus metrics collector (Hub → PromQL API)
- [ ] p99 computation engine
- [ ] Recommendation API
- [ ] Central Hub with multi-cluster support
- [ ] Web Dashboard
- [ ] Helm chart
- [ ] Slack / PagerDuty alerts for limit drift

---

## Contributing

PodOptix is in early development. PRs, issues, and ideas are welcome.

---

<div align="center">
Built with passion for platform engineers who are tired of paying for wasted compute.
</div>
