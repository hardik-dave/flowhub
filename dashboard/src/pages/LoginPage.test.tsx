import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { LoginPage } from './LoginPage'
import { AuthProvider } from '../auth/AuthContext'
import * as client from '../api/client'

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof client>('../api/client')
  return {
    ...actual,
    dash: {
      login: vi.fn(),
      logout: vi.fn(),
      listTenants: vi.fn().mockResolvedValue({ tenants: [] }),
    },
  }
})

function renderLogin() {
  render(
    <MemoryRouter initialEntries={['/login']}>
      <AuthProvider>
        <LoginPage />
      </AuthProvider>
    </MemoryRouter>,
  )
}

describe('LoginPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('submits credentials and calls dash.login', async () => {
    const user = userEvent.setup()
    vi.mocked(client.dash.login).mockResolvedValue({
      session_token: 'tok',
      user: { id: 1, username: 'alice', first_name: 'A', role: 'ADMIN', tenant_id: 1 },
    })
    renderLogin()
    await user.type(screen.getByLabelText(/Username/i), 'alice')
    await user.type(screen.getByLabelText(/Password/i), 'pw')
    await user.click(screen.getByRole('button', { name: /Sign in/i }))
    expect(client.dash.login).toHaveBeenCalledWith('alice', 'pw')
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: /Signing in/i })).not.toBeInTheDocument()
    })
  })

  it('shows ApiError message verbatim', async () => {
    const user = userEvent.setup()
    vi.mocked(client.dash.login).mockRejectedValue(new client.ApiError(401, 'Invalid username or password.', {}))
    renderLogin()
    await user.type(screen.getByLabelText(/Username/i), 'x')
    await user.type(screen.getByLabelText(/Password/i), 'y')
    await user.click(screen.getByRole('button', { name: /Sign in/i }))
    await screen.findByText(/Invalid username or password\./i)
  })
})
