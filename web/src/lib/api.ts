// Thin fetch wrapper — attaches JWT, handles JSON, throws on non-2xx.
import { auth } from './auth'

export interface Cluster {
  cluster_id:      string
  cluster_name:    string
  prometheus_url:  string
  lookback_window: string
  status:          'connected' | 'disconnected'
  created_by:      string
  last_synced_at:  string
  created_at:      string
  updated_at:      string
}

export interface Recommendation {
  recommendation_id:     string
  cluster_id:            string
  namespace:             string
  pod_name:              string
  container_name:        string
  status:                'ready' | 'new_service'
  current_cpu_limit:     number
  current_mem_limit:     number
  p99_cpu:               number
  p99_mem:               number
  recommended_cpu_limit: number
  recommended_mem_limit: number
  applied:               boolean
  created_at:            string
  updated_at:            string
}

export interface RecommendationWithCluster extends Recommendation {
  cluster_name: string
}

export interface ApiError { message: string; status: number; requestId?: string }

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined)     headers['Content-Type'] = 'application/json'
  const token = auth.getToken()
  if (token)                  headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(path, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  if (res.status === 204) return undefined as T

  const text = await res.text()
  const data = text ? JSON.parse(text) : {}

  if (!res.ok) {
    // token expired or invalid → force re-login
    if (res.status === 401 && auth.isLoggedIn()) {
      auth.clear()
      window.location.href = '/login'
    }
    throw {
      message:   data.error   ?? `Request failed (${res.status})`,
      status:    res.status,
      requestId: data.request_id,
    } as ApiError
  }
  return data as T
}

export const api = {
  // ── auth ──
  register(email: string, password: string) {
    return request<{ token: string; user_id: string; email: string }>(
      'POST', '/auth/register', { email, password },
    )
  },
  login(email: string, password: string) {
    return request<{ token: string; user_id: string; email: string }>(
      'POST', '/auth/login', { email, password },
    )
  },

  // ── clusters ──
  listClusters()                  { return request<Cluster[]>('GET', '/api/v1/clusters') },
  getCluster(id: string)          { return request<Cluster>('GET', `/api/v1/clusters/${id}`) },
  createCluster(body: {
    cluster_name:     string
    prometheus_url:   string
    prometheus_token: string
    lookback_window?: string
  }) {
    return request<Cluster>('POST', '/api/v1/clusters', body)
  },
  updateCluster(id: string, body: Partial<{
    cluster_name:     string
    prometheus_url:   string
    prometheus_token: string
    lookback_window:  string
  }>) {
    return request<Cluster>('PUT', `/api/v1/clusters/${id}`, body)
  },
  deleteCluster(id: string) {
    return request<void>('DELETE', `/api/v1/clusters/${id}`)
  },

  // ── recommendations ──
  listAllRecommendations() {
    return request<RecommendationWithCluster[]>('GET', '/api/v1/recommendations')
  },
  listRecommendations(clusterId: string) {
    return request<Recommendation[]>('GET', `/api/v1/clusters/${clusterId}/recommendations`)
  },
  recalculate(clusterId: string) {
    return request<{ message: string; cluster_id: string }>(
      'POST', `/api/v1/clusters/${clusterId}/recalculate`,
    )
  },
}
