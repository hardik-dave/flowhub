import { fireEvent, render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { UsersPage } from './UsersPage'
import * as client from '../api/client'
import type { TenantView, UsersListResponse } from '../api/types'

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof client>('../api/client')
  return {
    ...actual,
    dash: { ...actual.dash, listUsers: vi.fn(), createUser: vi.fn(), getTenant: vi.fn() },
  }
})

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <UsersPage />
    </QueryClientProvider>,
  )
}

describe('UsersPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // Regression: the backend used to serialize an empty license list as
  // null, which made `u.licenses.length` throw and blank the whole app.
  it('renders a user with licenses: null without crashing', async () => {
    vi.mocked(client.dash.listUsers).mockResolvedValue({
      users: [
        {
          id: 1,
          username: 'platform-admin',
          first_name: 'Platform',
          last_name: 'Admin',
          mobile: '+919876543210',
          email: '',
          city: '',
          state: '',
          status: 'ACTIVE',
          mobile_verified: true,
          role: 'SUPERADMIN',
          licenses: null,
        },
      ],
      page: 1,
      page_size: 20,
      total: 1,
    } as unknown as UsersListResponse)

    renderPage()

    expect(await screen.findByText('platform-admin')).toBeInTheDocument()
  })

  // The create-user modal must offer the tenant's granted products as a
  // dropdown (not a free-text "flowos" default) plus an access window,
  // so a created user can actually log in.
  it('offers tenant products and an access window on create', async () => {
    vi.mocked(client.dash.listUsers).mockResolvedValue({
      users: [], page: 1, page_size: 20, total: 0,
    } as unknown as UsersListResponse)
    vi.mocked(client.dash.getTenant).mockResolvedValue({
      products: ['optionalyzer', 'flowos'],
    } as unknown as TenantView)

    renderPage()

    fireEvent.click(await screen.findByRole('button', { name: 'Create user' }))

    expect(await screen.findByRole('option', { name: 'optionalyzer' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'flowos' })).toBeInTheDocument()
    expect(screen.getByText('Access window')).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'MONTHLY' })).toBeInTheDocument()
  })
})
