import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { login, register } from '../api/client'

export default function LoginPage() {
  const navigate = useNavigate()
  const [isRegister, setIsRegister] = useState(false)
  const [email, setEmail]           = useState('')
  const [password, setPassword]     = useState('')
  const [showPassword, setShowPassword] = useState(false)
const [error, setError]           = useState('')
  const [loading, setLoading]       = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const fn = isRegister ? register : login
      const response = await fn(email, password)
      localStorage.setItem('token', response.data.token)
      localStorage.setItem('email', response.data.email)
      navigate('/clusters')
    } catch {
      setError(isRegister ? 'Account with this email id already present. Try a different email.' : 'Invalid email or password')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-gray-950 flex items-center justify-center px-4"
      style={{ backgroundImage: 'radial-gradient(circle at 20% 80%, rgba(74,222,128,0.04) 0%, transparent 50%), radial-gradient(circle at 80% 20%, rgba(74,222,128,0.04) 0%, transparent 50%)' }}>

      <div className="w-full max-w-sm">

        {/* Logo */}
        <div className="text-center mb-8">
          <div className="flex items-center justify-center mb-2">
            <h1 className="text-4xl font-bold text-green-400 tracking-wide">PodOptix</h1>
          </div>
          <p className="text-green-600 text-xs font-mono tracking-[0.25em] uppercase">
            Kubernetes Resource Right-Sizing
          </p>
        </div>

        {/* Card */}
        <div className="bg-gray-900 border border-gray-800 rounded-2xl p-7 shadow-xl">
          <h2 className="text-white text-2xl font-bold mb-1">
            {isRegister ? 'Create account' : 'Welcome back'}
          </h2>
          <p className="text-gray-500 text-sm mb-6">
            {isRegister ? 'Register a new PodOptix user account' : 'Sign in to your PodOptix user account'}
          </p>

          <form onSubmit={handleSubmit} className="space-y-4">

            {/* Email */}
            <div>
              <label className="text-gray-400 text-sm block mb-1.5">Email</label>
              <div className="flex items-center bg-gray-800 border border-gray-700 focus-within:border-green-500 rounded-lg px-3 py-2.5 gap-2.5 transition-colors">
                <svg className="text-gray-500 shrink-0" width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/>
                </svg>
                <input
                  type="email"
                  value={email}
                  onChange={e => setEmail(e.target.value)}
                  required
                  className="bg-transparent text-white text-sm w-full focus:outline-none placeholder-gray-600"
                  placeholder=""
                />
              </div>
            </div>

            {/* Password */}
            <div>
              <label className="text-gray-400 text-sm block mb-1.5">Password</label>
              <div className="flex items-center bg-gray-800 border border-gray-700 focus-within:border-green-500 rounded-lg px-3 py-2.5 gap-2.5 transition-colors">
                <svg className="text-gray-500 shrink-0" width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
                </svg>
                <input
                  type={showPassword ? 'text' : 'password'}
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  required
                  className="bg-transparent text-white text-sm w-full focus:outline-none placeholder-gray-600"
                  placeholder=""
                />
                <button type="button" onClick={() => setShowPassword(!showPassword)} className="text-gray-500 hover:text-gray-300">
                  {showPassword ? (
                    <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"/>
                    </svg>
                  ) : (
                    <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"/>
                    </svg>
                  )}
                </button>
              </div>
            </div>


            {/* Error */}
            {error && <p className="text-red-400 text-sm">{error}</p>}

            {/* Sign in button */}
            <button
              type="submit"
              disabled={loading}
              className="w-full bg-green-500 hover:bg-green-400 disabled:bg-green-800 text-black font-semibold rounded-lg py-3 text-sm transition-colors flex items-center justify-center gap-2"
            >
              {loading ? (isRegister ? 'Creating account...' : 'Signing in...') : isRegister ? <>Create account <span>→</span></> : <>Sign in <span>→</span></>}
            </button>

          </form>

          {/* SSO divider */}
          <div className="flex items-center gap-3 my-5">
            <span className="h-px flex-1 bg-gray-800" />
            <span className="text-gray-600 text-xs">or continue with</span>
            <span className="h-px flex-1 bg-gray-800" />
          </div>

          <button className="w-full border border-gray-700 hover:border-gray-600 text-gray-300 rounded-lg py-2.5 text-sm flex items-center justify-center gap-2 transition-colors">
            <svg width="16" height="16" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/>
            </svg>
            Sign in with SSO
          </button>
        </div>

        <p className="text-center text-gray-500 text-sm mt-5">
          {isRegister ? 'Already have an account?' : "Don't have an account?"}
          {' '}
          <button onClick={() => { setIsRegister(!isRegister); setError('') }}
            className="text-green-500 hover:text-green-400 font-medium">
            {isRegister ? 'Sign in' : 'Register'}
          </button>
        </p>
        <p className="text-center text-gray-700 text-xs mt-3">PodOptix v0.1.0</p>
      </div>
    </div>
  )
}
