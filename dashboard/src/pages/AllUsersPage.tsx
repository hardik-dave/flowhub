import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ApiError, dash } from '../api/client'
import { Badge, Button, Card, ErrorText, Field, Input, Select } from '../components/ui'
import { useAuth } from '../auth/AuthContext'
import type { AllUsersQuery } from '../api/types'

export function AllUsersPage() {
  const navigate = useNavigate()
  const { setActAsTenant } = useAuth()
  const [filters, setFilters] = useState<AllUsersQuery>({ page: 1, q: '', status: '', product: '' })

  const tenants = useQuery({
    queryKey: ['tenants'],
    queryFn: () => dash.listTenants(),
  })

  const users = useQuery({
    queryKey: ['all-users', filters],
    queryFn: () => dash.listAllUsers(filters),
  })

  function view(tenantId: number, userId: number) {
    setActAsTenant(tenantId)
    navigate(`/users/${userId}`)
  }

  const data = users.data
  const totalPages = data ? Math.max(1, Math.ceil(data.total / data.page_size)) : 1

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold text-slate-800">All users</h1>
        <p className="text-sm text-slate-500">Every user across every tenant (platform admin only).</p>
      </div>

      <Card>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
          <Field label="Tenant">
            <Select
              value={filters.tenant_id ?? ''}
              onChange={(e) =>
                setFilters((f) => ({ ...f, tenant_id: e.target.value ? Number(e.target.value) : undefined, page: 1 }))
              }
            >
              <option value="">All tenants</option>
              {(tenants.data?.tenants ?? []).map((t) => (
                <option key={t.id} value={t.id}>{t.id} · {t.name}</option>
              ))}
            </Select>
          </Field>
          <Field label="Search">
            <Input
              placeholder="username, name, mobile..."
              value={filters.q ?? ''}
              onChange={(e) => setFilters((f) => ({ ...f, q: e.target.value, page: 1 }))}
            />
          </Field>
          <Field label="Status">
            <Select
              value={filters.status ?? ''}
              onChange={(e) => setFilters((f) => ({ ...f, status: e.target.value, page: 1 }))}
            >
              <option value="">Any</option>
              <option value="ACTIVE">ACTIVE</option>
              <option value="DEACTIVATED">DEACTIVATED</option>
              <option value="BANNED">BANNED</option>
            </Select>
          </Field>
          <Field label="Product">
            <Input
              placeholder="e.g. optionalyzer"
              value={filters.product ?? ''}
              onChange={(e) => setFilters((f) => ({ ...f, product: e.target.value, page: 1 }))}
            />
          </Field>
        </div>
      </Card>

      <Card>
        {users.error ? (
          <ErrorText>{users.error instanceof ApiError ? users.error.message : String(users.error)}</ErrorText>
        ) : null}
        <div className="overflow-x-auto text-sm">
          <table className="min-w-full divide-y divide-slate-200">
            <thead className="bg-slate-50">
              <tr>
                <th className="px-3 py-2 text-left">Tenant</th>
                <th className="px-3 py-2 text-left">ID</th>
                <th className="px-3 py-2 text-left">Username</th>
                <th className="px-3 py-2 text-left">Name</th>
                <th className="px-3 py-2 text-left">Mobile</th>
                <th className="px-3 py-2 text-left">Status</th>
                <th className="px-3 py-2 text-left">Role</th>
                <th className="px-3 py-2 text-left">Licenses</th>
                <th className="px-3 py-2 text-left">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {(users.data?.users ?? []).map((u) => (
                <tr key={`${u.tenant_id}:${u.id}`}>
                  <td className="px-3 py-2 text-slate-600">{u.tenant_id} · {u.tenant_name}</td>
                  <td className="px-3 py-2 text-slate-600">{u.id}</td>
                  <td className="px-3 py-2 font-medium text-slate-800">{u.username}</td>
                  <td className="px-3 py-2 text-slate-600">
                    {[u.first_name, u.last_name].filter(Boolean).join(' ') || '—'}
                  </td>
                  <td className="px-3 py-2 text-slate-600">{u.mobile || '—'}</td>
                  <td className="px-3 py-2"><Badge value={u.status} /></td>
                  <td className="px-3 py-2 text-slate-600">{u.role || '—'}</td>
                  <td className="px-3 py-2 text-slate-600">
                    {(u.licenses ?? []).length === 0 ? '—' : (u.licenses ?? []).map((l) => (
                      <div key={l.id} className="flex items-center gap-2">
                        <Badge value={l.status} />
                        <span>{l.product_code} · …{l.key_hint}</span>
                      </div>
                    ))}
                  </td>
                  <td className="px-3 py-2">
                    <button
                      type="button"
                      className="text-indigo-600 hover:text-indigo-500"
                      onClick={() => view(u.tenant_id, u.id)}
                    >
                      View
                    </button>
                  </td>
                </tr>
              ))}
              {users.data && users.data.users.length === 0 ? (
                <tr>
                  <td colSpan={9} className="px-3 py-6 text-center text-sm text-slate-500">No users found.</td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
        {users.data && users.data.total > users.data.page_size ? (
          <div className="mt-3 flex items-center justify-between text-sm text-slate-500">
            <span>Page {users.data.page} of {totalPages} · {users.data.total} users</span>
            <div className="flex gap-2">
              <Button
                variant="secondary"
                disabled={users.data.page <= 1}
                onClick={() => setFilters((f) => ({ ...f, page: (f.page ?? 1) - 1 }))}
              >
                Previous
              </Button>
              <Button
                variant="secondary"
                disabled={users.data.page >= totalPages}
                onClick={() => setFilters((f) => ({ ...f, page: (f.page ?? 1) + 1 }))}
              >
                Next
              </Button>
            </div>
          </div>
        ) : null}
      </Card>
    </div>
  )
}
