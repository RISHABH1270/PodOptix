import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import LoginPage from './pages/LoginPage'
import OverviewPage from './pages/OverviewPage'
import ClustersPage from './pages/ClustersPage'
import ClusterDetailPage from './pages/ClusterDetailPage'

const queryClient = new QueryClient()

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
          <Route path="/login"        element={<LoginPage />} />
          <Route path="/overview"     element={<ProtectedRoute><OverviewPage /></ProtectedRoute>} />
          <Route path="/clusters"     element={<ProtectedRoute><ClustersPage /></ProtectedRoute>} />
          <Route path="/clusters/:id" element={<ProtectedRoute><ClusterDetailPage /></ProtectedRoute>} />
          <Route path="*"             element={<Navigate to="/overview" replace />} />
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
