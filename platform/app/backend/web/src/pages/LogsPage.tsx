import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { fetchJSON } from '../lib/api'

type DownloadJob = {
  id: number
  status: string
  work_id?: string
  last_error?: string
  updated_at: string
}

type DownloadJobsResponse = { items: DownloadJob[] }

type ImportJob = {
  id: number
  status: string
  work_id?: string
  last_error?: string
  updated_at: string
}

type ImportJobsResponse = { items: ImportJob[] }

type LogRow = {
  id: string
  source: string
  entity: string
  status: string
  message: string
  updatedAt: string
}

export function LogsPage() {
  const downloadsQuery = useQuery({
    queryKey: ['system', 'logs', 'downloads'],
    queryFn: () => fetchJSON<DownloadJobsResponse>('/api/v1/download/jobs?limit=200'),
    refetchInterval: 10000
  })

  const importsQuery = useQuery({
    queryKey: ['system', 'logs', 'imports'],
    queryFn: () => fetchJSON<ImportJobsResponse>('/api/v1/import/jobs?limit=200'),
    refetchInterval: 10000
  })

  const rows = useMemo<LogRow[]>(() => {
    const downloadRows = (downloadsQuery.data?.items ?? []).map<LogRow>((item) => ({
      id: `download-${item.id}`,
      source: 'download',
      entity: item.work_id || '-',
      status: item.status,
      message: item.last_error?.trim() || 'status updated',
      updatedAt: item.updated_at
    }))
    const importRows = (importsQuery.data?.items ?? []).map<LogRow>((item) => ({
      id: `import-${item.id}`,
      source: 'import',
      entity: item.work_id || '-',
      status: item.status,
      message: item.last_error?.trim() || 'status updated',
      updatedAt: item.updated_at
    }))
    return [...downloadRows, ...importRows].sort(
      (a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
    )
  }, [downloadsQuery.data?.items, importsQuery.data?.items])

  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">Logs</h2>
        <p className="text-sm text-slate-400">Recent operational events from download and import pipelines.</p>
      </header>

      <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
        <table className="w-full text-left text-sm">
          <thead className="bg-slate-900 text-slate-300">
            <tr>
              <th className="px-3 py-2">Source</th>
              <th className="px-3 py-2">Entity</th>
              <th className="px-3 py-2">Status</th>
              <th className="px-3 py-2">Message</th>
              <th className="px-3 py-2">Updated</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.id} className="border-t border-slate-800 text-slate-100">
                <td className="px-3 py-2">{row.source}</td>
                <td className="px-3 py-2">{row.entity}</td>
                <td className="px-3 py-2">{row.status}</td>
                <td className="px-3 py-2 text-slate-300">{row.message}</td>
                <td className="px-3 py-2 text-slate-300">{new Date(row.updatedAt).toLocaleString()}</td>
              </tr>
            ))}
            {rows.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-3 py-6 text-center text-slate-400">
                  No events to display.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      {downloadsQuery.isLoading || importsQuery.isLoading ? <p className="text-sm text-slate-400">Loading logs...</p> : null}
      {downloadsQuery.isError || importsQuery.isError ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">
          Failed to load logs.
        </div>
      ) : null}
    </section>
  )
}
