type ManualSearchParams = {
  workID: string
  title?: string
  author?: string
  formats?: string[]
  languages?: string[]
  autorun?: boolean
  autoGrab?: boolean
}

export function buildManualSearchPath(params: ManualSearchParams): string {
  const query = new URLSearchParams()
  query.set('workID', params.workID)
  if (params.title?.trim()) {
    query.set('title', params.title.trim())
  }
  if (params.author?.trim()) {
    query.set('author', params.author.trim())
  }
  if (params.formats?.length) {
    query.set('formats', params.formats.join(','))
  }
  if (params.languages?.length) {
    query.set('languages', params.languages.join(','))
  }
  if (params.autorun) {
    query.set('autorun', '1')
  }
  return `/library/books/manual-search?${query.toString()}`
}

