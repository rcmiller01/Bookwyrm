import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { BulkActionBar } from '../components/BulkActionBar'
import { FilterBar } from '../components/FilterBar'
import { PageHeader } from '../components/PageHeader'
import { SavedViewsControl } from '../components/SavedViewsControl'
import { StatusBadge } from '../components/StatusBadge'
import { VirtualizedList } from '../components/VirtualizedList'
import { useToast } from '../components/ToastProvider'
import { useLocalStorageState } from '../hooks/useLocalStorageState'
import { useSavedViews } from '../hooks/useSavedViews'
import { deleteNoContent, fetchJSON, postJSON } from '../lib/api'
import { getPresetsForPage } from '../presets/views'

type LibraryItem = {
  work_id: string
  format?: string
}
type LibraryItemsResponse = { items: LibraryItem[] }

type WantedWork = {
  work_id: string
  enabled: boolean
  profile_id?: string
  ignore_upgrades?: boolean
}
type WantedWorksResponse = { items: WantedWork[] }

type WorkIntelligenceResponse = {
  work?: {
    title?: string
    authors?: Array<{ name?: string }>
  }
}

type ProfilesResponse = {
  items: Array<{ profile: { id: string; name: string } }>
  default_profile_id: string
}

type BookRow = {
  workID: string
  title: string
  author: string
  files: number
  monitored: boolean
  profileID: string
  hasFile: boolean
  bestFormat: string
  cutoffUnmet: boolean
}

type SortKey = 'title' | 'author' | 'files' | 'format'

type BooksViewState = {
  query: string
  monitorFilter: 'all' | 'monitored' | 'unmonitored'
  missingOnly: boolean
  cutoffOnly: boolean
  formatFilter: string
  sortKey: SortKey
  sortDir: 'asc' | 'desc'
}

const formatRank: Record<string, number> = {
  epub: 1,
  azw3: 2,
  mobi: 3,
  pdf: 4,
  m4b: 1,
  mp3: 2,
  m4a: 3
}

