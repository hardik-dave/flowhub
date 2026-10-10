import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { UserDetailPage } from './UserDetailPage'
import * as client from '../api/client'
import type { TenantView, UserDetail, UserView } from '../api/types'

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof client>('../api/client')
  return {
    ...actual,
    dash: {
      ...actual.dash,
      getUser: vi.fn(),
      getTenant: vi.fn(),
      updateUserStatus: vi.fn(),
      grantLicense: vi.fn(),
      updateLicenseStatus: vi.fn(),
      regenerateLicenseKey: vi.fn(),
      setLicenseValidity: vi.fn(),
    },
    app: {
      ...actual.app,
      otpRequest: vi.fn(),
      otpVerify: vi.fn(),
    },
  }
})

function makeUser(over: Partial<UserView> = {}): UserView {
  return {
    id: 42,
    username: 'alice',
    first_name: 'Alice',
    last_name: 'A',
    mobile: '+919876543210',
    email: '',
    city: '',
    state: '',
    status: 'ACTIVE',
    mobile_verified: true,
    role: 'ALGO_USER',
    licenses: [],
    ...over,
  }
}

function makeDetail(user: UserView, audit: UserDetail['audit'] = []): UserDetail {
  return { user, payments: [], audit }
}

function makeTenant(products: string[]): TenantView {
  return {
    id: 1,
    slug: 'flowos',
    name: 'House',
    status: 'ACTIVE',
    is_house: true,
    grace_working_days: 5,
    contact_person: '',
    contact_no: '',
    start_date: '2026-01-01',
    end_date: null,
    products,
  }
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={['/users/42']}>
        <Routes>
          <Route path="/users/:id" element={<UserDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('UserDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(client.dash.getTenant).mockResolvedValue(makeTenant(['flowos']))
  })

  it('requires a reason and calls updateUserStatus with it', async () => {
    const user = userEvent.setup()
    vi.mocked(client.dash.getUser).mockResolvedValue(makeDetail(makeUser()))
    vi.mocked(client.dash.updateUserStatus).mockResolvedValue({ ok: true, status: 'DEACTIVATED' })

    renderPage()
    await screen.findByText('alice')

    await user.click(screen.getByRole('button', { name: 'Change status' }))
    const save = screen.getByRole('button', { name: 'Save' })
    expect(save).toBeDisabled()

    await user.type(screen.getByLabelText(/Reason \(required\)/), 'Non-payment')
    await user.selectOptions(screen.getByLabelText('New status'), 'DEACTIVATED')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(client.dash.updateUserStatus).toHaveBeenCalledWith(42, 'DEACTIVATED', 'Non-payment')
  })

  it('grants a license the tenant holds and shows the one-time key', async () => {
    const user = userEvent.setup()
    vi.mocked(client.dash.getUser).mockResolvedValue(
      makeDetail(makeUser({ licenses: [{ id: 5, product_code: 'flowos', product_name: 'FlowOS', status: 'ACTIVE', key_hint: 'AAAA', valid_until: null }] })),
    )
    vi.mocked(client.dash.getTenant).mockResolvedValue(makeTenant(['flowos', 'optionalyzer']))
    vi.mocked(client.dash.grantLicense).mockResolvedValue({ license_id: 9, license_key: 'OP-1111-2222-3333-4444' })

    renderPage()
    await screen.findByText('alice')
    await user.click(screen.getByRole('button', { name: 'Grant license' }))

    await user.click(screen.getByRole('button', { name: 'Grant' }))
    expect(client.dash.grantLicense).toHaveBeenCalledWith(42, 'optionalyzer')
    expect(await screen.findByText('OP-1111-2222-3333-4444')).toBeInTheDocument()
  })

  it('regenerates a license key and shows the new one', async () => {
    const user = userEvent.setup()
    vi.mocked(client.dash.getUser).mockResolvedValue(
      makeDetail(makeUser({ licenses: [{ id: 7, product_code: 'flowos', product_name: 'FlowOS', status: 'ACTIVE', key_hint: 'ZZZZ', valid_until: null }] })),
    )
    vi.mocked(client.dash.regenerateLicenseKey).mockResolvedValue({ license_id: 7, license_key: 'FL-NEW1-NEW2-NEW3-NEW4' })

    renderPage()
    await screen.findByText('alice')
    await user.click(screen.getByRole('button', { name: 'Regenerate key' }))
    await user.click(screen.getByRole('button', { name: 'Regenerate' }))

    expect(client.dash.regenerateLicenseKey).toHaveBeenCalledWith(7)
    expect(await screen.findByText('FL-NEW1-NEW2-NEW3-NEW4')).toBeInTheDocument()
  })

  it('sets an access window on an existing license', async () => {
    const user = userEvent.setup()
    vi.mocked(client.dash.getUser).mockResolvedValue(
      makeDetail(makeUser({ licenses: [{ id: 5, product_code: 'flowos', product_name: 'FlowOS', status: 'ACTIVE', key_hint: 'AAAA', valid_until: null }] })),
    )
    vi.mocked(client.dash.setLicenseValidity).mockResolvedValue({ license_id: 5, valid_until: '2026-11-10' })

    renderPage()
    await screen.findByText('alice')
    await user.click(screen.getByRole('button', { name: 'Set access' }))
    await user.selectOptions(screen.getByLabelText('Plan'), 'MONTHLY')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    const today = new Date().toISOString().slice(0, 10)
    expect(client.dash.setLicenseValidity).toHaveBeenCalledWith(5, {
      plan: 'MONTHLY',
      paid_at: today,
      valid_from: today,
    })
  })

  it('sends and verifies a first-login OTP when the mobile is unverified', async () => {
    const user = userEvent.setup()
    vi.mocked(client.dash.getUser).mockResolvedValue(makeDetail(makeUser({ mobile_verified: false })))
    vi.mocked(client.app.otpRequest).mockResolvedValue({ otp_sent: true, expires_in_seconds: 300 })
    vi.mocked(client.app.otpVerify).mockResolvedValue({ verified: true })

    renderPage()
    await screen.findByText('alice')
    await user.click(screen.getByRole('button', { name: 'Send code' }))
    await user.click(screen.getByLabelText('Verification code'))
    await user.keyboard('123456')
    await user.click(screen.getByRole('button', { name: 'Verify' }))

    expect(client.app.otpRequest).toHaveBeenCalledWith({
      tenant_slug: 'flowos',
      username: 'alice',
      purpose: 'FIRST_LOGIN',
    })
    expect(client.app.otpVerify).toHaveBeenCalledWith({
      tenant_slug: 'flowos',
      username: 'alice',
      purpose: 'FIRST_LOGIN',
      code: '123456',
    })
  })

  it('reveals the code field when the OTP request is rate-limited (429)', async () => {
    const user = userEvent.setup()
    vi.mocked(client.dash.getUser).mockResolvedValue(makeDetail(makeUser({ mobile_verified: false })))
    vi.mocked(client.app.otpRequest).mockRejectedValue(
      new client.ApiError(429, 'A code was just sent. Please wait 60 seconds before requesting another.', {}),
    )

    renderPage()
    await screen.findByText('alice')
    await user.click(screen.getByRole('button', { name: 'Send code' }))

    expect(await screen.findByLabelText('Verification code')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Verify' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Resend/ })).toBeDisabled()
  })

  it('auto-fills the code when the dev echo returns dev_code', async () => {
    const user = userEvent.setup()
    vi.mocked(client.dash.getUser).mockResolvedValue(makeDetail(makeUser({ mobile_verified: false })))
    vi.mocked(client.app.otpRequest).mockResolvedValue({ otp_sent: true, expires_in_seconds: 300, dev_code: '123456' })
    vi.mocked(client.app.otpVerify).mockResolvedValue({ verified: true })

    renderPage()
    await screen.findByText('alice')
    await user.click(screen.getByRole('button', { name: 'Send code' }))

    expect(await screen.findByLabelText('Verification code')).toHaveValue('123456')

    await user.click(screen.getByRole('button', { name: 'Verify' }))
    expect(client.app.otpVerify).toHaveBeenCalledWith({
      tenant_slug: 'flowos',
      username: 'alice',
      purpose: 'FIRST_LOGIN',
      code: '123456',
    })
  })

  it('hides the mobile verification card once the mobile is verified', async () => {
    vi.mocked(client.dash.getUser).mockResolvedValue(makeDetail(makeUser({ mobile_verified: true })))
    renderPage()
    await screen.findByText('alice')
    expect(screen.queryByRole('button', { name: 'Send code' })).not.toBeInTheDocument()
  })

  it('renders the audit trail', async () => {
    vi.mocked(client.dash.getUser).mockResolvedValue(
      makeDetail(makeUser(), [
        { id: 1, action: 'USER_CREATED', actor_user_id: 1, subject_user_id: 42, reason: 'onboard', at: '2026-01-01T00:00:00Z' },
      ]),
    )
    renderPage()
    expect(await screen.findByText('USER_CREATED')).toBeInTheDocument()
    expect(screen.getByText('onboard')).toBeInTheDocument()
  })
})
