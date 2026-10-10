// Typed fetch wrapper + one function per hubd endpoint. All money stays in
// integer paise on the wire (AGENTS rule 5); convert only in the UI (lib/money).

import { getActAsTenant, getToken } from '../auth/storage'
import type {
  AppLoginResponse,
  AuditListResponse,
  CreateTenantInput,
  CreateTenantResponse,
  CreateUserResponse,
  GrantLicenseResponse,
  ImportResult,
  LoginResponse,
  OtpRequestResponse,
  SetLicenseValidityResponse,
  TenantView,
  UpdateTenantInput,
  UserDetail,
  UsersListResponse,
  UsersQuery,
  VerifyResponse,
} from './types'

export const API_BASE = import.meta.env.VITE_API_BASE ?? '/api/v1'

export class ApiError extends Error {
  readonly status: number
  readonly body: unknown

  constructor(status: number, message: string, body: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.body = body
  }
}

export interface RequestOptions {
  method?: string
  body?: unknown
  formData?: FormData
  /** Override the stored token (used by the login call itself). */
  token?: string | null
  /** Skip the stored X-Tenant-ID (for platform-admin global calls). */
  noActAs?: boolean
}

function safeParse(text: string): unknown {
  try {
    return JSON.parse(text)
  } catch {
    return text
  }
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = {}
  const token = opts.token !== undefined ? opts.token : getToken()
  if (token) headers.Authorization = `Bearer ${token}`
  if (!opts.noActAs) {
    const actAs = getActAsTenant()
    if (actAs) headers['X-Tenant-ID'] = String(actAs)
  }

  let body: BodyInit | undefined
  if (opts.formData) {
    body = opts.formData
  } else if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opts.body)
  }

  const res = await fetch(API_BASE + path, { method: opts.method ?? 'GET', headers, body })
  const text = await res.text()
  const data = text ? safeParse(text) : null
  if (!res.ok) {
    const message =
      data && typeof data === 'object' && 'error' in data
        ? String((data as { error: unknown }).error)
        : `Request failed (${res.status})`
    throw new ApiError(res.status, message, data)
  }
  return data as T
}

function queryString(params: Record<string, string | number | undefined>): string {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '') q.set(k, String(v))
  }
  const s = q.toString()
  return s ? `?${s}` : ''
}

// --- §7 dashboard ----------------------------------------------------

export const dash = {
  login(username: string, password: string): Promise<LoginResponse> {
    return request('/dash/login', { method: 'POST', body: { username, password }, token: null })
  },
  logout(): Promise<{ ok: boolean }> {
    return request('/dash/logout', { method: 'POST', body: {} })
  },
  listUsers(params: UsersQuery): Promise<UsersListResponse> {
    return request(
      '/dash/users' +
        queryString({ status: params.status, product: params.product, q: params.q, page: params.page }),
    )
  },
  createUser(body: Record<string, unknown>): Promise<CreateUserResponse> {
    return request('/dash/users', { method: 'POST', body })
  },
  getUser(id: number): Promise<UserDetail> {
    return request(`/dash/users/${id}`)
  },
  updateUserStatus(id: number, status: string, reason: string): Promise<{ ok: boolean; status: string }> {
    return request(`/dash/users/${id}/status`, { method: 'PATCH', body: { status, reason } })
  },
  grantLicense(id: number, productCode: string): Promise<GrantLicenseResponse> {
    return request(`/dash/users/${id}/licenses`, { method: 'POST', body: { product_code: productCode } })
  },
  updateLicenseStatus(id: number, status: string, reason: string): Promise<{ ok: boolean }> {
    return request(`/dash/licenses/${id}/status`, { method: 'PATCH', body: { status, reason } })
  },
  regenerateLicenseKey(id: number): Promise<GrantLicenseResponse> {
    return request(`/dash/licenses/${id}/regenerate-key`, { method: 'POST', body: {} })
  },
  setLicenseValidity(
    id: number,
    body: { plan: string; paid_at: string; valid_from: string },
  ): Promise<SetLicenseValidityResponse> {
    return request(`/dash/licenses/${id}/validity`, { method: 'POST', body })
  },
  listAudit(subjectUserId?: number, page = 1): Promise<AuditListResponse> {
    return request('/dash/audit' + queryString({ subject_user_id: subjectUserId, page }))
  },
  getTenant(): Promise<TenantView> {
    return request('/dash/tenant')
  },
  updateTenant(body: UpdateTenantInput): Promise<TenantView> {
    return request('/dash/tenant', { method: 'PATCH', body })
  },
  listTenants(): Promise<{ tenants: TenantView[] }> {
    return request('/dash/tenants', { noActAs: true })
  },
  createTenant(body: CreateTenantInput): Promise<CreateTenantResponse> {
    return request('/dash/tenants', { method: 'POST', body, noActAs: true })
  },
  updateTenantStatus(id: number, status: string, reason: string): Promise<{ ok: boolean }> {
    return request(`/dash/tenants/${id}/status`, { method: 'PATCH', body: { status, reason }, noActAs: true })
  },
  importCsv(file: File): Promise<ImportResult> {
    const fd = new FormData()
    fd.append('file', file)
    return request('/dash/imports', { method: 'POST', formData: fd })
  },
}

// --- §6 app-facing (API tools page) ---------------------------------

export const app = {
  verify(body: Record<string, unknown>): Promise<VerifyResponse> {
    return request('/app/verify', { method: 'POST', body })
  },
  login(body: Record<string, unknown>): Promise<AppLoginResponse> {
    return request('/app/login', { method: 'POST', body })
  },
  register(body: Record<string, unknown>): Promise<unknown> {
    return request('/app/register', { method: 'POST', body })
  },
  otpRequest(body: Record<string, unknown>): Promise<OtpRequestResponse> {
    return request('/app/otp/request', { method: 'POST', body })
  },
  otpVerify(body: Record<string, unknown>): Promise<{ verified: boolean }> {
    return request('/app/otp/verify', { method: 'POST', body })
  },
}
