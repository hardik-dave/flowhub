import { useQuery } from '@tanstack/react-query'
import { ApiError, dash } from '../api/client'
import { Button, Card, ErrorText } from '../components/ui'
import type { TenantView } from '../api/types'
import { useAuth } from '../auth/AuthContext'

export function TenantsPage() {
  const { isPlatformAdmin } = useAuth()

  const tenants = useQuery<{ tenants: TenantView[] }>({
    queryKey: ['tenants'],
    queryFn: () => dash.listTenants(),
  })

  if (!isPlatformAdmin) return <ErrorText>Platform admin access required.</ErrorText>

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-800">Platform tenants</h1>
          <p className="text-sm text-slate-500">Onboard and manage broker tenants.</p>
        </div>
        <Button disabled>Create tenant</Button>
      </div>

      <Card>
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
                  <td className="px-3 py-2 text-slate-600">{t.status}</td>
                  <td className="px-3 py-2 text-slate-600">{t.is_house ? 'Yes' : 'No'}</td>
                  <td className="px-3 py-2 text-slate-600">{t.contact_person} · {t.contact_no}</td>
                  <td className="px-3 py-2"><Button variant="secondary" disabled>Change status</Button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {tenants.error ? <ErrorText>{tenants.error instanceof ApiError ? tenants.error.message : String(tenants.error)}</ErrorText> : null}
      </Card>
    </div>
  )
}
