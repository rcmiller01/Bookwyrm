import { useMemo } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { PageHeader } from '../components/PageHeader'
import { StatusBadge } from '../components/StatusBadge'
import { useToast } from '../components/ToastProvider'
import { deleteNoContent, fetchJSON, postJSON } from '../lib/api'
import { buildWantedWorkPayload, type ProfilesResponse } from '../lib/wantedWork'
import { errorMessage } from '../lib/errorMessage'
import { buildManualSearchPath } from '../lib/manualSearch'

type WorkPayload = {
  work?: {
    id?: string
    title?: string
    description?: string
    authors?: Array<{ id?: string; name?: string }>
    subjects?: string[]
    editions?: Array<{ id?: string; title?: string }>
  }
  recommendations?: Array<Record<string, unknown>>
}

type LibraryItem = {
  id?: number
  work_id: string
  path?: string
  format?: string
  size_bytes?: number
}

type LibraryItemsResponse = { items: LibraryItem[] }

type WantedWorksResponse = { items: Array<{ work_id: string; enabled: boolean; profile_id?: string }> }

type TimelinePayload = {
  timeline?: {
    searches?: Array<{ status?: string; updated_at?: string; created_at?: string; candidate_id?: number; grab_id?: number }>
    grabs?: Array<{ status?: string; protocol?: string; grab_id?: number }>
    downloads?: Array<{ job?: { id?: number; status?: string; updated_at?: string; output_path?: string } }>
    imports?: Array<{ job?: { id?: number; status?: string; updated_at?: string; source_path?: string } }>
  }
}

function normalizeRecommendation(rec: Record<string, unknown>, idx: number): { key: string; title: string; reason: string } {
  const id = String(rec.id ?? rec.work_id ?? `rec-${idx}`)
  const title = String(rec.title ?? rec.name ?? rec.work_title ?? `Recommendation ${idx + 1}`)
  const reason = String(rec.reason ?? rec.why ?? rec.explanation ?? 'Related work')
  return { key: id, title, reason }
}

