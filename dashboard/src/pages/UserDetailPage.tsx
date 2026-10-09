import { useQuery } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import { ApiError, dash } from '../api/client'
import { Badge, Button, Card, ErrorText, Spinner } from '../components/ui'

export function UserDetailPage() {
  const { id } = useParams<{ id: string }>()
  const userId = Number(id)

  const detail = useQuery({
    queryKey: ['user', userId],
    queryFn: () => dash.getUser(userId),
    enabled: !Number.isNaN(userId) && userId > 0,
  })

  if (Number.isNaN(userId) || userId <= 0) return <ErrorText>No such user.</ErrorText>
  if (detail.isLoading) return <Spinner />
  if (detail.error) return <ErrorText>{detail.error instanceof ApiError ? detail.error.message : String(detail.error)}</ErrorText>

  const u = detail.data!.user

  return (
    <div className="space-y-4">
      <Card>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-2xl font-bold text-slate-800">{u.username}</h1>
            <p className="text-sm text-slate-500">User #{u.id}</p>
          </div>
          <div className="flex gap-2">
            <Button variant="secondary" disabled>Change status</Button>
            <Button variant="secondary" disabled>Grant license</Button>
          </div>
        </div>
        <div className="mt-3 grid grid-cols-1 gap-3 text-sm text-slate-600 md:grid-cols-3">
          <div><span className="font-medium text-slate-700">Status:</span> <Badge value={u.status} /></div>
          <div><span className="font-medium text-slate-700">Role:</span> {u.role || '—'}</div>
          <div><span className="font-medium text-slate-700">Mobile:</span> {u.mobile} {u.mobile_verified ? '(verified)' : '(not verified)'}</div>
          <div><span className="font-medium text-slate-700">Name:</span> {[u.first_name, u.last_name].filter(Boolean).join(' ') || '—'}</div>
          <div><span className="font-medium text-slate-700">Email:</span> {u.email || '—'}</div>
          <div><span className="font-medium text-slate-700">City/State:</span> {[u.city, u.state].filter(Boolean).join(', ') || '—'}</div>
        </div>
      </Card>

      <Card title="Licenses">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-50">
              <tr>
                <th className="px-3 py-2 text-left font-medium text-slate-600">ID</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Product</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Status</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Key hint</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Valid until</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 bg-white">
              {(u.licenses ?? []).map((l) => (
                <tr key={l.id}>
                  <td className="px-3 py-2 text-slate-600">{l.id}</td>
                  <td className="px-3 py-2 text-slate-600">{l.product_code} ({l.product_name})</td>
                  <td className="px-3 py-2"><Badge value={l.status} /></td>
                  <td className="px-3 py-2 text-slate-600">…{l.key_hint}</td>
                  <td className="px-3 py-2 text-slate-600">{l.valid_until ?? '—'}</td>
                  <td className="px-3 py-2"><Button variant="secondary" disabled>Regenerate key</Button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>

      <Card title="Payments (read-only)">
        <p className="text-sm text-slate-500">No payments recorded.</p>
        <p className="mt-2 text-xs text-slate-500">Recording payments from the dashboard is not wired yet (POST /dash/payments is 501).</p>
      </Card>

      <Card title="Audit trail">
        <p className="text-sm text-slate-500">Audit entries are available via the API; showing a condensed view here.</p>
      </Card>
    </div>
  )
}
