import axios from 'axios'

// create axios instance — base URL is empty so Vite proxy handles /api/* → localhost:8080
const api = axios.create({
  headers: { 'Content-Type': 'application/json' },
})

// attach JWT token to every request automatically
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// ── Auth ─────────────────────────────────────────────────────────────────────

export const login = (email: string, password: string) =>
  api.post<{ token: string; user_id: string; email: string }>('/auth/login', { email, password })

export const register = (email: string, password: string) =>
  api.post<{ token: string; user_id: string; email: string }>('/auth/register', { email, password })

// ── Clusters ─────────────────────────────────────────────────────────────────

export interface Cluster {
  cluster_id:      string
  name:            string
  prometheus_url:  string
  lookback_window: string
  created_at:      string
  updated_at:      string
}

export interface CreateClusterPayload {
  name:            string
  prometheus_url:  string
  token:           string
  lookback_window?: string
}

export const getClusters = () =>
  api.get<Cluster[]>('/api/v1/clusters')

export const getCluster = (id: string) =>
  api.get<Cluster>(`/api/v1/clusters/${id}`)

export const createCluster = (payload: CreateClusterPayload) =>
  api.post<Cluster>('/api/v1/clusters', payload)

export const deleteCluster = (id: string) =>
  api.delete(`/api/v1/clusters/${id}`)

// ── Recommendations ───────────────────────────────────────────────────────────

export interface Recommendation {
  recommendation_id:    string
  cluster_id:           string
  namespace:            string
  pod_name:             string
  container_name:       string
  status:               string
  current_cpu_limit:    number
  current_mem_limit:    number
  p99_cpu:              number
  p99_mem:              number
  recommended_cpu_limit: number
  recommended_mem_limit: number
  lookback_window:      string
  updated_at:           string
}

export const getRecommendations = (clusterId: string) =>
  api.get<Recommendation[]>(`/api/v1/clusters/${clusterId}/recommendations`)

export const recalculate = (clusterId: string) =>
  api.post(`/api/v1/clusters/${clusterId}/recalculate`)
