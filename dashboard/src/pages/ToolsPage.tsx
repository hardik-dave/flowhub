import { useState, type FormEvent } from 'react'
import { useMutation } from '@tanstack/react-query'
import { app, ApiError } from '../api/client'
import { Button, Card, ErrorText, Field, Input } from '../components/ui'
import type { VerifyResponse, AppLoginResponse, OtpRequestResponse } from '../api/types'

export function ToolsPage() {
  const [verifyRes, setVerifyRes] = useState<VerifyResponse | null>(null)
  const [loginRes, setLoginRes] = useState<AppLoginResponse | null>(null)
  const [otpReqRes, setOtpReqRes] = useState<OtpRequestResponse | null>(null)
  const [otpVerRes, setOtpVerRes] = useState<{ verified: boolean } | null>(null)
  const [error, setError] = useState('')

  function formBody(e: FormEvent<HTMLFormElement>): Record<string, unknown> {
    const fd = new FormData(e.currentTarget)
    const body: Record<string, unknown> = {}
    for (const [k, v] of fd.entries()) {
      const s = String(v).trim()
      if (s) body[k] = s
    }
    return body
  }

  const verify = useMutation({
    mutationFn: (body: Record<string, unknown>) => app.verify(body),
    onSuccess: (d) => { setVerifyRes(d); setError('') },
    onError: (err) => { setError(err instanceof ApiError ? err.message : 'Failed') },
  })
  const login = useMutation({
    mutationFn: (body: Record<string, unknown>) => app.login(body),
    onSuccess: (d) => { setLoginRes(d); setError('') },
    onError: (err) => { setError(err instanceof ApiError ? err.message : 'Failed') },
  })
  const otpReq = useMutation({
    mutationFn: (body: Record<string, unknown>) => app.otpRequest(body),
    onSuccess: (d) => { setOtpReqRes(d); setError('') },
    onError: (err) => { setError(err instanceof ApiError ? err.message : 'Failed') },
  })
  const otpVer = useMutation({
    mutationFn: (body: Record<string, unknown>) => app.otpVerify(body),
    onSuccess: (d) => { setOtpVerRes(d); setError('') },
    onError: (err) => { setError(err instanceof ApiError ? err.message : 'Failed') },
  })

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold text-slate-800">API tools</h1>
        <p className="text-sm text-slate-500">Exercise app-facing flows to check the entire system.</p>
      </div>
      {error ? <ErrorText>{error}</ErrorText> : null}

      <Card title="/app/verify">
        <form className="grid grid-cols-1 gap-3 md:grid-cols-2" onSubmit={(e) => { e.preventDefault(); verify.mutate(formBody(e)) }}>
          <Field label="Tenant slug"><Input name="tenant_slug" defaultValue="flowos" /></Field>
          <Field label="Username"><Input name="username" /></Field>
          <Field label="Product code"><Input name="product_code" defaultValue="flowos" /></Field>
          <Field label="License key"><Input name="license_key" /></Field>
          <Button type="submit" disabled={verify.isPending}>Verify</Button>
        </form>
        {verifyRes && (
          <pre className="mt-3 overflow-x-auto rounded bg-slate-900 p-3 text-xs text-slate-100">{JSON.stringify(verifyRes, null, 2)}</pre>
        )}
      </Card>

      <Card title="/app/login">
        <form className="grid grid-cols-1 gap-3 md:grid-cols-2" onSubmit={(e) => { e.preventDefault(); login.mutate(formBody(e)) }}>
          <Field label="Tenant slug"><Input name="tenant_slug" defaultValue="flowos" /></Field>
          <Field label="Username"><Input name="username" /></Field>
          <Field label="Password"><Input name="password" type="password" /></Field>
          <Field label="Product code"><Input name="product_code" defaultValue="flowos" /></Field>
          <Button type="submit" disabled={login.isPending}>Login</Button>
        </form>
        {loginRes && (
          <pre className="mt-3 overflow-x-auto rounded bg-slate-900 p-3 text-xs text-slate-100">{JSON.stringify(loginRes, null, 2)}</pre>
        )}
      </Card>

      <Card title="/app/otp/request">
        <form className="grid grid-cols-1 gap-3 md:grid-cols-2" onSubmit={(e) => { e.preventDefault(); otpReq.mutate(formBody(e)) }}>
          <Field label="Tenant slug"><Input name="tenant_slug" defaultValue="flowos" /></Field>
          <Field label="Username"><Input name="username" /></Field>
          <Field label="Purpose"><Input name="purpose" defaultValue="REGISTRATION" placeholder="REGISTRATION or FIRST_LOGIN" /></Field>
          <Button type="submit" disabled={otpReq.isPending}>Request</Button>
        </form>
        {otpReqRes && (
          <pre className="mt-3 overflow-x-auto rounded bg-slate-900 p-3 text-xs text-slate-100">{JSON.stringify(otpReqRes, null, 2)}</pre>
        )}
      </Card>

      <Card title="/app/otp/verify">
        <form className="grid grid-cols-1 gap-3 md:grid-cols-2" onSubmit={(e) => { e.preventDefault(); otpVer.mutate(formBody(e)) }}>
          <Field label="Tenant slug"><Input name="tenant_slug" defaultValue="flowos" /></Field>
          <Field label="Username"><Input name="username" /></Field>
          <Field label="Purpose"><Input name="purpose" defaultValue="REGISTRATION" /></Field>
          <Field label="Code"><Input name="code" /></Field>
          <Button type="submit" disabled={otpVer.isPending}>Verify</Button>
        </form>
        {otpVerRes && (
          <pre className="mt-3 overflow-x-auto rounded bg-slate-900 p-3 text-xs text-slate-100">{JSON.stringify(otpVerRes, null, 2)}</pre>
        )}
      </Card>
    </div>
  )
}
