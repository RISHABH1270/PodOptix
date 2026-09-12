import { Navigate, Route, Routes } from 'react-router-dom'
import { LoginPage } from './pages/Login'
import { RegisterPage } from './pages/Register'
import { ClustersPage } from './pages/Clusters'
import { RegisterClusterPage } from './pages/RegisterCluster'
import { EditClusterPage } from './pages/EditCluster'
import { ClusterDetailPage } from './pages/ClusterDetail'
import { Layout } from './components/Layout'
import { ProtectedRoute } from './components/ProtectedRoute'

export default function App() {
  return (
    <Routes>
      {/* public */}
      <Route path="/login"    element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />

      {/* protected — everything below requires JWT */}
      <Route element={<ProtectedRoute />}>
        <Route element={<Layout />}>
          <Route path="/"                 element={<Navigate to="/clusters" replace />} />
          <Route path="/clusters"         element={<ClustersPage />} />
          <Route path="/clusters/new"      element={<RegisterClusterPage />} />
          <Route path="/clusters/:id"      element={<ClusterDetailPage />} />
          <Route path="/clusters/:id/edit" element={<EditClusterPage />} />
        </Route>
      </Route>

      {/* catch-all */}
      <Route path="*" element={<Navigate to="/clusters" replace />} />
    </Routes>
  )
}
