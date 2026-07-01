import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { getClusters, getRecommendations, type Cluster, type Recommendation } from '../api/client'
import Sidebar from '../components/Sidebar'

export default function OverviewPage() {
  const navigate = useNavigate()

  const { data: clusters = [] } = useQuery({
    queryKey: ['clusters'],
    queryFn: () => getClusters().then(r => r.data),
  })

  // fetch recommendations for all clusters in parallel
  const recQueries = useQuery({
    queryKey: ['all-recommendations', clusters.map((c: Cluster) => c.cluster_id)],
    queryFn: async () => {
      if (clusters.length === 0) return []
      const results = await Promise.all(
        clusters.map((c: Cluster) => getRecommendations(c.cluster_id).then(r => r.data))
      )
      return results.flat()
    },
    enabled: clusters.length > 0,
  })

  const allRecs: Recommendation[] = recQueries.data ?? []
  const readyCount   = allRecs.filter(r => r.status === 'ready').length
  const newSvcCount  = allRecs.filter(r => r.status === 'new_service').length

  // savings: containers where current > recommended
  const savingsCount = allRecs.filter(
    r => r.status === 'ready' && r.current_cpu_limit > r.recommended_cpu_limit
  ).length

  return (
    <div className="flex bg-gray-950 min-h-screen text-white">
      <Sidebar active="Overview" />

      <div className="ml-56 flex-1 p-8">

        {/* Header */}
        <div className="mb-8">
          <h1 className="text-2xl font-bold text-white">Overview</h1>
          <p className="text-gray-500 text-sm mt-1">Monitor your Kubernetes environment and resource optimization</p>
        </div>

        {/* Stats cards */}
        <div className="grid grid-cols-4 gap-4 mb-8">
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-500 text-xs mb-1">Total Clusters</p>
            <p className="text-3xl font-bold text-white">{clusters.length}</p>
            <p className="text-green-400 text-xs mt-1">↑ registered endpoints</p>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-500 text-xs mb-1">Containers Monitored</p>
            <p className="text-3xl font-bold text-white">{allRecs.length}</p>
            <p className="text-green-400 text-xs mt-1">across all clusters</p>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-500 text-xs mb-1">Recommendations Ready</p>
            <p className="text-3xl font-bold text-green-400">{readyCount}</p>
            <p className="text-gray-500 text-xs mt-1">{newSvcCount} awaiting data</p>
          </div>
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-gray-500 text-xs mb-1">Savings Identified</p>
            <p className="text-3xl font-bold text-yellow-400">{savingsCount}</p>
            <p className="text-gray-500 text-xs mt-1">containers over-provisioned</p>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-6">

          {/* Cluster Health */}
          <div className="bg-gray-900 border border-gray-800 rounded-xl">
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-800">
              <h2 className="font-semibold text-white">Cluster Health</h2>
              <button onClick={() => navigate('/clusters')}
                className="text-green-500 text-xs hover:text-green-400">
                View all clusters →
              </button>
            </div>
            {clusters.length === 0 ? (
              <div className="px-6 py-10 text-center">
                <p className="text-gray-500 text-sm">No clusters registered</p>
                <button onClick={() => navigate('/clusters')}
                  className="text-green-500 text-xs mt-2 hover:text-green-400">
                  Register a cluster →
                </button>
              </div>
            ) : (
              <div className="divide-y divide-gray-800">
                {clusters.map((c: Cluster) => (
                  <div key={c.cluster_id}
                    className="flex items-center justify-between px-6 py-3 hover:bg-gray-800/40 cursor-pointer transition-colors"
                    onClick={() => navigate(`/clusters/${c.cluster_id}`)}>
                    <div className="flex items-center gap-3">
                      <span className="w-2 h-2 bg-green-400 rounded-full" />
                      <div>
                        <p className="text-white text-sm font-medium">{c.name}</p>
                        <p className="text-green-400 text-xs">Healthy</p>
                      </div>
                    </div>
                    <span className="text-xs font-mono text-gray-600 bg-gray-800 px-2 py-0.5 rounded">{c.lookback_window}</span>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Recommendation Summary */}
          <div className="bg-gray-900 border border-gray-800 rounded-xl">
            <div className="px-6 py-4 border-b border-gray-800">
              <h2 className="font-semibold text-white">Recommendation Summary</h2>
            </div>
            <div className="px-6 py-5 space-y-5">

              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <span className="text-gray-400 text-sm">Ready</span>
                  <span className="text-green-400 text-sm font-medium">{readyCount}</span>
                </div>
                <div className="w-full bg-gray-800 rounded-full h-1.5">
                  <div className="bg-green-400 h-1.5 rounded-full transition-all"
                    style={{ width: allRecs.length ? `${(readyCount / allRecs.length) * 100}%` : '0%' }} />
                </div>
              </div>

              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <span className="text-gray-400 text-sm">Awaiting Data</span>
                  <span className="text-yellow-400 text-sm font-medium">{newSvcCount}</span>
                </div>
                <div className="w-full bg-gray-800 rounded-full h-1.5">
                  <div className="bg-yellow-400 h-1.5 rounded-full transition-all"
                    style={{ width: allRecs.length ? `${(newSvcCount / allRecs.length) * 100}%` : '0%' }} />
                </div>
              </div>

              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <span className="text-gray-400 text-sm">Over-provisioned</span>
                  <span className="text-orange-400 text-sm font-medium">{savingsCount}</span>
                </div>
                <div className="w-full bg-gray-800 rounded-full h-1.5">
                  <div className="bg-orange-400 h-1.5 rounded-full transition-all"
                    style={{ width: readyCount ? `${(savingsCount / readyCount) * 100}%` : '0%' }} />
                </div>
              </div>

              <div className="pt-2 border-t border-gray-800">
                <p className="text-gray-600 text-xs">
                  Scheduler runs every 24h · {clusters.length} cluster{clusters.length !== 1 ? 's' : ''} active
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
