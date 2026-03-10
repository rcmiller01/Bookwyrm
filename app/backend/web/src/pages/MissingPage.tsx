import { FormEvent, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { FilterBar } from '../components/FilterBar'
import { PageHeader } from '../components/PageHeader'
import { SavedViewsControl } from '../components/SavedViewsControl'
import { StatusBadge } from '../components/StatusBadge'
import { VirtualizedList } from '../components/VirtualizedList'
import { useToast } from '../components/ToastProvider'
import { useLocalStorageState } from '../hooks/useLocalStorageState'
import { useSavedViews } from '../hooks/useSavedViews'
import { deleteNoContent, fetchJSON, postJSON } from '../lib/api'
import { errorMessage } from '../lib/errorMessage'
import { buildWantedPayload } from '../lib/wantedPayload'
import { getPresetsForPage } from '../presets/views'

type WantedWork = {
  work_id: string
  enabled: boolean
  priority: number
  profile_id?: string
  formats?: string[]
  languages?: string[]
  last_enqueued_at?: string
}

type WantedWorksResponse = { items: WantedWork[] }

type LibraryItem = {
  work_id: string
}

type LibraryItemsResponse = { items: LibraryItem[] }

type WorkIntelligenceResponse = {
  work?: {
    title?: string
    authors?: Array<{ name?: string }>
  }
}

type MissingRow = {
  workID: string
  title: string
  author: string
  priority: number
  profileID: string
  lastEnqueued: string
}

type MissingViewState = {
  query: string
  mediaType: 'all' | 'ebook' | 'audiobook'
}

export function MissingPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()
  const [workIDInput, setWorkIDInput] = useState('')
  const [query, setQuery] = useLocalStorageState<string>('missing.filter.query', '')
  const [mediaType, setMediaType] = useLocalStorageState<'all' | 'ebook' | 'audiobook'>('missing.filter.mediaType', 'all')

  const savedViews = useSavedViews<MissingViewState>({
    pageKey: 'missing',
    currentState: { query, mediaType },
    presetViews: getPresetsForPage<MissingViewState>('missing'),
    applyState: (state) => {
      setQuery(state.query)
      setMediaType(state.mediaType)
    }
  })

  const wantedQuery = useQuery({
    queryKey: ['wanted', 'works'],
    queryFn: () => fetchJSON<WantedWorksResponse>('/ui-api/indexer/wanted/works'),
    refetchInterval: 45000
  })

  const libraryItemsQuery = useQuery({
    queryKey: ['library', 'missing', 'library-items'],
    queryFn: () => fetchJSON<LibraryItemsResponse>('/api/v1/library/items?limit=1000'),
    refetchInterval: 45000
  })

  const libraryWorkIDs = useMemo(() => {
    const ids = new Set<string>()
    for (const item of libraryItemsQuery.data?.items ?? []) {
      if (item.work_id?.trim()) {
        ids.add(item.work_id.trim())
      }
    }
    return ids
  }, [libraryItemsQuery.data])

  const missingWorkIDs = useMemo(() => {
    const ids: string[] = []
    for (const item of wantedQuery.data?.items ?? []) {
      if (!item.enabled) continue
      const workID = item.work_id?.trim()
      if (!workID || libraryWorkIDs.has(workID)) continue
      ids.push(workID)
    }
    return ids.slice(0, 250)
  }, [libraryWorkIDs, wantedQuery.data])

  const titleQuery = useQuery({
    queryKey: ['wanted', 'missing', 'titles', missingWorkIDs.join(',')],
    enabled: missingWorkIDs.length > 0,
    queryFn: async () => {
      const pairs = await Promise.all(
        missingWorkIDs.map(async (workID) => {
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

  const createMutation = useMutation({
    mutationFn: async (payload: { workID: string; formats?: string[]; languages?: string[]; profileID?: string }) => {
      await postJSON(
        `/ui-api/indexer/wanted/works/${encodeURIComponent(payload.workID)}`,
        buildWantedPayload({
          enabled: true,
          profileID: payload.profileID,
          formats: payload.formats,
          languages: payload.languages
        })
      )
    },
    onSuccess: async () => {
      pushToast('Wanted work added')
      setWorkIDInput('')
      await queryClient.invalidateQueries({ queryKey: ['wanted', 'works'] })
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const deleteMutation = useMutation({
    mutationFn: async (workID: string) => {
      await deleteNoContent(`/ui-api/indexer/wanted/works/${encodeURIComponent(workID)}`)
    },
    onSuccess: async () => {
      pushToast('Wanted work removed')
      await queryClient.invalidateQueries({ queryKey: ['wanted', 'works'] })
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const searchMutation = useMutation({
    mutationFn: async (payload: { workID: string; title: string }) => {
      await postJSON(`/ui-api/indexer/search/work/${encodeURIComponent(payload.workID)}`, { title: payload.title })
    },
    onSuccess: () => pushToast('Search request enqueued'),
    onError: (error) => pushToast(errorMessage(error))
  })

  const bulkSearchMutation = useMutation({
    mutationFn: async (payload: Array<{ workID: string; title: string }>) => {
      await postJSON('/ui-api/indexer/search/bulk', {
        items: payload.map((item) => ({
          entity_type: 'work',
          entity_id: item.workID,
          title: item.title
        }))
      })
    },
    onSuccess: (_data, variables) => pushToast(`${variables.length} searches queued`),
    onError: (error) => pushToast(errorMessage(error))
  })

  const rows = useMemo<MissingRow[]>(() => {
    const base = wantedQuery.data?.items ?? []
    return base
      .filter((item) => item.enabled && item.work_id?.trim() && !libraryWorkIDs.has(item.work_id.trim()))
      .map((item) => {
        const workID = item.work_id.trim()
        return {
          workID,
          title: titleQuery.data?.get(workID)?.title ?? workID,
          author: titleQuery.data?.get(workID)?.author ?? '',
          priority: item.priority,
          profileID: item.profile_id || '-',
          lastEnqueued: item.last_enqueued_at ? new Date(item.last_enqueued_at).toLocaleString() : '-'
        }
      })
      .sort((a, b) => a.title.localeCompare(b.title))
  }, [libraryWorkIDs, titleQuery.data, wantedQuery.data?.items])

  const filteredRows = useMemo(() => {
    const lowered = query.trim().toLowerCase()
    return rows.filter((row) => {
      if (mediaType === 'ebook' && row.profileID.toLowerCase().includes('audio')) return false
      if (mediaType === 'audiobook' && !row.profileID.toLowerCase().includes('audio')) return false
      if (!lowered) return true
      return row.title.toLowerCase().includes(lowered) || row.author.toLowerCase().includes(lowered) || row.workID.toLowerCase().includes(lowered)
    })
  }, [mediaType, query, rows])

  const onAdd = (event: FormEvent) => {
    event.preventDefault()
    const workID = workIDInput.trim()
    if (!workID) return
    createMutation.mutate({ workID })
  }

  return (
    <section className="space-y-4">
      <PageHeader
        title="Missing"
        subtitle="Monitored works that are wanted but not currently in the library."
        actions={
          <button
            className="rounded border border-emerald-700 px-3 py-1.5 text-sm text-emerald-300"
            type="button"
            disabled={filteredRows.length === 0 || bulkSearchMutation.isPending}
            onClick={() => bulkSearchMutation.mutate(filteredRows.map((row) => ({ workID: row.workID, title: row.title })))}
          >
            Search All Missing
          </button>
        }
      />

      <form className="flex flex-wrap items-end gap-2 rounded border border-slate-800 bg-slate-900/60 p-3" onSubmit={onAdd}>
        <label className="flex-1 text-sm text-slate-300">
          Add work ID
          <input
            className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100"
            value={workIDInput}
            onChange={(event) => setWorkIDInput(event.target.value)}
            placeholder="work-123"
          />
        </label>
        <button className="rounded border border-sky-700 px-3 py-1.5 text-sm text-sky-300" disabled={createMutation.isPending} type="submit">
          Add
        </button>
      </form>

      <FilterBar>
        <input
          className="w-full sm:w-64 rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100"
          placeholder="Filter title, author, work id"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
        <select className="rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100" value={mediaType} onChange={(event) => setMediaType(event.target.value as 'all' | 'ebook' | 'audiobook')}>
          <option value="all">All</option>
          <option value="ebook">Ebooks</option>
          <option value="audiobook">Audiobooks</option>
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

      <VirtualizedList
        rows={filteredRows}
        estimateSize={70}
        maxHeight={600}
        rowKey={(row) => row.workID}
        empty="No missing monitored works."
        header={
          <div className="grid grid-cols-[minmax(0,1.7fr)_110px_150px] md:grid-cols-[minmax(0,1.8fr)_minmax(0,1fr)_minmax(0,1fr)_80px_100px_130px_170px] bg-slate-900 text-left text-sm text-slate-300">
            <div className="px-3 py-2">Title</div>
            <div className="hidden md:block px-3 py-2">Author</div>
            <div className="hidden md:block px-3 py-2">Work ID</div>
            <div className="hidden md:block px-3 py-2">Priority</div>
            <div className="hidden md:block px-3 py-2">Profile</div>
            <div className="px-3 py-2">Status</div>
            <div className="px-3 py-2">Action</div>
          </div>
        }
        renderRow={(row) => (
          <div className="grid h-full grid-cols-[minmax(0,1.7fr)_110px_150px] md:grid-cols-[minmax(0,1.8fr)_minmax(0,1fr)_minmax(0,1fr)_80px_100px_130px_170px] items-center text-sm">
            <div className="px-3 py-2 min-w-0">
              <Link className="truncate text-sky-300 hover:underline block" to={`/library/books/${encodeURIComponent(row.workID)}`}>
                {row.title}
              </Link>
              <p className="text-xs text-slate-400 md:hidden truncate">{row.author || row.workID}</p>
            </div>
            <div className="hidden md:block px-3 py-2 text-slate-300 truncate">{row.author || '-'}</div>
            <div className="hidden md:block px-3 py-2 text-slate-300 truncate">{row.workID}</div>
            <div className="hidden md:block px-3 py-2">{row.priority}</div>
            <div className="hidden md:block px-3 py-2">{row.profileID}</div>
            <div className="px-3 py-2"><StatusBadge label="Missing" /></div>
            <div className="px-3 py-2">
              <div className="flex flex-wrap gap-2">
                <button
                  className="rounded border border-emerald-700 px-2 py-1 text-xs text-emerald-300"
                  disabled={searchMutation.isPending}
                  onClick={() => searchMutation.mutate({ workID: row.workID, title: row.title })}
                >
                  Search
                </button>
                <button
                  className="rounded border border-red-700 px-2 py-1 text-xs text-red-300"
                  disabled={deleteMutation.isPending}
                  onClick={() => deleteMutation.mutate(row.workID)}
                >
                  Remove
                </button>
              </div>
            </div>
          </div>
        )}
      />

      {wantedQuery.isLoading || libraryItemsQuery.isLoading ? <p className="text-sm text-slate-400">Loading missing list...</p> : null}
      {wantedQuery.isError || libraryItemsQuery.isError || titleQuery.isError ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">
          Failed to load missing list.
        </div>
      ) : null}
    </section>
  )
}