export function BookDetailPage() {
  const { workID = '' } = useParams<{ workID: string }>()
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const tab = params.get('tab') || 'overview'
  const queryClient = useQueryClient()
  const { pushToast } = useToast()

  const workQuery = useQuery({
    queryKey: ['book-detail', 'work', workID],
    enabled: workID.trim().length > 0,
    queryFn: () => fetchJSON<WorkPayload>(`/api/v1/works/${encodeURIComponent(workID)}/intelligence`)
  })

  const libraryQuery = useQuery({
    queryKey: ['book-detail', 'library', workID],
    enabled: workID.trim().length > 0,
    queryFn: () => fetchJSON<LibraryItemsResponse>('/api/v1/library/items?limit=1000')
  })

  const wantedQuery = useQuery({
    queryKey: ['book-detail', 'wanted', workID],
    enabled: workID.trim().length > 0,
    queryFn: () => fetchJSON<WantedWorksResponse>('/ui-api/indexer/wanted/works')
  })

  const timelineQuery = useQuery({
    queryKey: ['book-detail', 'timeline', workID],
    enabled: workID.trim().length > 0,
    queryFn: () => fetchJSON<TimelinePayload>(`/api/v1/work/${encodeURIComponent(workID)}/timeline`)
  })

  const profilesQuery = useQuery({
    queryKey: ['book-detail', 'profiles'],
    queryFn: () => fetchJSON<ProfilesResponse>('/ui-api/indexer/profiles')
  })
  const wanted = useMemo(() => (wantedQuery.data?.items ?? []).find((item) => item.work_id === workID), [wantedQuery.data?.items, workID])
  const files = useMemo(() => (libraryQuery.data?.items ?? []).filter((item) => item.work_id === workID), [libraryQuery.data?.items, workID])

  const monitorMutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      if (!enabled) {
        await deleteNoContent(`/ui-api/indexer/wanted/works/${encodeURIComponent(workID)}`)
        return
      }
      await postJSON(`/ui-api/indexer/wanted/works/${encodeURIComponent(workID)}`, buildWantedWorkPayload(profilesQuery.data, wanted?.profile_id))
    },
    onSuccess: async () => {
      pushToast('Monitoring updated')
      await queryClient.invalidateQueries({ queryKey: ['book-detail', 'wanted', workID] })
      await queryClient.invalidateQueries({ queryKey: ['wanted', 'works'] })
    },
    onError: (error) => pushToast(errorMessage(error))
  })


  const timelineRows = useMemo(() => {
    const timeline = timelineQuery.data?.timeline
    if (!timeline) return [] as Array<{ key: string; stage: string; status: string; detail: string; ts: string }>
    const rows: Array<{ key: string; stage: string; status: string; detail: string; ts: string }> = []
    for (const item of timeline.searches ?? []) {
      rows.push({ key: `s-${item.candidate_id}-${item.grab_id}`, stage: 'search', status: item.status || '-', detail: `candidate=${item.candidate_id ?? '-'} grab=${item.grab_id ?? '-'}`, ts: item.updated_at || item.created_at || '' })
    }
    for (const item of timeline.grabs ?? []) {
      rows.push({ key: `g-${item.grab_id}`, stage: 'grab', status: item.status || '-', detail: item.protocol || '-', ts: '' })
    }
    for (const item of timeline.downloads ?? []) {
      rows.push({ key: `d-${item.job?.id}`, stage: 'download', status: item.job?.status || '-', detail: item.job?.output_path || '-', ts: item.job?.updated_at || '' })
    }
    for (const item of timeline.imports ?? []) {
      rows.push({ key: `i-${item.job?.id}`, stage: 'import', status: item.job?.status || '-', detail: item.job?.source_path || '-', ts: item.job?.updated_at || '' })
    }
    return rows.sort((a, b) => new Date(b.ts).getTime() - new Date(a.ts).getTime())
  }, [timelineQuery.data?.timeline])

  const recommendationRows = useMemo(() => {
    return (workQuery.data?.recommendations ?? []).slice(0, 12).map(normalizeRecommendation)
  }, [workQuery.data?.recommendations])

  const manualSearchPath = useMemo(() => {
    const title = workQuery.data?.work?.title?.trim() || workID
    const author = (workQuery.data?.work?.authors ?? []).map((entry) => entry.name?.trim()).filter(Boolean).join(', ')
    return buildManualSearchPath({
      workID,
      title,
      author,
      formats: buildWantedWorkPayload(profilesQuery.data, wanted?.profile_id).formats
    })
  }, [profilesQuery.data, wanted?.profile_id, workID, workQuery.data?.work?.authors, workQuery.data?.work?.title])

  const setTab = (next: string) => {
    const nextParams = new URLSearchParams(params)
    nextParams.set('tab', next)
    setParams(nextParams)
  }

  return (
    <section className="space-y-4">
      <PageHeader
        title={workQuery.data?.work?.title?.trim() || workID}
        subtitle={(workQuery.data?.work?.authors ?? []).map((author) => author.name?.trim()).filter(Boolean).join(', ') || 'Book detail'}
        actions={
          <div className="flex flex-wrap gap-2">
            <button className="rounded border border-sky-700 px-3 py-1.5 text-sm text-sky-300" onClick={() => monitorMutation.mutate(!wanted?.enabled)}>
              {wanted?.enabled ? 'Unmonitor' : 'Monitor'}
            </button>
            <button className="rounded border border-emerald-700 px-3 py-1.5 text-sm text-emerald-300" onClick={() => navigate(`${manualSearchPath}&autorun=1`)}>
              Search now
            </button>
            <Link className="rounded border border-slate-700 px-3 py-1.5 text-sm text-slate-200" to={manualSearchPath}>
              Manual search
            </Link>
            <Link className="rounded border border-slate-700 px-3 py-1.5 text-sm text-slate-200" to="/library/books">
              Back
            </Link>
          </div>
        }
      />

      <div className="flex flex-wrap gap-2">
        {['overview', 'files', 'search', 'history', 'recommendations'].map((name) => (
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
        <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
          <article className="rounded border border-slate-800 bg-slate-900/50 p-4 text-sm text-slate-200">
            <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-400">Metadata</h3>
            <p className="mt-2 whitespace-pre-wrap">{workQuery.data?.work?.description?.trim() || 'No description available.'}</p>
            <p className="mt-3 text-xs text-slate-400">Subjects: {(workQuery.data?.work?.subjects ?? []).join(', ') || '-'}</p>
          </article>
          <aside className="rounded border border-slate-800 bg-slate-900/50 p-4 text-sm">
            <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-400">Status</h3>
            <div className="mt-3 space-y-2">
              <div>{wanted?.enabled ? <StatusBadge label="Monitored" /> : <StatusBadge label="Unmonitored" />}</div>
              <div>{files.length === 0 ? <StatusBadge label="Missing" /> : <StatusBadge label="Has file" />}</div>
              <p className="text-xs text-slate-400">Profile: {wanted?.profile_id || 'default'}</p>
              <p className="text-xs text-slate-400">Files: {files.length}</p>
            </div>
          </aside>
        </div>
      ) : null}

      {tab === 'files' ? (
        <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-900 text-slate-300">
              <tr>
                <th className="px-3 py-2">Path</th>
                <th className="px-3 py-2">Format</th>
                <th className="px-3 py-2">Size</th>
              </tr>
            </thead>
            <tbody>
              {files.map((file, idx) => (
                <tr key={`${file.id ?? idx}`} className="border-t border-slate-800 text-slate-100">
                  <td className="px-3 py-2 text-slate-300">{file.path || '-'}</td>
                  <td className="px-3 py-2">{file.format || '-'}</td>
                  <td className="px-3 py-2">{file.size_bytes ? `${Math.round(file.size_bytes / (1024 * 1024))} MB` : '-'}</td>
                </tr>
              ))}
              {files.length === 0 ? (
                <tr><td colSpan={3} className="px-3 py-6 text-center text-slate-400">No files in library.</td></tr>
              ) : null}
            </tbody>
          </table>
        </div>
      ) : null}

      {tab === 'search' ? (
        <div className="rounded border border-slate-800 bg-slate-900/50 p-4 text-sm text-slate-200">
          <p>Manual search and candidate scoring are available in the dedicated search view.</p>
          <div className="mt-3 flex gap-2">
            <button className="rounded border border-emerald-700 px-3 py-1.5 text-sm text-emerald-300" onClick={() => navigate(`${manualSearchPath}&autorun=1`)}>
              Search now
            </button>
            <Link className="rounded border border-sky-700 px-3 py-1.5 text-sm text-sky-300" to={manualSearchPath}>
              Open manual search
            </Link>
          </div>
        </div>
      ) : null}

      {tab === 'history' ? (
        <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-900 text-slate-300">
              <tr>
                <th className="px-3 py-2">Stage</th>
                <th className="px-3 py-2">Status</th>
                <th className="px-3 py-2">Detail</th>
                <th className="px-3 py-2">Timestamp</th>
              </tr>
            </thead>
            <tbody>
              {timelineRows.map((row) => (
                <tr key={row.key} className="border-t border-slate-800 text-slate-100">
                  <td className="px-3 py-2">{row.stage}</td>
                  <td className="px-3 py-2"><StatusBadge label={row.status} /></td>
                  <td className="px-3 py-2 text-slate-300">{row.detail}</td>
                  <td className="px-3 py-2">{row.ts ? new Date(row.ts).toLocaleString() : '-'}</td>
                </tr>
              ))}
              {timelineRows.length === 0 ? (
                <tr><td colSpan={4} className="px-3 py-6 text-center text-slate-400">No timeline events found.</td></tr>
              ) : null}
            </tbody>
          </table>
        </div>
      ) : null}

      {tab === 'recommendations' ? (
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {recommendationRows.map((rec) => (
            <article key={rec.key} className="rounded border border-slate-800 bg-slate-900/50 p-3">
              <h3 className="text-sm font-semibold text-slate-100">{rec.title}</h3>
              <p className="mt-2 text-xs text-slate-300">{rec.reason}</p>
            </article>
          ))}
          {recommendationRows.length === 0 ? <div className="rounded border border-slate-800 bg-slate-900/50 p-6 text-center text-slate-400">No recommendations available.</div> : null}
        </div>
      ) : null}

      {workQuery.isLoading || libraryQuery.isLoading || wantedQuery.isLoading ? <p className="text-sm text-slate-400">Loading book detail...</p> : null}
      {workQuery.isError || libraryQuery.isError || wantedQuery.isError || timelineQuery.isError || profilesQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load book detail.</div> : null}
    </section>
  )
}








