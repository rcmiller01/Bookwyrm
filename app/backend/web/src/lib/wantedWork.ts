export type ProfileQuality = {
  profile_id: string
  quality: string
  rank: number
}

export type ProfileRecord = {
  id: string
  name: string
  cutoff_quality: string
  upgrade_action: string
  default_profile: boolean
}

export type ProfileWithQualities = {
  profile: ProfileRecord
  qualities: ProfileQuality[]
}

export type ProfilesResponse = {
  items: ProfileWithQualities[]
  default_profile_id: string
}

export type MediaType = 'all' | 'ebook' | 'audiobook'

export type WantedWorkUpsertPayload = {
  enabled: boolean
  priority: number
  cadence_minutes: number
  profile_id?: string
  formats: string[]
  languages: string[]
}

const AUDIOBOOK_FORMATS = ['m4b', 'mp3', 'm4a', 'flac']

function expandQualityToken(quality: string): string[] {
  const normalized = quality.trim().toLowerCase()
  if (!normalized) {
    return []
  }
  if (normalized === 'audiobook') {
    return [...AUDIOBOOK_FORMATS]
  }
  return [normalized]
}

function unique(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean)))
}

export function getProfileFormats(profiles: ProfilesResponse | undefined, profileID?: string): string[] {
  const defaultProfileID = profiles?.default_profile_id?.trim() || ''
  const resolvedProfileID = profileID?.trim() || defaultProfileID
  const resolvedProfile = profiles?.items.find((item) => item.profile.id === resolvedProfileID)
    ?? profiles?.items.find((item) => item.profile.default_profile)
    ?? profiles?.items[0]

  return unique(
    (resolvedProfile?.qualities ?? [])
      .slice()
      .sort((a, b) => a.rank - b.rank)
      .flatMap((quality) => expandQualityToken(quality.quality ?? ''))
  )
}

export function isAudiobookProfile(item: ProfileWithQualities | undefined): boolean {
  if (!item) {
    return false
  }
  const profileTokens = [
    item.profile.id,
    item.profile.name,
    item.profile.cutoff_quality,
    ...item.qualities.map((quality) => quality.quality)
  ]
    .map((value) => (value ?? '').trim().toLowerCase())
    .filter(Boolean)

  return profileTokens.some((value) => value.includes('audio') || value === 'm4b' || value === 'mp3' || value === 'm4a' || value === 'flac')
}

export function pickProfileIDForMediaType(profiles: ProfilesResponse | undefined, mediaType: MediaType = 'all'): string | undefined {
  if (!profiles?.items?.length) {
    return undefined
  }
  if (mediaType === 'audiobook') {
    return profiles.items.find((item) => isAudiobookProfile(item))?.profile.id
  }
  if (mediaType === 'ebook') {
    return profiles.items.find((item) => item.profile.default_profile && !isAudiobookProfile(item))?.profile.id
      ?? profiles.items.find((item) => !isAudiobookProfile(item))?.profile.id
      ?? profiles.default_profile_id
  }
  return profiles.default_profile_id || profiles.items.find((item) => item.profile.default_profile)?.profile.id || profiles.items[0]?.profile.id
}

export function buildWantedWorkPayload(
  profiles: ProfilesResponse | undefined,
  profileID?: string,
  mediaType: MediaType = 'all'
): WantedWorkUpsertPayload {
  const resolvedProfileID = profileID?.trim() || pickProfileIDForMediaType(profiles, mediaType) || ''
  const formats = getProfileFormats(profiles, resolvedProfileID)

  return {
    enabled: true,
    priority: 100,
    cadence_minutes: 60,
    profile_id: resolvedProfileID || undefined,
    formats,
    languages: []
  }
}
