import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Plus, RefreshCw, Server, ExternalLink, Trash2 } from 'lucide-react'
import { api, Cluster } from '../lib/api'
import { StatusPill } from '../components/StatusPill'

export function ClustersPage() {
  const [clusters, setClusters] = useState<Cluster[]>([])
  const [loading, setLoading]   = useState(true)
  const [error, setError]       = useState<string | null>(null)

  const load = async () => {
    setLoading(true); setError(null)
    try {
      setClusters(await api.listClusters())
    } catch (err: any) {
      setError(err.message ?? 'Failed to load clusters')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const remove = async (c: Cluster) => {
    if (!confirm(`Delete cluster "${c.cluster_name}"? This removes all its recommendations.`)) return
    try {
      await api.deleteCluster(c.cluster_id)
      setClusters((prev) => prev.filter((x) => x.cluster_id !== c.cluster_id))
    } catch (err: any) {
      alert(err.message ?? 'Delete failed')
    }
  }

  const connected    = clusters.filter((c) => c.status === 'connected').length
  const disconnected = clusters.length - connected

  return (
    <div className="p-8 max-w-7xl mx-auto">
      {/* ── header ────────────────────────────────────────────── */}
      <div className="flex items-start justify-between mb-6">
        <div>
          <div className="text-[10px] uppercase tracking-widest text-accent font-semibold mb-1">Clusters</div>
          <h1 className="text-2xl font-bold text-ink">Overview</h1>
          <p className="text-sm text-muted mt-1">All Kubernetes clusters registered with PodOptix.</p>
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
            to="/clusters/new"
            className="inline-flex items-center gap-2 bg-accent hover:bg-accentHi text-white text-sm font-semibold px-4 py-2 rounded-md transition"
          >
            <Plus className="w-4 h-4" />
            Register cluster
          </Link>
        </div>
      </div>

      {/* ── stat cards ────────────────────────────────────────── */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
        <StatCard label="Total clusters"  value={clusters.length}  color="ink" />
        <StatCard label="Connected"       value={connected}        color="ok" />
        <StatCard label="Disconnected"    value={disconnected}     color="danger" />
      </div>

      {/* ── content ───────────────────────────────────────────── */}
      {error && (
        <div className="px-4 py-3 rounded-md bg-dangerBg border border-danger/30 text-danger text-sm mb-4">
          {error}
        </div>
      )}

      {loading && clusters.length === 0 ? (
        <div className="text-center py-16 text-dim text-sm">Loading clusters…</div>
      ) : clusters.length === 0 && !error ? (
        <EmptyState />
      ) : (
        <div className="bg-surface border border-border rounded-lg shadow-card overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-[10px] uppercase tracking-widest text-dim font-semibold bg-elevated/50 border-b border-border">
                <th className="px-4 py-3">Name</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3">Prometheus</th>
                <th className="px-4 py-3">Lookback</th>
                <th className="px-4 py-3">Last synced</th>
                <th className="px-4 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {clusters.map((c) => (
                <tr key={c.cluster_id} className="border-b border-border last:border-b-0 hover:bg-elevated/40 transition">
                  <td className="px-4 py-3">
                    <Link to={`/clusters/${c.cluster_id}`} className="text-ink font-medium hover:text-accent inline-flex items-center gap-2">
                      <Server className="w-3.5 h-3.5 text-dim" />
                      {c.cluster_name}
                    </Link>
                  </td>
                  <td className="px-4 py-3">
                    {c.status === 'connected'
                      ? <StatusPill kind="ok" label="connected" />
                      : <StatusPill kind="danger" label="disconnected" />}
                  </td>
                  <td className="px-4 py-3 text-muted font-mono text-xs truncate max-w-[220px]" title={c.prometheus_url}>
                    {c.prometheus_url}
                  </td>
                  <td className="px-4 py-3 text-muted font-mono text-xs">{c.lookback_window}</td>
                  <td className="px-4 py-3 text-muted text-xs">{formatSynced(c.last_synced_at)}</td>
                  <td className="px-4 py-3 text-right">
                    <div className="inline-flex items-center gap-1">
                      <Link
                        to={`/clusters/${c.cluster_id}`}
                        className="p-1.5 rounded hover:bg-elevated text-dim hover:text-info transition"
                        title="View recommendations"
                      >
                        <ExternalLink className="w-3.5 h-3.5" />
                      </Link>
                      <button
                        onClick={() => remove(c)}
                        className="p-1.5 rounded hover:bg-elevated text-dim hover:text-danger transition"
                        title="Delete"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function StatCard({ label, value, color }: { label: string; value: number; color: 'ink' | 'ok' | 'danger' }) {
  const colors = { ink: 'text-ink', ok: 'text-ok', danger: 'text-danger' }
  return (
    <div className="bg-surface border border-border rounded-lg p-5 shadow-card">
      <div className="text-[10px] uppercase tracking-widest text-dim font-semibold mb-2">{label}</div>
      <div className={`text-3xl font-mono font-semibold ${colors[color]}`}>{value}</div>
    </div>
  )
}

function EmptyState() {
  return (
    <div className="bg-surface border border-border rounded-lg p-12 text-center shadow-card">
      <div className="inline-flex w-12 h-12 rounded-lg bg-accentBg border border-accent/30 items-center justify-center mb-4">
        <Server className="w-6 h-6 text-accent" />
      </div>
      <h3 className="text-lg font-semibold text-ink mb-2">No clusters yet</h3>
      <p className="text-sm text-muted mb-6 max-w-sm mx-auto">
        Register your first Kubernetes cluster by pointing PodOptix at its Prometheus endpoint.
      </p>
      <Link to="/clusters/new"
        className="inline-flex items-center gap-2 bg-accent hover:bg-accentHi text-white text-sm font-semibold px-4 py-2 rounded-md transition">
        <Plus className="w-4 h-4" />
        Register your first cluster
      </Link>
    </div>
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
