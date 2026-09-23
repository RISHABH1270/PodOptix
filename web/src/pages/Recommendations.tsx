import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Search, RefreshCw, Server, ArrowUpDown } from 'lucide-react'
import { api, RecommendationWithCluster } from '../lib/api'
import { StatusPill } from '../components/StatusPill'

type SortKey = 'cpu_delta' | 'mem_delta' | 'cluster' | 'namespace'

export function RecommendationsPage() {
  const [rows, setRows]         = useState<RecommendationWithCluster[]>([])
  const [loading, setLoading]   = useState(true)
  const [error, setError]       = useState<string | null>(null)
  const [search, setSearch]     = useState('')
  const [clusterFilter, setClusterFilter] = useState<string>('all')
  const [statusFilter, setStatusFilter]   = useState<string>('all')
  const [sortKey, setSortKey]   = useState<SortKey>('cpu_delta')

  const load = async () => {
    setLoading(true); setError(null)
    try { setRows(await api.listAllRecommendations()) }
    catch (err: any) { setError(err.message ?? 'Failed to load recommendations') }
    finally { setLoading(false) }
  }

  useEffect(() => { load() }, [])

  // unique cluster names for the filter dropdown
  const clusterOptions = useMemo(() => {
    const set = new Set(rows.map(r => r.cluster_name))
    return Array.from(set).sort()
  }, [rows])

  // filter + sort
  const filtered = useMemo(() => {
    let out = rows
    if (clusterFilter !== 'all') out = out.filter(r => r.cluster_name === clusterFilter)
    if (statusFilter !== 'all')  out = out.filter(r => r.status === statusFilter)
    if (search) {
      const q = search.toLowerCase()
      out = out.filter(r =>
        r.namespace.toLowerCase().includes(q) ||
        r.pod_name.toLowerCase().includes(q) ||
        r.container_name.toLowerCase().includes(q),
      )
    }
    const sorted = [...out]
    switch (sortKey) {
      case 'cpu_delta':
        sorted.sort((a, b) => (b.current_cpu_limit - b.recommended_cpu_limit) - (a.current_cpu_limit - a.recommended_cpu_limit))
        break
      case 'mem_delta':
        sorted.sort((a, b) => (b.current_mem_limit - b.recommended_mem_limit) - (a.current_mem_limit - a.recommended_mem_limit))
        break
      case 'cluster':
        sorted.sort((a, b) => a.cluster_name.localeCompare(b.cluster_name))
        break
      case 'namespace':
        sorted.sort((a, b) => a.namespace.localeCompare(b.namespace))
        break
    }
    return sorted
  }, [rows, clusterFilter, statusFilter, search, sortKey])

  const stats = useMemo(() => {
    const ready       = rows.filter(r => r.status === 'ready').length
    const applied     = rows.filter(r => r.applied).length
    const totalCpuSaved = rows.reduce((sum, r) => sum + Math.max(0, r.current_cpu_limit - r.recommended_cpu_limit), 0)
    const totalMemSaved = rows.reduce((sum, r) => sum + Math.max(0, r.current_mem_limit - r.recommended_mem_limit), 0)
    return { ready, applied, totalCpuSaved, totalMemSaved }
  }, [rows])

  return (
    <div className="p-8 max-w-7xl mx-auto">
      <div className="flex items-start justify-between mb-6">
        <div>
          <div className="text-[10px] uppercase tracking-widest text-accent font-semibold mb-1">Recommendations</div>
          <h1 className="text-2xl font-bold text-ink">Cross-cluster overview</h1>
          <p className="text-sm text-muted mt-1">Every container in every cluster, sorted by biggest waste first.</p>
        </div>
        <button
          onClick={load}
          className="p-2 rounded-md bg-surface border border-border text-muted hover:text-ink hover:border-borderHi transition"
          title="Refresh"
        >
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {/* ── stat cards ──────────────────────────────────────────── */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <StatCard label="Total containers"     value={rows.length.toString()} />
        <StatCard label="Ready"                value={stats.ready.toString()}   accent="ok" />
        <StatCard label="CPU waste (m)"        value={stats.totalCpuSaved.toLocaleString()} accent="warn"   suffix="m" />
        <StatCard label="Memory waste (Mi)"    value={stats.totalMemSaved.toLocaleString()} accent="warn"   suffix="Mi" />
      </div>

      {error && (
        <div className="px-4 py-3 rounded-md bg-dangerBg border border-danger/30 text-danger text-sm mb-4">{error}</div>
      )}

      {/* ── filters ────────────────────────────────────────────── */}
      <div className="bg-surface border border-border rounded-lg shadow-card">
        <div className="px-5 py-4 border-b border-border flex items-center justify-between gap-3 flex-wrap">
          <div className="flex items-center gap-3 flex-wrap">
            <div className="relative">
              <Search className="w-4 h-4 text-dim absolute left-3 top-1/2 -translate-y-1/2" />
              <input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Filter namespace, pod, container…"
                className="w-72 bg-elevated border border-border focus:border-accent rounded-md pl-9 pr-3 py-1.5 text-sm text-ink placeholder:text-dim outline-none transition"
              />
            </div>
            <select
              value={clusterFilter}
              onChange={(e) => setClusterFilter(e.target.value)}
              className="bg-elevated border border-border rounded-md px-2 py-1.5 text-sm text-ink outline-none"
            >
              <option value="all">All clusters</option>
              {clusterOptions.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="bg-elevated border border-border rounded-md px-2 py-1.5 text-sm text-ink outline-none"
            >
              <option value="all">All statuses</option>
              <option value="ready">ready</option>
              <option value="new_service">new_service</option>
            </select>
          </div>
          <div className="flex items-center gap-2 text-xs text-muted">
            <ArrowUpDown className="w-3.5 h-3.5" />
            <span>Sort:</span>
            <select
              value={sortKey}
              onChange={(e) => setSortKey(e.target.value as SortKey)}
              className="bg-elevated border border-border rounded-md px-2 py-1 text-xs text-ink outline-none"
            >
              <option value="cpu_delta">Biggest CPU waste</option>
              <option value="mem_delta">Biggest memory waste</option>
              <option value="cluster">Cluster</option>
              <option value="namespace">Namespace</option>
            </select>
          </div>
        </div>

        {loading && rows.length === 0 ? (
          <div className="text-center py-16 text-dim text-sm">Loading recommendations…</div>
        ) : filtered.length === 0 ? (
          <div className="text-center py-16 text-dim text-sm">
            {rows.length === 0 ? 'No recommendations yet. Register a cluster and run Recalculate.' : 'No matches for your filters.'}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-[10px] uppercase tracking-widest text-dim font-semibold bg-elevated/40 border-b border-border">
                  <th className="px-4 py-3">Cluster</th>
                  <th className="px-4 py-3">Namespace</th>
                  <th className="px-4 py-3">Pod / Container</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-4 py-3 text-right">Current CPU</th>
                  <th className="px-4 py-3 text-right">Recommended CPU</th>
                  <th className="px-4 py-3 text-right">Current Mem</th>
                  <th className="px-4 py-3 text-right">Recommended Mem</th>
                  <th className="px-4 py-3 text-center">Applied</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((r) => (
                  <tr key={r.recommendation_id} className="border-b border-border last:border-b-0 hover:bg-elevated/40 transition">
                    <td className="px-4 py-3">
                      <Link to={`/clusters/${r.cluster_id}`} className="inline-flex items-center gap-2 text-xs text-muted hover:text-accent">
                        <Server className="w-3 h-3" />
                        {r.cluster_name}
                      </Link>
                    </td>
                    <td className="px-4 py-3 text-muted font-mono text-xs">{r.namespace}</td>
                    <td className="px-4 py-3">
                      <div className="text-ink text-xs font-medium">{r.pod_name}</div>
                      <div className="text-dim text-[11px] font-mono">{r.container_name}</div>
                    </td>
                    <td className="px-4 py-3">
                      {r.status === 'ready'
                        ? <StatusPill kind="ok" label="ready" />
                        : <StatusPill kind="warn" label="new service" />}
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-xs text-muted">
                      {r.current_cpu_limit || '—'}<span className="text-dim ml-0.5">m</span>
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-xs">
                      <Delta current={r.current_cpu_limit} recommended={r.recommended_cpu_limit} unit="m" />
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-xs text-muted">
                      {r.current_mem_limit || '—'}<span className="text-dim ml-0.5">Mi</span>
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-xs">
                      <Delta current={r.current_mem_limit} recommended={r.recommended_mem_limit} unit="Mi" />
                    </td>
                    <td className="px-4 py-3 text-center">
                      {r.applied ? <span className="text-ok text-xs">✓</span> : <span className="text-dim text-xs">—</span>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}

function StatCard({ label, value, suffix, accent }: { label: string; value: string; suffix?: string; accent?: 'ok' | 'warn' }) {
  const color = accent === 'ok' ? 'text-ok' : accent === 'warn' ? 'text-warn' : 'text-ink'
  return (
    <div className="bg-surface border border-border rounded-lg p-5 shadow-card">
      <div className="text-[10px] uppercase tracking-widest text-dim font-semibold mb-2">{label}</div>
      <div className={`text-3xl font-mono font-semibold ${color}`}>{value}{suffix && <span className="text-dim ml-1 text-lg">{suffix}</span>}</div>
    </div>
  )
}

function Delta({ current, recommended, unit }: { current: number; recommended: number; unit: string }) {
  if (!recommended) return <span className="text-dim">—</span>
  if (!current)     return <span className="text-ink">{recommended}<span className="text-dim ml-0.5">{unit}</span></span>
  const diff = recommended - current
  const pct  = current > 0 ? Math.round((diff / current) * 100) : 0
  const color = diff < 0 ? 'text-ok' : diff > 0 ? 'text-warn' : 'text-ink'
  const arrow = diff < 0 ? '↓' : diff > 0 ? '↑' : '='
  return (
    <span>
      <span className="text-ink">{recommended}<span className="text-dim ml-0.5">{unit}</span></span>
      <span className={`ml-2 text-[10px] ${color}`}>{arrow}{Math.abs(pct)}%</span>
    </span>
  )
}
