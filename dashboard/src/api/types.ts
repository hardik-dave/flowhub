// Response shapes mirror the implemented hubd §7 API (backend/internal/dashboard).
// Keep these in sync with the Go structs — snake_case on the wire.

export type UserStatus = 'PENDING_VERIFICATION' | 'ACTIVE' | 'DEACTIVATED' | 'BANNED'
export type LicenseStatus = 'ACTIVE' | 'DISABLED'
export type TenantStatus = 'ACTIVE' | 'INACTIVE'
export type Role = 'SUPERADMIN' | 'ADMIN' | 'EDITOR' | 'ALGO_USER' | ''

export interface LicenseView {
  id: number
  product_code: string
  product_name: string
  status: LicenseStatus
  key_hint: string
  valid_until: string | null
}

export interface UserView {
  id: number
  username: string
  first_name: string
  last_name: string
  mobile: string
  email: string
  city: string
  state: string
  status: UserStatus
  mobile_verified: boolean
  role: Role
  licenses: LicenseView[]
}

export interface PaymentView {
  id: number
  amount_minor_units: number
  currency: string
  method: string
  plan: string
  paid_at: string
  valid_from: string
  valid_until: string
  note?: string
}

export interface AuditView {
  id: number
  action: string
  actor_user_id: number | null
  subject_user_id: number | null
  before?: unknown
  after?: unknown
  reason?: string
  at: string
}

export interface UserDetail {
  user: UserView
  payments: PaymentView[]
  audit: AuditView[]
}

export interface TenantView {
  id: number
  slug: string
  name: string
  status: TenantStatus
  is_house: boolean
  grace_working_days: number
  contact_person: string
  contact_no: string
  start_date: string
  end_date: string | null
  /** Granted product codes; only present on GET /dash/tenant. */
  products?: string[]
}

export interface UsersListResponse {
  users: UserView[]
  page: number
  page_size: number
  total: number
}

export interface AuditListResponse {
  entries: AuditView[]
  page: number
  page_size: number
  total: number
}

export interface SessionUser {
  id: number
  username: string
  first_name: string
  role: Role
  tenant_id: number
}

export interface LoginResponse {
  session_token: string
  user: SessionUser
}

export interface CreateUserResponse {
  user_id: number
  username: string
  password: string
  license_key: string
}

export interface GrantLicenseResponse {
  license_id: number
  license_key: string
}

export interface ImportRowError {
  row: number
  reason: string
}

export interface ImportCreated {
  row: number
  username: string
  password: string
  license_key: string
}

export interface ImportResult {
  batch_id: number
  filename: string
  row_count: number
  ok_count: number
  error_count: number
  errors: ImportRowError[]
  created: ImportCreated[]
}

export interface CreateTenantResponse {
  tenant_id: number
  admin_user_id: number
  admin_username: string
  password?: string
}

export interface CreateTenantAdminInput {
  username: string
  password?: string
  first_name?: string
  last_name?: string
  mobile: string
  email?: string
}

export interface CreateTenantInput {
  slug: string
  name: string
  contact_person?: string
  contact_no?: string
  start_date: string
  end_date?: string
  grace_working_days?: number
  products: string[]
  admin: CreateTenantAdminInput
}

export interface UsersQuery {
  status?: string
  product?: string
  q?: string
  page?: number
}

export interface UpdateTenantInput {
  name?: string
  grace_working_days?: number
}

// --- app-facing (§6) — used by the API tools page --------------------

export interface VerifyResponse {
  status: 'valid' | 'paused' | 'revoked' | 'not_found'
  reason: string
  warning?: string
  entitlements?: unknown
  subscription?: unknown
}

export interface AppLoginResponse {
  session_token: string
  user: { id: number; username: string; first_name: string; role: string }
  verify: VerifyResponse
}

export interface OtpRequestResponse {
  otp_sent: boolean
  expires_in_seconds: number
}
