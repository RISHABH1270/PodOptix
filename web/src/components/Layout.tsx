import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { Layers, LogOut } from 'lucide-react'
import { auth } from '../lib/auth'
import { Logo } from './Logo'

export function Layout() {
  const navigate = useNavigate()
  const email = auth.getEmail() ?? 'user'

  const logout = () => {
    auth.clear()
    navigate('/login')
  }

  return (
    <div className="flex h-screen bg-bg text-ink">
      {/* ── sidebar ───────────────────────────────────────────── */}
      <aside className="w-60 flex-shrink-0 bg-surface border-r border-border flex flex-col">
        <div className="px-5 py-5 border-b border-border flex items-center gap-3">
          <Logo size={36} className="rounded-lg flex-shrink-0" />
          <div>
            <div className="text-sm font-semibold text-ink leading-tight">PodOptix</div>
            <div className="text-[10px] uppercase tracking-widest text-dim">v0.1.0</div>
          </div>
        </div>

        <nav className="flex-1 py-3">
          <div className="px-5 pt-3 pb-2 text-[10px] uppercase tracking-widest text-dim font-semibold">
            Overview
          </div>
          <SideLink to="/clusters" icon={<Layers className="w-4 h-4" />} label="Clusters" />
        </nav>

        <div className="border-t border-border p-3">
          <div className="px-2 py-2 flex items-center justify-between">
            <div className="min-w-0 flex-1">
              <div className="text-xs text-muted truncate" title={email}>{email}</div>
              <div className="text-[10px] uppercase tracking-widest text-dim">Signed in</div>
            </div>
            <button
              onClick={logout}
              className="p-2 rounded hover:bg-elevated text-dim hover:text-danger transition"
              title="Log out"
            >
              <LogOut className="w-4 h-4" />
            </button>
          </div>
        </div>
      </aside>

      {/* ── main ──────────────────────────────────────────────── */}
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  )
}

function SideLink({ to, icon, label }: { to: string; icon: React.ReactNode; label: string }) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        `px-5 py-2 flex items-center gap-3 text-sm transition border-l-2 ${
          isActive
            ? 'text-ink bg-elevated border-accent'
            : 'text-muted border-transparent hover:text-ink hover:bg-elevated/50'
        }`
      }
    >
      {icon}
      <span>{label}</span>
    </NavLink>
  )
}
