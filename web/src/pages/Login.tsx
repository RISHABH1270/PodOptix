import { FormEvent, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ArrowRight } from 'lucide-react'
import { api } from '../lib/api'
import { auth } from '../lib/auth'
import { Logo } from '../components/Logo'

export function LoginPage() {
  const navigate = useNavigate()
  const [email, setEmail]       = useState('')
  const [password, setPassword] = useState('')
  const [error, setError]       = useState<string | null>(null)
  const [loading, setLoading]   = useState(false)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null); setLoading(true)
    try {
      const res = await api.login(email, password)
      auth.save(res.token, res.email)
      navigate('/clusters')
    } catch (err: any) {
      setError(err.message ?? 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen grid lg:grid-cols-2">
      {/* ── left: brand pane ─────────────────────────────────── */}
      <div className="hidden lg:flex flex-col justify-between p-12 bg-surface border-r border-border relative overflow-hidden">
        <div className="relative z-10 flex items-center gap-3">
          <Logo size={44} className="rounded-lg flex-shrink-0" />
          <div>
            <div className="text-lg font-semibold text-ink">PodOptix</div>
            <div className="text-[10px] uppercase tracking-widest text-dim">Right-sizing for Kubernetes</div>
          </div>
        </div>

        <div className="relative z-10 space-y-5 max-w-md">
          <h1 className="text-3xl font-bold text-ink leading-tight">
            Stop guessing<br />pod resource limits.
          </h1>
          <p className="text-muted leading-relaxed">
            PodOptix queries your Prometheus, computes p99 usage over a rolling window
            (7d, 10d, or 30d), and recommends limits at the engineering sweet spot — 2× the p99.
            One Hub. No sidecars inside your workload clusters.
          </p>
          <div className="grid grid-cols-3 gap-3 pt-4">
            <Stat value="p99×2" label="formula" />
            <Stat value="24h"   label="sync cadence" />
            <Stat value="0"     label="workload agents" />
          </div>
        </div>

        <div className="relative z-10 text-[11px] text-dim uppercase tracking-widest">
          v0.1.0 · Go 1.26 · Prometheus native
        </div>

        {/* ambient glow */}
        <div className="absolute -top-32 -left-32 w-96 h-96 rounded-full bg-accent/10 blur-3xl pointer-events-none" />
        <div className="absolute -bottom-32 -right-32 w-96 h-96 rounded-full bg-info/5 blur-3xl pointer-events-none" />
      </div>

      {/* ── right: form ──────────────────────────────────────── */}
      <div className="flex items-center justify-center p-8">
        <form onSubmit={onSubmit} className="w-full max-w-sm space-y-6">
          <div>
            <div className="text-[10px] uppercase tracking-widest text-accent font-semibold mb-2">
              Sign in
            </div>
            <h2 className="text-2xl font-bold text-ink">Welcome back</h2>
            <p className="text-sm text-muted mt-1">Sign in to your PodOptix account.</p>
          </div>

          <div className="space-y-4">
            <Field
              label="Email"
              type="email"
              value={email}
              onChange={setEmail}
              placeholder="you@company.com"
              autoFocus
            />
            <Field
              label="Password"
              type="password"
              value={password}
              onChange={setPassword}
              placeholder="••••••••"
            />
          </div>

          {error && (
            <div className="px-3 py-2 rounded-md bg-dangerBg border border-danger/30 text-danger text-sm">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full flex items-center justify-center gap-2 bg-accent hover:bg-accentHi disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-semibold py-2.5 rounded-md transition"
          >
            {loading ? 'Signing in…' : (<>Sign in <ArrowRight className="w-4 h-4" /></>)}
          </button>

          <div className="text-sm text-muted text-center">
            No account?{' '}
            <Link to="/register" className="text-accent hover:text-accentHi font-medium">
              Create one
            </Link>
          </div>
        </form>
      </div>
    </div>
  )
}

function Stat({ value, label }: { value: string; label: string }) {
  return (
    <div className="bg-elevated/60 border border-border rounded-md p-3">
      <div className="text-lg font-mono font-semibold text-accent">{value}</div>
      <div className="text-[10px] uppercase tracking-widest text-dim mt-1">{label}</div>
    </div>
  )
}

function Field(props: {
  label: string; type: string; value: string; onChange: (v: string) => void
  placeholder?: string; autoFocus?: boolean
}) {
  return (
    <label className="block">
      <div className="text-[11px] uppercase tracking-widest text-muted font-semibold mb-1.5">
        {props.label}
      </div>
      <input
        type={props.type}
        value={props.value}
        onChange={(e) => props.onChange(e.target.value)}
        placeholder={props.placeholder}
        autoFocus={props.autoFocus}
        required
        className="w-full bg-surface border border-border focus:border-accent rounded-md px-3 py-2 text-sm text-ink placeholder:text-dim outline-none transition"
      />
    </label>
  )
}
