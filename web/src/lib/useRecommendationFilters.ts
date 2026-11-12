import { useMemo, useState } from 'react'

/**
 * Shared filter / sort state for the recommendations table on both
 * the Cluster Detail page (single-cluster) and the cross-cluster
 * Recommendations page. Keeps the filter semantics in one place so
 * both pages behave identically when the operator toggles the same
 * filter from either view.
 */

// Minimum shape every row must have so the filters work.
// Both Recommendation and RecommendationWithCluster satisfy this.
export interface FilterableRec {
  namespace: string
  workload_name: string
  container_name: string
  status: 'ready' | 'new_service'
  applied: boolean
  first_missed_at: string | null
  current_cpu_request: number
  current_cpu_limit: number
  current_mem_request: number
  current_mem_limit: number
  recommended_cpu_request: number
  recommended_cpu_limit: number
  recommended_mem_request: number
  recommended_mem_limit: number
}

export interface FilterState {
  search: string
  // STATUS group
  showReady: boolean
  showNewService: boolean
  // APPLIED group
  showPending: boolean    // applied === false
  showApplied: boolean    // applied === true
  // LIFECYCLE group
  showAlive: boolean      // first_missed_at === null
  showOrphaned: boolean   // first_missed_at !== null  (default off)
  sortKey: string
}

export const DEFAULT_FILTERS: FilterState = {
  search: '',
  showReady: true,
  showNewService: true,
  showPending: true,
  showApplied: true,
  showAlive: true,
  showOrphaned: false,
  sortKey: 'cpu_delta',
}

/**
 * countActive — how many filter checkboxes diverge from the default
 * (default = everything on except orphaned). Drives the "Filters (N)"
 * badge on the toolbar button so the operator can see at a glance
 * whether they're looking at a filtered or unfiltered view.
 */
export function countActive(f: FilterState): number {
  let n = 0
  if (f.showReady       !== DEFAULT_FILTERS.showReady)       n++
  if (f.showNewService  !== DEFAULT_FILTERS.showNewService)  n++
  if (f.showPending     !== DEFAULT_FILTERS.showPending)     n++
  if (f.showApplied     !== DEFAULT_FILTERS.showApplied)     n++
  if (f.showAlive       !== DEFAULT_FILTERS.showAlive)       n++
  if (f.showOrphaned    !== DEFAULT_FILTERS.showOrphaned)    n++
  return n
}

/**
 * Sort options per page. Cluster Detail doesn't have a "cluster" sort
 * because it's already scoped to one cluster; cross-cluster Recommendations
 * does. Both share the delta/namespace options.
 */
export const CLUSTER_DETAIL_SORT: SortOption[] = [
  { value: 'cpu_delta', label: 'Biggest CPU waste' },
  { value: 'mem_delta', label: 'Biggest memory waste' },
  { value: 'namespace', label: 'Namespace' },
  { value: 'workload',  label: 'Workload name' },
]

export const RECOMMENDATIONS_SORT: SortOption[] = [
  { value: 'cpu_delta', label: 'Biggest CPU waste' },
  { value: 'mem_delta', label: 'Biggest memory waste' },
  { value: 'cluster',   label: 'Cluster' },
  { value: 'namespace', label: 'Namespace' },
]

export interface SortOption {
  value: string
  label: string
}

/**
 * applyFilters — returns a NEW array of rows that pass every filter and
 * are sorted by the chosen sort key. Memoised by the caller via useMemo.
 */
export function applyFilters<T extends FilterableRec & { cluster_name?: string }>(
  items: T[],
  f: FilterState,
): T[] {
  const q = f.search.trim().toLowerCase()

  const filtered = items.filter((r) => {
    // search ─ namespace / workload / container
    if (q) {
      const hit =
        r.namespace.toLowerCase().includes(q) ||
        r.workload_name.toLowerCase().includes(q) ||
        r.container_name.toLowerCase().includes(q)
      if (!hit) return false
    }
    // STATUS group — at least one checked box must match the row
    const okStatus =
      (r.status === 'ready'       && f.showReady) ||
      (r.status === 'new_service' && f.showNewService)
    if (!okStatus) return false
    // APPLIED group
    const okApplied =
      (r.applied  && f.showApplied) ||
      (!r.applied && f.showPending)
    if (!okApplied) return false
    // LIFECYCLE group
    const isOrphan = !!r.first_missed_at
    const okLifecycle =
      (isOrphan  && f.showOrphaned) ||
      (!isOrphan && f.showAlive)
    if (!okLifecycle) return false
    return true
  })

  const sorted = [...filtered]
  switch (f.sortKey) {
    case 'cpu_delta':
      sorted.sort((a, b) =>
        (b.current_cpu_limit - b.recommended_cpu_limit) -
        (a.current_cpu_limit - a.recommended_cpu_limit),
      )
      break
    case 'mem_delta':
      sorted.sort((a, b) =>
        (b.current_mem_limit - b.recommended_mem_limit) -
        (a.current_mem_limit - a.recommended_mem_limit),
      )
      break
    case 'cluster':
      sorted.sort((a, b) => (a.cluster_name ?? '').localeCompare(b.cluster_name ?? ''))
      break
    case 'namespace':
      sorted.sort((a, b) => a.namespace.localeCompare(b.namespace))
      break
    case 'workload':
      sorted.sort((a, b) => a.workload_name.localeCompare(b.workload_name))
      break
  }
  return sorted
}

/**
 * useRecommendationFilters — convenience hook that bundles the state +
 * the memoised applied-and-sorted result. Pages just do:
 *     const { state, setState, visible } = useRecommendationFilters(rows)
 */
export function useRecommendationFilters<T extends FilterableRec & { cluster_name?: string }>(
  rows: T[],
  initial: Partial<FilterState> = {},
) {
  const [state, setState] = useState<FilterState>({ ...DEFAULT_FILTERS, ...initial })
  const visible = useMemo(() => applyFilters(rows, state), [rows, state])
  return { state, setState, visible }
}
