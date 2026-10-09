import { render, screen } from '@testing-library/react'
import { Badge, Button, ErrorText, Modal } from './ui'

describe('ui components', () => {
  it('Badge maps status values', () => {
    render(<Badge value="ACTIVE" />)
    expect(screen.getByText('ACTIVE')).toBeInTheDocument()
  })

  it('Button renders variants', () => {
    render(
      <>
        <Button>primary</Button>
        <Button variant="secondary">secondary</Button>
        <Button variant="danger">danger</Button>
        <Button variant="ghost">ghost</Button>
      </>
    )
    expect(screen.getByText('primary')).toBeInTheDocument()
    expect(screen.getByText('secondary')).toBeInTheDocument()
    expect(screen.getByText('danger')).toBeInTheDocument()
    expect(screen.getByText('ghost')).toBeInTheDocument()
  })

  it('ErrorText shows/hides children', () => {
    const { rerender } = render(<ErrorText />)
    expect(screen.queryByText(/error/i)).not.toBeInTheDocument()
    rerender(<ErrorText>Something went wrong</ErrorText>)
    expect(screen.getByText(/Something went wrong/i)).toBeInTheDocument()
  })

  it('Modal renders with title and close button', () => {
    render(
      <Modal title="Test Modal" onClose={() => {}}>
        <p>content</p>
      </Modal>
    )
    expect(screen.getByText('Test Modal')).toBeInTheDocument()
    expect(screen.getByText('content')).toBeInTheDocument()
    expect(screen.getByLabelText(/Close/i)).toBeInTheDocument()
  })
})
