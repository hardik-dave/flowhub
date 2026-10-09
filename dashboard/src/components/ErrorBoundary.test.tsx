import { render, screen } from '@testing-library/react'
import { ErrorBoundary } from './ErrorBoundary'

function Boom(): never {
  throw new Error('kaboom')
}

describe('ErrorBoundary', () => {
  it('shows a readable fallback instead of a blank screen', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>,
    )
    expect(screen.getByText(/Something went wrong/i)).toBeInTheDocument()
    expect(screen.getByText('kaboom')).toBeInTheDocument()
    spy.mockRestore()
  })
})
