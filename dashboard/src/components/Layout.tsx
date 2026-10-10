import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { dash } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { Button } from './ui'

const linkBase = 'block rounded-md px-3 py-2 text-sm font-medium'

function NavItem({ to, label, end = false }: { to: string; label: string; end?: boolean }) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        `${linkBase} ${isActive ? 'bg-indigo-600 text-white' : 'text-slate-600 hover:bg-slate-100'}`
      }
    >
      {label}
    </NavLink>
  )
}

export function Layout() {
  const { user, isPlatformAdmin, logout, actAsTenant, setActAsTenant } = useAuth()
  const navigate = useNavigate()

  const tenants = useQuery({
    queryKey: ['tenants'],
    queryFn: () => dash.listTenants(),
    enabled: isPlatformAdmin,
  })

  async function handleLogout() {
    await logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="flex min-h-screen">
      <aside className="flex w-60 flex-col border-r border-slate-200 bg-white">
        <div className="px-4 py-4">
          <p className="text-lg font-bold text-indigo-700">FlowHub</p>
          <p className="text-xs text-slate-500">Licensing console</p>
        </div>
        <nav className="flex-1 space-y-1 px-2">
          <NavItem to="/" label="Users" end />
          <NavItem to="/import" label="CSV import" />
          <NavItem to="/tenant" label="Tenant settings" />
          {isPlatformAdmin ? <NavItem to="/tenants" label="Platform tenants" /> : null}
          {isPlatformAdmin ? <NavItem to="/all-users" label="All users" /> : null}
          <NavItem to="/tools" label="API tools" />
        </nav>
        <div className="border-t border-slate-200 p-3 text-xs text-slate-500">
          <p className="truncate">
            Signed in as <span className="font-medium text-slate-700">{user?.username}</span>
          </p>
          <p>Role: {user?.role}</p>
          <Button variant="secondary" className="mt-2 w-full" onClick={handleLogout}>
            Sign out
          </Button>
        </div>
      </aside>

      <div className="flex flex-1 flex-col">
        {isPlatformAdmin ? (
          <div className="flex items-center gap-3 border-b border-slate-200 bg-amber-50 px-6 py-2 text-sm">
            <span className="font-medium text-amber-800">Platform admin</span>
            <label className="flex items-center gap-2 text-amber-900">
              Acting on tenant:
              <select
                className="rounded-md border border-amber-300 bg-white px-2 py-1 text-sm"
                value={actAsTenant ?? ''}
                onChange={(e) => setActAsTenant(e.target.value ? Number(e.target.value) : null)}
              >
                <option value="">House ({user?.tenant_id})</option>
                {(tenants.data?.tenants ?? [])
                  .filter((t) => t.id !== user?.tenant_id)
                  .map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.id} · {t.name}
                    </option>
                  ))}
              </select>
            </label>
          </div>
        ) : null}

        <main className="flex-1 p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
