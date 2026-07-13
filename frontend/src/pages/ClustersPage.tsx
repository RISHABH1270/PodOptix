import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getClusters, createCluster, deleteCluster, type Cluster } from '../api/client'

function Sidebar({ active }: { active: string }) {
  const navigate = useNavigate()
  const email = localStorage.getItem('email') || ''
  const initials = email.slice(0, 2).toUpperCase()

  function logout() {
    localStorage.clear()
    navigate('/login')
  }

  const nav = [
    { label: 'Overview', icon: '⊞' },
    { label: 'Clusters', icon: '◈' },
  ]

  return (
    <aside className="w-56 bg-gray-900 border-r border-gray-800 flex flex-col h-screen fixed left-0 top-0">
      {/* Logo */}
      <div className="px-5 py-5 border-b border-gray-800">
        <span className="text-green-400 font-bold text-lg tracking-wide">PodOptix</span>
      </div>

      {/* Nav */}
      <nav className="flex-1 px-3 py-4 space-y-1">
        {nav.map(item => (
          <button key={item.label}
            onClick={() => {
              if (item.label === 'Overview') navigate('/overview')
            }}
            className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors text-left
              ${active === item.label
                ? 'bg-green-500/10 text-green-400 font-medium'
                : 'text-gray-400 hover:text-white hover:bg-gray-800'}`}>
            <span className="text-base">{item.icon}</span>
            {item.label}
          </button>
        ))}
      </nav>

      {/* User */}
      <div className="px-4 py-4 border-t border-gray-800">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 bg-green-500 rounded-full flex items-center justify-center text-black text-xs font-bold shrink-0">
            {initials}
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-white text-xs font-medium truncate">{email}</p>
          </div>
          <button onClick={logout} title="Sign out" className="text-gray-500 hover:text-red-400 text-xs">↪</button>
        </div>
      </div>
    </aside>
  )
}

// formats last_synced_at into "2 hours ago", "3 days ago" etc.
function timeAgo(dateStr: string | null): string {
  if (!dateStr) return 'Never'
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins  = Math.floor(diff / 60000)
  const hours = Math.floor(diff / 3600000)
  const days  = Math.floor(diff / 86400000)
  if (mins < 1)   return 'Just now'
  if (mins < 60)  return `${mins}m ago`
  if (hours < 24) return `${hours}h ago`
  return `${days}d ago`
}

export default function ClustersPage() {
  const navigate    = useNavigate()
  const qc          = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [name, setName]             = useState('')
  const [prometheusURL, setPrometheusURL] = useState('')
  const [token, setToken]           = useState('')
  const [window, setWindow]         = useState('7d')
  const [formError, setFormError]   = useState('')

  const { data: clusters = [], isLoading } = useQuery({
    queryKey: ['clusters'],
    queryFn: () => getClusters().then(r => r.data),
  })

  const createMutation = useMutation({
    mutationFn: createCluster,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['clusters'] })
      setShowForm(false)
      setName(''); setPrometheusURL(''); setToken(''); setWindow('7d')
    },
    onError: () => setFormError('Failed to register cluster. Check the details and try again.'),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteCluster,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['clusters'] }),
  })

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    setFormError('')
    createMutation.mutate({ name, prometheus_url: prometheusURL, token, lookback_window: window })
  }

  return (
    <div className="flex bg-gray-950 min-h-screen text-white">
      <Sidebar active="Clusters" />

      {/* Main content */}
      <div className="ml-56 flex-1 p-8">

        {/* Page header */}
        <div className="flex items-start justify-between mb-8">
          <div>
            <h1 className="text-2xl font-bold text-white">Clusters</h1>
            <p className="text-gray-500 text-sm mt-1">Monitor your registered Prometheus endpoints</p>
          </div>
          <button onClick={() => setShowForm(true)}
            className="bg-green-500 hover:bg-green-400 text-black font-semibold px-4 py-2 rounded-lg text-sm flex items-center gap-2 transition-colors">
            + Register Cluster
          </button>
        </div>

        {/* Stats cards */}
        <div className="grid grid-cols-3 gap-4 mb-8">
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-500 text-xs mb-1">Total Clusters</p>
            <p className="text-3xl font-bold text-white">{clusters.length}</p>
            <p className="text-green-400 text-xs mt-1">↑ registered endpoints</p>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-500 text-xs mb-1">Active</p>
            <p className="text-3xl font-bold text-white">{clusters.length}</p>
            <p className="text-green-400 text-xs mt-1">↑ ready for collection</p>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-500 text-xs mb-1">Scheduler</p>
            <p className="text-3xl font-bold text-green-400">24h</p>
            <p className="text-gray-500 text-xs mt-1">collection interval</p>
          </div>
        </div>

        {/* Cluster health table */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl">
          <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800">
            <h2 className="font-semibold text-white">Connected Clusters</h2>
            <span className="text-gray-600 text-xs">{clusters.length} registered</span>
          </div>

          {isLoading ? (
            <div className="px-6 py-8 text-center text-gray-500 text-sm">Loading...</div>
          ) : clusters.length === 0 ? (
            <div className="px-6 py-16 text-center">
              <p className="text-gray-500 text-sm mb-2">No clusters registered yet</p>
              <button onClick={() => setShowForm(true)} className="text-green-500 text-sm hover:text-green-400">
                Register your first cluster →
              </button>
            </div>
          ) : (
            <div className="divide-y divide-gray-800">
              {clusters.map((c: Cluster) => (
                <div key={c.cluster_id} className="flex items-center gap-4 px-6 py-4 hover:bg-gray-800/40 transition-colors">
                  <span className="w-2 h-2 bg-green-400 rounded-full shrink-0" />
                  <div className="flex-1 min-w-0">
                    <p className="text-white text-sm font-medium">{c.name}</p>
                    <p className="text-gray-500 text-xs truncate mt-0.5">{c.prometheus_url}</p>
                  </div>
                  <span className="text-xs text-gray-600 font-mono">
                    Last synced: {timeAgo(c.last_synced_at)}
                  </span>
                  <span className="text-xs font-mono text-gray-600 bg-gray-800 px-2 py-0.5 rounded">{c.lookback_window}</span>
                  <span className={`text-xs px-2 py-0.5 rounded-full ${c.status === 'unhealthy' ? 'text-red-400 bg-red-400/10' : 'text-green-400 bg-green-400/10'}`}>
                    {c.status === 'unhealthy' ? 'Unhealthy' : 'Healthy'}
                  </span>
                  <div className="flex items-center gap-3">
                    <button onClick={() => navigate(`/clusters/${c.cluster_id}`)}
                      className="text-green-500 hover:text-green-400 text-sm font-medium transition-colors">
                      View →
                    </button>
                    <button onClick={() => deleteMutation.mutate(c.cluster_id)}
                      className="text-gray-600 hover:text-red-400 text-xs transition-colors">
                      Delete
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Register cluster modal */}
      {showForm && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center px-4 z-50">
          <div className="bg-gray-900 border border-gray-800 rounded-2xl p-6 w-full max-w-md">
            <div className="flex items-center justify-between mb-5">
              <h3 className="text-white font-bold text-lg">Register Cluster</h3>
              <button onClick={() => setShowForm(false)} className="text-gray-500 hover:text-white text-xl">✕</button>
            </div>
            <form onSubmit={handleCreate} className="space-y-4">
              <div>
                <label className="text-gray-400 text-sm block mb-1">Cluster Name</label>
                <input value={name} onChange={e => setName(e.target.value)} required
                  className="w-full bg-gray-800 border border-gray-700 focus:border-green-500 text-white rounded-lg px-4 py-2.5 text-sm focus:outline-none" />
              </div>
              <div>
                <label className="text-gray-400 text-sm block mb-1">Prometheus URL</label>
                <input value={prometheusURL} onChange={e => setPrometheusURL(e.target.value)} required
                  placeholder="https://prometheus.example.com"
                  className="w-full bg-gray-800 border border-gray-700 focus:border-green-500 text-white rounded-lg px-4 py-2.5 text-sm focus:outline-none placeholder-gray-600" />
              </div>
              <div>
                <label className="text-gray-400 text-sm block mb-1">Auth Token</label>
                <input type="password" value={token} onChange={e => setToken(e.target.value)} required
                  className="w-full bg-gray-800 border border-gray-700 focus:border-green-500 text-white rounded-lg px-4 py-2.5 text-sm focus:outline-none" />
              </div>
              {formError && <p className="text-red-400 text-sm">{formError}</p>}
              <div className="flex gap-3 pt-2">
                <button type="button" onClick={() => setShowForm(false)}
                  className="flex-1 border border-gray-700 text-gray-300 rounded-lg py-2.5 text-sm hover:border-gray-600 transition-colors">
                  Cancel
                </button>
                <button type="submit" disabled={createMutation.isPending}
                  className="flex-1 bg-green-500 hover:bg-green-400 disabled:bg-green-800 text-black font-semibold rounded-lg py-2.5 text-sm transition-colors">
                  {createMutation.isPending ? 'Registering...' : 'Register'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
