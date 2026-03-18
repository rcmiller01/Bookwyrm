export type PageKey = 'books' | 'authors' | 'missing' | 'cutoff-unmet'

export type PresetView<T> = {
  id: string
  pageKey: PageKey
  name: string
  state: T
  isDefault: true
  version: number
}

type Registry = Record<PageKey, PresetView<any>[]>

const registry: Registry = {
  books: [
    {
      id: 'preset.books.all.v1',
      pageKey: 'books',
      name: 'All Books',
      isDefault: true,
      version: 1,
      state: {
        query: '',
        monitorFilter: 'all',
        missingOnly: false,
        cutoffOnly: false,
        formatFilter: 'all',
        sortKey: 'title',
        sortDir: 'asc'
      }
    },
    {
      id: 'preset.books.monitored.v1',
      pageKey: 'books',
      name: 'Monitored',
      isDefault: true,
      version: 1,
      state: {
        query: '',
        monitorFilter: 'monitored',
        missingOnly: false,
        cutoffOnly: false,
        formatFilter: 'all',
        sortKey: 'title',
        sortDir: 'asc'
      }
    },
    {
      id: 'preset.books.missing.v1',
      pageKey: 'books',
      name: 'Missing',
      isDefault: true,
      version: 1,
      state: {
        query: '',
        monitorFilter: 'all',
        missingOnly: true,
        cutoffOnly: false,
        formatFilter: 'all',
        sortKey: 'title',
        sortDir: 'asc'
      }
    },
    {
      id: 'preset.books.cutoff-unmet.v1',
      pageKey: 'books',
      name: 'Cutoff Unmet',
      isDefault: true,
      version: 1,
      state: {
        query: '',
        monitorFilter: 'all',
        missingOnly: false,
        cutoffOnly: true,
        formatFilter: 'all',
        sortKey: 'title',
        sortDir: 'asc'
      }
    },
    {
      id: 'preset.books.audiobooks.v1',
      pageKey: 'books',
      name: 'Audiobooks',
      isDefault: true,
      version: 1,
      state: {
        query: '',
        monitorFilter: 'all',
        missingOnly: false,
        cutoffOnly: false,
        formatFilter: 'm4b',
        sortKey: 'title',
        sortDir: 'asc'
      }
    },
    {
      id: 'preset.books.ebooks.v1',
      pageKey: 'books',
      name: 'Ebooks',
      isDefault: true,
      version: 1,
      state: {
        query: '',
        monitorFilter: 'all',
        missingOnly: false,
        cutoffOnly: false,
        formatFilter: 'epub',
        sortKey: 'title',
        sortDir: 'asc'
      }
    }
  ],
  authors: [
    {
      id: 'preset.authors.all.v1',
      pageKey: 'authors',
      name: 'All Authors',
      isDefault: true,
      version: 1,
      state: { query: '', monitorFilter: 'all', sortKey: 'name', sortDir: 'asc' }
    },
    {
      id: 'preset.authors.monitored.v1',
      pageKey: 'authors',
      name: 'Monitored Authors',
      isDefault: true,
      version: 1,
      state: { query: '', monitorFilter: 'monitored', sortKey: 'name', sortDir: 'asc' }
    }
  ],
  missing: [
    {
      id: 'preset.missing.all.v1',
      pageKey: 'missing',
      name: 'All Missing',
      isDefault: true,
      version: 1,
      state: { query: '', mediaType: 'all' }
    },
    {
      id: 'preset.missing.ebooks.v1',
      pageKey: 'missing',
      name: 'Missing (Ebooks)',
      isDefault: true,
      version: 1,
      state: { query: '', mediaType: 'ebook' }
    },
    {
      id: 'preset.missing.audiobooks.v1',
      pageKey: 'missing',
      name: 'Missing (Audiobooks)',
      isDefault: true,
      version: 1,
      state: { query: '', mediaType: 'audiobook' }
    }
  ],
  'cutoff-unmet': [
    {
      id: 'preset.cutoff.all.v1',
      pageKey: 'cutoff-unmet',
      name: 'All Cutoff Unmet',
      isDefault: true,
      version: 1,
      state: { query: '', profileFilter: 'all', mediaType: 'all' }
    },
    {
      id: 'preset.cutoff.ebooks.v1',
      pageKey: 'cutoff-unmet',
      name: 'Ebook Upgrades',
      isDefault: true,
      version: 1,
      state: { query: '', profileFilter: 'all', mediaType: 'ebook' }
    },
    {
      id: 'preset.cutoff.audiobooks.v1',
      pageKey: 'cutoff-unmet',
      name: 'Audiobook Upgrades',
      isDefault: true,
      version: 1,
      state: { query: '', profileFilter: 'all', mediaType: 'audiobook' }
    },
    {
      id: 'preset.cutoff.ignore-hidden.v1',
      pageKey: 'cutoff-unmet',
      name: 'Ignore Upgrades Hidden',
      isDefault: true,
      version: 1,
      state: { query: '', profileFilter: 'all', mediaType: 'all' }
    }
  ]
}

export function getPresetsForPage<T>(pageKey: PageKey): PresetView<T>[] {
  return registry[pageKey] as PresetView<T>[]
}
