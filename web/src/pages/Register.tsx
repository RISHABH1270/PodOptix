import { FormEvent, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ArrowRight } from 'lucide-react'
import { api } from '../lib/api'
import { auth } from '../lib/auth'
import { Logo } from '../components/Logo'

export function RegisterPage() {
  const navigate = useNavigate()
  const [email, setEmail]       = useState('')
  const [password, setPassword] = useState('')
  const [error, setError]       = useState<string | null>(null)
  const [loading, setLoading]   = useState(false)

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null); setLoading(true)
    try {
      const res = await api.register(email, password)
      auth.save(res.token, res.email)
      navigate('/clusters')
    } catch (err: any) {
      setError(err.message ?? 'Registration failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-8">
      <form onSubmit={onSubmit} className="w-full max-w-sm space-y-6">
        <div className="text-center space-y-3">
          <Logo size={52} className="inline-block rounded-lg" />
          <div>
            <h2 className="text-2xl font-bold text-ink">Create your account</h2>
            <p className="text-sm text-muted mt-1">Start right-sizing pods in minutes.</p>
          </div>
        </div>

        <div className="space-y-4">
          <label className="block">
            <div className="text-[11px] uppercase tracking-widest text-muted font-semibold mb-1.5">Email</div>
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)}
              placeholder="you@company.com" required autoFocus
              className="w-full bg-surface border border-border focus:border-accent rounded-md px-3 py-2 text-sm text-ink placeholder:text-dim outline-none transition" />
          </label>
          <label className="block">
            <div className="text-[11px] uppercase tracking-widest text-muted font-semibold mb-1.5">Password</div>
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)}
              placeholder="At least 8 characters" required minLength={8}
              className="w-full bg-surface border border-border focus:border-accent rounded-md px-3 py-2 text-sm text-ink placeholder:text-dim outline-none transition" />
          </label>
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
          {loading ? 'Creating account…' : (<>Create account <ArrowRight className="w-4 h-4" /></>)}
        </button>

        <div className="text-sm text-muted text-center">
          Already have an account?{' '}
          <Link to="/login" className="text-accent hover:text-accentHi font-medium">
            Sign in
          </Link>
        </div>
      </form>
    </div>
  )
}
