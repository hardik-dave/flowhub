import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import { ApiError, app, dash } from '../api/client'
import type { GrantLicenseResponse, LicenseView } from '../api/types'
import { OneTimeSecret } from '../components/OneTimeSecret'
import { StatusModal } from '../components/StatusModal'
import { Badge, Button, Card, ErrorText, Field, Input, Modal, Select, Spinner } from '../components/ui'
import { productLabel } from '../lib/products'

const LICENSE_STATUSES = [
  { value: 'ACTIVE', label: 'ACTIVE' },
  { value: 'DISABLED', label: 'DISABLED' },
]

function todayISO(): string {
  return new Date().toISOString().slice(0, 10)
}

function AccessForm({
  busy,
  error,
  onSubmit,
  onClose,
}: {
  busy: boolean
  error: string
  onSubmit: (v: { plan: string; paid_at: string; valid_from: string }) => void
  onClose: () => void
}) {
  const [plan, setPlan] = useState('MONTHLY')
  const [paidAt, setPaidAt] = useState(() => todayISO())
  const [validFrom, setValidFrom] = useState(() => todayISO())
  return (
    <div className="space-y-3">
      <Field label="Plan">
        <Select value={plan} onChange={(e) => setPlan(e.target.value)}>
          <option value="MONTHLY">MONTHLY (+1 month)</option>
          <option value="QUARTERLY">QUARTERLY (+3 months)</option>
          <option value="ANNUAL">ANNUAL (+12 months)</option>
        </Select>
      </Field>
      <Field label="Access from">
        <Input type="date" value={validFrom} onChange={(e) => setValidFrom(e.target.value)} />
      </Field>
      <Field label="Paid on">
        <Input type="date" value={paidAt} onChange={(e) => setPaidAt(e.target.value)} />
      </Field>
      <p className="text-xs text-slate-500">
        The license verifies as valid from “Access from” until the plan term ends. A ₹0 MANUAL payment is recorded, so
        this overwrites any previous access window.
      </p>
      {error ? <ErrorText>{error}</ErrorText> : null}
      <div className="flex justify-end gap-2">
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button disabled={busy} onClick={() => onSubmit({ plan, paid_at: paidAt, valid_from: validFrom })}>
          Save
        </Button>
      </div>
    </div>
  )
}

function MobileVerifyCard({
  slug,
  username,
  onVerified,
}: {
  slug: string
  username: string
  onVerified: () => void
}) {
  const [sent, setSent] = useState(false)
  const [code, setCode] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [cooldown, setCooldown] = useState(0)

  useEffect(() => {
    if (cooldown <= 0) return
    const t = setTimeout(() => setCooldown((c) => (c <= 1 ? 0 : c - 1)), 1000)
    return () => clearTimeout(t)
  }, [cooldown])

  const send = useMutation({
    mutationFn: () => app.otpRequest({ tenant_slug: slug, username, purpose: 'FIRST_LOGIN' }),
    onSuccess: (data) => {
      setSent(true)
      setError('')
      setCooldown(60)
      const mins = Math.max(1, Math.round(data.expires_in_seconds / 60))
      if (data.dev_code) {
        setCode(data.dev_code)
        setNotice(`Dev mode — code ${data.dev_code} auto-filled (expires in ${mins} min). Click Verify.`)
      } else {
        setNotice(
          `Code sent — it expires in ${mins} minute${mins === 1 ? '' : 's'}. In dev, read it from the hubd console ([dev-sms]).`,
        )
      }
    },
    onError: (e) => {
      // 429 = the 60s resend cooldown: a code is already pending, so reveal
      // the code field instead of only showing the error (otherwise the
      // admin can never reach the input to enter the code they received).
      if (e instanceof ApiError && e.status === 429) {
        setSent(true)
        setError('')
        setCooldown(60)
        setNotice('A code was already sent — enter it below, or wait for the timer to resend.')
        return
      }
      setError(e instanceof ApiError ? e.message : 'Could not send the code.')
    },
  })

  const verify = useMutation({
    mutationFn: () =>
      app.otpVerify({ tenant_slug: slug, username, purpose: 'FIRST_LOGIN', code: code.trim() }),
    onSuccess: () => {
      setError('')
      onVerified()
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : 'Could not verify the code.'),
  })

  return (
    <Card title="Mobile verification">
      <p className="text-sm text-slate-600">
        {username}&apos;s mobile is not verified, so their login stays paused. Send a one-time code and enter it below.
      </p>
      {!sent ? (
        <div className="mt-3">
          <Button disabled={send.isPending} onClick={() => send.mutate()}>
            Send code
          </Button>
        </div>
      ) : (
        <div className="mt-3 space-y-3">
          <Field label="Verification code">
            <Input
              inputMode="numeric"
              autoComplete="one-time-code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
            />
          </Field>
          <div className="flex gap-2">
            <Button disabled={verify.isPending || code.trim() === ''} onClick={() => verify.mutate()}>
              Verify
            </Button>
            <Button variant="secondary" disabled={send.isPending || cooldown > 0} onClick={() => send.mutate()}>
              {cooldown > 0 ? `Resend in ${cooldown}s` : 'Resend code'}
            </Button>
          </div>
        </div>
      )}
      {notice ? <p className="mt-2 text-sm text-slate-500">{notice}</p> : null}
      {error ? <ErrorText>{error}</ErrorText> : null}
    </Card>
  )
}

