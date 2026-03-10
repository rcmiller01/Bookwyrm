type WantedPayloadOptions = {
  enabled: boolean
  priority?: number
  cadenceMinutes?: number
  profileID?: string
  ignoreUpgrades?: boolean
  formats?: string[] | null
  languages?: string[] | null
}

function normalizeStringList(values?: string[] | null): string[] {
  if (!values || values.length === 0) return []
  const out: string[] = []
  for (const value of values) {
    const normalized = (value ?? '').trim()
    if (!normalized) continue
    out.push(normalized)
  }
  return out
}

export function buildWantedPayload(options: WantedPayloadOptions) {
  const payload: Record<string, unknown> = {
    enabled: options.enabled,
    priority: options.priority ?? 100,
    cadence_minutes: options.cadenceMinutes ?? 60,
    formats: normalizeStringList(options.formats),
    languages: normalizeStringList(options.languages)
  }
  if (options.profileID && options.profileID.trim().length > 0) {
    payload.profile_id = options.profileID.trim()
  }
  if (typeof options.ignoreUpgrades === 'boolean') {
    payload.ignore_upgrades = options.ignoreUpgrades
  }
  return payload
}
