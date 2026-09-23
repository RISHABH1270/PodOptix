import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { RefreshCw, Server, TrendingDown, CheckCircle2, Zap, Database } from 'lucide-react'
import { api, RecommendationWithCluster } from '../lib/api'

export function SavingsPage() {
  const [rows, setRows]       = useState<RecommendationWithCluster[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError]     = useState<string | null>(null)

  const load = async () => {
    setLoading(true); setError(null)
    try { setRows(await api.listAllRecommendations()) }
    catch (err: any) { setError(err.message ?? 'Failed to load recommendations') }
    finally { setLoading(false) }
  }
  useEffect(() => { load() }, [])

  // ── aggregation — pure client-side math over the flat recommendation list ──
  const insights = useMemo(() => calcInsights(rows), [rows])

  return (
    <div className="p-8 max-w-7xl mx-auto">
      {/* header */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <div className="text-[10px] uppercase tracking-widest text-accent font-semibold mb-1">Savings</div>
          <h1 className="text-2xl font-bold text-ink">Resource savings overview</h1>
          <p className="text-sm text-muted mt-1">How much CPU and memory PodOptix has surfaced you can reclaim across every cluster.</p>
        </div>
        <button
          onClick={load}
          className="p-2 rounded-md bg-surface border border-border text-muted hover:text-ink hover:border-borderHi transition"
          title="Refresh"
        >
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {error && (
        <div className="px-4 py-3 rounded-md bg-dangerBg border border-danger/30 text-danger text-sm mb-4">{error}</div>
      )}

      {loading && rows.length === 0 ? (
        <div className="text-center py-16 text-dim text-sm">Loading…</div>
      ) : rows.length === 0 ? (
        <EmptyState />
      ) : (
        <>
          {/* ── hero: potential + realized ─────────────────────────── */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
            <HeroCard
              icon={<TrendingDown className="w-4 h-4 text-warn" />}
              label="Potential CPU saved"
              value={formatCores(insights.potentialCPU)}
              unit="cores"
              accent="warn"
            />
            <HeroCard
              icon={<TrendingDown className="w-4 h-4 text-warn" />}
              label="Potential memory saved"
              value={formatGiB(insights.potentialMem)}
              unit="GiB"
              accent="warn"
            />
            <HeroCard
              icon={<CheckCircle2 className="w-4 h-4 text-ok" />}
              label="Realized CPU saved"
              value={formatCores(insights.realizedCPU)}
              unit="cores"
              accent="ok"
            />
            <HeroCard
              icon={<CheckCircle2 className="w-4 h-4 text-ok" />}
              label="Realized memory saved"
              value={formatGiB(insights.realizedMem)}
              unit="GiB"
              accent="ok"
            />
          </div>

          {/* ── adoption progress ──────────────────────────────────── */}
          <div className="bg-surface border border-border rounded-lg p-5 shadow-card mb-6">
            <div className="flex items-baseline justify-between mb-2">
              <div>
                <div className="text-[10px] uppercase tracking-widest text-dim font-semibold">Adoption progress</div>
                <div className="text-sm text-muted mt-1">{insights.applied} of {insights.ready} recommendations applied</div>
              </div>
              <div className="text-2xl font-mono font-semibold text-accent">{insights.adoptionPct}%</div>
            </div>
            <div className="w-full h-2 rounded-full bg-elevated overflow-hidden">
              <div className="h-full bg-accent transition-all duration-500" style={{ width: `${insights.adoptionPct}%` }} />
            </div>
          </div>

          {/* ── top waste (two columns: CPU, memory) ───────────────── */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
            <TopWasteCard
              title="Top 10 — biggest CPU waste"
              icon={<Zap className="w-4 h-4 text-warn" />}
              items={insights.topCPU}
              unit="m"
              scale={1}
            />
            <TopWasteCard
              title="Top 10 — biggest memory waste"
              icon={<Database className="w-4 h-4 text-warn" />}
              items={insights.topMem}
              unit="Mi"
              scale={1}
            />
          </div>

          {/* ── by cluster ─────────────────────────────────────────── */}
          <SectionTable
            title="By cluster"
            headers={['Cluster', 'CPU waste (cores)', 'Mem waste (GiB)', 'Recommendations', 'Applied']}
            rows={insights.byCluster.map(c => [
              <Link to={`/clusters/${c.clusterId}`} key={c.clusterId} className="inline-flex items-center gap-2 text-ink hover:text-accent">
                <Server className="w-3 h-3 text-dim" />
                {c.clusterName}
              </Link>,
              <span className="font-mono text-warn">{formatCores(c.cpuWaste)}</span>,
              <span className="font-mono text-warn">{formatGiB(c.memWaste)}</span>,
              <span className="font-mono text-muted">{c.count}</span>,
              <span className="font-mono text-ok">{c.applied}</span>,
            ])}
          />

          {/* ── by namespace ───────────────────────────────────────── */}
          <SectionTable
            title="By namespace (top 10)"
            headers={['Namespace', 'Cluster', 'CPU waste (cores)', 'Mem waste (GiB)', 'Recs']}
            rows={insights.byNamespace.slice(0, 10).map(n => [
              <span className="font-mono text-xs text-ink">{n.namespace}</span>,
              <span className="text-xs text-muted">{n.clusterName}</span>,
              <span className="font-mono text-warn">{formatCores(n.cpuWaste)}</span>,
              <span className="font-mono text-warn">{formatGiB(n.memWaste)}</span>,
              <span className="font-mono text-muted">{n.count}</span>,
            ])}
          />
        </>
      )}
    </div>
  )
}

// ─── aggregation helpers ────────────────────────────────────────────────

function calcInsights(rows: RecommendationWithCluster[]) {
  let potentialCPU = 0, potentialMem = 0
  let realizedCPU  = 0, realizedMem  = 0
  let applied = 0, ready = 0

  const perCluster   = new Map<string, { clusterId: string; clusterName: string; cpuWaste: number; memWaste: number; count: number; applied: number }>()
  const perNamespace = new Map<string, { namespace: string; clusterName: string; cpuWaste: number; memWaste: number; count: number }>()

  for (const r of rows) {
    if (r.status !== 'ready') continue
    ready++
    const cpuDelta = Math.max(0, r.current_cpu_limit - r.recommended_cpu_limit)
    const memDelta = Math.max(0, r.current_mem_limit - r.recommended_mem_limit)
    potentialCPU += cpuDelta
    potentialMem += memDelta
    if (r.applied) {
      applied++
      realizedCPU += cpuDelta
      realizedMem += memDelta
    }
    // per-cluster aggregate
    const c = perCluster.get(r.cluster_id) ?? {
      clusterId: r.cluster_id, clusterName: r.cluster_name, cpuWaste: 0, memWaste: 0, count: 0, applied: 0,
    }
    c.cpuWaste += cpuDelta
    c.memWaste += memDelta
    c.count += 1
    if (r.applied) c.applied += 1
    perCluster.set(r.cluster_id, c)
    // per-namespace (scoped by cluster to avoid collisions)
    const nsKey = `${r.cluster_id}/${r.namespace}`
    const n = perNamespace.get(nsKey) ?? {
      namespace: r.namespace, clusterName: r.cluster_name, cpuWaste: 0, memWaste: 0, count: 0,
    }
    n.cpuWaste += cpuDelta
    n.memWaste += memDelta
    n.count += 1
    perNamespace.set(nsKey, n)
  }

  const byCluster   = Array.from(perCluster.values()).sort((a, b) => b.cpuWaste - a.cpuWaste)
  const byNamespace = Array.from(perNamespace.values()).sort((a, b) => b.cpuWaste - a.cpuWaste)

  const topCPU = [...rows]
    .filter(r => r.status === 'ready' && r.current_cpu_limit > r.recommended_cpu_limit)
    .sort((a, b) => (b.current_cpu_limit - b.recommended_cpu_limit) - (a.current_cpu_limit - a.recommended_cpu_limit))
    .slice(0, 10)

  const topMem = [...rows]
    .filter(r => r.status === 'ready' && r.current_mem_limit > r.recommended_mem_limit)
    .sort((a, b) => (b.current_mem_limit - b.recommended_mem_limit) - (a.current_mem_limit - a.recommended_mem_limit))
    .slice(0, 10)

  return {
    potentialCPU, potentialMem,
    realizedCPU, realizedMem,
    applied, ready,
    adoptionPct: ready > 0 ? Math.round((applied / ready) * 100) : 0,
    byCluster, byNamespace,
    topCPU, topMem,
  }
}

// millicores → cores string (2000 → "2.0", 1250 → "1.3", 350 → "0.35")
function formatCores(millicores: number): string {
  if (millicores === 0) return '0'
  const cores = millicores / 1000
  if (cores >= 10)  return cores.toFixed(0)
  if (cores >= 1)   return cores.toFixed(1)
  return cores.toFixed(2)
}

// MiB → GiB string
function formatGiB(mebibytes: number): string {
  if (mebibytes === 0) return '0'
  const gib = mebibytes / 1024
  if (gib >= 10) return gib.toFixed(0)
  if (gib >= 1)  return gib.toFixed(1)
  return gib.toFixed(2)
}

// ─── UI subcomponents ──────────────────────────────────────────────────

function HeroCard(props: { icon: React.ReactNode; label: string; value: string; unit: string; accent: 'ok' | 'warn' }) {
  const color = props.accent === 'ok' ? 'text-ok' : 'text-warn'
  return (
    <div className="bg-surface border border-border rounded-lg p-5 shadow-card">
      <div className="flex items-center gap-2 text-[10px] uppercase tracking-widest text-dim font-semibold mb-2">
        {props.icon}
        {props.label}
      </div>
      <div className={`text-3xl font-mono font-semibold ${color}`}>
        {props.value}
        <span className="text-dim text-base ml-1">{props.unit}</span>
      </div>
    </div>
  )
}

function TopWasteCard({ title, icon, items, unit }: { title: string; icon: React.ReactNode; items: RecommendationWithCluster[]; unit: string; scale: number }) {
  return (
    <div className="bg-surface border border-border rounded-lg shadow-card overflow-hidden">
      <div className="px-5 py-4 border-b border-border flex items-center gap-2">
        {icon}
        <span className="text-sm font-semibold text-ink">{title}</span>
      </div>
      {items.length === 0 ? (
        <div className="px-5 py-8 text-center text-dim text-sm">No waste detected — every container is right-sized ✓</div>
      ) : (
        <ol className="divide-y divide-border">
          {items.map((r, i) => {
            const delta = unit === 'm'
              ? r.current_cpu_limit - r.recommended_cpu_limit
              : r.current_mem_limit - r.recommended_mem_limit
            const current = unit === 'm' ? r.current_cpu_limit : r.current_mem_limit
            const recommended = unit === 'm' ? r.recommended_cpu_limit : r.recommended_mem_limit
            return (
              <li key={r.recommendation_id} className="px-5 py-3 flex items-center gap-4 hover:bg-elevated/40 transition">
                <span className="text-dim font-mono text-xs w-4 text-right">{i + 1}</span>
                <div className="flex-1 min-w-0">
                  <div className="text-ink text-xs font-medium truncate">
                    <Link to={`/clusters/${r.cluster_id}`} className="text-muted hover:text-accent">{r.cluster_name}</Link>
                    <span className="text-dim mx-1">·</span>
                    <span className="text-muted font-mono">{r.namespace}</span>
                    <span className="text-dim mx-1">·</span>
                    <span className="font-mono">{r.pod_name}/{r.container_name}</span>
                  </div>
                  <div className="text-[11px] text-dim font-mono mt-0.5">
                    {current}{unit} → {recommended}{unit}
                  </div>
                </div>
                <div className="text-warn font-mono text-sm font-semibold">
                  ↓{delta}<span className="text-dim ml-0.5 text-xs">{unit}</span>
                </div>
              </li>
            )
          })}
        </ol>
      )}
    </div>
  )
}

function SectionTable({ title, headers, rows }: { title: string; headers: string[]; rows: React.ReactNode[][] }) {
  return (
    <div className="bg-surface border border-border rounded-lg shadow-card overflow-hidden mb-6">
      <div className="px-5 py-4 border-b border-border">
        <span className="text-sm font-semibold text-ink">{title}</span>
      </div>
      {rows.length === 0 ? (
        <div className="px-5 py-8 text-center text-dim text-sm">No data</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-[10px] uppercase tracking-widest text-dim font-semibold bg-elevated/40 border-b border-border">
                {headers.map(h => <th key={h} className="px-4 py-3">{h}</th>)}
              </tr>
            </thead>
            <tbody>
              {rows.map((r, i) => (
                <tr key={i} className="border-b border-border last:border-b-0 hover:bg-elevated/40 transition">
                  {r.map((cell, j) => <td key={j} className="px-4 py-3">{cell}</td>)}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function EmptyState() {
  return (
    <div className="bg-surface border border-border rounded-lg p-12 text-center shadow-card">
      <div className="inline-flex w-12 h-12 rounded-lg bg-accentBg border border-accent/30 items-center justify-center mb-4">
        <TrendingDown className="w-6 h-6 text-accent" />
      </div>
      <h3 className="text-lg font-semibold text-ink mb-2">No recommendations yet</h3>
      <p className="text-sm text-muted mb-6 max-w-sm mx-auto">
        Register a cluster and let PodOptix analyse it. Savings insights appear as soon as the first sync completes.
      </p>
      <Link to="/clusters/new" className="inline-flex items-center gap-2 bg-accent hover:bg-accentHi text-white text-sm font-semibold px-4 py-2 rounded-md transition">
        Register your first cluster
      </Link>
    </div>
  )
}
