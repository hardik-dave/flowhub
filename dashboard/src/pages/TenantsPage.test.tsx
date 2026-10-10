import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { TenantsPage } from './TenantsPage'
import * as client from '../api/client'
import type { TenantView } from '../api/types'

vi.mock('../auth/AuthContext', () => ({
  useAuth: () => ({ isPlatformAdmin: true }),
}))

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof client>('../api/client')
  return {
    ...actual,
    dash: {
      ...actual.dash,
      listTenants: vi.fn(),
      createTenant: vi.fn(),
      updateTenantStatus: vi.fn(),
    },
  }
})

function makeTenant(over: Partial<TenantView> = {}): TenantView {
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
    ...over,
  }
}

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <TenantsPage />
    </QueryClientProvider>,
  )
}

describe('TenantsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(client.dash.listTenants).mockResolvedValue({ tenants: [makeTenant()] })
  })

  it('creates a tenant and shows the generated admin password once', async () => {
    const user = userEvent.setup()
    vi.mocked(client.dash.createTenant).mockResolvedValue({
      tenant_id: 2,
      admin_user_id: 9,
      admin_username: 'acme-admin',
      password: 'generated-pass-123',
    })

    renderPage()
    await screen.findByRole('button', { name: 'Change status' })
    await user.click(screen.getByRole('button', { name: 'Create tenant' }))

    await user.type(screen.getByLabelText('Slug *'), 'acme')
    await user.type(screen.getByLabelText('Name *'), 'Acme Brokers')
    await user.type(screen.getByLabelText('Username *'), 'acme-admin')
    await user.type(screen.getByLabelText('Mobile * (E.164)'), '+919876543210')
    await user.click(screen.getByRole('button', { name: 'Create' }))

    expect(await screen.findByText('generated-pass-123')).toBeInTheDocument()
    const body = vi.mocked(client.dash.createTenant).mock.calls[0][0]
    expect(body.slug).toBe('acme')
    expect(body.products).toContain('flowos')
    expect(body.admin.username).toBe('acme-admin')
  })

  it('changes a tenant status with a reason', async () => {
    const user = userEvent.setup()
    vi.mocked(client.dash.updateTenantStatus).mockResolvedValue({ ok: true })

    renderPage()
    await screen.findByRole('button', { name: 'Change status' })
    await user.click(screen.getByRole('button', { name: 'Change status' }))
    await user.type(screen.getByLabelText('Reason (required)'), 'Trial ended')
    await user.selectOptions(screen.getByLabelText('New status'), 'INACTIVE')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(client.dash.updateTenantStatus).toHaveBeenCalledWith(1, 'INACTIVE', 'Trial ended')
  })
})
