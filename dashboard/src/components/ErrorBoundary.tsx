import { Component, type ErrorInfo, type ReactNode } from 'react'
import { Button, Card } from './ui'

interface Props {
  children: ReactNode
}

interface State {
  error: Error | null
}

// Catches render-time errors in the routed tree so a single bad field can
// never leave the operator staring at a blank white screen.
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[ErrorBoundary]', error.message, info.componentStack)
  }

  render() {
    if (!this.state.error) return this.props.children
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-100 p-4">
        <Card className="w-full max-w-lg">
          <h1 className="text-lg font-bold text-red-700">Something went wrong</h1>
          <p className="mt-2 text-sm text-slate-600">
            The dashboard hit an unexpected error and could not finish loading.
          </p>
          <p className="mt-2 rounded-md bg-red-50 px-3 py-2 font-mono text-xs text-red-700">
            {this.state.error.message}
          </p>
          <Button className="mt-4" onClick={() => window.location.reload()}>
            Reload dashboard
          </Button>
        </Card>
      </div>
    )
  }
}
