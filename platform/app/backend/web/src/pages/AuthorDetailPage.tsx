import { useMemo } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { PageHeader } from '../components/PageHeader'
import { StatusBadge } from '../components/StatusBadge'
import { useToast } from '../components/ToastProvider'
import { deleteNoContent, fetchJSON, postJSON } from '../lib/api'
import { errorMessage } from '../lib/errorMessage'

type LibraryItemsResponse = { items: Array<{ work_id: string; format?: string; path?: string }> }
type WantedAuthorsResponse = { items: Array<{ author_id: string; enabled: boolean; profile_id?: string }> }
type WorkIntelligenceResponse = {
  work?: {
    title?: string
    authors?: Array<{ id?: string; name?: string }>
  }
}

type TimelinePayload = {
  timeline?: {
    searches?: Array<{ status?: string; updated_at?: string; created_at?: string }>
    grabs?: Array<{ status?: string; protocol?: string }>
    downloads?: Array<{ job?: { status?: string; updated_at?: string } }>
    imports?: Array<{ job?: { status?: string; updated_at?: string } }>
  }
}

type AuthorWork = {
  workID: string
  title: string
  monitored: boolean
  hasFile: boolean
  format: string
}

export function AuthorDetailPage() {
  const { authorID = '' } = useParams<{ authorID: string }>()
  const [params, setParams] = useSearchParams()
  const tab = params.get('tab') || 'overview'
  const queryClient = useQueryClient()
  const { pushToast } = useToast()

  const libraryQuery = useQuery({
    queryKey: ['author-detail', 'library-items', authorID],
    enabled: authorID.trim().length > 0,
    queryFn: () => fetchJSON<LibraryItemsResponse>('/api/v1/library/items?limit=1000')
  })

  const wantedAuthorsQuery = useQuery({
    queryKey: ['author-detail', 'wanted-authors', authorID],
    enabled: authorID.trim().length > 0,
    queryFn: () => fetchJSON<WantedAuthorsResponse>('/ui-api/indexer/wanted/authors')
  })

  const workIDs = useMemo(() => {
    const ids = new Set<string>()
    for (const item of libraryQuery.data?.items ?? []) {
      const workID = item.work_id?.trim()
      if (workID) ids.add(workID)
    }
    return Array.from(ids).slice(0, 250)
  }, [libraryQuery.data?.items])

  const worksQuery = useQuery({
    queryKey: ['author-detail', 'works', authorID, workIDs.join(',')],
    enabled: authorID.trim().length > 0 && workIDs.length > 0,
    queryFn: async () => {
      const rows = await Promise.all(
        workIDs.map(async (workID) => {
          try {
            const payload = await fetchJSON<WorkIntelligenceResponse>(`/api/v1/works/${encodeURIComponent(workID)}/intelligence`)
            const matchesAuthor = (payload.work?.authors ?? []).some((author) => author.id?.trim() === authorID)
            if (!matchesAuthor) return null
            return {
              workID,
              title: payload.work?.title?.trim() || workID
            }
          } catch {
            return null
          }
        })
      )
      return rows.filter((row): row is { workID: string; title: string } => row !== null)
    }
  })

  const timelineQueries = useQuery({
    queryKey: ['author-detail', 'timeline', authorID, (worksQuery.data ?? []).map((row) => row.workID).join(',')],
    enabled: (worksQuery.data ?? []).length > 0,
    queryFn: async () => {
      const slices = await Promise.all(
        (worksQuery.data ?? []).slice(0, 20).map(async (row) => {
          try {
            const payload = await fetchJSON<TimelinePayload>(`/api/v1/work/${encodeURIComponent(row.workID)}/timeline`)
            return {
              workID: row.workID,
              title: row.title,
              timeline: payload.timeline
            }
          } catch {
            return { workID: row.workID, title: row.title, timeline: undefined }
          }
        })
      )
      return slices
    }
  })

  const monitored = useMemo(() => {
    return (wantedAuthorsQuery.data?.items ?? []).find((item) => item.author_id === authorID)
  }, [authorID, wantedAuthorsQuery.data?.items])

  const works = useMemo<AuthorWork[]>(() => {
    const filesByWork = new Map<string, Array<{ format?: string }>>()
    for (const item of libraryQuery.data?.items ?? []) {
      const workID = item.work_id?.trim()
      if (!workID) continue
      const list = filesByWork.get(workID) ?? []
      list.push({ format: item.format })
      filesByWork.set(workID, list)
    }

    return (worksQuery.data ?? []).map((work) => {
      const files = filesByWork.get(work.workID) ?? []
      return {
        workID: work.workID,
        title: work.title,
        monitored: Boolean(monitored?.enabled),
        hasFile: files.length > 0,
        format: files[0]?.format || '-'
      }
    })
  }, [libraryQuery.data?.items, monitored?.enabled, worksQuery.data])

  const monitorMutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      if (!enabled) {
        await deleteNoContent(`/ui-api/indexer/wanted/authors/${encodeURIComponent(authorID)}`)
        return
      }
      await postJSON(`/ui-api/indexer/wanted/authors/${encodeURIComponent(authorID)}`, {
        enabled: true,
        priority: 100,
        cadence_minutes: 60,
        profile_id: monitored?.profile_id
      })
    },
    onSuccess: async () => {
      pushToast('Author monitoring updated')
      await queryClient.invalidateQueries({ queryKey: ['author-detail', 'wanted-authors', authorID] })
      await queryClient.invalidateQueries({ queryKey: ['wanted', 'authors'] })
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const searchMutation = useMutation({
    mutationFn: async () => {
      const items = works.map((work) => ({ entity_type: 'work', entity_id: work.workID, title: work.title }))
      if (items.length === 0) return
      await postJSON('/ui-api/indexer/search/bulk', { items })
    },
    onSuccess: () => pushToast('Author searches queued'),
    onError: (error) => pushToast(errorMessage(error))
  })

  const historyRows = useMemo(() => {
    const rows: Array<{ key: string; work: string; stage: string; status: string; ts: string }> = []
    for (const slice of timelineQueries.data ?? []) {
      for (const event of slice.timeline?.searches ?? []) {
        rows.push({ key: `${slice.workID}-s-${event.updated_at}-${event.status}`, work: slice.title, stage: 'search', status: event.status || '-', ts: event.updated_at || event.created_at || '' })
      }
      for (const event of slice.timeline?.downloads ?? []) {
        rows.push({ key: `${slice.workID}-d-${event.job?.updated_at}-${event.job?.status}`, work: slice.title, stage: 'download', status: event.job?.status || '-', ts: event.job?.updated_at || '' })
      }
      for (const event of slice.timeline?.imports ?? []) {
        rows.push({ key: `${slice.workID}-i-${event.job?.updated_at}-${event.job?.status}`, work: slice.title, stage: 'import', status: event.job?.status || '-', ts: event.job?.updated_at || '' })
      }
    }
    return rows.sort((a, b) => new Date(b.ts).getTime() - new Date(a.ts).getTime()).slice(0, 60)
  }, [timelineQueries.data])

  const setTab = (next: string) => {
    const nextParams = new URLSearchParams(params)
    nextParams.set('tab', next)
    setParams(nextParams)
  }

  const authorName = useMemo(() => {
    for (const row of worksQuery.data ?? []) {
      const match = row.title
      if (match) return `Author ${authorID}`
    }
    return `Author ${authorID}`
  }, [authorID, worksQuery.data])

  return (
    <section className="space-y-4">
      <PageHeader
        title={authorName}
        subtitle={`Author ID: ${authorID}`}
        actions={
          <div className="flex flex-wrap gap-2">
            <button className="rounded border border-sky-700 px-3 py-1.5 text-sm text-sky-300" onClick={() => monitorMutation.mutate(!monitored?.enabled)}>
              {monitored?.enabled ? 'Unmonitor' : 'Monitor'}
            </button>
            <button className="rounded border border-emerald-700 px-3 py-1.5 text-sm text-emerald-300" onClick={() => searchMutation.mutate()}>
              Search monitored
            </button>
            <Link className="rounded border border-slate-700 px-3 py-1.5 text-sm text-slate-200" to="/library/authors">
              Back
            </Link>
          </div>
        }
      />

      <div className="flex flex-wrap gap-2">
        {['overview', 'books', 'history'].map((name) => (
          <button
            key={name}
            className={[
              'rounded border px-3 py-1 text-xs capitalize',
              tab === name ? 'border-sky-600 bg-sky-900/30 text-sky-300' : 'border-slate-700 text-slate-300'
            ].join(' ')}
            onClick={() => setTab(name)}
          >
            {name}
          </button>
        ))}
      </div>

      {tab === 'overview' ? (
        <div className="grid gap-4 lg:grid-cols-3">
          <article className="rounded border border-slate-800 bg-slate-900/50 p-4">
            <h3 className="text-xs uppercase tracking-wide text-slate-400">Monitoring</h3>
            <div className="mt-2">{monitored?.enabled ? <StatusBadge label="Monitored" /> : <StatusBadge label="Unmonitored" />}</div>
            <p className="mt-2 text-xs text-slate-400">Profile: {monitored?.profile_id || 'default'}</p>
          </article>
          <article className="rounded border border-slate-800 bg-slate-900/50 p-4">
            <h3 className="text-xs uppercase tracking-wide text-slate-400">Works</h3>
            <p className="mt-2 text-2xl font-semibold text-slate-100">{works.length}</p>
          </article>
          <article className="rounded border border-slate-800 bg-slate-900/50 p-4">
            <h3 className="text-xs uppercase tracking-wide text-slate-400">Missing</h3>
            <p className="mt-2 text-2xl font-semibold text-slate-100">{works.filter((work) => !work.hasFile).length}</p>
          </article>
        </div>
      ) : null}

      {tab === 'books' ? (
        <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-900 text-slate-300">
              <tr>
                <th className="px-3 py-2">Title</th>
                <th className="px-3 py-2">Work ID</th>
                <th className="px-3 py-2">Format</th>
                <th className="px-3 py-2">Status</th>
                <th className="px-3 py-2">Actions</th>
              </tr>
            </thead>
            <tbody>
              {works.map((work) => (
                <tr key={work.workID} className="border-t border-slate-800 text-slate-100">
                  <td className="px-3 py-2">
                    <Link className="text-sky-300 hover:underline" to={`/library/books/${encodeURIComponent(work.workID)}`}>{work.title}</Link>
                  </td>
                  <td className="px-3 py-2 text-slate-300">{work.workID}</td>
                  <td className="px-3 py-2">{work.format}</td>
                  <td className="px-3 py-2">{work.hasFile ? <StatusBadge label="Has file" /> : <StatusBadge label="Missing" />}</td>
                  <td className="px-3 py-2">
                    <button className="rounded border border-emerald-700 px-2 py-1 text-xs text-emerald-300" onClick={() => postJSON(`/ui-api/indexer/search/work/${encodeURIComponent(work.workID)}`, { title: work.title }).then(() => pushToast('Search queued')).catch((error: unknown) => pushToast(errorMessage(error)))}>
                      Search
                    </button>
                  </td>
                </tr>
              ))}
              {works.length === 0 ? <tr><td colSpan={5} className="px-3 py-6 text-center text-slate-400">No books for this author in library.</td></tr> : null}
            </tbody>
          </table>
        </div>
      ) : null}

      {tab === 'history' ? (
        <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-900 text-slate-300">
              <tr>
                <th className="px-3 py-2">Work</th>
                <th className="px-3 py-2">Stage</th>
                <th className="px-3 py-2">Status</th>
                <th className="px-3 py-2">Timestamp</th>
              </tr>
            </thead>
            <tbody>
              {historyRows.map((row) => (
                <tr key={row.key} className="border-t border-slate-800 text-slate-100">
                  <td className="px-3 py-2">{row.work}</td>
                  <td className="px-3 py-2">{row.stage}</td>
                  <td className="px-3 py-2"><StatusBadge label={row.status} /></td>
                  <td className="px-3 py-2">{row.ts ? new Date(row.ts).toLocaleString() : '-'}</td>
                </tr>
              ))}
              {historyRows.length === 0 ? <tr><td colSpan={4} className="px-3 py-6 text-center text-slate-400">No history events for this author.</td></tr> : null}
            </tbody>
          </table>
        </div>
      ) : null}

      {libraryQuery.isLoading || worksQuery.isLoading ? <p className="text-sm text-slate-400">Loading author detail...</p> : null}
      {libraryQuery.isError || worksQuery.isError || wantedAuthorsQuery.isError || timelineQueries.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load author detail.</div> : null}
    </section>
  )
}
