import { useMemo } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useToast } from '../components/ToastProvider'
import { fetchJSON, postNoContent } from '../lib/api'

type JobRecord = {
  id: string
  type: string
  state: string
  attempt_count: number
  max_attempts: number
  run_at: string
  updated_at: string
}

type JobsResponse = { items: JobRecord[] }

export function TasksPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()

  const jobsQuery = useQuery({
    queryKey: ['system', 'tasks'],
    queryFn: () => fetchJSON<JobsResponse>('/api/v1/jobs?limit=200'),
    refetchInterval: 10000
  })

  const retryMutation = useMutation({
    mutationFn: async (id: string) => {
      await postNoContent(`/api/v1/jobs/${encodeURIComponent(id)}/retry`)
    },
    onSuccess: async () => {
      pushToast('Task retried')
      await queryClient.invalidateQueries({ queryKey: ['system', 'tasks'] })
    },
    onError: (error) => pushToast((error as Error).message)
  })

  const cancelMutation = useMutation({
    mutationFn: async (id: string) => {
      await postNoContent(`/api/v1/jobs/${encodeURIComponent(id)}/cancel`)
    },
    onSuccess: async () => {
      pushToast('Task canceled')
      await queryClient.invalidateQueries({ queryKey: ['system', 'tasks'] })
    },
    onError: (error) => pushToast((error as Error).message)
  })

  const rows = useMemo(() => {
    return [...(jobsQuery.data?.items ?? [])].sort(
      (a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
    )
  }, [jobsQuery.data?.items])

  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">Tasks</h2>
        <p className="text-sm text-slate-400">Background task queue from backend job service.</p>
      </header>

      <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
        <table className="w-full text-left text-sm">
          <thead className="bg-slate-900 text-slate-300">
            <tr>
              <th className="px-3 py-2">ID</th>
              <th className="px-3 py-2">Type</th>
              <th className="px-3 py-2">State</th>
              <th className="px-3 py-2">Attempts</th>
              <th className="px-3 py-2">Run At</th>
              <th className="px-3 py-2">Updated</th>
              <th className="px-3 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.id} className="border-t border-slate-800 text-slate-100">
                <td className="px-3 py-2">{row.id}</td>
                <td className="px-3 py-2">{row.type}</td>
                <td className="px-3 py-2">{row.state}</td>
                <td className="px-3 py-2">{row.attempt_count}/{row.max_attempts}</td>
                <td className="px-3 py-2 text-slate-300">{new Date(row.run_at).toLocaleString()}</td>
                <td className="px-3 py-2 text-slate-300">{new Date(row.updated_at).toLocaleString()}</td>
                <td className="px-3 py-2">
                  <div className="flex flex-wrap gap-2">
                    <button
                      className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300"
                      disabled={retryMutation.isPending}
                      onClick={() => retryMutation.mutate(row.id)}
                    >
                      Retry
                    </button>
                    <button
                      className="rounded border border-red-700 px-2 py-1 text-xs text-red-300"
                      disabled={cancelMutation.isPending}
                      onClick={() => cancelMutation.mutate(row.id)}
                    >
                      Cancel
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {rows.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-3 py-6 text-center text-slate-400">
                  No tasks found.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      {jobsQuery.isLoading ? <p className="text-sm text-slate-400">Loading tasks...</p> : null}
      {jobsQuery.isError ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">
          Failed to load tasks.
        </div>
      ) : null}
    </section>
  )
}
