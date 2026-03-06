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

type LibraryItemsResponse = { items: Array<{ work_id: string }> }
type WantedAuthor = { author_id: string; enabled: boolean; profile_id?: string }
type WantedAuthorsResponse = { items: WantedAuthor[] }
type WorkIntelligenceResponse = { work?: { title?: string; authors?: Array<{ id?: string; name?: string }> } }
type ProfilesResponse = { items: Array<{ profile: { id: string; name: string } }>; default_profile_id: string }

type AuthorRow = {
  key: string
  authorID: string
  name: string
  monitored: boolean
  profileID: string
  worksCount: number
  workRefs: Array<{ workID: string; title: string }>
}

type AuthorsViewState = {
  query: string
  monitorFilter: 'all' | 'monitored' | 'unmonitored'
  sortKey: 'name' | 'works'
  sortDir: 'asc' | 'desc'
}

export function AuthorsPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()
  const [selected, setSelected] = useState<Record<string, boolean>>({})
  const [bulkProfileID, setBulkProfileID] = useState('')

  const [query, setQuery] = useLocalStorageState<string>('authors.filter.query', '')
  const [monitorFilter, setMonitorFilter] = useLocalStorageState<'all' | 'monitored' | 'unmonitored'>('authors.filter.monitor', 'all')
  const [sortKey, setSortKey] = useLocalStorageState<'name' | 'works'>('authors.sort.key', 'name')
  const [sortDir, setSortDir] = useLocalStorageState<'asc' | 'desc'>('authors.sort.dir', 'asc')

  const currentViewState: AuthorsViewState = {
    query,
    monitorFilter,
    sortKey,
    sortDir
  }

  const savedViews = useSavedViews<AuthorsViewState>({
    pageKey: 'authors',
    currentState: currentViewState,
    presetViews: getPresetsForPage<AuthorsViewState>('authors'),
    applyState: (state) => {
      setQuery(state.query)
      setMonitorFilter(state.monitorFilter)
      setSortKey(state.sortKey)
      setSortDir(state.sortDir)
    }
  })

  const libraryItemsQuery = useQuery({
    queryKey: ['library', 'authors', 'library-items'],
    queryFn: () => fetchJSON<LibraryItemsResponse>('/api/v1/library/items?limit=1000'),
    refetchInterval: 30000
  })
  const wantedAuthorsQuery = useQuery({
    queryKey: ['wanted', 'authors'],
    queryFn: () => fetchJSON<WantedAuthorsResponse>('/ui-api/indexer/wanted/authors'),
    refetchInterval: 30000
  })
  const profilesQuery = useQuery({
    queryKey: ['settings', 'profiles', 'authors'],
    queryFn: () => fetchJSON<ProfilesResponse>('/ui-api/indexer/profiles')
  })

  const workIDs = useMemo(() => {
    const ids = new Set<string>()
    for (const item of libraryItemsQuery.data?.items ?? []) {
      const workID = item.work_id?.trim()
      if (workID) ids.add(workID)
    }
    return Array.from(ids).slice(0, 250)
  }, [libraryItemsQuery.data?.items])

  const workDetailsQuery = useQuery({
    queryKey: ['library', 'authors', 'work-details', workIDs.join(',')],
    enabled: workIDs.length > 0,
    queryFn: async () => {
      const details = await Promise.all(
        workIDs.map(async (workID) => {
          try {
            const payload = await fetchJSON<WorkIntelligenceResponse>(`/api/v1/works/${encodeURIComponent(workID)}/intelligence`)
            return {
              workID,
              title: payload.work?.title?.trim() || workID,
              authors: payload.work?.authors ?? []
            }
          } catch {
            return { workID, title: workID, authors: [] }
          }
        })
      )
      return details
    }
  })

  const toggleMutation = useMutation({
    mutationFn: async (payload: { authorID: string; enable: boolean; profileID?: string }) => {
      if (!payload.enable) {
        await deleteNoContent(`/ui-api/indexer/wanted/authors/${encodeURIComponent(payload.authorID)}`)
        return
      }
      await postJSON(`/ui-api/indexer/wanted/authors/${encodeURIComponent(payload.authorID)}`, {
        enabled: true,
        priority: 100,
        cadence_minutes: 60,
        profile_id: payload.profileID
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['wanted', 'authors'] })
    },
    onError: (error) => pushToast((error as Error).message)
  })

  const wantedByID = new Map((wantedAuthorsQuery.data?.items ?? []).map((item) => [item.author_id, item]))
  const defaultProfileID = profilesQuery.data?.default_profile_id ?? ''

  const rows = useMemo<AuthorRow[]>(() => {
    const aggregate = new Map<string, AuthorRow>()
    for (const work of workDetailsQuery.data ?? []) {
      for (const author of work.authors) {
        const authorID = author.id?.trim() ?? ''
        const name = author.name?.trim() || authorID || 'Unknown'
        const key = authorID || `name:${name.toLowerCase()}`
        const existing = aggregate.get(key)
        if (existing) {
          existing.worksCount += 1
          existing.workRefs.push({ workID: work.workID, title: work.title })
          continue
        }
        const wanted = authorID ? wantedByID.get(authorID) : undefined
        aggregate.set(key, {
          key,
          authorID,
          name,
          monitored: Boolean(wanted?.enabled),
          profileID: wanted?.profile_id || defaultProfileID,
          worksCount: 1,
          workRefs: [{ workID: work.workID, title: work.title }]
        })
      }
    }
    return Array.from(aggregate.values())
  }, [defaultProfileID, wantedByID, workDetailsQuery.data])

  const filteredRows = useMemo(() => {
    const lowered = query.trim().toLowerCase()
    const filtered = rows.filter((row) => {
      if (monitorFilter === 'monitored' && !row.monitored) return false
      if (monitorFilter === 'unmonitored' && row.monitored) return false
      if (!lowered) return true
      return row.name.toLowerCase().includes(lowered) || row.authorID.toLowerCase().includes(lowered)
    })

    return [...filtered].sort((a, b) => {
      const dir = sortDir === 'asc' ? 1 : -1
      if (sortKey === 'works') return (a.worksCount - b.worksCount) * dir
      return a.name.localeCompare(b.name) * dir
    })
  }, [monitorFilter, query, rows, sortDir, sortKey])

  const selectedIDs = useMemo(() => filteredRows.filter((row) => row.authorID && selected[row.authorID]).map((row) => row.authorID), [filteredRows, selected])

  const applyBulkMonitor = async (enable: boolean) => {
    for (const authorID of selectedIDs) {
      const row = rows.find((entry) => entry.authorID === authorID)
      await toggleMutation.mutateAsync({ authorID, enable, profileID: row?.profileID || defaultProfileID })
    }
    pushToast(`Updated ${selectedIDs.length} author(s)`)
  }

  const applyBulkProfile = async () => {
    if (!bulkProfileID.trim()) {
      pushToast('Select a profile first')
      return
    }
    for (const authorID of selectedIDs) {
      await toggleMutation.mutateAsync({ authorID, enable: true, profileID: bulkProfileID.trim() })
    }
    pushToast(`Assigned profile to ${selectedIDs.length} author(s)`)
  }

  const applyBulkSearch = async () => {
    const workMap = new Map<string, { workID: string; title: string }>()
    for (const row of filteredRows) {
      if (!row.authorID || !selected[row.authorID]) continue
      for (const work of row.workRefs) {
        workMap.set(work.workID, work)
      }
    }
    const items = Array.from(workMap.values()).map((item) => ({
      entity_type: 'work',
      entity_id: item.workID,
      title: item.title
    }))
    if (items.length === 0) {
      pushToast('No works found for selected authors')
      return
    }
    await postJSON('/ui-api/indexer/search/bulk', { items })
    pushToast(`${items.length} searches queued`)
  }

  const toggleSort = (key: 'name' | 'works') => {
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
        title="Authors"
        subtitle="Known authors with monitoring, profile assignment, and bulk search."
      />

      <FilterBar>
        <input
          className="w-full sm:w-64 rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100"
          placeholder="Filter by author or id"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
        <select className="rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100" value={monitorFilter} onChange={(event) => setMonitorFilter(event.target.value as 'all' | 'monitored' | 'unmonitored')}>
          <option value="all">All</option>
          <option value="monitored">Monitored</option>
          <option value="unmonitored">Unmonitored</option>
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
        estimateSize={68}
        maxHeight={640}
        rowKey={(row) => row.key}
        empty="No author data found."
        header={
          <div className="grid grid-cols-[36px_minmax(0,1.8fr)_90px_120px] md:grid-cols-[36px_minmax(0,1.8fr)_minmax(0,1fr)_100px_140px_130px_160px] bg-slate-900 text-left text-sm text-slate-300">
            <div className="px-3 py-2">
              <input
                type="checkbox"
                checked={filteredRows.length > 0 && selectedIDs.length === filteredRows.filter((row) => row.authorID).length}
                onChange={(event) => setSelected(Object.fromEntries(filteredRows.filter((row) => row.authorID).map((row) => [row.authorID, event.target.checked])))}
              />
            </div>
            <div className="px-3 py-2"><button className="hover:text-sky-300" onClick={() => toggleSort('name')}>Author</button></div>
            <div className="hidden md:block px-3 py-2 text-slate-400">Author ID</div>
            <div className="px-3 py-2"><button className="hover:text-sky-300" onClick={() => toggleSort('works')}>Works</button></div>
            <div className="hidden md:block px-3 py-2">Status</div>
            <div className="hidden md:block px-3 py-2">Action</div>
            <div className="hidden md:block px-3 py-2">Profile</div>
          </div>
        }
        renderRow={(row) => (
          <div className="grid h-full grid-cols-[36px_minmax(0,1.8fr)_90px_120px] md:grid-cols-[36px_minmax(0,1.8fr)_minmax(0,1fr)_100px_140px_130px_160px] items-center text-sm">
            <div className="px-3 py-2">
              <input type="checkbox" disabled={!row.authorID} checked={Boolean(row.authorID && selected[row.authorID])} onChange={(event) => setSelected((prev) => ({ ...prev, [row.authorID]: event.target.checked }))} />
            </div>
            <div className="px-3 py-2 min-w-0">
              {row.authorID ? (
                <Link className="truncate text-sky-300 hover:underline block" to={`/library/authors/${encodeURIComponent(row.authorID)}`}>
                  {row.name}
                </Link>
              ) : (
                <span className="truncate block">{row.name}</span>
              )}
              <p className="text-xs text-slate-400 md:hidden">{row.monitored ? 'Monitored' : 'Unmonitored'}</p>
            </div>
            <div className="hidden md:block px-3 py-2 truncate text-slate-400">{row.authorID || '-'}</div>
            <div className="px-3 py-2">{row.worksCount}</div>
            <div className="hidden md:block px-3 py-2">{row.monitored ? <StatusBadge label="Monitored" /> : <StatusBadge label="Unmonitored" />}</div>
            <div className="hidden md:block px-3 py-2">
              <button className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300 disabled:opacity-50" disabled={!row.authorID} onClick={() => toggleMutation.mutate({ authorID: row.authorID, enable: !row.monitored, profileID: row.profileID })}>
                {row.monitored ? 'Unmonitor' : 'Monitor'}
              </button>
            </div>
            <div className="hidden md:block px-3 py-2 truncate">{row.profileID || '-'}</div>
          </div>
        )}
      />

      {libraryItemsQuery.isLoading || workDetailsQuery.isLoading ? <p className="text-sm text-slate-400">Loading authors...</p> : null}
      {libraryItemsQuery.isError || workDetailsQuery.isError || wantedAuthorsQuery.isError || profilesQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load author data.</div> : null}
    </section>
  )
}
