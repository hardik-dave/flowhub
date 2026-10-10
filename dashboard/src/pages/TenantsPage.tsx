import { useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError, dash } from '../api/client'
import type { CreateTenantInput, CreateTenantResponse, TenantView } from '../api/types'
import { OneTimeSecret } from '../components/OneTimeSecret'
import { StatusModal } from '../components/StatusModal'
import { Badge, Button, Card, ErrorText, Field, Input, Modal } from '../components/ui'
import { PRODUCTS } from '../lib/products'
import { useAuth } from '../auth/AuthContext'

const TENANT_STATUSES = [
  { value: 'ACTIVE', label: 'ACTIVE' },
  { value: 'INACTIVE', label: 'INACTIVE' },
]

export function TenantsPage() {
  const qc = useQueryClient()
  const { isPlatformAdmin } = useAuth()
  const [createOpen, setCreateOpen] = useState(false)
  const [created, setCreated] = useState<CreateTenantResponse | null>(null)
  const [statusFor, setStatusFor] = useState<TenantView | null>(null)
  const [error, setError] = useState('')

  const tenants = useQuery({
    queryKey: ['tenants'],
    queryFn: () => dash.listTenants(),
  })

  const createTenant = useMutation({
    mutationFn: (body: CreateTenantInput) => dash.createTenant(body),
    onSuccess: (data) => {
      setCreated(data)
      setError('')
      qc.invalidateQueries({ queryKey: ['tenants'] })
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : 'Could not create the tenant.'),
  })

  const updateStatus = useMutation({
    mutationFn: (v: { id: number; status: string; reason: string }) =>
      dash.updateTenantStatus(v.id, v.status, v.reason),
    onSuccess: () => {
      setStatusFor(null)
      setError('')
      qc.invalidateQueries({ queryKey: ['tenants'] })
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : 'Could not change the tenant status.'),
  })

  function handleCreate(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    const products = PRODUCTS.map((p) => p.code).filter((code) => fd.get(`product_${code}`) !== null)
    const body: CreateTenantInput = {
      slug: String(fd.get('slug') ?? '').trim(),
      name: String(fd.get('name') ?? '').trim(),
      contact_person: String(fd.get('contact_person') ?? '').trim(),
      contact_no: String(fd.get('contact_no') ?? '').trim(),
      start_date: String(fd.get('start_date') ?? '').trim(),
      grace_working_days: Number(fd.get('grace_working_days') ?? 5),
      products,
      admin: {
        username: String(fd.get('username') ?? '').trim(),
        password: String(fd.get('password') ?? ''),
        first_name: String(fd.get('first_name') ?? '').trim(),
        last_name: String(fd.get('last_name') ?? '').trim(),
        mobile: String(fd.get('mobile') ?? '').trim(),
        email: String(fd.get('email') ?? '').trim(),
      },
    }
    createTenant.mutate(body)
  }

  const today = new Date().toISOString().slice(0, 10)

  if (!isPlatformAdmin) return <ErrorText>Platform admin access required.</ErrorText>

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold text-slate-800">Platform tenants</h1>
          <p className="text-sm text-slate-500">Onboard and manage broker tenants.</p>
        </div>
        <Button
          onClick={() => {
            setCreateOpen(true)
            setCreated(null)
            setError('')
          }}
        >
          Create tenant
        </Button>
      </div>

      <Card>
        {tenants.error ? (
          <ErrorText>{tenants.error instanceof ApiError ? tenants.error.message : String(tenants.error)}</ErrorText>
        ) : null}
        <div className="overflow-x-auto text-sm">
          <table className="min-w-full divide-y divide-slate-200">
            <thead className="bg-slate-50">
              <tr>
                <th className="px-3 py-2 text-left">ID</th>
                <th className="px-3 py-2 text-left">Slug</th>
                <th className="px-3 py-2 text-left">Name</th>
                <th className="px-3 py-2 text-left">Status</th>
                <th className="px-3 py-2 text-left">House</th>
                <th className="px-3 py-2 text-left">Contact</th>
                <th className="px-3 py-2 text-left">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 bg-white">
              {(tenants.data?.tenants ?? []).map((t: TenantView) => (
                <tr key={t.id}>
                  <td className="px-3 py-2 text-slate-600">{t.id}</td>
                  <td className="px-3 py-2 text-slate-600">{t.slug}</td>
                  <td className="px-3 py-2 text-slate-600">{t.name}</td>
                  <td className="px-3 py-2"><Badge value={t.status} /></td>
                  <td className="px-3 py-2 text-slate-600">{t.is_house ? 'Yes' : 'No'}</td>
                  <td className="px-3 py-2 text-slate-600">{t.contact_person} · {t.contact_no}</td>
                  <td className="px-3 py-2">
                    <Button
                      variant="secondary"
                      onClick={() => {
                        setError('')
                        setStatusFor(t)
                      }}
                    >
                      Change status
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>

      {createOpen ? (
        <Modal
          title="Create tenant"
          onClose={() => {
            setCreateOpen(false)
            setCreated(null)
            setError('')
          }}
        >
          {created ? (
            <div className="space-y-3">
              <div className="space-y-1 text-sm text-slate-700">
                <p><span className="font-semibold">Tenant ID:</span> {created.tenant_id}</p>
                <p><span className="font-semibold">Admin user ID:</span> {created.admin_user_id}</p>
                <p><span className="font-semibold">Admin username:</span> {created.admin_username}</p>
              </div>
              {created.password ? (
                <OneTimeSecret label="Admin password" value={created.password} />
              ) : (
                <p className="text-xs text-slate-500">The admin uses the password you supplied.</p>
              )}
              <div className="flex justify-end">
                <Button
                  onClick={() => {
                    setCreateOpen(false)
                    setCreated(null)
                  }}
                >
                  Done
                </Button>
              </div>
            </div>
          ) : (
            <form className="space-y-4" onSubmit={handleCreate}>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                <Field label="Slug *">
                  <Input name="slug" placeholder="acme-brokers" required pattern="[a-z0-9]([a-z0-9-]{0,62}[a-z0-9])?" />
                </Field>
                <Field label="Name *">
                  <Input name="name" required />
                </Field>
                <Field label="Contact person">
                  <Input name="contact_person" />
                </Field>
                <Field label="Contact no">
                  <Input name="contact_no" placeholder="+919876543210" />
                </Field>
                <Field label="Start date *">
                  <Input name="start_date" type="date" defaultValue={today} required />
                </Field>
                <Field label="Grace working days">
                  <Input name="grace_working_days" type="number" min={0} max={30} defaultValue={5} />
                </Field>
              </div>

              <fieldset>
                <legend className="mb-2 text-sm font-medium text-slate-700">Products *</legend>
                <div className="grid grid-cols-2 gap-2 md:grid-cols-3">
                  {PRODUCTS.map((p) => (
                    <label key={p.code} className="flex items-center gap-2 text-sm text-slate-700">
                      <input
                        type="checkbox"
                        name={`product_${p.code}`}
                        defaultChecked={p.code === 'flowos'}
                      />
                      {p.label} <span className="text-slate-400">({p.code})</span>
                    </label>
                  ))}
                </div>
              </fieldset>

              <fieldset>
                <legend className="mb-2 text-sm font-medium text-slate-700">Tenant admin</legend>
                <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                  <Field label="Username *">
                    <Input name="username" required />
                  </Field>
                  <Field label="Password (blank = generate)">
                    <Input name="password" type="text" autoComplete="new-password" />
                  </Field>
                  <Field label="First name">
                    <Input name="first_name" />
                  </Field>
                  <Field label="Last name">
                    <Input name="last_name" />
                  </Field>
                  <Field label="Mobile * (E.164)">
                    <Input name="mobile" placeholder="+919876543210" required />
                  </Field>
                  <Field label="Email">
                    <Input name="email" type="email" />
                  </Field>
                </div>
              </fieldset>

              {error ? <ErrorText>{error}</ErrorText> : null}
              <div className="flex justify-end gap-2">
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => {
                    setCreateOpen(false)
                    setError('')
                  }}
                >
                  Cancel
                </Button>
                <Button type="submit" disabled={createTenant.isPending}>
                  Create
                </Button>
              </div>
            </form>
          )}
        </Modal>
      ) : null}

      {statusFor ? (
        <StatusModal
          title="Change tenant status"
          initial={statusFor.status}
          options={TENANT_STATUSES}
          busy={updateStatus.isPending}
          error={error}
          onClose={() => {
            setStatusFor(null)
            setError('')
          }}
          onSubmit={(status, reason) => updateStatus.mutate({ id: statusFor.id, status, reason })}
        />
      ) : null}
    </div>
  )
}
