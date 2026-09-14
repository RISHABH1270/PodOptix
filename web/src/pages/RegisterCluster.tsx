import { FormEvent, useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { ArrowLeft, Server } from 'lucide-react'
import { api } from '../lib/api'

export function RegisterClusterPage() {
  const navigate = useNavigate()

  const [clusterName, setClusterName]         = useState('')
  const [prometheusURL, setPrometheusURL]     = useState('')
  const [prometheusToken, setPrometheusToken] = useState('')
  const [lookback, setLookback]               = useState('7d')
  const [error, setError]                     = useState<string | null>(null)
  const [loading, setLoading]                 = useState(false)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null); setLoading(true)
    try {
      const cluster = await api.createCluster({
        cluster_name:     clusterName,
        prometheus_url:   prometheusURL,
        prometheus_token: prometheusToken,
        lookback_window:  lookback,
      })
      navigate(`/clusters/${cluster.cluster_id}`)
    } catch (err: any) {
      setError(err.message ?? 'Failed to register cluster')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="p-8 max-w-3xl mx-auto">
      <Link to="/clusters" className="inline-flex items-center gap-2 text-sm text-muted hover:text-ink mb-6 transition">
        <ArrowLeft className="w-4 h-4" /> Back to clusters
      </Link>

      <div className="flex items-start gap-4 mb-8">
        <div className="w-10 h-10 rounded-lg bg-accentBg border border-accent/30 flex items-center justify-center flex-shrink-0">
          <Server className="w-5 h-5 text-accent" />
        </div>
        <div>
          <div className="text-[10px] uppercase tracking-widest text-accent font-semibold mb-1">New cluster</div>
          <h1 className="text-2xl font-bold text-ink">Register a Kubernetes cluster</h1>
          <p className="text-sm text-muted mt-1">
            PodOptix will ping Prometheus immediately and start collecting metrics in the background.
          </p>
        </div>
      </div>

      <form onSubmit={onSubmit} className="bg-surface border border-border rounded-lg shadow-card p-6 space-y-5">
        <Field
          label="Cluster name"
          value={clusterName}
          onChange={setClusterName}
          placeholder="production-us-east"
          hint="A human-friendly name — shown in the dashboard."
          autoFocus
          required
        />

        <Field
          label="Prometheus URL"
          value={prometheusURL}
          onChange={setPrometheusURL}
          placeholder="https://prometheus.your-cluster.example.com"
          hint="Full HTTPS endpoint. Must be reachable from the PodOptix Hub."
          required
        />

        <Field
          label="Prometheus token"
          type="password"
          value={prometheusToken}
          onChange={setPrometheusToken}
          placeholder="Bearer token for Prometheus authentication"
          hint="Encrypted at rest with AES-256-GCM. Never logged."
          required
        />

        <div>
          <div className="text-[11px] uppercase tracking-widest text-muted font-semibold mb-1.5">
            Lookback window
          </div>
          <div className="grid grid-cols-3 gap-2">
            {(['7d', '10d', '30d'] as const).map((w) => (
              <button
                key={w}
                type="button"
                onClick={() => setLookback(w)}
                className={`py-2 rounded-md border text-sm font-mono transition ${
                  lookback === w
                    ? 'bg-accentBg border-accent text-accent'
                    : 'bg-elevated border-border text-muted hover:text-ink hover:border-borderHi'
                }`}
              >
                {w}
              </button>
            ))}
          </div>
          <div className="text-xs text-dim mt-1.5">How much history to analyze for p99 computation.</div>
        </div>

        {error && (
          <div className="px-3 py-2 rounded-md bg-dangerBg border border-danger/30 text-danger text-sm">
            {error}
          </div>
        )}

        <div className="flex items-center gap-3 pt-2">
          <button
            type="submit"
            disabled={loading}
            className="bg-accent hover:bg-accentHi disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-semibold px-5 py-2.5 rounded-md transition"
          >
            {loading ? 'Registering…' : 'Register cluster'}
          </button>
          <Link to="/clusters" className="text-sm text-muted hover:text-ink px-4 py-2.5 transition">
            Cancel
          </Link>
        </div>
      </form>
    </div>
  )
}

function Field(props: {
  label: string
  value: string
  onChange: (v: string) => void
  placeholder?: string
  hint?: string
  type?: string
  autoFocus?: boolean
  required?: boolean
}) {
  return (
    <label className="block">
      <div className="text-[11px] uppercase tracking-widest text-muted font-semibold mb-1.5">
        {props.label}
      </div>
      <input
        type={props.type ?? 'text'}
        value={props.value}
        onChange={(e) => props.onChange(e.target.value)}
        placeholder={props.placeholder}
        autoFocus={props.autoFocus}
        required={props.required}
        className="w-full bg-elevated border border-border focus:border-accent rounded-md px-3 py-2 text-sm text-ink placeholder:text-dim outline-none transition font-mono"
      />
      {props.hint && <div className="text-xs text-dim mt-1.5">{props.hint}</div>}
    </label>
  )
}
