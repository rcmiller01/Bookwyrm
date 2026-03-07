import { KeyboardEvent, ReactNode, useEffect, useMemo, useRef, useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useLocalStorageState } from '../hooks/useLocalStorageState'
import { fetchJSON } from '../lib/api'

type NavItem = {
  section: string
  items: { label: string; path: string }[]
}

type SearchResult = {
  id: string
  title: string
  subtitle: string
  kind: 'book' | 'author'
}

type ReadinessResponse = {
  status: string
  ready: boolean
  blocking_count: number
  warning_count: number
}

const navItems: NavItem[] = [
  { section: 'Dashboard', items: [{ label: 'Dashboard', path: '/' }] },
  {
    section: 'Library',
    items: [
      { label: 'Authors', path: '/library/authors' },
      { label: 'Books', path: '/library/books' },
      { label: 'Series', path: '/library/series' }
    ]
  },
  {
    section: 'Activity',
    items: [
      { label: 'Queue', path: '/activity/queue' },
      { label: 'History', path: '/activity/history' },
      { label: 'Import List', path: '/activity/import-list' }
    ]
  },
  {
    section: 'Wanted',
    items: [
      { label: 'Missing', path: '/wanted/missing' },
      { label: 'Cutoff Unmet', path: '/wanted/cutoff-unmet' }
    ]
  },
  {
    section: 'Settings',
    items: [
      { label: 'Media Management', path: '/settings/media-management' },
      { label: 'Profiles', path: '/settings/profiles' },
      { label: 'Indexers', path: '/settings/indexers' },
      { label: 'Download Clients', path: '/settings/download-clients' },
      { label: 'Metadata', path: '/settings/metadata' },
      { label: 'General', path: '/settings/general' }
    ]
  },
  {
    section: 'System',
    items: [
      { label: 'Setup', path: '/system/setup' },
      { label: 'Status', path: '/system/status' },
      { label: 'Tasks', path: '/system/tasks' },
      { label: 'Logs', path: '/system/logs' }
    ]
  }
]

