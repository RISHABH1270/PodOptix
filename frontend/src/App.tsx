import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import LoginPage from './pages/LoginPage'
import ClustersPage from './pages/ClustersPage'
import ClusterDetailPage from './pages/ClusterDetailPage'

// create a React Query client — handles caching and data fetching
const queryClient = new QueryClient()

// ProtectedRoute — redirects to /login if no token stored
function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const token = localStorage.getItem('token')
  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login"       element={<LoginPage />} />
          <Route path="/clusters"    element={<ProtectedRoute><ClustersPage /></ProtectedRoute>} />
          <Route path="/clusters/:id" element={<ProtectedRoute><ClusterDetailPage /></ProtectedRoute>} />
          <Route path="*"            element={<Navigate to="/clusters" replace />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
