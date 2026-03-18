import { buildWantedWorkPayload, getProfileFormats, pickProfileIDForMediaType, type ProfilesResponse } from './wantedWork'

const profiles: ProfilesResponse = {
  default_profile_id: 'default',
  items: [
    {
      profile: {
        id: 'default',
        name: 'Default Ebook',
        cutoff_quality: 'epub',
        upgrade_action: 'ask',
        default_profile: true
      },
      qualities: [
        { profile_id: 'default', quality: 'epub', rank: 1 },
        { profile_id: 'default', quality: 'azw3', rank: 2 }
      ]
    },
    {
      profile: {
        id: 'audio',
        name: 'Audiobook',
        cutoff_quality: 'audiobook',
        upgrade_action: 'ask',
        default_profile: false
      },
      qualities: [
        { profile_id: 'audio', quality: 'audiobook', rank: 1 }
      ]
    }
  ]
}

describe('wantedWork helpers', () => {
  it('expands audiobook profiles into concrete searchable formats', () => {
    expect(getProfileFormats(profiles, 'audio')).toEqual(['m4b', 'mp3', 'm4a', 'flac'])
  })

  it('selects the dedicated audiobook profile when requested', () => {
    expect(pickProfileIDForMediaType(profiles, 'audiobook')).toBe('audio')
    expect(buildWantedWorkPayload(profiles, undefined, 'audiobook')).toMatchObject({
      profile_id: 'audio',
      formats: ['m4b', 'mp3', 'm4a', 'flac']
    })
  })
})
