import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery } from '@tanstack/react-query'
import { FilterBar } from '../components/FilterBar'
import { PageHeader } from '../components/PageHeader'
import { SavedViewsControl } from '../components/SavedViewsControl'
import { StatusBadge } from '../components/StatusBadge'
import { VirtualizedList } from '../components/VirtualizedList'
import { useToast } from '../components/ToastProvider'
import { useLocalStorageState } from '../hooks/useLocalStorageState'
import { useSavedViews } from '../hooks/useSavedViews'
import { fetchJSON, postJSON } from '../lib/api'
import { getPresetsForPage } from '../presets/views'

type WantedWork = {
  work_id: string
  enabled: boolean
  profile_id?: string
  ignore_upgrades?: boolean
  last_enqueued_at?: string
}

type WantedWorksResponse = { items: WantedWork[] }

type LibraryItem = {
  work_id: string
  format: string
}

type LibraryItemsResponse = { items: LibraryItem[] }

type ProfileWithQualities = {
  profile: { id: string; cutoff_quality: string; name: string }
  qualities: { quality: string; rank: number }[]
}

type ProfilesResponse = { items: ProfileWithQualities[]; default_profile_id: string }

type WorkIntelligenceResponse = { work?: { title?: string; authors?: Array<{ name?: string }> } }

type Row = {
  workID: string
  title: string
  author: string
  profile: string
  cutoff: string
  best: string
  lastSearch: string
}

type CutoffViewState = {
  query: string
  profileFilter: string
  mediaType: 'all' | 'ebook' | 'audiobook'
}

function toRank(profile: ProfileWithQualities | undefined): Map<string, number> {
  const map = new Map<string, number>()
  for (const quality of profile?.qualities ?? []) {
    map.set(quality.quality.toLowerCase(), quality.rank)
  }
  return map
}

