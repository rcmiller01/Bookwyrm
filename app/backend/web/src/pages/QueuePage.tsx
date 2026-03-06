import { useCallback, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { useToast } from '../components/ToastProvider'
import { usePolling } from '../hooks/usePolling'
import { fetchJSON, postNoContent } from '../lib/api'

type DownloadJob = {
  id: number
  work_id: string
  protocol: string
  client_name: string
  status: string
  output_path?: string
  last_error?: string
  updated_at: string
}

type DownloadJobsResponse = { items: DownloadJob[] }

export function QueuePage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()
  const [cancelTarget, setCancelTarget] = useState<DownloadJob | null>(null)

  const queueQuery = useQuery({
    queryKey: ['activity', 'queue'],
    queryFn: () => fetchJSON<DownloadJobsResponse>('/api/v1/download/jobs?limit=200')
  })

  const refetchQueue = useCallback(() => {
    void queueQuery.refetch()
  }, [queueQuery])
  usePolling(refetchQueue, 3000, true)

  const cancelMutation = useMutation({
    mutationFn: (jobID: number) => postNoContent(`/api/v1/download/jobs/${jobID}/cancel`),
    onSuccess: async () => {
      pushToast('Download canceled')
      setCancelTarget(null)
      await queryClient.invalidateQueries({ queryKey: ['activity', 'queue'] })
    },
    onError: (error) => pushToast((error as Error).message)
  })

  const retryMutation = useMutation({
    mutationFn: (jobID: number) => postNoContent(`/api/v1/download/jobs/${jobID}/retry`),
    onSuccess: async () => {
      pushToast('Download retried')
      await queryClient.invalidateQueries({ queryKey: ['activity', 'queue'] })
    },
    onError: (error) => pushToast((error as Error).message)
  })

  const jobs = queueQuery.data?.items ?? []

  const inProgressCount = useMemo(() => {
    const active = new Set(['queued', 'submitted', 'downloading', 'repairing', 'unpacking'])
    return jobs.filter((job) => active.has(job.status.toLowerCase())).length
  }, [jobs])

  return (
    <section className="space-y-4">
      <header className="flex items-end justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-slate-100">Queue</h2>
          <p className="text-sm text-slate-400">Live download queue (auto-refresh every 3s).</p>
        </div>
        <div className="rounded border border-slate-700 bg-slate-900/70 px-3 py-2 text-sm text-slate-200">
          In progress: <span className="font-semibold text-slate-100">{inProgressCount}</span>
        </div>
      </header>

      {queueQuery.isLoading ? <p className="text-sm text-slate-400">Loading queue...</p> : null}

      <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
        <table className="w-full text-left text-sm">
          <thead className="bg-slate-900 text-slate-300">
            <tr>
              <th className="px-3 py-2">ID</th>
              <th className="px-3 py-2">Work</th>
              <th className="px-3 py-2">Client</th>
              <th className="px-3 py-2">Protocol</th>
              <th className="px-3 py-2">Status</th>
              <th className="px-3 py-2">Output</th>
              <th className="px-3 py-2">Updated</th>
              <th className="px-3 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {jobs.map((job) => (
              <tr key={job.id} className="border-t border-slate-800 text-slate-100">
                <td className="px-3 py-2">{job.id}</td>
                <td className="px-3 py-2">{job.work_id || '-'}</td>
                <td className="px-3 py-2">{job.client_name}</td>
                <td className="px-3 py-2">{job.protocol}</td>
                <td className="px-3 py-2">{job.status}</td>
                <td className="max-w-xs truncate px-3 py-2 text-slate-300" title={job.output_path || ''}>
                  {job.output_path || '-'}
                </td>
                <td className="px-3 py-2 text-slate-300">{new Date(job.updated_at).toLocaleString()}</td>
                <td className="px-3 py-2">
                  <div className="flex gap-2">
                    <button
                      className="rounded border border-red-700 px-2 py-1 text-xs text-red-300"
                      onClick={() => setCancelTarget(job)}
                    >
                      Cancel
                    </button>
                    <button
                      className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300"
                      onClick={() => retryMutation.mutate(job.id)}
                    >
                      Retry
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {jobs.length === 0 ? (
              <tr>
                <td className="px-3 py-6 text-center text-slate-400" colSpan={8}>
                  No download jobs found.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      {queueQuery.isError ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load queue.</div>
      ) : null}

      <ConfirmDialog
        open={cancelTarget !== null}
        title="Cancel download job"
        description={cancelTarget ? `Cancel job #${cancelTarget.id}?` : ''}
        onCancel={() => setCancelTarget(null)}
        onConfirm={() => {
          if (cancelTarget) {
            cancelMutation.mutate(cancelTarget.id)
          }
        }}
      />
    </section>
  )
}
