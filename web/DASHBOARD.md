# PodOptix Dashboard

Web dashboard for PodOptix — React + TypeScript + Vite + Tailwind.

## Development

```bash
# 1. Install dependencies (once)
npm install

# 2. Make sure the Go backend is running (from project root)
#    export $(cat .env | xargs) && go run ./cmd/hub

# 3. Start the Vite dev server
npm run dev
```

Open <http://localhost:5173> — the dashboard proxies API calls to the Go backend on `:8080` automatically.

## Production build

```bash
npm run build
```

Outputs directly to `../internal/dashboard/dist/`. Go's `//go:embed` picks it up on the next `go build` and bakes everything into a single binary.

**Full single-artifact build** (dashboard + backend in one binary at `bin/podoptix`):

```bash
make build       # from project root
```

Run the binary — it serves the dashboard on `/` and the API on `/api/v1/*`, both on the same origin:

```bash
./bin/podoptix
open http://localhost:8080
```

## Structure

```
web/
├── src/
│   ├── pages/           Login · Register · Clusters · RegisterCluster · EditCluster · ClusterDetail · Recommendations (cross-cluster) · Savings (reclaimable CPU/mem, adoption, breakdowns)
│   ├── components/      Layout (sidebar — 3 nav items: Clusters, Recommendations, Savings) · ProtectedRoute · StatusPill
│   ├── lib/
│   │   ├── api.ts       Typed fetch wrapper — attaches JWT, throws on non-2xx
│   │   └── auth.ts      JWT storage in localStorage
│   ├── App.tsx          React Router setup
│   ├── main.tsx         React entry point
│   └── index.css        Tailwind + global styles
├── tailwind.config.js   Grafana-style dark palette
├── vite.config.ts       Dev server + API proxy to Go backend
└── package.json
```

