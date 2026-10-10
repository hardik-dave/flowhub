import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError, dash } from '../api/client'
import { Badge, Button, Card, ErrorText, Field, Input, Modal, Select } from '../components/ui'
import type { UserView, CreateUserResponse, UsersQuery } from '../api/types'

export function UsersPage() {
  const qc = useQueryClient()
  const [filters, setFilters] = useState<UsersQuery>({ page: 1, q: '', status: '', product: '' })
  const [showCreate, setShowCreate] = useState(false)
  const [created, setCreated] = useState<CreateUserResponse | null>(null)
  const [error, setError] = useState('')

  const users = useQuery({
    queryKey: ['users', filters],
    queryFn: () => dash.listUsers(filters),
  })

  const tenant = useQuery({
    queryKey: ['tenant'],
    queryFn: () => dash.getTenant(),
  })
  const products = tenant.data?.products ?? []
  const today = new Date().toISOString().slice(0, 10)

  const createUser = useMutation({
    mutationFn: (body: Record<string, unknown>) => dash.createUser(body),
    onSuccess: (data) => {
      setCreated(data)
      qc.invalidateQueries({ queryKey: ['users'] })
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Could not create the user.')
    },
  })

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold text-slate-800">Users</h1>
          <p className="text-sm text-slate-500">Tenant-scoped list of accounts.</p>
        </div>
        <Button onClick={() => { setShowCreate(true); setCreated(null); setError('') }}>Create user</Button>
      </div>

      <Card>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-4">
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
              onChange={(e) => setFilters((f) => ({ ...f, status: e.target.value || undefined, page: 1 }))}
            >
              <option value="">Any</option>
              <option value="ACTIVE">ACTIVE</option>
              <option value="PENDING_VERIFICATION">PENDING VERIFICATION</option>
              <option value="DEACTIVATED">DEACTIVATED</option>
              <option value="BANNED">BANNED</option>
            </Select>
          </Field>
          <Field label="Product">
            <Input
              placeholder="e.g. flowos"
              value={filters.product ?? ''}
              onChange={(e) => setFilters((f) => ({ ...f, product: e.target.value || undefined, page: 1 }))}
            />
          </Field>
        </div>
      </Card>

      <Card>
        {users.isLoading ? <p className="text-sm text-slate-500">Loading users…</p> : null}
        {users.error ? <ErrorText>{users.error instanceof ApiError ? users.error.message : String(users.error)}</ErrorText> : null}
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-50">
              <tr>
                <th className="px-3 py-2 text-left font-medium text-slate-600">ID</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Username</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Name</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Mobile</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Status</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Role</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Licenses</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 bg-white">
              {(users.data?.users ?? []).map((u: UserView) => (
                <tr key={u.id} className="hover:bg-slate-50">
                  <td className="px-3 py-2 text-slate-600">{u.id}</td>
                  <td className="px-3 py-2 font-medium text-slate-800">{u.username}</td>
                  <td className="px-3 py-2 text-slate-600">{[u.first_name, u.last_name].filter(Boolean).join(' ') || '—'}</td>
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
                    <a className="text-indigo-600 hover:text-indigo-500" href={`/users/${u.id}`}>View</a>
                  </td>
                </tr>
              ))}
              {users.data && users.data.users.length === 0 ? (
                <tr><td colSpan={8} className="px-3 py-6 text-center text-sm text-slate-500">No users found.</td></tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </Card>

      {showCreate && (
        <Modal title="Create user" onClose={() => setShowCreate(false)}>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              setError('')
              const fd = new FormData(e.currentTarget)
              const body: Record<string, unknown> = {}
              for (const [k, v] of fd.entries()) {
                const s = String(v).trim()
                if (s) body[k] = s
              }
              if (!body.plan) {
                delete body.paid_at
                delete body.valid_from
              } else if (!body.paid_at || !body.valid_from) {
                setError('Pick a plan and both dates together.')
                return
              }
              createUser.mutate(body)
            }}
          >
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
              <Field label="Username*"><Input name="username" required /></Field>
              <Field label="Mobile* (E.164)"><Input name="mobile" placeholder="+919876543210" required /></Field>
              <Field label="First name"><Input name="first_name" /></Field>
              <Field label="Last name"><Input name="last_name" /></Field>
              <Field label="Email"><Input name="email" type="email" /></Field>
              <Field label="City"><Input name="city" /></Field>
              <Field label="State"><Input name="state" /></Field>
              <Field label="Broker client code"><Input name="broker_client_code" /></Field>
              <Field label="Referral code"><Input name="referral_code" /></Field>
              <Field label="Product*">
                <Select name="product_code" required defaultValue={products[0] ?? ''}>
                  {products.length === 0 ? (
                    <option value="">No products granted to this tenant</option>
                  ) : (
                    products.map((code) => <option key={code} value={code}>{code}</option>)
                  )}
                </Select>
              </Field>
            </div>

            <div className="rounded-md border border-slate-200 p-3">
              <p className="mb-1 text-sm font-semibold text-slate-700">Access window</p>
              <p className="mb-2 text-xs text-slate-500">Choose a plan and dates so the user can log in. Leave the plan blank to create a user with no access yet (they will be paused).</p>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                <Field label="Plan">
                  <Select name="plan" defaultValue="">
                    <option value="">No access window</option>
                    <option value="MONTHLY">MONTHLY</option>
                    <option value="QUARTERLY">QUARTERLY</option>
                    <option value="ANNUAL">ANNUAL</option>
                  </Select>
                </Field>
                <Field label="Valid from"><Input name="valid_from" type="date" defaultValue={today} /></Field>
                <Field label="Paid at"><Input name="paid_at" type="date" defaultValue={today} /></Field>
              </div>
            </div>

            {error ? <ErrorText>{error}</ErrorText> : null}
            <div className="flex justify-end gap-2">
              <Button type="button" variant="secondary" onClick={() => setShowCreate(false)}>Cancel</Button>
              <Button type="submit" disabled={createUser.isPending}>Create</Button>
            </div>
          </form>
          {created && (
            <div className="mt-4 space-y-2 rounded-md bg-emerald-50 p-3 text-sm">
              <p className="font-medium text-emerald-800">User created. Secrets shown ONCE.</p>
              <p className="text-emerald-900"><span className="font-semibold">User ID:</span> {created.user_id}</p>
              <p className="text-emerald-900"><span className="font-semibold">Username:</span> {created.username}</p>
              <p className="text-emerald-900"><span className="font-semibold">Password:</span> {created.password}</p>
              <p className="text-emerald-900"><span className="font-semibold">License key:</span> {created.license_key}</p>
              {created.valid_until ? (
                <p className="text-emerald-900"><span className="font-semibold">Access valid until:</span> {created.valid_until}</p>
              ) : (
                <p className="text-amber-700">No access window set — this user will be paused until one is added.</p>
              )}
            </div>
          )}
        </Modal>
 
      )}
    </div>
  )
}