export function BooksPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()
  const [selected, setSelected] = useState<Record<string, boolean>>({})
  const [bulkProfileID, setBulkProfileID] = useState('')

  const [query, setQuery] = useLocalStorageState<string>('books.filter.query', '')
  const [monitorFilter, setMonitorFilter] = useLocalStorageState<'all' | 'monitored' | 'unmonitored'>('books.filter.monitor', 'all')
  const [missingOnly, setMissingOnly] = useLocalStorageState<boolean>('books.filter.missingOnly', false)
  const [cutoffOnly, setCutoffOnly] = useLocalStorageState<boolean>('books.filter.cutoffOnly', false)
  const [formatFilter, setFormatFilter] = useLocalStorageState<string>('books.filter.format', 'all')
  const [sortKey, setSortKey] = useLocalStorageState<SortKey>('books.sort.key', 'title')
  const [sortDir, setSortDir] = useLocalStorageState<'asc' | 'desc'>('books.sort.dir', 'asc')

  const currentViewState: BooksViewState = {
    query,
    monitorFilter,
    missingOnly,
    cutoffOnly,
    formatFilter,
    sortKey,
    sortDir
  }

  const savedViews = useSavedViews<BooksViewState>({
    pageKey: 'books',
    currentState: currentViewState,
    presetViews: getPresetsForPage<BooksViewState>('books'),
    applyState: (state) => {
      setQuery(state.query)
      setMonitorFilter(state.monitorFilter)
      setMissingOnly(state.missingOnly)
      setCutoffOnly(state.cutoffOnly)
      setFormatFilter(state.formatFilter)
      setSortKey(state.sortKey)
      setSortDir(state.sortDir)
    }
  })

  const libraryItemsQuery = useQuery({
    queryKey: ['library', 'books', 'library-items'],
    queryFn: () => fetchJSON<LibraryItemsResponse>('/api/v1/library/items?limit=1000'),
    refetchInterval: 30000
  })
  const wantedWorksQuery = useQuery({
    queryKey: ['wanted', 'works'],
    queryFn: () => fetchJSON<WantedWorksResponse>('/ui-api/indexer/wanted/works'),
    refetchInterval: 30000
  })
  const profilesQuery = useQuery({
    queryKey: ['settings', 'profiles', 'books'],
    queryFn: () => fetchJSON<ProfilesResponse>('/ui-api/indexer/profiles')
  })

  const workIDs = useMemo(() => {
    const ids = new Set<string>()
    for (const item of libraryItemsQuery.data?.items ?? []) {
      const workID = item.work_id?.trim()
      if (workID) ids.add(workID)
    }
    for (const item of wantedWorksQuery.data?.items ?? []) {
      const workID = item.work_id?.trim()
      if (workID) ids.add(workID)
    }
    return Array.from(ids).slice(0, 250)
  }, [libraryItemsQuery.data?.items, wantedWorksQuery.data?.items])

  const titleQuery = useQuery({
    queryKey: ['library', 'books', 'titles', workIDs.join(',')],
    enabled: workIDs.length > 0,
    queryFn: async () => {
      const pairs = await Promise.all(
        workIDs.map(async (workID) => {
          try {
            const payload = await fetchJSON<WorkIntelligenceResponse>(`/api/v1/works/${encodeURIComponent(workID)}/intelligence`)
            return [
              workID,
              {
                title: payload.work?.title?.trim() || workID,
                author: (payload.work?.authors ?? [])
                  .map((author) => author.name?.trim())
                  .filter(Boolean)
                  .join(', ')
              }
            ] as const
          } catch {
            return [workID, { title: workID, author: '' }] as const
          }
        })
      )
      return new Map<string, { title: string; author: string }>(pairs)
    }
  })

  const upsertWanted = useMutation({
    mutationFn: async (payload: { workID: string; enabled: boolean; profileID?: string }) => {
      if (!payload.enabled) {
        await deleteNoContent(`/ui-api/indexer/wanted/works/${encodeURIComponent(payload.workID)}`)
        return
      }
      await postJSON(`/ui-api/indexer/wanted/works/${encodeURIComponent(payload.workID)}`, {
        enabled: true,
        priority: 100,
        cadence_minutes: 60,
        profile_id: payload.profileID
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['wanted', 'works'] })
    },
    onError: (error) => pushToast((error as Error).message)
  })

  const searchMutation = useMutation({
    mutationFn: async (payload: { workID: string; title: string }) => {
      await postJSON(`/ui-api/indexer/search/work/${encodeURIComponent(payload.workID)}`, { title: payload.title })
    },
    onSuccess: () => pushToast('Search request enqueued'),
    onError: (error) => pushToast((error as Error).message)
  })

  const monitoredByWork = new Map((wantedWorksQuery.data?.items ?? []).map((r) => [r.work_id, r]))
  const defaultProfileID = profilesQuery.data?.default_profile_id ?? ''

  const rows = useMemo<BookRow[]>(() => {
    const formatByWork = new Map<string, string[]>()
    const fileCountByWork = new Map<string, number>()
    for (const item of libraryItemsQuery.data?.items ?? []) {
      const workID = item.work_id?.trim()
      if (!workID) continue
      fileCountByWork.set(workID, (fileCountByWork.get(workID) ?? 0) + 1)
      const list = formatByWork.get(workID) ?? []
      if (item.format?.trim()) {
        list.push(item.format.trim().toLowerCase())
      }
      formatByWork.set(workID, list)
    }

    return workIDs
      .map((workID) => {
        const wanted = monitoredByWork.get(workID)
        const formats = formatByWork.get(workID) ?? []
        const bestFormat = [...formats].sort((a, b) => (formatRank[a] ?? 99) - (formatRank[b] ?? 99))[0] || '-'
        return {
          workID,
          files: fileCountByWork.get(workID) ?? 0,
          title: titleQuery.data?.get(workID)?.title ?? workID,
          author: titleQuery.data?.get(workID)?.author ?? '',
          monitored: Boolean(wanted?.enabled),
          profileID: wanted?.profile_id || defaultProfileID,
          hasFile: (fileCountByWork.get(workID) ?? 0) > 0,
          bestFormat,
          cutoffUnmet: Boolean(wanted?.enabled && wanted?.ignore_upgrades !== true && (fileCountByWork.get(workID) ?? 0) > 0 && bestFormat === 'pdf')
        }
      })
      .sort((a, b) => a.title.localeCompare(b.title))
  }, [libraryItemsQuery.data?.items, monitoredByWork, titleQuery.data, defaultProfileID, workIDs])

  const filteredRows = useMemo(() => {
    const lowered = query.trim().toLowerCase()
    const filtered = rows.filter((row) => {
      if (monitorFilter === 'monitored' && !row.monitored) return false
      if (monitorFilter === 'unmonitored' && row.monitored) return false
      if (missingOnly && row.hasFile) return false
      if (cutoffOnly && !row.cutoffUnmet) return false
      if (formatFilter !== 'all' && row.bestFormat !== formatFilter) return false
      if (!lowered) return true
      return (
        row.title.toLowerCase().includes(lowered) ||
        row.author.toLowerCase().includes(lowered) ||
        row.workID.toLowerCase().includes(lowered)
      )
    })

    const sorted = [...filtered].sort((a, b) => {
      const dir = sortDir === 'asc' ? 1 : -1
      if (sortKey === 'files') return (a.files - b.files) * dir
      if (sortKey === 'format') return a.bestFormat.localeCompare(b.bestFormat) * dir
      if (sortKey === 'author') return a.author.localeCompare(b.author) * dir
      return a.title.localeCompare(b.title) * dir
    })
    return sorted
  }, [cutoffOnly, formatFilter, missingOnly, monitorFilter, query, rows, sortDir, sortKey])

  const selectedIDs = useMemo(() => filteredRows.filter((row) => selected[row.workID]).map((row) => row.workID), [filteredRows, selected])

  const applyBulkMonitor = async (enabled: boolean) => {
    for (const workID of selectedIDs) {
      const row = rows.find((entry) => entry.workID === workID)
      await upsertWanted.mutateAsync({ workID, enabled, profileID: row?.profileID || defaultProfileID })
    }
    pushToast(`Applied monitor=${enabled} to ${selectedIDs.length} item(s)`)
  }

  const applyBulkProfile = async () => {
    if (!bulkProfileID.trim()) {
      pushToast('Select a profile first')
      return
    }
    for (const workID of selectedIDs) {
      await upsertWanted.mutateAsync({ workID, enabled: true, profileID: bulkProfileID.trim() })
    }
    pushToast(`Assigned profile to ${selectedIDs.length} item(s)`)
  }

  const applyBulkSearch = async () => {
    const items = selectedIDs.map((workID) => ({
      entity_type: 'work',
      entity_id: workID,
      title: rows.find((row) => row.workID === workID)?.title || workID
    }))
    await postJSON('/ui-api/indexer/search/bulk', { items })
    pushToast(`${items.length} searches queued`)
  }

  const toggleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir(sortDir === 'asc' ? 'desc' : 'asc')
      return
    }
    setSortKey(key)
    setSortDir('asc')
  }

  return (
    <section className="space-y-4">
      <PageHeader
        title="Books"
        subtitle="Arr-style list with persistent filters, bulk operations, and quick actions."
        actions={
          <Link className="rounded border border-slate-700 px-3 py-2 text-sm text-slate-200 hover:bg-slate-800" to="/library/books/manual-search">
            Manual Search
          </Link>
        }
      />

      <FilterBar>
        <input
          className="w-full sm:w-64 rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100"
          placeholder="Filter title, author, or work id"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
        <select className="rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100" value={monitorFilter} onChange={(event) => setMonitorFilter(event.target.value as 'all' | 'monitored' | 'unmonitored')}>
          <option value="all">All</option>
          <option value="monitored">Monitored</option>
          <option value="unmonitored">Unmonitored</option>
        </select>
        <label className="inline-flex items-center gap-2 rounded border border-slate-700 px-2 py-1.5 text-xs text-slate-300">
          <input type="checkbox" checked={missingOnly} onChange={(event) => setMissingOnly(event.target.checked)} />
          Missing
        </label>
        <label className="inline-flex items-center gap-2 rounded border border-slate-700 px-2 py-1.5 text-xs text-slate-300">
          <input type="checkbox" checked={cutoffOnly} onChange={(event) => setCutoffOnly(event.target.checked)} />
          Cutoff Unmet
        </label>
        <select className="rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100" value={formatFilter} onChange={(event) => setFormatFilter(event.target.value)}>
          <option value="all">All Formats</option>
          <option value="epub">epub</option>
          <option value="pdf">pdf</option>
          <option value="m4b">m4b</option>
          <option value="mp3">mp3</option>
        </select>
        <SavedViewsControl
          selectedViewID={savedViews.selectedViewID}
          defaultViewID={savedViews.defaultViewID}
          defaultViews={savedViews.defaultViews}
          myViews={savedViews.myViews}
          onSelectView={savedViews.selectView}
          onSaveCurrentAsMyView={(name) => {
            const ok = savedViews.saveCurrentAsMyView(name)
            if (!ok) pushToast('View name already exists')
            else pushToast(`Saved view "${name}"`)
            return ok
          }}
          onClonePresetToMyView={(presetID, name) => {
            const ok = savedViews.clonePresetToMy(presetID, name)
            if (!ok) pushToast('Unable to clone preset (name may already exist)')
            else pushToast('Preset cloned to My Views')
            return ok
          }}
          onUpdateCurrentMyView={() => {
            const ok = savedViews.updateCurrentMyView()
            if (ok) pushToast('View updated')
            return ok
          }}
          onDeleteCurrentMyView={() => {
            const ok = savedViews.deleteCurrentMyView()
            if (ok) pushToast('View deleted')
            return ok
          }}
          onSetDefaultView={(viewID) => {
            const ok = savedViews.setDefaultView(viewID)
            if (ok) pushToast('Default view set')
            return ok
          }}
          onHidePreset={(presetID) => {
            const ok = savedViews.hidePreset(presetID)
            if (ok) pushToast('Preset hidden')
            return ok
          }}
        />
      </FilterBar>

      {selectedIDs.length > 0 ? (
        <BulkActionBar count={selectedIDs.length}>
          <button className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300" onClick={() => void applyBulkMonitor(true)}>Monitor</button>
          <button className="rounded border border-slate-700 px-2 py-1 text-xs text-slate-300" onClick={() => void applyBulkMonitor(false)}>Unmonitor</button>
          <button className="rounded border border-emerald-700 px-2 py-1 text-xs text-emerald-300" onClick={() => void applyBulkSearch()}>Search Selected</button>
          <select className="rounded border border-slate-700 bg-slate-900 px-2 py-1 text-xs text-slate-100" value={bulkProfileID} onChange={(e) => setBulkProfileID(e.target.value)}>
            <option value="">Assign profile...</option>
            {(profilesQuery.data?.items ?? []).map((profile) => (
              <option key={profile.profile.id} value={profile.profile.id}>{profile.profile.name}</option>
            ))}
          </select>
          <button className="rounded border border-amber-700 px-2 py-1 text-xs text-amber-300" onClick={() => void applyBulkProfile()}>Apply Profile</button>
        </BulkActionBar>
      ) : null}

      <VirtualizedList
        rows={filteredRows}
        estimateSize={72}
        maxHeight={640}
        rowKey={(row) => row.workID}
        empty="No books match these filters."
        header={
          <div className="grid grid-cols-[36px_minmax(0,1.6fr)_120px_170px] md:grid-cols-[36px_minmax(0,1.8fr)_minmax(0,1.2fr)_minmax(0,1fr)_90px_90px_140px_170px] bg-slate-900 text-left text-sm text-slate-300">
            <div className="px-3 py-2"><input type="checkbox" checked={filteredRows.length > 0 && selectedIDs.length === filteredRows.length} onChange={(e) => setSelected(Object.fromEntries(filteredRows.map((row) => [row.workID, e.target.checked])))} /></div>
            <div className="px-3 py-2"><button className="hover:text-sky-300" onClick={() => toggleSort('title')}>Title</button></div>
            <div className="hidden md:block px-3 py-2"><button className="hover:text-sky-300" onClick={() => toggleSort('author')}>Author</button></div>
            <div className="hidden md:block px-3 py-2 text-slate-400">Work ID</div>
            <div className="hidden md:block px-3 py-2"><button className="hover:text-sky-300" onClick={() => toggleSort('files')}>Files</button></div>
            <div className="hidden md:block px-3 py-2"><button className="hover:text-sky-300" onClick={() => toggleSort('format')}>Format</button></div>
            <div className="px-3 py-2">Status</div>
            <div className="px-3 py-2">Action</div>
          </div>
        }
        renderRow={(row) => (
          <div className="grid h-full grid-cols-[36px_minmax(0,1.6fr)_120px_170px] md:grid-cols-[36px_minmax(0,1.8fr)_minmax(0,1.2fr)_minmax(0,1fr)_90px_90px_140px_170px] items-center text-sm">
            <div className="px-3 py-2"><input type="checkbox" checked={Boolean(selected[row.workID])} onChange={(e) => setSelected((prev) => ({ ...prev, [row.workID]: e.target.checked }))} /></div>
            <div className="px-3 py-2 min-w-0">
              <Link className="truncate text-sky-300 hover:underline block" to={`/library/books/${encodeURIComponent(row.workID)}`}>{row.title}</Link>
              <p className="truncate text-xs text-slate-400 md:hidden">{row.author || row.workID}</p>
            </div>
            <div className="hidden md:block px-3 py-2 truncate text-slate-300">{row.author || '-'}</div>
            <div className="hidden md:block px-3 py-2 truncate text-slate-400">{row.workID}</div>
            <div className="hidden md:block px-3 py-2">{row.files}</div>
            <div className="hidden md:block px-3 py-2">{row.bestFormat}</div>
            <div className="px-3 py-2">
              <div className="flex flex-wrap gap-1">
                {row.monitored ? <StatusBadge label="Monitored" /> : <StatusBadge label="Unmonitored" />}
                {!row.hasFile ? <StatusBadge label="Missing" /> : null}
                {row.cutoffUnmet ? <StatusBadge label="Cutoff Unmet" /> : null}
              </div>
            </div>
            <div className="px-3 py-2">
              <div className="flex flex-wrap gap-2">
                <button className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300" onClick={() => upsertWanted.mutate({ workID: row.workID, enabled: !row.monitored, profileID: row.profileID })}>
                  {row.monitored ? 'Unmonitor' : 'Monitor'}
                </button>
                <button className="rounded border border-emerald-700 px-2 py-1 text-xs text-emerald-300" onClick={() => searchMutation.mutate({ workID: row.workID, title: row.title })}>
                  Search
                </button>
              </div>
            </div>
          </div>
        )}
      />

      {libraryItemsQuery.isLoading || titleQuery.isLoading ? <p className="text-sm text-slate-400">Loading books...</p> : null}
      {libraryItemsQuery.isError || wantedWorksQuery.isError || titleQuery.isError || profilesQuery.isError ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load book data.</div>
      ) : null}
    </section>
  )
}
