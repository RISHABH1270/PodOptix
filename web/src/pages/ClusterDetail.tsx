import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ArrowLeft, RefreshCw, Play, Search, Server, AlertTriangle, Settings } from 'lucide-react'
import { api, Cluster, Recommendation } from '../lib/api'
import { StatusPill } from '../components/StatusPill'

export function ClusterDetailPage() {
  const { id = '' } = useParams()

  const [cluster, setCluster]   = useState<Cluster | null>(null)
  const [recs, setRecs]         = useState<Recommendation[]>([])
  const [loading, setLoading]   = useState(true)
  const [recalculating, setRe]  = useState(false)
  const [error, setError]       = useState<string | null>(null)
  const [flash, setFlash]       = useState<string | null>(null)
  const [search, setSearch]     = useState('')
  const pollRef = useRef<number | null>(null)

  const load = async () => {
    setLoading(true); setError(null)
    try {
      const [c, r] = await Promise.all([api.getCluster(id), api.listRecommendations(id)])
      setCluster(c); setRecs(r)
    } catch (err: any) {
      setError(err.message ?? 'Failed to load cluster')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
    return () => { if (pollRef.current) window.clearInterval(pollRef.current) }
  }, [id])

  // Poll cluster status every 3s for up to 2 minutes after recalculate.
  // Detects both success (last_synced_at moves forward) and failure (status flips to disconnected).
  const pollUntilDone = (startedAt: string) => {
    if (pollRef.current) window.clearInterval(pollRef.current)
    const start = Date.now()
    pollRef.current = window.setInterval(async () => {
      try {
        const c = await api.getCluster(id)
        // success — sync completed
        if (c.last_synced_at && c.last_synced_at !== 'not yet synced' && c.last_synced_at !== startedAt) {
          window.clearInterval(pollRef.current!); pollRef.current = null
          setCluster(c); setRe(false)
          setFlash(`Recalculate completed at ${new Date().toLocaleTimeString()}. Refreshing…`)
          setRecs(await api.listRecommendations(id))
          setTimeout(() => setFlash(null), 5000)
          return
        }
        // failure — collector marked cluster disconnected
        if (c.status === 'disconnected' && cluster?.status === 'connected') {
          window.clearInterval(pollRef.current!); pollRef.current = null
          setCluster(c); setRe(false); setFlash(null)
          setError('Recalculate failed — Prometheus became unreachable. Check the cluster URL and token.')
          return
        }
        // timeout — 2 min
        if (Date.now() - start > 120_000) {
          window.clearInterval(pollRef.current!); pollRef.current = null
          setRe(false); setFlash(null)
          setError('Recalculate is taking longer than expected. Check backend logs.')
        }
      } catch { /* transient — keep polling */ }
    }, 3000)
  }

  const recalculate = async () => {
    if (cluster?.status === 'disconnected') {
      setError('Cannot recalculate — cluster is disconnected. Fix the Prometheus URL/token first.')
      return
    }
    setRe(true); setError(null)
    try {
      await api.recalculate(id)
      setFlash('Recalculation running… this may take up to a minute.')
      pollUntilDone(cluster?.last_synced_at ?? '')
    } catch (err: any) {
      setError(err.message ?? 'Recalculate failed')
      setRe(false)
    }
  }

  const filtered = useMemo(() => {
    if (!search) return recs
    const q = search.toLowerCase()
    return recs.filter(
      (r) =>
        r.namespace.toLowerCase().includes(q) ||
        r.pod_name.toLowerCase().includes(q) ||
        r.container_name.toLowerCase().includes(q),
    )
  }, [recs, search])

  const stats = useMemo(() => {
    const ready       = recs.filter((r) => r.status === 'ready').length
    const newService  = recs.length - ready
    const applied     = recs.filter((r) => r.applied).length
    return { ready, newService, applied }
  }, [recs])

  return (
    <div className="p-8 max-w-7xl mx-auto">
      <Link to="/clusters" className="inline-flex items-center gap-2 text-sm text-muted hover:text-ink mb-6 transition">
        <ArrowLeft className="w-4 h-4" /> Back to clusters
      </Link>

      {/* ── header ────────────────────────────────────────────── */}
      <div className="flex items-start justify-between mb-6">
        <div className="flex items-start gap-4">
          <div className="w-11 h-11 rounded-lg bg-accentBg border border-accent/30 flex items-center justify-center flex-shrink-0">
            <Server className="w-5 h-5 text-accent" />
          </div>
          <div>
            <div className="text-[10px] uppercase tracking-widest text-accent font-semibold mb-1">Cluster</div>
            <h1 className="text-2xl font-bold text-ink">{cluster?.cluster_name ?? '…'}</h1>
            <div className="text-xs text-muted font-mono mt-1">{cluster?.prometheus_url}</div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={load}
            className="p-2 rounded-md bg-surface border border-border text-muted hover:text-ink hover:border-borderHi transition"
            title="Refresh"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <Link
            to={`/clusters/${id}/edit`}
            className="inline-flex items-center gap-2 bg-surface border border-border hover:border-borderHi text-muted hover:text-ink text-sm font-semibold px-4 py-2 rounded-md transition"
            title="Edit cluster settings"
          >
            <Settings className="w-4 h-4" />
            Edit
          </Link>
          <button
            onClick={recalculate}
            disabled={recalculating || cluster?.status === 'disconnected'}
            title={cluster?.status === 'disconnected' ? 'Cluster is disconnected — cannot recalculate' : 'Trigger a fresh scan'}
            className="inline-flex items-center gap-2 bg-accent hover:bg-accentHi disabled:opacity-40 disabled:cursor-not-allowed text-white text-sm font-semibold px-4 py-2 rounded-md transition"
          >
            <Play className="w-4 h-4" />
            {recalculating ? 'Running…' : 'Recalculate'}
          </button>
        </div>
      </div>

      {/* ── info + stats row ──────────────────────────────────── */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <InfoCard label="Status" value={
          cluster
            ? (cluster.status === 'connected'
                ? <StatusPill kind="ok" label="connected" />
                : <StatusPill kind="danger" label="disconnected" />)
            : <span className="text-dim text-sm">—</span>
        } />
        <InfoCard label="Lookback"        mono value={cluster?.lookback_window ?? '—'} />
        <InfoCard label="Last synced"     value={cluster ? formatSynced(cluster.last_synced_at) : '—'} />
        <InfoCard label="Registered by"   value={cluster?.created_by ?? '—'} />
      </div>

      {/* ── disconnected warning ──────────────────────────────── */}
      {cluster?.status === 'disconnected' && (
        <div className="px-4 py-3 rounded-md bg-warnBg border border-warn/30 text-warn text-sm mb-4 flex items-start gap-3">
          <AlertTriangle className="w-4 h-4 mt-0.5 flex-shrink-0" />
          <div>
            <div className="font-medium">Cluster is disconnected</div>
            <div className="text-warn/80 text-xs mt-0.5">
              PodOptix cannot reach the Prometheus endpoint at <code className="font-mono">{cluster.prometheus_url}</code>.
              Recalculate is disabled until connectivity is restored. Update the cluster with a valid URL / token.
            </div>
          </div>
        </div>
      )}

      {/* ── flash message ─────────────────────────────────────── */}
      {flash && (
        <div className="px-4 py-3 rounded-md bg-okBg border border-ok/30 text-ok text-sm mb-4">
          {flash}
        </div>
      )}
      {error && (
        <div className="px-4 py-3 rounded-md bg-dangerBg border border-danger/30 text-danger text-sm mb-4">
          {error}
        </div>
      )}

      {/* ── recommendations panel ─────────────────────────────── */}
      <div className="bg-surface border border-border rounded-lg shadow-card overflow-hidden">
        <div className="px-5 py-4 border-b border-border flex items-center justify-between gap-4 flex-wrap">
          <div>
            <div className="text-base font-semibold text-ink">Recommendations</div>
            <div className="text-xs text-muted mt-0.5">
              {recs.length} containers · {stats.ready} ready · {stats.newService} new service · {stats.applied} applied
            </div>
          </div>
          <div className="relative">
            <Search className="w-4 h-4 text-dim absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Filter by namespace, pod, or container…"
              className="w-72 bg-elevated border border-border focus:border-accent rounded-md pl-9 pr-3 py-1.5 text-sm text-ink placeholder:text-dim outline-none transition"
            />
          </div>
        </div>

        {loading && recs.length === 0 ? (
          <div className="text-center py-16 text-dim text-sm">Loading recommendations…</div>
        ) : filtered.length === 0 ? (
          <div className="text-center py-16 text-dim text-sm">
            {recs.length === 0
              ? 'No recommendations yet. Click Recalculate to generate them.'
              : 'No matches for your search.'}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-[10px] uppercase tracking-widest text-dim font-semibold bg-elevated/40 border-b border-border">
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
                      <RecommendedValue current={r.current_cpu_limit} recommended={r.recommended_cpu_limit} unit="m" />
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-xs text-muted">
                      {r.current_mem_limit || '—'}<span className="text-dim ml-0.5">Mi</span>
                    </td>
                    <td className="px-4 py-3 text-right font-mono text-xs">
                      <RecommendedValue current={r.current_mem_limit} recommended={r.recommended_mem_limit} unit="Mi" />
                    </td>
                    <td className="px-4 py-3 text-center">
                      {r.applied
                        ? <span className="text-ok text-xs">✓</span>
                        : <span className="text-dim text-xs">—</span>}
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

function InfoCard({ label, value, mono }: { label: string; value: React.ReactNode; mono?: boolean }) {
  return (
    <div className="bg-surface border border-border rounded-lg p-4 shadow-card">
      <div className="text-[10px] uppercase tracking-widest text-dim font-semibold mb-2">{label}</div>
      <div className={`text-sm text-ink truncate ${mono ? 'font-mono' : ''}`}>{value}</div>
    </div>
  )
}

function RecommendedValue({ current, recommended, unit }: { current: number; recommended: number; unit: string }) {
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

function formatSynced(s: string): string {
  if (!s || s === 'not yet synced') return 'not yet synced'
  const d = new Date(s)
  const mins = Math.floor((Date.now() - d.getTime()) / 60000)
  if (mins < 1)     return 'just now'
  if (mins < 60)    return `${mins} min ago`
  if (mins < 1440)  return `${Math.floor(mins / 60)}h ago`
  return `${Math.floor(mins / 1440)}d ago`
}
