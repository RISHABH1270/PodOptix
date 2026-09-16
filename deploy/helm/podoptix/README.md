# PodOptix — Helm Chart

Deploy PodOptix on any Kubernetes cluster with a single `helm install`.

## What it deploys

```
┌─── Deployment/podoptix ──────────┐    stateless — replicas: 1 by default,
│  (1..N replicas)                 │    can scale to N without changing the DB
└──────────┬───────────────────────┘
           │  via K8s Services
           ├─→ Service/podoptix-postgres → StatefulSet/postgres (1) + PVC 10Gi
           └─→ Service/podoptix-redis    → Deployment/redis (1) + emptyDir
```

- **PodOptix Deployment** — the Go binary + embedded React dashboard, port `:8080`
- **PostgreSQL StatefulSet** — 1 replica with a PersistentVolumeClaim (data survives pod restarts)
- **Redis Deployment** — 1 replica with `emptyDir` (cache only — safe to lose)
- **Secret** — auto-generated Postgres password, JWT secret, encryption key

## Prerequisites

- Kubernetes 1.24+
- Helm 3
- A default `StorageClass` in your cluster (for the Postgres PVC)

## Quick install (from the OCI registry — no clone needed)

```bash
helm install podoptix oci://ghcr.io/rishabh1270/charts/podoptix \
  --version 0.1.0 \
  -n podoptix --create-namespace
```

That single command:
1. Pulls the chart from ghcr.io
2. Creates the namespace
3. Deploys the 3 workloads (PodOptix Deployment + Postgres StatefulSet + Redis Deployment)

Follow the on-screen NOTES to access the dashboard.

### Alternative — install from a local clone

```bash
git clone https://github.com/RISHABH1270/PodOptix.git
helm install podoptix ./PodOptix/deploy/helm/podoptix
```

## Production install

Pass a values file with real secrets and a public service:

```bash
helm install podoptix ./deploy/helm/podoptix \
  --namespace podoptix --create-namespace \
  --set image.tag=v0.1.0 \
  --set postgres.password=<a-strong-random-password> \
  --set podoptix.jwtSecret=<a-32-plus-char-random> \
  --set podoptix.encryptionKey=<exactly-32-bytes> \
  --set service.type=LoadBalancer
```

## Common overrides

| Key | Default | Purpose |
|-----|---------|---------|
| `image.repository` | `ghcr.io/rishabh1270/podoptix` | Your container image |
| `image.tag` | `0.1.0` | Image tag |
| `podoptix.replicaCount` | `1` | Number of PodOptix pods (stateless — scale freely) |
| `postgres.storage.size` | `10Gi` | Postgres PVC size |
| `postgres.storage.storageClass` | `""` (cluster default) | Explicit StorageClass |
| `service.type` | `ClusterIP` | Set to `LoadBalancer` for public access |
| `service.port` | `8080` | External port |
| `service.annotations` | `{}` | e.g. AWS NLB tuning |

## Public access

**Option 1 — `LoadBalancer`** (production, cloud K8s):
```bash
helm upgrade podoptix ./deploy/helm/podoptix --set service.type=LoadBalancer
kubectl get svc podoptix --watch     # wait for EXTERNAL-IP
```

**Option 2 — `NodePort`** (bare-metal, dev):
```bash
helm upgrade podoptix ./deploy/helm/podoptix --set service.type=NodePort
```

**Option 3 — `kubectl port-forward`** (local access, no changes):
```bash
kubectl port-forward svc/podoptix 8080:8080
```

## Uninstall

```bash
helm uninstall podoptix
```

**Note:** The Postgres PVC is NOT deleted automatically (data protection). Delete manually if you're sure:
```bash
kubectl delete pvc -l app.kubernetes.io/name=podoptix
```

## Upgrade

```bash
helm upgrade podoptix ./deploy/helm/podoptix --set image.tag=v0.2.0
```

Zero-downtime — `maxUnavailable: 0` in the Deployment ensures at least one PodOptix pod always serves traffic during rollout. Postgres and Redis are untouched.

## Verify the chart before installing

```bash
# Render templates locally
helm template podoptix ./deploy/helm/podoptix

# Lint for syntax issues
helm lint ./deploy/helm/podoptix

# Dry run against a real cluster (no changes made)
helm install podoptix ./deploy/helm/podoptix --dry-run --debug
```
