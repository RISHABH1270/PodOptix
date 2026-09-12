import { Navigate, Outlet } from 'react-router-dom'
import { auth } from '../lib/auth'

// Wraps protected pages — redirects to /login if no JWT.
export function ProtectedRoute() {
  return auth.isLoggedIn() ? <Outlet /> : <Navigate to="/login" replace />
}
