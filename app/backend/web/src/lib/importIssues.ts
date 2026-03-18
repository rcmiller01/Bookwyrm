export function isMissingSourceError(value?: string): boolean {
  const text = (value || '').toLowerCase()
  return text.includes('source path no longer exists') || text.includes('cannot find the file specified')
}

export function importIssueLabel(value?: string): string {
  const text = (value || '').toLowerCase()
  if (isMissingSourceError(value)) return 'Source Missing'
  if (text.includes('collision')) return 'Collision'
  if (text.includes('confidence')) return 'Low confidence match'
  if (text.includes('ambiguous')) return 'Ambiguous match'
  if (text.includes('isbn')) return 'Ambiguous ISBN'
  return value?.trim() || 'Needs review'
}

export function importRecoveryHint(value?: string): string | null {
  if (isMissingSourceError(value)) {
    return 'Retry the download to recreate the missing source folder, then import again.'
  }
  return null
}
