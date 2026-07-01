import { useNavigate } from 'react-router-dom'

const nav = [
  { label: 'Overview', icon: '⊞', path: '/overview' },
  { label: 'Clusters', icon: '◈', path: '/clusters' },
]

export default function Sidebar({ active }: { active: string }) {
  const navigate = useNavigate()
  const email    = localStorage.getItem('email') || ''
  const initials = email.slice(0, 2).toUpperCase()

  function logout() {
    localStorage.clear()
    navigate('/login')
  }

  return (
    <aside className="w-56 bg-gray-900 border-r border-gray-800 flex flex-col h-screen fixed left-0 top-0">
      <div className="px-5 py-5 border-b border-gray-800">
        <span className="text-green-400 font-bold text-lg tracking-wide">PodOptix</span>
      </div>

      <nav className="flex-1 px-3 py-4 space-y-1">
        {nav.map(item => (
          <button key={item.label}
            onClick={() => item.path && navigate(item.path)}
            className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm transition-colors text-left
              ${active === item.label
                ? 'bg-green-500/10 text-green-400 font-medium'
                : 'text-gray-400 hover:text-white hover:bg-gray-800'}`}>
            <span className="text-base">{item.icon}</span>
            {item.label}
          </button>
        ))}
      </nav>

      <div className="px-4 py-4 border-t border-gray-800">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 bg-green-500 rounded-full flex items-center justify-center text-black text-xs font-bold shrink-0">
            {initials}
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-white text-xs font-medium truncate">{email}</p>
          </div>
          <button onClick={logout} title="Sign out" className="text-gray-500 hover:text-red-400 text-xs">↪</button>
        </div>
      </div>
    </aside>
  )
}
