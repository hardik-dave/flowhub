import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthProvider, useAuth } from './auth/AuthContext'
import { RequireAuth } from './auth/RequireAuth'
import { ErrorBoundary } from './components/ErrorBoundary'
import { Layout } from './components/Layout'
import { LoginPage } from './pages/LoginPage'
import { UsersPage } from './pages/UsersPage'
import { UserDetailPage } from './pages/UserDetailPage'
import { ImportPage } from './pages/ImportPage'
import { TenantPage } from './pages/TenantPage'
import { TenantsPage } from './pages/TenantsPage'
import { ToolsPage } from './pages/ToolsPage'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
})

function TenantsRouteGuard() {
  const { isPlatformAdmin } = useAuth()
  if (!isPlatformAdmin) return <Navigate to="/" replace />
  return <TenantsPage />
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <BrowserRouter>
          <ErrorBoundary>
            <Routes>
              <Route path="/login" element={<LoginPage />} />
              <Route element={<RequireAuth />}>
                <Route element={<Layout />}>
                  <Route index element={<UsersPage />} />
                  <Route path="users/:id" element={<UserDetailPage />} />
                  <Route path="import" element={<ImportPage />} />
                  <Route path="tenant" element={<TenantPage />} />
                  <Route path="tenants" element={<TenantsRouteGuard />} />
                  <Route path="tools" element={<ToolsPage />} />
                  <Route path="*" element={<Navigate to="/" replace />} />
                </Route>
              </Route>
            </Routes>
          </ErrorBoundary>
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  )
}
