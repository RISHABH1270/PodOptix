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

Outputs to `dist/`. In production, the Go binary will `//go:embed` this folder and serve it directly (single-binary deployment — no separate frontend server needed).

## Structure

```
web/
├── src/
│   ├── pages/           Login · Register · Clusters · RegisterCluster · ClusterDetail
│   ├── components/      Layout (sidebar) · ProtectedRoute · StatusPill
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

## Design language

Grafana-style dark data dashboard:
- Background `#0b0f14` with subtle dot grid
- Cards `#141a20` with soft shadows
- Accent green `#22c55e` (health/success)
- Status pills (ok / warn / danger / info)
- Monospace numbers for metrics
- Inter font for text, JetBrains Mono for values
