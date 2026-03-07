import { screen, waitFor } from '@testing-library/react'
import { DashboardPage } from './DashboardPage'
import { renderWithProviders, mockFetchRoutes } from '../test-utils'

describe('DashboardPage', () => {
  let cleanup: () => void

  beforeEach(() => {
    cleanup = mockFetchRoutes([
      { path: '/api/v1/import/stats', response: { library_root: '/books', library_root_configured: true, counts_by_status: {} } },
      { path: '/api/v1/download/jobs', response: { items: [{ status: 'downloading' }, { status: 'completed' }] } },
      { path: '/api/v1/import/jobs', response: { items: [] } },
      { path: '/api/v1/download/clients', response: { items: [{ id: 'sab', enabled: true, reliability_score: 95, tier: 'healthy' }] } },
      { path: '/ui-api/metadata/reliability', response: { providers: [{ name: 'openlibrary', score: 90, status: 'healthy' }] } },
      { path: '/ui-api/indexer/backends', response: { backends: [{ id: 'prowlarr', name: 'Prowlarr', enabled: true, reliability_score: 92, tier: 'healthy' }] } },
    ])
  })

  afterEach(() => cleanup())

  it('renders dashboard heading', async () => {
    renderWithProviders(<DashboardPage />)
    expect(screen.getByText('Dashboard')).toBeTruthy()
  })

  it('shows download stats after data loads', async () => {
    renderWithProviders(<DashboardPage />)
    await waitFor(() => {
      expect(screen.getByText('Downloads In Progress')).toBeTruthy()
    })
  })

  it('shows setup checklist', async () => {
    renderWithProviders(<DashboardPage />)
    await waitFor(() => {
      expect(screen.getByText('Library root configured')).toBeTruthy()
    })
  })
})