const USER_STATUSES = [
  { value: 'ACTIVE', label: 'ACTIVE' },
  { value: 'DEACTIVATED', label: 'DEACTIVATED' },
  { value: 'BANNED', label: 'BANNED' },
]

function GrantForm({
  products,
  pending,
  error,
  onSubmit,
  onClose,
}: {
  products: string[]
  pending: boolean
  error: string
  onSubmit: (code: string) => void
  onClose: () => void
}) {
  const [code, setCode] = useState(products[0])
  return (
    <div className="space-y-3">
      <Field label="Product">
        <Select value={code} onChange={(e) => setCode(e.target.value)}>
          {products.map((p) => (
            <option key={p} value={p}>
              {productLabel(p)} ({p})
            </option>
          ))}
        </Select>
      </Field>
      {error ? <ErrorText>{error}</ErrorText> : null}
      <div className="flex justify-end gap-2">
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button disabled={pending} onClick={() => onSubmit(code)}>
          Grant
        </Button>
      </div>
    </div>
  )
}

export function UserDetailPage() {
  const { id } = useParams<{ id: string }>()
  const userId = Number(id)
  const valid = !Number.isNaN(userId) && userId > 0
  const qc = useQueryClient()

  const [statusOpen, setStatusOpen] = useState(false)
  const [grantOpen, setGrantOpen] = useState(false)
  const [granted, setGranted] = useState<GrantLicenseResponse | null>(null)
  const [licenseFor, setLicenseFor] = useState<LicenseView | null>(null)
  const [accessFor, setAccessFor] = useState<LicenseView | null>(null)
  const [regenFor, setRegenFor] = useState<LicenseView | null>(null)
  const [regenKey, setRegenKey] = useState<string | null>(null)
  const [error, setError] = useState('')

  const detail = useQuery({
    queryKey: ['user', userId],
    queryFn: () => dash.getUser(userId),
    enabled: valid,
  })

  const tenant = useQuery({
    queryKey: ['tenant'],
    queryFn: () => dash.getTenant(),
    enabled: valid,
  })

  const invalidateUser = () => qc.invalidateQueries({ queryKey: ['user', userId] })

  const changeStatus = useMutation({
    mutationFn: (v: { status: string; reason: string }) => dash.updateUserStatus(userId, v.status, v.reason),
    onSuccess: () => {
      invalidateUser()
      setStatusOpen(false)
      setError('')
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : 'Could not change the status.'),
  })

  const grant = useMutation({
    mutationFn: (code: string) => dash.grantLicense(userId, code),
    onSuccess: (data) => {
      setGranted(data)
      invalidateUser()
      setError('')
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : 'Could not grant the license.'),
  })

  const licenseStatus = useMutation({
    mutationFn: (v: { licenseId: number; status: string; reason: string }) =>
      dash.updateLicenseStatus(v.licenseId, v.status, v.reason),
    onSuccess: () => {
      invalidateUser()
      setLicenseFor(null)
      setError('')
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : 'Could not change the license status.'),
  })

  const regenerate = useMutation({
    mutationFn: (licenseId: number) => dash.regenerateLicenseKey(licenseId),
    onSuccess: (data) => {
      setRegenKey(data.license_key)
      invalidateUser()
      setError('')
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : 'Could not regenerate the key.'),
  })

  const setValidity = useMutation({
    mutationFn: (v: { licenseId: number; plan: string; paid_at: string; valid_from: string }) =>
      dash.setLicenseValidity(v.licenseId, { plan: v.plan, paid_at: v.paid_at, valid_from: v.valid_from }),
    onSuccess: () => {
      invalidateUser()
      setAccessFor(null)
      setError('')
    },
    onError: (e) => setError(e instanceof ApiError ? e.message : 'Could not set the access window.'),
  })

  if (!valid) return <ErrorText>No such user.</ErrorText>
  if (detail.isLoading) return <Spinner />
  if (detail.error) {
    return <ErrorText>{detail.error instanceof ApiError ? detail.error.message : String(detail.error)}</ErrorText>
  }

  const u = detail.data!.user
  const licenses = u.licenses ?? []
  const held = new Set(licenses.map((l) => l.product_code))
  const grantable = (tenant.data?.products ?? []).filter((c) => !held.has(c))

  return (
    <div className="space-y-4">
      <Card>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-2xl font-bold text-slate-800">{u.username}</h1>
            <p className="text-sm text-slate-500">User #{u.id}</p>
          </div>
          <div className="flex gap-2">
            <Button
              variant="secondary"
              onClick={() => {
                setError('')
                setStatusOpen(true)
              }}
            >
              Change status
            </Button>
            <Button
              variant="secondary"
              onClick={() => {
                setGranted(null)
                setError('')
                setGrantOpen(true)
              }}
            >
              Grant license
            </Button>
          </div>
        </div>
        <div className="mt-3 grid grid-cols-1 gap-3 text-sm text-slate-600 md:grid-cols-3">
          <div><span className="font-medium text-slate-700">Status:</span> <Badge value={u.status} /></div>
          <div><span className="font-medium text-slate-700">Role:</span> {u.role || '—'}</div>
          <div><span className="font-medium text-slate-700">Mobile:</span> {u.mobile} {u.mobile_verified ? '(verified)' : '(not verified)'}</div>
          <div><span className="font-medium text-slate-700">Name:</span> {[u.first_name, u.last_name].filter(Boolean).join(' ') || '—'}</div>
          <div><span className="font-medium text-slate-700">Email:</span> {u.email || '—'}</div>
          <div><span className="font-medium text-slate-700">City:</span> {u.city || '—'}</div>
        </div>
      </Card>

      {!u.mobile_verified && tenant.data ? (
        <MobileVerifyCard slug={tenant.data.slug} username={u.username} onVerified={invalidateUser} />
      ) : null}

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
              {licenses.map((l) => (
                <tr key={l.id}>
                  <td className="px-3 py-2 text-slate-600">{l.id}</td>
                  <td className="px-3 py-2 text-slate-600">{l.product_code} ({l.product_name})</td>
                  <td className="px-3 py-2"><Badge value={l.status} /></td>
                  <td className="px-3 py-2 text-slate-600">…{l.key_hint}</td>
                  <td className="px-3 py-2 text-slate-600">{l.valid_until ?? '—'}</td>
                  <td className="px-3 py-2">
                    <div className="flex gap-2">
                      <Button
                        variant="secondary"
                        onClick={() => {
                          setError('')
                          setAccessFor(l)
                        }}
                      >
                        Set access
                      </Button>
                      <Button
                        variant="secondary"
                        onClick={() => {
                          setError('')
                          setLicenseFor(l)
                        }}
                      >
                        {l.status === 'ACTIVE' ? 'Disable' : 'Enable'}
                      </Button>
                      <Button
                        variant="secondary"
                        onClick={() => {
                          setError('')
                          setRegenKey(null)
                          setRegenFor(l)
                        }}
                      >
                        Regenerate key
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              {licenses.length === 0 ? (
                <tr><td colSpan={6} className="px-3 py-6 text-center text-sm text-slate-500">No licenses.</td></tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </Card>

      <Card title="Payments (read-only)">
        <p className="text-sm text-slate-500">No payments recorded.</p>
        <p className="mt-2 text-xs text-slate-500">Recording payments from the dashboard is not wired yet (POST /dash/payments is 501).</p>
      </Card>

      <Card title="Audit trail">
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-slate-200 text-sm">
            <thead className="bg-slate-50">
              <tr>
                <th className="px-3 py-2 text-left font-medium text-slate-600">When</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Action</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Actor</th>
                <th className="px-3 py-2 text-left font-medium text-slate-600">Reason</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 bg-white">
              {(detail.data!.audit ?? []).map((a) => (
                <tr key={a.id}>
                  <td className="px-3 py-2 text-slate-600">{new Date(a.at).toLocaleString()}</td>
                  <td className="px-3 py-2 text-slate-600">{a.action}</td>
                  <td className="px-3 py-2 text-slate-600">{a.actor_user_id ?? '—'}</td>
                  <td className="px-3 py-2 text-slate-600">{a.reason || '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>

      {statusOpen ? (
        <StatusModal
          title="Change status"
          initial={u.status === 'BANNED' || u.status === 'DEACTIVATED' ? u.status : 'ACTIVE'}
          options={USER_STATUSES}
          busy={changeStatus.isPending}
          error={error}
          onClose={() => {
            setStatusOpen(false)
            setError('')
          }}
          onSubmit={(status, reason) => changeStatus.mutate({ status, reason })}
        />
      ) : null}

      {grantOpen ? (
        <Modal
          title="Grant license"
          onClose={() => {
            setGrantOpen(false)
            setGranted(null)
            setError('')
          }}
        >
          {granted ? (
            <div className="space-y-3">
              <OneTimeSecret
                label="License key"
                value={granted.license_key}
                note={`License #${granted.license_id} is now ACTIVE; it verifies as paused until a payment sets its validity.`}
              />
              <div className="flex justify-end">
                <Button
                  onClick={() => {
                    setGrantOpen(false)
                    setGranted(null)
                  }}
                >
                  Done
                </Button>
              </div>
            </div>
          ) : grantable.length === 0 ? (
            <div className="space-y-3">
              <p className="text-sm text-slate-600">
                This workspace distributes no additional products the user does not already hold.
              </p>
              <div className="flex justify-end">
                <Button variant="secondary" onClick={() => setGrantOpen(false)}>
                  Close
                </Button>
              </div>
            </div>
          ) : (
            <GrantForm
              products={grantable}
              pending={grant.isPending}
              error={error}
              onSubmit={(code) => grant.mutate(code)}
              onClose={() => {
                setGrantOpen(false)
                setError('')
              }}
            />
          )}
        </Modal>
      ) : null}

      {licenseFor ? (
        <StatusModal
          title={`${licenseFor.status === 'ACTIVE' ? 'Disable' : 'Enable'} license`}
          initial={licenseFor.status === 'ACTIVE' ? 'DISABLED' : 'ACTIVE'}
          options={LICENSE_STATUSES}
          busy={licenseStatus.isPending}
          error={error}
          onClose={() => {
            setLicenseFor(null)
            setError('')
          }}
          onSubmit={(status, reason) => licenseStatus.mutate({ licenseId: licenseFor.id, status, reason })}
        />
      ) : null}

      {regenFor ? (
        <Modal
          title="Regenerate license key"
          onClose={() => {
            setRegenFor(null)
            setRegenKey(null)
            setError('')
          }}
        >
          {regenKey ? (
            <div className="space-y-3">
              <OneTimeSecret label="New license key" value={regenKey} note="The previous key no longer works." />
              <div className="flex justify-end">
                <Button
                  onClick={() => {
                    setRegenFor(null)
                    setRegenKey(null)
                  }}
                >
                  Done
                </Button>
              </div>
            </div>
          ) : (
            <div className="space-y-3">
              <p className="text-sm text-slate-600">
                This invalidates the current key for {regenFor.product_code}. Give the user the new key afterwards.
              </p>
              {error ? <ErrorText>{error}</ErrorText> : null}
              <div className="flex justify-end gap-2">
                <Button
                  variant="secondary"
                  onClick={() => {
                    setRegenFor(null)
                    setError('')
                  }}
                >
                  Cancel
                </Button>
                <Button disabled={regenerate.isPending} onClick={() => regenerate.mutate(regenFor.id)}>
                  Regenerate
                </Button>
              </div>
            </div>
          )}
        </Modal>
      ) : null}

      {accessFor ? (
        <Modal
          title={`Set access — ${accessFor.product_code}`}
          onClose={() => {
            setAccessFor(null)
            setError('')
          }}
        >
          <AccessForm
            busy={setValidity.isPending}
            error={error}
            onSubmit={(v) => setValidity.mutate({ licenseId: accessFor.id, ...v })}
            onClose={() => {
              setAccessFor(null)
              setError('')
            }}
          />
        </Modal>
      ) : null}
    </div>
  )
}
