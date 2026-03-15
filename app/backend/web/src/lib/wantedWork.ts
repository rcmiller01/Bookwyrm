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

export type WantedWorkUpsertPayload = {
  enabled: boolean
  priority: number
  cadence_minutes: number
  profile_id?: string
  formats: string[]
  languages: string[]
}

export function buildWantedWorkPayload(
  profiles: ProfilesResponse | undefined,
  profileID?: string
): WantedWorkUpsertPayload {
  const defaultProfileID = profiles?.default_profile_id?.trim() || ''
  const resolvedProfileID = profileID?.trim() || defaultProfileID
  const resolvedProfile = profiles?.items.find((item) => item.profile.id === resolvedProfileID)
    ?? profiles?.items.find((item) => item.profile.default_profile)
    ?? profiles?.items[0]

  const formats = (resolvedProfile?.qualities ?? [])
    .slice()
    .sort((a, b) => a.rank - b.rank)
    .map((quality) => quality.quality?.trim().toLowerCase())
    .filter((quality): quality is string => Boolean(quality))

  return {
    enabled: true,
    priority: 100,
    cadence_minutes: 60,
    profile_id: resolvedProfile?.profile.id || resolvedProfileID || undefined,
    formats,
    languages: []
  }
}
