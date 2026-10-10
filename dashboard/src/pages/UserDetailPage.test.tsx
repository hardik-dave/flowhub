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