export function AppShell({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  const [sidebarCollapsed, setSidebarCollapsed] = useLocalStorageState<boolean>('ui.sidebar.collapsed', false)
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<SearchResult[]>([])
  const [open, setOpen] = useState(false)
  const [recentSearches, setRecentSearches] = useLocalStorageState<string[]>('ui.search.recent', [])
  const readinessQuery = useQuery({
    queryKey: ['system', 'readiness'],
    queryFn: () => fetchJSON<ReadinessResponse>('/api/v1/system/readiness'),
    refetchInterval: 30000
  })
  const searchRef = useRef<HTMLInputElement>(null)
  const activeAbort = useRef<AbortController | null>(null)

  useEffect(() => {
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      const target = event.target as HTMLElement | null
      const isTypingElement =
        target?.tagName === 'INPUT' ||
        target?.tagName === 'TEXTAREA' ||
        target?.tagName === 'SELECT' ||
        Boolean(target?.closest('[contenteditable="true"]'))
      if (event.key === '/' && !isTypingElement) {
        event.preventDefault()
        searchRef.current?.focus()
      }
      if (event.key === 'Escape') {
        setOpen(false)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])

  useEffect(() => {
    const trimmed = query.trim()
    if (!trimmed) {
      setResults([])
      setOpen(false)
      activeAbort.current?.abort()
      return
    }
    const controller = new AbortController()
    activeAbort.current?.abort()
    activeAbort.current = controller
    const timer = window.setTimeout(async () => {
      try {
        const response = await fetch(`/api/v1/search?q=${encodeURIComponent(trimmed)}`, {
          headers: { Accept: 'application/json' },
          signal: controller.signal
        })
        if (!response.ok) {
          setResults([])
          setOpen(false)
          return
        }
        const payload = (await response.json()) as {
          works?: Array<{
            id?: string
            title?: string
            authors?: Array<{ id?: string; name?: string }>
          }>
        }

        const mappedBooks: SearchResult[] =
          payload.works?.slice(0, 6).map((work) => ({
            id: work.id?.trim() || '',
            kind: 'book',
            title: work.title?.trim() || work.id?.trim() || 'Unknown Book',
            subtitle: (work.authors ?? []).map((author) => author.name?.trim()).filter(Boolean).join(', ')
          })) ?? []

        const authorMap = new Map<string, SearchResult>()
        for (const work of payload.works ?? []) {
          for (const author of work.authors ?? []) {
            const authorID = author.id?.trim()
            const name = author.name?.trim()
            if (!authorID || !name || authorMap.has(authorID)) continue
            authorMap.set(authorID, {
              id: authorID,
              kind: 'author',
              title: name,
              subtitle: 'Author'
            })
          }
        }

        const mapped = [...mappedBooks, ...Array.from(authorMap.values()).slice(0, 4)].filter((entry) => entry.id)
        setResults(mapped)
        setOpen(mapped.length > 0)
      } catch {
        if (!controller.signal.aborted) {
          setResults([])
          setOpen(false)
        }
      }
    }, 250)
    return () => window.clearTimeout(timer)
  }, [query])

  const recentItems = useMemo(() => recentSearches.slice(0, 10), [recentSearches])

  const openEntry = (entry: SearchResult, queryValue?: string) => {
    if (!entry.id.trim()) return
    if (queryValue?.trim()) {
      const next = [queryValue.trim(), ...recentSearches.filter((item) => item !== queryValue.trim())].slice(0, 10)
      setRecentSearches(next)
    }
    setOpen(false)
    setQuery('')
    navigate(entry.kind === 'author' ? `/library/authors/${encodeURIComponent(entry.id)}` : `/library/books/${encodeURIComponent(entry.id)}`)
  }

  const onSearchKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') {
      event.preventDefault()
      const best = results[0]
      if (best) {
        openEntry(best, query)
      }
    }
  }

  return (
    <div className="flex min-h-screen bg-slate-950">
      {mobileNavOpen ? (
        <button
          aria-label="Close navigation"
          className="fixed inset-0 z-40 bg-black/50 md:hidden"
          onClick={() => setMobileNavOpen(false)}
        />
      ) : null}
      <aside
        className={[
          'fixed left-0 top-0 z-50 h-screen border-r border-slate-800 bg-slate-900/95 p-4 transition-transform duration-150 md:static md:z-auto md:h-auto md:bg-slate-900/70',
          mobileNavOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0',
          sidebarCollapsed ? 'w-72 md:w-20' : 'w-72'
        ].join(' ')}
      >
        <div className="mb-6 flex items-center justify-between gap-2">
          {!sidebarCollapsed ? <h1 className="text-xl font-semibold tracking-wide text-slate-100">Bookwyrm</h1> : <h1 className="text-xl font-semibold tracking-wide text-slate-100">BW</h1>}
          <button
            className="rounded border border-slate-700 px-2 py-1 text-xs text-slate-300"
            onClick={() => setSidebarCollapsed((prev) => !prev)}
          >
            {sidebarCollapsed ? 'Expand' : 'Collapse'}
          </button>
        </div>
        <nav className="space-y-6">
          {navItems.map((group) => (
            <div key={group.section}>
              {!sidebarCollapsed ? <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-400">{group.section}</p> : null}
              <ul className="space-y-1">
                {group.items.map((item) => (
                  <li key={item.path}>
                    <NavLink
                      to={item.path}
                      onClick={() => setMobileNavOpen(false)}
                      className={({ isActive }) =>
                        [
                          'block rounded px-3 py-2 text-sm transition-colors',
                          isActive ? 'bg-sky-600/25 text-sky-300' : 'text-slate-200 hover:bg-slate-800'
                        ].join(' ')
                      }
                      title={sidebarCollapsed ? item.label : undefined}
                    >
                      {sidebarCollapsed ? item.label.slice(0, 1) : item.label}
                    </NavLink>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </nav>
      </aside>
      <main className="flex-1 p-3 md:p-6">
        {readinessQuery.data && !readinessQuery.data.ready ? (
          <div className="mb-3 rounded border border-amber-800/60 bg-amber-950/40 px-3 py-2 text-sm text-amber-200">
            System is in {readinessQuery.data.status.replace(/_/g, ' ')} mode. {readinessQuery.data.blocking_count} blocking issue(s), {readinessQuery.data.warning_count} warning(s).{' '}
            <NavLink className="underline hover:text-amber-100" to="/system/setup">
              Open Setup Checklist
            </NavLink>
          </div>
        ) : null}
        <div className="mb-3 flex items-center justify-between md:hidden">
          <button
            className="rounded border border-slate-700 px-2 py-1 text-xs text-slate-300"
            onClick={() => setMobileNavOpen(true)}
          >
            Menu
          </button>
          <button
            className="rounded border border-slate-700 px-2 py-1 text-xs text-slate-300"
            onClick={() => searchRef.current?.focus()}
          >
            Search
          </button>
        </div>
        <div className="mb-4">
          <div className="relative max-w-2xl">
            <input
              ref={searchRef}
              className="w-full rounded border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-100 outline-none focus:border-sky-500"
              placeholder="Quick jump to books/authors (/)"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              onFocus={() => setOpen(results.length > 0 || recentItems.length > 0)}
              onKeyDown={onSearchKeyDown}
            />
            {open ? (
              <div className="absolute z-40 mt-1 max-h-80 w-full overflow-auto rounded border border-slate-700 bg-slate-900 shadow-xl">
                {results.map((item) => (
                  <button
                    key={`${item.kind}:${item.id}`}
                    className="block w-full border-b border-slate-800 px-3 py-2 text-left hover:bg-slate-800"
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={() => openEntry(item, query)}
                  >
                    <p className="text-xs uppercase text-slate-500">{item.kind}</p>
                    <p className="text-sm text-slate-100">{item.title}</p>
                    <p className="text-xs text-slate-400">{item.subtitle || item.id}</p>
                  </button>
                ))}
                {results.length === 0 ? (
                  <div className="px-3 py-2">
                    <p className="text-xs uppercase text-slate-500">Recent</p>
                    <div className="mt-1 flex flex-wrap gap-1">
                      {recentItems.map((item) => (
                        <button
                          key={item}
                          className="rounded border border-slate-700 px-2 py-0.5 text-xs text-slate-300 hover:bg-slate-800"
                          onMouseDown={(event) => event.preventDefault()}
                          onClick={() => setQuery(item)}
                        >
                          {item}
                        </button>
                      ))}
                      {recentItems.length === 0 ? <span className="text-xs text-slate-500">No recent searches</span> : null}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>
        </div>
        {children}
      </main>
    </div>
  )
}
