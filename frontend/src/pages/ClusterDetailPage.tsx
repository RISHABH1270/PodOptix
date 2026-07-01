import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { getCluster, getRecommendations, recalculate, type Recommendation } from '../api/client'

function Sidebar({ active }: { active: string }) {
  const navigate = useNavigate()
  const email = localStorage.getItem('email') || ''
  const initials = email.slice(0, 2).toUpperCase()

  function logout() {
    localStorage.clear()
    navigate('/login')
  }

  const nav = [
    { label: 'Overview',        icon: '⊞' },
    { label: 'Clusters',        icon: '◈' },
    { label: 'Recommendations', icon: '✦' },
    { label: 'Settings',        icon: '⚙' },
  ]

  return (
    <aside className="w-56 bg-gray-900 border-r border-gray-800 flex flex-col h-screen fixed left-0 top-0">
      <div className="px-5 py-5 border-b border-gray-800">
        <span className="text-green-400 font-bold text-lg tracking-wide">PodOptix</span>
      </div>
      <nav className="flex-1 px-3 py-4 space-y-1">
        {nav.map(item => (
          <button key={item.label}
            onClick={() => item.label === 'Clusters' && navigate('/clusters')}
            className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors text-left
              ${active === item.label
                ? 'bg-green-500/10 text-green-400 font-medium'
                : 'text-gray-400 hover:text-white hover:bg-gray-800'}`}>
            <span className="text-base">{item.icon}</span>
            {item.label}
          </button>
        ))}
      </nav>
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

// convert millicores to readable string
function fmtCPU(m: number) {
  if (m === 0) return '—'
  return m >= 1000 ? `${(m / 1000).toFixed(1)} cores` : `${m}m`
}

// convert MiB to readable string
function fmtMem(mib: number) {
  if (mib === 0) return '—'
  return mib >= 1024 ? `${(mib / 1024).toFixed(1)} Gi` : `${mib} Mi`
}

// calculate savings percentage
function savings(current: number, recommended: number) {
  if (current === 0 || recommended === 0) return null
  const pct = Math.round(((current - recommended) / current) * 100)
  return pct
}

export default function ClusterDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const { data: cluster } = useQuery({
    queryKey: ['cluster', id],
    queryFn: () => getCluster(id!).then(r => r.data),
  })

  const { data: recommendations = [], isLoading } = useQuery({
    queryKey: ['recommendations', id],
    queryFn: () => getRecommendations(id!).then(r => r.data),
  })

  const recalcMutation = useMutation({
    mutationFn: () => recalculate(id!),
  })

  const readyCount    = recommendations.filter(r => r.status === 'ready').length
  const newSvcCount   = recommendations.filter(r => r.status === 'new_service').length

  return (
    <div className="flex bg-gray-950 min-h-screen text-white">
      <Sidebar active="Recommendations" />

      <div className="ml-56 flex-1 p-8">

        {/* Header */}
        <div className="flex items-start justify-between mb-6">
          <div>
            <button onClick={() => navigate('/clusters')}
              className="text-gray-500 hover:text-white text-sm flex items-center gap-1 mb-2 transition-colors">
              ← Back to Clusters
            </button>
            <h1 className="text-2xl font-bold text-white">{cluster?.name ?? 'Cluster'}</h1>
            <p className="text-gray-500 text-xs mt-1 font-mono truncate max-w-md">{cluster?.prometheus_url}</p>
          </div>
          <button
            onClick={() => recalcMutation.mutate()}
            disabled={recalcMutation.isPending || recalcMutation.isSuccess}
            className="bg-green-500 hover:bg-green-400 disabled:bg-green-800 text-black font-semibold px-4 py-2 rounded-lg text-sm flex items-center gap-2 transition-colors">
            {recalcMutation.isPending  ? '⟳ Recalculating...' :
             recalcMutation.isSuccess  ? '✓ Queued' :
             '⟳ Recalculate'}
          </button>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-3 gap-4 mb-8">
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-500 text-xs mb-1">Total Containers</p>
            <p className="text-3xl font-bold text-white">{recommendations.length}</p>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-500 text-xs mb-1">Recommendations Ready</p>
            <p className="text-3xl font-bold text-green-400">{readyCount}</p>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-500 text-xs mb-1">Awaiting Data</p>
            <p className="text-3xl font-bold text-yellow-400">{newSvcCount}</p>
          </div>
        </div>

        {/* Recommendations table */}
        <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
          <div className="px-6 py-4 border-b border-gray-800">
            <h2 className="font-semibold text-white">Container Recommendations</h2>
            <p className="text-gray-500 text-xs mt-0.5">Based on p99 × 2 over {cluster?.lookback_window ?? '7d'} lookback</p>
          </div>

          {isLoading ? (
            <div className="px-6 py-10 text-center text-gray-500 text-sm">Loading recommendations...</div>
          ) : recommendations.length === 0 ? (
            <div className="px-6 py-16 text-center">
              <p className="text-gray-500 text-sm mb-1">No recommendations yet</p>
              <p className="text-gray-600 text-xs">Click "Recalculate" to generate your first recommendations</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-gray-500 text-xs border-b border-gray-800">
                    <th className="text-left px-6 py-3 font-medium">Container</th>
                    <th className="text-left px-4 py-3 font-medium">Status</th>
                    <th className="text-right px-4 py-3 font-medium">Current CPU</th>
                    <th className="text-right px-4 py-3 font-medium">Recommended CPU</th>
                    <th className="text-right px-4 py-3 font-medium">Current Mem</th>
                    <th className="text-right px-4 py-3 font-medium">Recommended Mem</th>
                    <th className="text-right px-6 py-3 font-medium">Savings</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-800">
                  {recommendations.map((r: Recommendation) => {
                    const cpuSave = savings(r.current_cpu_limit, r.recommended_cpu_limit)
                    const memSave = savings(r.current_mem_limit, r.recommended_mem_limit)
                    return (
                      <tr key={r.recommendation_id} className="hover:bg-gray-800/40 transition-colors">
                        <td className="px-6 py-4">
                          <p className="text-white font-medium">{r.container_name}</p>
                          <p className="text-gray-500 text-xs mt-0.5">{r.namespace} / {r.pod_name}</p>
                        </td>
                        <td className="px-4 py-4">
                          {r.status === 'ready' ? (
                            <span className="text-xs text-green-400 bg-green-400/10 px-2 py-0.5 rounded-full">Ready</span>
                          ) : (
                            <span className="text-xs text-yellow-400 bg-yellow-400/10 px-2 py-0.5 rounded-full">Collecting</span>
                          )}
                        </td>
                        <td className="px-4 py-4 text-right text-gray-400 font-mono text-xs">{fmtCPU(r.current_cpu_limit)}</td>
                        <td className="px-4 py-4 text-right text-green-400 font-mono text-xs font-medium">{fmtCPU(r.recommended_cpu_limit)}</td>
                        <td className="px-4 py-4 text-right text-gray-400 font-mono text-xs">{fmtMem(r.current_mem_limit)}</td>
                        <td className="px-4 py-4 text-right text-green-400 font-mono text-xs font-medium">{fmtMem(r.recommended_mem_limit)}</td>
                        <td className="px-6 py-4 text-right">
                          {cpuSave !== null && cpuSave > 0 ? (
                            <span className="text-xs text-green-400">↓ {cpuSave}% CPU</span>
                          ) : r.status === 'new_service' ? (
                            <span className="text-xs text-gray-600">—</span>
                          ) : (
                            <span className="text-xs text-gray-500">No change</span>
                          )}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {recalcMutation.isSuccess && (
          <p className="text-green-400 text-sm mt-4 text-center">
            Recalculation queued — check back in a few minutes.
          </p>
        )}
      </div>
    </div>
  )
}
