import { screen, waitFor } from '@testing-library/react'
import { ProfilesPage } from './ProfilesPage'
import { renderWithProviders, mockFetchRoutes } from '../test-utils'

const mockProfiles = [
  { profile: { id: 'default', name: 'Default', cutoff_quality: 'epub', default_profile: true }, qualities: [{ profile_id: 'default', quality: 'epub', rank: 1 }] },
  { profile: { id: 'audio', name: 'Audiobook', cutoff_quality: 'audiobook', default_profile: false }, qualities: [{ profile_id: 'audio', quality: 'audiobook', rank: 1 }] },
]

describe('ProfilesPage', () => {
  let cleanup: () => void

  beforeEach(() => {
    cleanup = mockFetchRoutes([
      { path: '/ui-api/indexer/profiles', response: { items: mockProfiles, default_profile_id: 'default' } },
    ])
  })

  afterEach(() => cleanup())

  it('renders profiles heading', async () => {
    renderWithProviders(<ProfilesPage />)
    expect(screen.getByText('Profiles')).toBeTruthy()
  })

  it('displays profiles after data loads', async () => {
    renderWithProviders(<ProfilesPage />)
    await waitFor(() => {
      expect(screen.getAllByText('Default').length).toBeGreaterThan(0)
      expect(screen.getByText('Audiobook')).toBeTruthy()
    })
  })
})
