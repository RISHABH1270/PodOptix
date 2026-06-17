# PodOptix

> Kubernetes resource right-sizing — optimize pod CPU & memory limits based on real p99 usage data.

---

## What is PodOptix?

Most Kubernetes teams set pod resource limits by guesswork or copy-paste. PodOptix analyzes actual Prometheus metrics and recommends limits at **2x the p99 percentile** of observed usage — making clusters cost-effective without sacrificing reliability.

## Architecture

Hub & Spoke model:
- **Agent** — runs inside each cluster, reads from Prometheus, computes recommendations
- **Hub** — central aggregator with a single dashboard across all clusters

## Quick Start

```bash
helm install podoptix podoptix/podoptix \
  --set hub.token=<your-token> \
  --set hub.url=<hub-url>
```

---

*Documentation and architecture decisions are tracked in `/docs`.*
