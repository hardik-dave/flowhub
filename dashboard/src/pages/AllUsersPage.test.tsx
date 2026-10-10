import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { AllUsersPage } from './AllUsersPage'
import * as client from '../api/client'
import type { AllUserRow, AllUsersListResponse, TenantView } from '../api/types'

const setActAsTenant = vi.fn()

vi.mock('../auth/AuthContext', () => ({
  useAuth: () => ({ isPlatformAdmin: true, setActAsTenant }),
}))

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof client>('../api/client')
  return {
    ...actual,
    dash: { ...actual.dash, listAllUsers: vi.fn(), listTenants: vi.fn() },
  }
})

function makeRow(over: Partial<AllUserRow> = {}): AllUserRow {
  return {
    id: 1,
    username: 'acme-user',
    first_name: 'Acme',
    last_name: 'User',
    mobile: '+919876543210',
    email: '',
    city: '',
    state: '',
    status: 'ACTIVE',
    mobile_verified: true,
    role: 'ALGO_USER',
    licenses: [],
    tenant_id: 2,
    tenant_slug: 'acme',
    tenant_name: 'Acme Brokers',
    ...over,
  }
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <AllUsersPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('AllUsersPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(client.dash.listTenants).mockResolvedValue({
      tenants: [
        { id: 2, slug: 'acme', name: 'Acme Brokers', status: 'ACTIVE', is_house: false, grace_working_days: 5, contact_person: '', contact_no: '', start_date: '2026-01-01', end_date: null },
      ] as TenantView[],
    })
  })

  it('lists users from multiple tenants with their tenant labels', async () => {
    vi.mocked(client.dash.listAllUsers).mockResolvedValue({
      users: [
        makeRow({ id: 1, username: 'acme-user', tenant_id: 2, tenant_name: 'Acme Brokers' }),
        makeRow({ id: 2, username: 'globex-user', tenant_id: 5, tenant_name: 'Globex' }),
      ],
      page: 1,
      page_size: 25,
      total: 2,
    } as AllUsersListResponse)

    renderPage()

    expect(await screen.findByText('acme-user')).toBeInTheDocument()
    expect(screen.getByText('globex-user')).toBeInTheDocument()
    expect(screen.getAllByText('2 · Acme Brokers').length).toBeGreaterThan(0)
    expect(screen.getByText('5 · Globex')).toBeInTheDocument()
  })

  it('switches the act-as tenant when viewing a row', async () => {
    const user = userEvent.setup()
    vi.mocked(client.dash.listAllUsers).mockResolvedValue({
      users: [makeRow({ id: 7, tenant_id: 5 })],
      page: 1,
      page_size: 25,
      total: 1,
    } as AllUsersListResponse)

    renderPage()
    await user.click(await screen.findByRole('button', { name: 'View' }))

    expect(setActAsTenant).toHaveBeenCalledWith(5)
  })
})
