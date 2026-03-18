import { Component, ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  private handleReload = () => {
    window.location.reload()
  }

  private handleGoHome = () => {
    window.location.href = '/'
  }

  private handleCopy = () => {
    const { error } = this.state
    if (!error) return
    const text = `${error.name}: ${error.message}\n${error.stack ?? ''}`
    navigator.clipboard.writeText(text).catch(() => {})
  }

  render() {
    const { error } = this.state
    if (!error) return this.props.children

    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-950 p-8">
        <div className="w-full max-w-lg space-y-6 text-center">
          <h1 className="text-2xl font-semibold text-slate-100">Something went wrong</h1>
          <p className="text-sm text-slate-400">
            An unexpected error caused the UI to crash. You can try reloading the page or navigating home.
          </p>
          <pre className="max-h-40 overflow-auto rounded border border-slate-700 bg-slate-900 p-3 text-left text-xs text-red-400">
            {error.message}
          </pre>
          <div className="flex justify-center gap-3">
            <button
              onClick={this.handleReload}
              className="rounded bg-sky-600 px-4 py-2 text-sm font-medium text-white hover:bg-sky-500"
            >
              Reload
            </button>
            <button
              onClick={this.handleGoHome}
              className="rounded border border-slate-600 px-4 py-2 text-sm text-slate-300 hover:border-slate-500"
            >
              Go Home
            </button>
            <button
              onClick={this.handleCopy}
              className="rounded border border-slate-600 px-4 py-2 text-sm text-slate-300 hover:border-slate-500"
            >
              Copy error details
            </button>
          </div>
        </div>
      </div>
    )
  }
}
