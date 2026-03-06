import { useCallback, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { FilterBar } from '../components/FilterBar'
import { PageHeader } from '../components/PageHeader'
import { StatusBadge } from '../components/StatusBadge'
import { useToast } from '../components/ToastProvider'
import { useLocalStorageState } from '../hooks/useLocalStorageState'
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

const activeStatuses = new Set(['queued', 'submitted', 'downloading', 'repairing', 'unpacking'])
const completedStatuses = new Set(['completed', 'imported'])

function statusGroup(status: string): 'downloading' | 'repairing' | 'unpacking' | 'completed' | 'other' {
  const lowered = status.toLowerCase()
  if (completedStatuses.has(lowered)) return 'completed'
  if (lowered.includes('repair')) return 'repairing'
  if (lowered.includes('unpack')) return 'unpacking'
  if (lowered.includes('download') || lowered.includes('queued') || lowered.includes('submit')) return 'downloading'
  return 'other'
}

function progressForStatus(status: string): number {
  const lowered = status.toLowerCase()
  if (lowered === 'queued') return 10
  if (lowered === 'submitted') return 20
  if (lowered === 'downloading') return 55
  if (lowered === 'repairing') return 75
  if (lowered === 'unpacking') return 90
  if (completedStatuses.has(lowered)) return 100
  return 5
}

export function QueuePage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()
  const [cancelTarget, setCancelTarget] = useState<DownloadJob | null>(null)
  const [groupFilter, setGroupFilter] = useLocalStorageState<'all' | 'downloading' | 'repairing' | 'unpacking' | 'completed' | 'other'>('queue.filter.group', 'all')
  const [showCompleted, setShowCompleted] = useLocalStorageState<boolean>('queue.filter.showCompleted', true)

  const queueQuery = useQuery({
    queryKey: ['activity', 'queue'],
    queryFn: () => fetchJSON<DownloadJobsResponse>('/api/v1/download/jobs?limit=300')
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

  const filteredJobs = useMemo(() => {
    return jobs
      .filter((job) => (showCompleted ? true : !completedStatuses.has(job.status.toLowerCase())))
      .filter((job) => (groupFilter === 'all' ? true : statusGroup(job.status) === groupFilter))
      .sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime())
  }, [groupFilter, jobs, showCompleted])

  const inProgressCount = useMemo(() => jobs.filter((job) => activeStatuses.has(job.status.toLowerCase())).length, [jobs])

  return (
    <section className="space-y-4">
      <PageHeader
        title="Queue"
        subtitle="Live download queue grouped by status (auto-refresh every 3s)."
        actions={
          <div className="rounded border border-slate-700 bg-slate-900/70 px-3 py-2 text-sm text-slate-200">
            In progress: <span className="font-semibold text-slate-100">{inProgressCount}</span>
          </div>
        }
      />

      <FilterBar>
        <select className="rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100" value={groupFilter} onChange={(event) => setGroupFilter(event.target.value as 'all' | 'downloading' | 'repairing' | 'unpacking' | 'completed' | 'other')}>
          <option value="all">All groups</option>
          <option value="downloading">Downloading</option>
          <option value="repairing">Repairing</option>
          <option value="unpacking">Unpacking</option>
          <option value="completed">Completed</option>
          <option value="other">Other</option>
        </select>
        <label className="inline-flex items-center gap-2 rounded border border-slate-700 px-2 py-1.5 text-xs text-slate-300">
          <input type="checkbox" checked={showCompleted} onChange={(event) => setShowCompleted(event.target.checked)} />
          Show completed
        </label>
      </FilterBar>

      {queueQuery.isLoading ? <p className="text-sm text-slate-400">Loading queue...</p> : null}

      <div className="overflow-x-auto rounded border border-slate-800 bg-slate-900/50">
        <table className="w-full min-w-[760px] text-left text-sm">
          <thead className="bg-slate-900 text-slate-300">
            <tr>
              <th className="px-3 py-2">ID</th>
              <th className="px-3 py-2">Work</th>
              <th className="hidden md:table-cell px-3 py-2">Client</th>
              <th className="hidden md:table-cell px-3 py-2">Protocol</th>
              <th className="hidden md:table-cell px-3 py-2">Group</th>
              <th className="px-3 py-2">Status</th>
              <th className="hidden md:table-cell px-3 py-2">Progress</th>
              <th className="hidden md:table-cell px-3 py-2">Updated</th>
              <th className="px-3 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {filteredJobs.map((job) => {
              const group = statusGroup(job.status)
              const progress = progressForStatus(job.status)
              return (
                <tr key={job.id} className="border-t border-slate-800 text-slate-100">
                  <td className="px-3 py-2">{job.id}</td>
                  <td className="px-3 py-2 text-slate-300">{job.work_id || '-'}</td>
                  <td className="hidden md:table-cell px-3 py-2">{job.client_name}</td>
                  <td className="hidden md:table-cell px-3 py-2">{job.protocol}</td>
                  <td className="hidden md:table-cell px-3 py-2 capitalize text-slate-300">{group}</td>
                  <td className="px-3 py-2"><StatusBadge label={job.status} /></td>
                  <td className="hidden md:table-cell px-3 py-2">
                    <div className="w-36 rounded bg-slate-800">
                      <div className="h-2 rounded bg-sky-500" style={{ width: `${progress}%` }} />
                    </div>
                  </td>
                  <td className="hidden md:table-cell px-3 py-2 text-slate-300">{new Date(job.updated_at).toLocaleString()}</td>
                  <td className="px-3 py-2">
                    <div className="flex gap-2">
                      <button className="rounded border border-red-700 px-2 py-1 text-xs text-red-300" onClick={() => setCancelTarget(job)}>
                        Cancel
                      </button>
                      <button className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300" onClick={() => retryMutation.mutate(job.id)}>
                        Retry
                      </button>
                    </div>
                  </td>
                </tr>
              )
            })}
            {filteredJobs.length === 0 ? (
              <tr>
                <td className="px-3 py-6 text-center text-slate-400" colSpan={9}>
                  No download jobs for the current filters.
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
