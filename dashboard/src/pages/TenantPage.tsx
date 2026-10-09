import { useEffect, useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError, dash } from '../api/client'
import { Button, Card, ErrorText, Field, Input, Spinner } from '../components/ui'
import type { TenantView } from '../api/types'

export function TenantPage() {
  const qc = useQueryClient()
  const [error, setError] = useState('')
  const [name, setName] = useState('')
  const [grace, setGrace] = useState('')

  const tenant = useQuery<TenantView>({
    queryKey: ['tenant'],
    queryFn: () => dash.getTenant(),
  })

  useEffect(() => {
    if (tenant.data) {
      setName(tenant.data.name)
      setGrace(String(tenant.data.grace_working_days))
    }
  }, [tenant.data])

  const update = useMutation<TenantView, ApiError, { name?: string; grace_working_days?: number }>({
    mutationFn: (body) => dash.updateTenant(body),
    onSuccess: () => {
      setError('')
      qc.invalidateQueries({ queryKey: ['tenant'] })
    },
    onError: (err) => {
      setError(err.message)
    },
  })

  if (tenant.isLoading) return <Spinner />
  if (tenant.error) return <ErrorText>{tenant.error instanceof ApiError ? tenant.error.message : String(tenant.error)}</ErrorText>

  const t = tenant.data as TenantView

  return (
    <div className="space-y-4">
      <Card>
        <h1 className="text-2xl font-bold text-slate-800">Tenant settings</h1>
        <p className="text-sm text-slate-500">Workspace details for {t.slug}</p>
        <form className="mt-4 grid max-w-xl grid-cols-1 gap-3" onSubmit={(e: FormEvent) => { e.preventDefault(); update.mutate({ name: name.trim() || undefined, grace_working_days: grace ? Number(grace) : undefined }) }}>
          <Field label="Tenant ID">{t.id}</Field>
          <Field label="Slug">{t.slug}</Field>
          <Field label="Name">
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </Field>
          <Field label="Grace working days (0–30)">
            <Input type="number" min={0} max={30} value={grace} onChange={(e) => setGrace(e.target.value)} />
          </Field>
          <Field label="Status">{t.status}</Field>
          <Field label="House">{t.is_house ? 'Yes' : 'No'}</Field>
          <Field label="Contact">{t.contact_person} · {t.contact_no}</Field>
          <Field label="Start date">{t.start_date}</Field>
          <Field label="End date">{t.end_date ?? '—'}</Field>
          {error ? <ErrorText>{error}</ErrorText> : null}
          <div>
            <Button type="submit" disabled={update.isPending}>Save changes</Button>
          </div>
        </form>
      </Card>
    </div>
  )
}
