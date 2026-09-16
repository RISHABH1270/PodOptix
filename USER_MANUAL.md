# PodOptix — User Manual

Everything an end user needs to install PodOptix, register their first cluster, and start applying recommendations.

> Looking to develop or contribute? See [docs/dev-setup.md](docs/dev-setup.md) instead.

---

## What is PodOptix

A single Hub that connects to your workload clusters' Prometheus, computes p99 CPU/memory usage over a rolling window (7d, 10d, or 30d), and recommends resource limits at `ceil(p99 × 2)`. No agents, no sidecars — one Hub queries every cluster's Prometheus remotely.

---

## Requirements

| Requirement | Minimum |
|-------------|---------|
| Kubernetes (for Helm install) | 1.24+ |
| Helm | 3.8+ (OCI support required) |
| Access to `ghcr.io` from the cluster | Read-only pull |
| A Prometheus endpoint per cluster you want to analyze | Reachable from the Hub with a bearer token |
| `kube-state-metrics` scraped by Prometheus | Optional — needed to see current limits alongside recommendations |

---

## Installation

### Option 1 — Kubernetes via Helm (recommended)

Single command deploys PodOptix + Postgres + Redis:

```bash
helm install podoptix oci://ghcr.io/rishabh1270/charts/podoptix \
  --version 0.1.0 \
  --namespace podoptix --create-namespace \
  --set service.type=LoadBalancer
```

Wait for the LoadBalancer IP:

```bash
kubectl get svc podoptix -n podoptix --watch
# NAME       TYPE           EXTERNAL-IP    PORT(S)
# podoptix   LoadBalancer   34.120.55.213  8080:32345/TCP
```

Open `http://<EXTERNAL-IP>:8080`.

**No LoadBalancer available?** Use port-forward instead:
```bash
kubectl port-forward -n podoptix svc/podoptix 8080:8080
# open http://localhost:8080
```

### Option 2 — Docker Compose (single VM)

For a single machine without Kubernetes:

```bash
git clone https://github.com/RISHABH1270/PodOptix.git
cd PodOptix
cp .env.example .env
docker compose up -d           # PostgreSQL + Redis
docker run -d --name podoptix -p 8080:8080 \
  -e DATABASE_URL='postgres://postgres:password@host.docker.internal:5432/podoptix?sslmode=disable' \
  -e REDIS_URL='redis://host.docker.internal:6379' \
  -e JWT_SECRET='change-me-to-a-long-random-secret' \
  -e ENCRYPTION_KEY='change-me-to-32-byte-random-key!' \
  ghcr.io/rishabh1270/podoptix:0.1.0
```

Open `http://localhost:8080`.

### Option 3 — Standalone binary

