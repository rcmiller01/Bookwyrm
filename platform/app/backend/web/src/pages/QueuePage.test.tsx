import { screen, waitFor } from '@testing-library/react'
import { QueuePage } from './QueuePage'
import { renderWithProviders, mockFetchRoutes } from '../test-utils'

const mockJobs = [
  { id: 1, work_id: 'w1', protocol: 'usenet', client_name: 'SABnzbd', status: 'downloading', output_path: '/tmp/book.epub', updated_at: '2024-01-01T00:00:00Z' },
  { id: 2, work_id: 'w2', protocol: 'torrent', client_name: 'qBittorrent', status: 'completed', output_path: '/tmp/book2.epub', updated_at: '2024-01-01T01:00:00Z' },
  { id: 3, work_id: 'w3', protocol: 'usenet', client_name: 'SABnzbd', status: 'failed', last_error: 'download timeout', updated_at: '2024-01-01T02:00:00Z' },
]

describe('QueuePage', () => {
  let cleanup: () => void

  beforeEach(() => {
    cleanup = mockFetchRoutes([
      { path: '/api/v1/download/jobs', response: { items: mockJobs } },
    ])
  })

  afterEach(() => cleanup())

  it('renders queue heading', async () => {
    renderWithProviders(<QueuePage />)
    expect(screen.getByText('Queue')).toBeTruthy()
  })

  it('displays download jobs after data loads', async () => {
    renderWithProviders(<QueuePage />)
    await waitFor(() => {
      expect(screen.getAllByText('SABnzbd').length).toBeGreaterThan(0)
    })
  })

  it('shows filter controls', async () => {
    renderWithProviders(<QueuePage />)
    await waitFor(() => {
      expect(screen.getByText('Show completed')).toBeTruthy()
    })
  })
})
