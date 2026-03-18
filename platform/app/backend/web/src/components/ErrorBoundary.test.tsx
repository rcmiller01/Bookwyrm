import { render, screen } from '@testing-library/react'
import { ErrorBoundary } from './ErrorBoundary'

function ThrowingComponent(): JSX.Element {
  throw new Error('Test crash')
}

describe('ErrorBoundary', () => {
  // Suppress React error boundary console output during test
  const originalError = console.error
  beforeAll(() => { console.error = vi.fn() })
  afterAll(() => { console.error = originalError })

  it('renders children when no error', () => {
    render(
      <ErrorBoundary>
        <div>Hello</div>
      </ErrorBoundary>
    )
    expect(screen.getByText('Hello')).toBeTruthy()
  })

  it('renders error UI when child throws', () => {
    render(
      <ErrorBoundary>
        <ThrowingComponent />
      </ErrorBoundary>
    )
    expect(screen.getByText('Something went wrong')).toBeTruthy()
    expect(screen.getByText('Test crash')).toBeTruthy()
    expect(screen.getByText('Reload')).toBeTruthy()
    expect(screen.getByText('Go Home')).toBeTruthy()
    expect(screen.getByText('Copy error details')).toBeTruthy()
  })
})
