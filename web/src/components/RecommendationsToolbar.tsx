import { ReactNode, useEffect, useRef, useState } from 'react'
import { Search, SlidersHorizontal, ArrowUpDown, Check } from 'lucide-react'
import { FilterState, DEFAULT_FILTERS, countActive, SortOption } from '../lib/useRecommendationFilters'

/**
 * RecommendationsToolbar — one component used by both the Cluster Detail
 * page (single cluster) and the Recommendations page (cross-cluster) so
 * the two surfaces present the same filter/sort UX. The optional `before`
 * slot lets each page inject its own extra control before the shared
 * Filters/Sort block — Recommendations uses it to render the cluster
 * dropdown, Cluster Detail passes nothing.
 */
export function RecommendationsToolbar({
  state,
  onChange,
  sortOptions,
  before,
  rightSlot,
}: {
  state: FilterState
  onChange: (next: FilterState) => void
  sortOptions: SortOption[]
  before?: ReactNode
  rightSlot?: ReactNode
}) {
  return (
    <div className="px-5 py-4 border-b border-border flex items-center justify-between gap-3 flex-wrap">
      <div className="flex items-center gap-3 flex-wrap">
        {/* Search */}
        <div className="relative">
          <Search className="w-4 h-4 text-dim absolute left-3 top-1/2 -translate-y-1/2" />
          <input
            value={state.search}
            onChange={(e) => onChange({ ...state, search: e.target.value })}
            placeholder="Filter namespace, workload, container…"
            className="w-72 bg-elevated border border-border focus:border-accent rounded-md pl-9 pr-3 py-1.5 text-sm text-ink placeholder:text-dim outline-none transition"
          />
        </div>

        {before}

        {/* Multi-select Filters */}
        <FiltersDropdown state={state} onChange={onChange} />

        {/* Sort */}
        <SortDropdown state={state} onChange={onChange} options={sortOptions} />
      </div>

      {rightSlot}
    </div>
  )
}

/* ─────────────────────────────────────────────────────────── Filters ── */

function FiltersDropdown({
  state,
  onChange,
}: {
  state: FilterState
  onChange: (next: FilterState) => void
}) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const active = countActive(state)

  // click-outside closes
  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [open])

  const toggle = (key: keyof FilterState) =>
    onChange({ ...state, [key]: !state[key] } as FilterState)

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className={`inline-flex items-center gap-2 px-3 py-1.5 rounded-md border text-sm transition ${
          active > 0
            ? 'bg-accentBg border-accent/40 text-accent'
            : 'bg-elevated border-border text-ink hover:border-borderHi'
        }`}
      >
        <SlidersHorizontal className="w-3.5 h-3.5" />
        <span>Filters</span>
        {active > 0 && (
          <span className="inline-flex items-center justify-center min-w-[1.25rem] h-5 px-1 rounded-full bg-accent/20 text-accent text-[11px] font-mono">
            {active}
          </span>
        )}
      </button>

      {open && (
        <div className="absolute left-0 top-full mt-1.5 w-48 bg-surface border border-border rounded-md shadow-xl z-10 overflow-hidden">
          <FilterGroup label="Status">
            <FilterItem checked={state.showReady}      onClick={() => toggle('showReady')}      label="Ready" />
            <FilterItem checked={state.showNewService} onClick={() => toggle('showNewService')} label="New service" />
          </FilterGroup>
          <FilterGroup label="Applied">
            <FilterItem checked={state.showPending}    onClick={() => toggle('showPending')}    label="Pending" />
            <FilterItem checked={state.showApplied}    onClick={() => toggle('showApplied')}    label="Applied" />
          </FilterGroup>
          <FilterGroup label="Lifecycle" last={active === 0}>
            <FilterItem checked={state.showAlive}      onClick={() => toggle('showAlive')}      label="Alive" />
            <FilterItem checked={state.showOrphaned}   onClick={() => toggle('showOrphaned')}   label="Orphaned" />
          </FilterGroup>
          {active > 0 && (
            <button
              onClick={() => onChange({ ...DEFAULT_FILTERS, search: state.search, sortKey: state.sortKey })}
              className="w-full text-left px-3 py-2 text-xs text-muted hover:text-ink hover:bg-elevated border-t border-border transition"
            >
              Reset filters
            </button>
          )}
        </div>
      )}
    </div>
  )
}

function FilterGroup({ label, children, last }: { label: string; children: ReactNode; last?: boolean }) {
  return (
    <div className={last ? '' : 'border-b border-border/60'}>
      <div className="px-3 pt-2 pb-1 text-[9px] uppercase tracking-widest text-dim font-semibold">{label}</div>
      <div className="pb-1.5">{children}</div>
    </div>
  )
}

function FilterItem({
  checked,
  onClick,
  label,
}: {
  checked: boolean
  onClick: () => void
  label: string
}) {
  return (
    <button
      onClick={onClick}
      className="w-full flex items-center gap-2.5 px-3 py-1 text-[13px] text-ink hover:bg-elevated transition text-left"
    >
      <span
        className={`inline-flex items-center justify-center w-3.5 h-3.5 rounded-sm border flex-shrink-0 transition ${
          checked
            ? 'bg-accent/15 border-accent text-accent'
            : 'bg-elevated border-border text-transparent'
        }`}
      >
        <Check className="w-2.5 h-2.5 stroke-[3]" />
      </span>
      {label}
    </button>
  )
}

/* ──────────────────────────────────────────────────────────── Sort ──── */

function SortDropdown({
  state,
  onChange,
  options,
}: {
  state: FilterState
  onChange: (next: FilterState) => void
  options: SortOption[]
}) {
  return (
    <div className="flex items-center gap-2 text-xs text-muted">
      <ArrowUpDown className="w-3.5 h-3.5" />
      <span>Sort:</span>
      <select
        value={state.sortKey}
        onChange={(e) => onChange({ ...state, sortKey: e.target.value })}
        className="bg-elevated border border-border rounded-md px-2 py-1 text-xs text-ink outline-none"
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </div>
  )
}
