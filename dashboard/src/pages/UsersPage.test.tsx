import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { UsersPage } from './UsersPage'
import * as client from '../api/client'
import type { UsersListResponse } from '../api/types'

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof client>('../api/client')
  return {
    ...actual,
    dash: { ...actual.dash, listUsers: vi.fn(), createUser: vi.fn() },
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
})
