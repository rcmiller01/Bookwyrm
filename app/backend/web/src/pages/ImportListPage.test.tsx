import { screen, waitFor } from '@testing-library/react'
import { ImportListPage } from './ImportListPage'
import { renderWithProviders, mockFetchRoutes } from '../test-utils'

const mockImportJobs = [
  { id: 1, status: 'needs_review', source_path: '/incoming/book.epub', work_id: 'w1', matched_title: 'Test Book', created_at: '2024-01-01T00:00:00Z' },
  { id: 2, status: 'needs_review', source_path: '/incoming/book2.epub', work_id: 'w2', matched_title: 'Another Book', created_at: '2024-01-02T00:00:00Z' },
]

describe('ImportListPage', () => {
  let cleanup: () => void

  beforeEach(() => {
    cleanup = mockFetchRoutes([
      { path: '/api/v1/import/jobs', response: { items: mockImportJobs } },
    ])
  })

  afterEach(() => cleanup())

  it('renders import list heading', async () => {
    renderWithProviders(<ImportListPage />)
    expect(screen.getByText('Import Needs Review')).toBeTruthy()
  })

  it('displays import jobs after data loads', async () => {
    renderWithProviders(<ImportListPage />)
    await waitFor(() => {
      expect(screen.getByText(/book\.epub/)).toBeTruthy()
    })
  })
})
