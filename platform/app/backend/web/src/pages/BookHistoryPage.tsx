import { useMemo } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { fetchJSON } from '../lib/api'

type TimelinePayload = {
  work_id: string
  timeline: {
    searches?: Array<{ candidate_id?: number; grab_id?: number; download_job_id?: number; status?: string; updated_at?: string; created_at?: string }>
    grabs?: Array<{ grab_id?: number; candidate_id?: number; download_job_id?: number; protocol?: string; status?: string }>
    downloads?: Array<{ job?: { id?: number; status?: string; updated_at?: string; output_path?: string } }>
    imports?: Array<{ job?: { id?: number; status?: string; updated_at?: string; source_path?: string } }>
    library_items?: Array<{ id?: number; path?: string; format?: string }>
  }
}

type Row = {
  id: string
  stage: string
  status: string
  detail: string
  ts: string
}

export function BookHistoryPage() {
  const { workID = '' } = useParams<{ workID: string }>()

  const timelineQuery = useQuery({
    queryKey: ['book-history', workID],
    enabled: workID.trim().length > 0,
    queryFn: () => fetchJSON<TimelinePayload>(`/api/v1/work/${encodeURIComponent(workID)}/timeline`)
  })

  const rows = useMemo<Row[]>(() => {
    const timeline = timelineQuery.data?.timeline
    if (!timeline) return []
    const mapped: Row[] = []
    for (const item of timeline.searches ?? []) {
      mapped.push({
        id: `search-${item.download_job_id || item.grab_id || item.candidate_id || Math.random()}`,
        stage: 'search',
        status: item.status || '-',
        detail: `candidate=${item.candidate_id ?? '-'} grab=${item.grab_id ?? '-'}`,
        ts: item.updated_at || item.created_at || ''
      })
    }
    for (const item of timeline.grabs ?? []) {
      mapped.push({
        id: `grab-${item.grab_id || Math.random()}`,
        stage: 'grab',
        status: item.status || '-',
        detail: `protocol=${item.protocol || '-'} candidate=${item.candidate_id ?? '-'}`,
        ts: ''
      })
    }
    for (const item of timeline.downloads ?? []) {
      const job = item.job ?? {}
      mapped.push({
        id: `download-${job.id || Math.random()}`,
        stage: 'download',
        status: job.status || '-',
        detail: job.output_path || '-',
        ts: job.updated_at || ''
      })
    }
    for (const item of timeline.imports ?? []) {
      const job = item.job ?? {}
      mapped.push({
        id: `import-${job.id || Math.random()}`,
        stage: 'import',
        status: job.status || '-',
        detail: job.source_path || '-',
        ts: job.updated_at || ''
      })
    }
    for (const item of timeline.library_items ?? []) {
      mapped.push({
        id: `library-${item.id || Math.random()}`,
        stage: 'library',
        status: item.format || '-',
        detail: item.path || '-',
        ts: ''
      })
    }
    return mapped.sort((a, b) => new Date(b.ts).getTime() - new Date(a.ts).getTime())
  }, [timelineQuery.data?.timeline])

  return (
    <section className="space-y-4">
      <header className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-slate-100">Book History</h2>
          <p className="text-sm text-slate-400">Timeline for work {workID}</p>
        </div>
        <Link className="rounded border border-slate-700 px-3 py-1.5 text-sm text-slate-200" to="/library/books">
          Back to Books
        </Link>
      </header>

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
            {rows.map((row) => (
              <tr key={row.id} className="border-t border-slate-800 text-slate-100">
                <td className="px-3 py-2">{row.stage}</td>
                <td className="px-3 py-2">{row.status}</td>
                <td className="max-w-xl truncate px-3 py-2 text-slate-300" title={row.detail}>
                  {row.detail}
                </td>
                <td className="px-3 py-2">{row.ts ? new Date(row.ts).toLocaleString() : '-'}</td>
              </tr>
            ))}
            {rows.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-3 py-6 text-center text-slate-400">No timeline events found.</td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
      {timelineQuery.isLoading ? <p className="text-sm text-slate-400">Loading timeline...</p> : null}
      {timelineQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load timeline.</div> : null}
    </section>
  )
}
