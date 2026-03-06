import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { fetchJSON } from '../lib/api'

type DownloadJob = {
  id: number
  work_id: string
  status: string
  updated_at: string
  protocol: string
  client_name: string
}
type DownloadJobsResponse = { items: DownloadJob[] }

type ImportJob = {
  id: number
  work_id?: string
  status: string
  updated_at: string
  source_path: string
}
type ImportJobsResponse = { items: ImportJob[] }

type HistoryItem = {
  id: string
  kind: 'download' | 'import'
  sourceID: number
  workID: string
  status: string
  updatedAt: string
  detail: string
}

export function HistoryPage() {
  const downloadsQuery = useQuery({
    queryKey: ['activity', 'history', 'downloads'],
    queryFn: () => fetchJSON<DownloadJobsResponse>('/api/v1/download/jobs?limit=200'),
    refetchInterval: 10000
  })

  const importsQuery = useQuery({
    queryKey: ['activity', 'history', 'imports'],
    queryFn: () => fetchJSON<ImportJobsResponse>('/api/v1/import/jobs?limit=200'),
    refetchInterval: 10000
  })

  const items = useMemo(() => {
    const fromDownloads: HistoryItem[] = (downloadsQuery.data?.items ?? []).map((job) => ({
      id: `download-${job.id}`,
      kind: 'download',
      sourceID: job.id,
      workID: job.work_id || '-',
      status: job.status,
      updatedAt: job.updated_at,
      detail: `${job.client_name} • ${job.protocol}`
    }))

    const fromImports: HistoryItem[] = (importsQuery.data?.items ?? []).map((job) => ({
      id: `import-${job.id}`,
      kind: 'import',
      sourceID: job.id,
      workID: job.work_id || '-',
      status: job.status,
      updatedAt: job.updated_at,
      detail: job.source_path
    }))

    return [...fromDownloads, ...fromImports].sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())
  }, [downloadsQuery.data?.items, importsQuery.data?.items])

  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">History</h2>
        <p className="text-sm text-slate-400">Recent download and import activity.</p>
      </header>

      <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
        <table className="w-full text-left text-sm">
          <thead className="bg-slate-900 text-slate-300">
            <tr>
              <th className="px-3 py-2">Type</th>
              <th className="px-3 py-2">ID</th>
              <th className="px-3 py-2">Work</th>
              <th className="px-3 py-2">Status</th>
              <th className="px-3 py-2">Detail</th>
              <th className="px-3 py-2">Updated</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.id} className="border-t border-slate-800 text-slate-100">
                <td className="px-3 py-2 capitalize">{item.kind}</td>
                <td className="px-3 py-2">{item.sourceID}</td>
                <td className="px-3 py-2">{item.workID}</td>
                <td className="px-3 py-2">{item.status}</td>
                <td className="max-w-sm truncate px-3 py-2 text-slate-300" title={item.detail}>
                  {item.detail}
                </td>
                <td className="px-3 py-2 text-slate-300">{new Date(item.updatedAt).toLocaleString()}</td>
              </tr>
            ))}
            {items.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-3 py-6 text-center text-slate-400">
                  No recent history.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      {downloadsQuery.isError || importsQuery.isError ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load history data.</div>
      ) : null}
    </section>
  )
}