Download the binary from a [Release](https://github.com/RISHABH1270/PodOptix/releases), export env vars, run it. See [docs/dev-setup.md](docs/dev-setup.md) for details.

---

## First-time setup

### 1. Register a user

Open the dashboard → click **Create one** on the login screen → enter email + password (min 8 chars).

You're auto-logged in and land on the **Clusters** page.

### 2. Register your first cluster

Click **+ Register cluster** and fill in:

| Field | What to enter |
|-------|---------------|
| Cluster name | A friendly label, e.g. `production-us-east` |
| Prometheus URL | Full URL — must start with `http://` or `https://` |
| Prometheus token | Bearer token (encrypted at rest with AES-256-GCM) |
| Lookback window | `7d` (default), `10d`, or `30d` |

Click **Register cluster**. Behind the scenes:
1. PodOptix pings the URL immediately (10s timeout)
2. Status is set to `connected` or `disconnected` — never pending
3. If connected, a background sync fires — first recommendations appear within a minute

### 3. Wait for recommendations

- **`connected` + first sync running** → recommendations appear as they're computed
- **`disconnected`** → check the URL and token, then click **Edit** to fix

### 4. Reviewing recommendations

The cluster detail page shows a table with:

| Column | What it means |
|--------|---------------|
| Namespace / Pod / Container | The workload |
| Status | `ready` = has data · `new_service` = not enough history yet |
| Current CPU / Mem | What's set today, from `kube_pod_container_resource_limits` |
| Recommended CPU / Mem | `ceil(p99 × 2)` — the engineering sweet spot |
| ↓% / ↑% | How much smaller/bigger the recommendation is vs current |
| Applied | Toggle when you apply the change to your cluster |

### 5. Applying a recommendation

PodOptix does NOT apply changes automatically — you decide. To apply:

1. Note the recommended values
2. Update your Deployment/StatefulSet manifest, e.g.:
   ```yaml
   resources:
     limits:
       cpu:    "500m"    # was 2000m
       memory: "512Mi"   # was 2Gi
   ```
3. `kubectl apply` the change
4. In PodOptix, toggle the **Applied** checkbox on that row — used later for savings reports

### 6. Triggering a fresh scan

Click **▶ Recalculate** at the top of the cluster detail page. Behind the scenes:
- Distributed Redis lock prevents duplicate runs
- Same collect → compute → recommend → upsert pipeline as the scheduler
- Status updates in real time via polling

If **Recalculate** is disabled → cluster is currently `disconnected`. Fix connectivity first.

---

## Configuration

All configuration is via environment variables (for direct/binary/Docker) or Helm values (for K8s).

### Required

| Env var | Helm value | Notes |
|---------|-----------|-------|
| `DATABASE_URL` | (auto-derived) | `postgres://user:pass@host:5432/podoptix?sslmode=disable` |
| `REDIS_URL` | (auto-derived) | `redis://host:6379` |
| `JWT_SECRET` | `podoptix.jwtSecret` | ≥32 random chars |
| `ENCRYPTION_KEY` | `podoptix.encryptionKey` | Exactly 32 bytes — encrypts Prometheus tokens at rest |

### Optional

| Env var | Default | Purpose |
|---------|---------|---------|
| `PORT` | `8080` | HTTP port |

### Helm-only

| Value | Default | Purpose |
|-------|---------|---------|
| `image.tag` | `0.1.0` | Container image version |
| `podoptix.replicaCount` | `1` | Scale to N — PodOptix is stateless |
| `postgres.storage.size` | `10Gi` | PVC size for Postgres data |
| `service.type` | `ClusterIP` | Set to `LoadBalancer` for public access |
| `service.port` | `8080` | External port |
| `service.annotations` | `{}` | e.g. AWS NLB / GCP LB tuning |

Full list: `helm show values oci://ghcr.io/rishabh1270/charts/podoptix --version 0.1.0`

---

## Upgrading

### Kubernetes (Helm)

```bash
helm upgrade podoptix oci://ghcr.io/rishabh1270/charts/podoptix \
  --version 0.2.0 \
  -n podoptix
```

Zero-downtime rollout — the chart uses `maxUnavailable: 0`, so at least one PodOptix pod always serves traffic during the upgrade. Postgres and Redis are untouched.

### Docker

```bash
docker pull ghcr.io/rishabh1270/podoptix:0.2.0
docker stop podoptix && docker rm podoptix
docker run -d --name podoptix ...  # same command as install, new tag
```

---

## Uninstalling

### Kubernetes

```bash
helm uninstall podoptix -n podoptix
```

**Important:** The Postgres PVC is NOT deleted automatically (data protection). Delete manually if you're sure:

```bash
kubectl delete pvc -l app.kubernetes.io/name=podoptix -n podoptix
kubectl delete namespace podoptix
```

### Docker

```bash
docker stop podoptix && docker rm podoptix
docker compose down -v          # -v also wipes Postgres and Redis data
```

---

## Troubleshooting

### Cluster shows `disconnected` immediately after registration

- Check the URL is reachable from PodOptix. From inside a K8s cluster, the URL must be internally resolvable (use the service DNS, not a bookmark like `localhost:9090`).
- Check the token — try `curl -H "Authorization: Bearer <token>" https://prometheus-url/api/v1/query?query=up`
- Check TLS. If self-signed cert, terminate TLS at your ingress in front of Prometheus.

### `Recalculate` button is greyed out

The cluster is `disconnected`. Click **Edit**, verify URL + token, save. PodOptix re-pings on save and updates the status.

### No recommendations appear after sync

- Prometheus must have `container_cpu_usage_seconds_total` and `container_memory_working_set_bytes` (from cAdvisor). Most K8s Prometheus installs do.
- For "current limit" columns to populate, `kube-state-metrics` must be scraped by Prometheus and expose `kube_pod_container_resource_limits`.
- `new_service` status = the pod exists but has no metric history yet. Wait for one full lookback window (7d default).

### Login fails after upgrade

Auto-generated JWT secret changed. In production, always set `podoptix.jwtSecret` explicitly:

```bash
helm upgrade podoptix ... --set podoptix.jwtSecret=<your-32-plus-char-secret>
```

### Database connection errors during pod start

PodOptix waits for Postgres/Redis via init containers, but on a very slow first boot, the pod may restart 1–2 times. This is expected. If it continues:

```bash
kubectl logs -n podoptix statefulset/podoptix-postgres
kubectl describe pod -n podoptix -l app.kubernetes.io/component=postgres
```

Common causes: no default StorageClass, PVC provisioning failed, resource limits too tight.

---

## FAQ

**Q: Do I need to install anything in my workload clusters?**
No. PodOptix runs as a single Hub in your management cluster and queries each workload cluster's existing Prometheus remotely.

**Q: How often do recommendations refresh?**
Automatically every 24 hours per cluster. Also once on startup, once when a cluster is registered, and on-demand via **Recalculate**.

**Q: Why p99 × 2?**
The p99 covers 99% of real traffic and ignores freak spikes. Doubling gives headroom for growth and unforeseen bursts without the overhead of provisioning for the peak of the peak. It's the engineering sweet spot between reliability (OOMKill avoidance) and cost.

**Q: Where are Prometheus tokens stored?**
Encrypted at rest with AES-256-GCM before being written to Postgres. The `ENCRYPTION_KEY` env var is the master key — losing it means all stored tokens become undecryptable. Never rotate mid-deployment.

**Q: Can I scale PodOptix to multiple replicas?**
Yes. PodOptix is stateless — set `podoptix.replicaCount: 3` in Helm. All replicas share the same Postgres + Redis. Distributed Redis lock prevents duplicate recalculate runs.

**Q: Does PodOptix apply changes automatically?**
No. It only recommends. You decide when and where to `kubectl apply`. This is intentional — the human stays in the loop.

**Q: What Kubernetes versions are supported?**
Tested on 1.24+. Should work on older versions but not tested.

**Q: What Prometheus versions are supported?**
Any version supporting `/api/v1/query_range`. Tested on Prometheus 2.x.

---

## Getting help

- Bugs / feature requests: <https://github.com/RISHABH1270/PodOptix/issues>
- Source code: <https://github.com/RISHABH1270/PodOptix>
- Design docs:
  - [High Level Design](docs/hld.md)
  - [Low Level Design](docs/lld.md)
  - [Engineering Trade-offs](docs/engineering-trade-offs.md)

---

<div align="center">
<sub>For every platform engineer who got paged at midnight because someone set a memory limit by guesswork.</sub>
</div>
