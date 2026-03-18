import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { render, RenderOptions } from '@testing-library/react'
import { ToastProvider } from './components/ToastProvider'
import { ReactElement } from 'react'

export function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  })
}

export function renderWithProviders(
  ui: ReactElement,
  { route = '/', ...options }: RenderOptions & { route?: string } = {}
) {
  const queryClient = createTestQueryClient()
  function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <ToastProvider>
          <MemoryRouter initialEntries={[route]}>
            {children}
          </MemoryRouter>
        </ToastProvider>
      </QueryClientProvider>
    )
  }
  return { ...render(ui, { wrapper: Wrapper, ...options }), queryClient }
}

type MockRoute = {
  path: string | RegExp
  response: unknown
  status?: number
}

export function mockFetchRoutes(routes: MockRoute[]) {
  const original = globalThis.fetch
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    for (const route of routes) {
      const match = typeof route.path === 'string'
        ? url.includes(route.path)
        : route.path.test(url)
      if (match) {
        return new Response(JSON.stringify(route.response), {
          status: route.status ?? 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
    }
    return new Response(JSON.stringify({ error: 'not found' }), { status: 404 })
  }) as typeof fetch

  return () => { globalThis.fetch = original }
}