export function CutoffUnmetPage() {
  const { pushToast } = useToast()
  const [query, setQuery] = useLocalStorageState<string>('cutoff.filter.query', '')
  const [profileFilter, setProfileFilter] = useLocalStorageState<string>('cutoff.filter.profile', 'all')
  const [mediaType, setMediaType] = useLocalStorageState<'all' | 'ebook' | 'audiobook'>('cutoff.filter.mediaType', 'all')

  const savedViews = useSavedViews<CutoffViewState>({
    pageKey: 'cutoff-unmet',
    currentState: { query, profileFilter, mediaType },
    presetViews: getPresetsForPage<CutoffViewState>('cutoff-unmet'),
    applyState: (state) => {
      setQuery(state.query)
      setProfileFilter(state.profileFilter)
      setMediaType(state.mediaType)
    }
  })

  const wantedQuery = useQuery({
    queryKey: ['wanted', 'works', 'cutoff-unmet'],
    queryFn: () => fetchJSON<WantedWorksResponse>('/ui-api/indexer/wanted/works'),
    refetchInterval: 60000
  })
  const profilesQuery = useQuery({
    queryKey: ['settings', 'profiles', 'cutoff-unmet'],
    queryFn: () => fetchJSON<ProfilesResponse>('/ui-api/indexer/profiles')
  })
  const libraryQuery = useQuery({
    queryKey: ['library', 'items', 'cutoff-unmet'],
    queryFn: () => fetchJSON<LibraryItemsResponse>('/api/v1/library/items?limit=1000'),
    refetchInterval: 60000
  })

  const unmetWorkIDs = useMemo(() => {
    const profiles = profilesQuery.data?.items ?? []
    const defaultProfileID = profilesQuery.data?.default_profile_id ?? ''
    const byID = new Map(profiles.map((profile) => [profile.profile.id, profile]))
    const byWork = new Map<string, string[]>()
    for (const item of libraryQuery.data?.items ?? []) {
      const workID = item.work_id?.trim()
      const format = item.format?.trim().toLowerCase()
      if (!workID || !format) continue
      const list = byWork.get(workID) ?? []
      list.push(format)
      byWork.set(workID, list)
    }

    const ids: string[] = []
    for (const wanted of wantedQuery.data?.items ?? []) {
      if (!wanted.enabled || wanted.ignore_upgrades) continue
      const workID = wanted.work_id?.trim()
      if (!workID) continue
      const profileID = wanted.profile_id || defaultProfileID
      const profile = byID.get(profileID)
      if (!profile) continue
      const cutoffRank = toRank(profile).get(profile.profile.cutoff_quality.toLowerCase())
      if (!cutoffRank) continue
      const available = (byWork.get(workID) ?? []).map((format) => toRank(profile).get(format)).filter((rank): rank is number => rank !== undefined)
      if (available.length === 0) continue
      const bestRank = Math.min(...available)
      if (bestRank > cutoffRank) {
        ids.push(workID)
      }
    }
    return ids
  }, [libraryQuery.data?.items, profilesQuery.data?.default_profile_id, profilesQuery.data?.items, wantedQuery.data?.items])

  const titlesQuery = useQuery({
    queryKey: ['wanted', 'cutoff-unmet', 'titles', unmetWorkIDs.join(',')],
    enabled: unmetWorkIDs.length > 0,
    queryFn: async () => {
      const pairs = await Promise.all(
        unmetWorkIDs.map(async (workID) => {
          try {
            const payload = await fetchJSON<WorkIntelligenceResponse>(`/api/v1/works/${encodeURIComponent(workID)}/intelligence`)
            return [
              workID,
              {
                title: payload.work?.title?.trim() || workID,
                author: (payload.work?.authors ?? []).map((author) => author.name?.trim()).filter(Boolean).join(', ')
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

  const upgradeMutation = useMutation({
    mutationFn: async (payload: { workID: string; title: string }) => {
      await postJSON(`/ui-api/indexer/search/work/${encodeURIComponent(payload.workID)}`, { title: payload.title })
    },
    onSuccess: () => pushToast('Upgrade search enqueued'),
    onError: (error) => pushToast((error as Error).message)
  })

  const rows = useMemo<Row[]>(() => {
    const profiles = profilesQuery.data?.items ?? []
    const defaultProfileID = profilesQuery.data?.default_profile_id ?? ''
    const byID = new Map(profiles.map((profile) => [profile.profile.id, profile]))
    const formatsByWork = new Map<string, string[]>()
    for (const item of libraryQuery.data?.items ?? []) {
      const workID = item.work_id?.trim()
      const format = item.format?.trim().toLowerCase()
      if (!workID || !format) continue
      const list = formatsByWork.get(workID) ?? []
      list.push(format)
      formatsByWork.set(workID, list)
    }

    return (wantedQuery.data?.items ?? [])
      .filter((wanted) => wanted.enabled && !wanted.ignore_upgrades)
      .map((wanted) => {
        const workID = wanted.work_id.trim()
        const profile = byID.get(wanted.profile_id || defaultProfileID)
        if (!profile) return null
        const rankMap = toRank(profile)
        const cutoff = profile.profile.cutoff_quality.toLowerCase()
        const cutoffRank = rankMap.get(cutoff)
        if (!cutoffRank) return null
        const bestFormat = (formatsByWork.get(workID) ?? [])
          .map((format) => ({ format, rank: rankMap.get(format) }))
          .filter((item): item is { format: string; rank: number } => item.rank !== undefined)
          .sort((a, b) => a.rank - b.rank)[0]
        if (!bestFormat || bestFormat.rank <= cutoffRank) return null
        return {
          workID,
          title: titlesQuery.data?.get(workID)?.title ?? workID,
          author: titlesQuery.data?.get(workID)?.author ?? '',
          profile: profile.profile.name,
          cutoff,
          best: bestFormat.format,
          lastSearch: wanted.last_enqueued_at ? new Date(wanted.last_enqueued_at).toLocaleString() : '-'
        }
      })
      .filter((row): row is Row => row !== null)
      .sort((a, b) => a.title.localeCompare(b.title))
  }, [libraryQuery.data?.items, profilesQuery.data?.default_profile_id, profilesQuery.data?.items, titlesQuery.data, wantedQuery.data?.items])

  const filteredRows = useMemo(() => {
    const lowered = query.trim().toLowerCase()
    return rows.filter((row) => {
      if (profileFilter !== 'all' && row.profile !== profileFilter) return false
      if (mediaType === 'ebook' && !['epub', 'azw3', 'mobi', 'pdf'].includes(row.best)) return false
      if (mediaType === 'audiobook' && !['m4b', 'mp3', 'm4a'].includes(row.best)) return false
      if (!lowered) return true
      return row.title.toLowerCase().includes(lowered) || row.author.toLowerCase().includes(lowered) || row.workID.toLowerCase().includes(lowered)
    })
  }, [mediaType, profileFilter, query, rows])

  return (
    <section className="space-y-4">
      <PageHeader
        title="Cutoff Unmet"
        subtitle="Works in library that do not meet profile cutoff quality and should be upgraded."
      />

      <FilterBar>
        <input
          className="w-full sm:w-64 rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100"
          placeholder="Filter title, author, work id"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
        <select className="rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100" value={profileFilter} onChange={(event) => setProfileFilter(event.target.value)}>
          <option value="all">All Profiles</option>
          {(profilesQuery.data?.items ?? []).map((profile) => (
            <option key={profile.profile.id} value={profile.profile.name}>{profile.profile.name}</option>
          ))}
        </select>
        <select className="rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100" value={mediaType} onChange={(event) => setMediaType(event.target.value as 'all' | 'ebook' | 'audiobook')}>
          <option value="all">All</option>
          <option value="ebook">Ebook Upgrades</option>
          <option value="audiobook">Audiobook Upgrades</option>
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
        empty="No cutoff unmet items."
        header={
          <div className="grid grid-cols-[minmax(0,1.6fr)_130px_150px] md:grid-cols-[minmax(0,1.8fr)_minmax(0,1fr)_90px_90px_120px_130px_170px] bg-slate-900 text-left text-sm text-slate-300">
            <div className="px-3 py-2">Title</div>
            <div className="hidden md:block px-3 py-2">Author</div>
            <div className="hidden md:block px-3 py-2">Current</div>
            <div className="hidden md:block px-3 py-2">Cutoff</div>
            <div className="hidden md:block px-3 py-2">Profile</div>
            <div className="px-3 py-2">Status</div>
            <div className="px-3 py-2">Action</div>
          </div>
        }
        renderRow={(row) => (
          <div className="grid h-full grid-cols-[minmax(0,1.6fr)_130px_150px] md:grid-cols-[minmax(0,1.8fr)_minmax(0,1fr)_90px_90px_120px_130px_170px] items-center text-sm">
            <div className="px-3 py-2 min-w-0">
              <Link className="truncate text-sky-300 hover:underline block" to={`/library/books/${encodeURIComponent(row.workID)}`}>
                {row.title}
              </Link>
              <p className="truncate text-xs text-slate-400 md:hidden">{row.author || row.best}</p>
            </div>
            <div className="hidden md:block px-3 py-2 truncate text-slate-300">{row.author || '-'}</div>
            <div className="hidden md:block px-3 py-2">{row.best}</div>
            <div className="hidden md:block px-3 py-2">{row.cutoff}</div>
            <div className="hidden md:block px-3 py-2">{row.profile}</div>
            <div className="px-3 py-2"><StatusBadge label="Cutoff Unmet" /></div>
            <div className="px-3 py-2">
              <button className="rounded border border-emerald-700 px-2 py-1 text-xs text-emerald-300" onClick={() => upgradeMutation.mutate({ workID: row.workID, title: row.title })}>
                Search Upgrade
              </button>
            </div>
          </div>
        )}
      />
      {wantedQuery.isLoading || profilesQuery.isLoading || libraryQuery.isLoading || titlesQuery.isLoading ? <p className="text-sm text-slate-400">Loading cutoff unmet data...</p> : null}
      {wantedQuery.isError || profilesQuery.isError || libraryQuery.isError || titlesQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load cutoff unmet data.</div> : null}
    </section>
  )
}
