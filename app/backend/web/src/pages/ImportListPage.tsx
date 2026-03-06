import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { PageHeader } from '../components/PageHeader'
import { StatusBadge } from '../components/StatusBadge'
import { useToast } from '../components/ToastProvider'
import { useLocalStorageState } from '../hooks/useLocalStorageState'
import { fetchJSON, postNoContent } from '../lib/api'

type ImportJob = {
  id: number
  download_job_id: number
  work_id?: string
  edition_id?: string
  source_path: string
  target_path?: string
  status: string
  last_error?: string
  naming_result_json?: Record<string, unknown>
  decision_json?: Record<string, unknown>
  updated_at: string
}

type ImportJobsResponse = { items: ImportJob[] }
type ImportJobDetailResponse = { job: ImportJob; events: unknown[] }

const DECISIONS = ['keep_both', 'replace_existing', 'skip'] as const

function reasonLabel(value?: string): string {
  const text = (value || '').toLowerCase()
  if (text.includes('collision')) return 'Collision'
  if (text.includes('confidence')) return 'Low confidence match'
  if (text.includes('ambiguous')) return 'Ambiguous match'
  if (text.includes('isbn')) return 'Ambiguous ISBN'
  return value?.trim() || 'Needs review'
}

export function ImportListPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()
  const [selectedID, setSelectedID] = useState<number | null>(null)
  const [approveWorkID, setApproveWorkID] = useState('')
  const [approveEditionID, setApproveEditionID] = useState('')
  const [decideAction, setDecideAction] = useState<(typeof DECISIONS)[number]>('keep_both')
  const [confirmSkip, setConfirmSkip] = useState(false)
  const [query, setQuery] = useLocalStorageState<string>('import.review.query', '')

  const listQuery = useQuery({
    queryKey: ['activity', 'import-needs-review'],
    queryFn: () => fetchJSON<ImportJobsResponse>('/api/v1/import/jobs?status=needs_review&limit=200'),
    refetchInterval: 15000
  })

  const selectedJob = useMemo(
    () => (listQuery.data?.items ?? []).find((item) => item.id === selectedID) ?? null,
    [listQuery.data?.items, selectedID]
  )

  const detailQuery = useQuery({
    queryKey: ['activity', 'import-job', selectedID],
    queryFn: () => fetchJSON<ImportJobDetailResponse>(`/api/v1/import/jobs/${selectedID}`),
    enabled: selectedID !== null
  })

  const refreshList = async () => {
    await queryClient.invalidateQueries({ queryKey: ['activity', 'import-needs-review'] })
    if (selectedID !== null) {
      await queryClient.invalidateQueries({ queryKey: ['activity', 'import-job', selectedID] })
    }
  }

  const approveMutation = useMutation({
    mutationFn: (payload: { id: number; workID: string; editionID: string }) =>
      postNoContent(`/api/v1/import/jobs/${payload.id}/approve`, {
        work_id: payload.workID,
        edition_id: payload.editionID
      }),
    onSuccess: async () => {
      pushToast('Import job approved')
      await refreshList()
    },
    onError: (error) => pushToast((error as Error).message)
  })

  const retryMutation = useMutation({
    mutationFn: (id: number) => postNoContent(`/api/v1/import/jobs/${id}/retry`),
    onSuccess: async () => {
      pushToast('Import job retried')
      await refreshList()
    },
    onError: (error) => pushToast((error as Error).message)
  })

  const skipMutation = useMutation({
    mutationFn: (id: number) => postNoContent(`/api/v1/import/jobs/${id}/skip`, { reason: 'operator skip from UI' }),
    onSuccess: async () => {
      pushToast('Import job skipped')
      setConfirmSkip(false)
      await refreshList()
    },
    onError: (error) => pushToast((error as Error).message)
  })

  const decideMutation = useMutation({
    mutationFn: (payload: { id: number; action: string }) =>
      postNoContent(`/api/v1/import/jobs/${payload.id}/decide`, { action: payload.action }),
    onSuccess: async () => {
      pushToast('Decision applied')
      await refreshList()
    },
    onError: (error) => pushToast((error as Error).message)
  })

  const filtered = useMemo(() => {
    const lowered = query.trim().toLowerCase()
    if (!lowered) return listQuery.data?.items ?? []
    return (listQuery.data?.items ?? []).filter((job) => {
      return (
        String(job.id).includes(lowered) ||
        (job.work_id || '').toLowerCase().includes(lowered) ||
        (job.source_path || '').toLowerCase().includes(lowered) ||
        reasonLabel(job.last_error).toLowerCase().includes(lowered)
      )
    })
  }, [listQuery.data?.items, query])

  const decisionContext = detailQuery.data?.job.decision_json ?? selectedJob?.decision_json ?? {}
  const namingPreview = detailQuery.data?.job.naming_result_json ?? selectedJob?.naming_result_json ?? {}

  return (
    <section className="space-y-4">
      <PageHeader
        title="Import Needs Review"
        subtitle="Resolve collisions and low-confidence matches quickly."
      />

      <div className="rounded border border-slate-800 bg-slate-900/60 p-3">
        <input
          className="w-full max-w-sm rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100"
          placeholder="Filter by id, work, reason, source path"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-[1.2fr_1fr]">
        <div className="overflow-x-auto rounded border border-slate-800 bg-slate-900/50">
          <table className="w-full min-w-[720px] text-left text-sm">
            <thead className="bg-slate-900 text-slate-300">
              <tr>
                <th className="px-3 py-2">ID</th>
                <th className="px-3 py-2">Work</th>
                <th className="px-3 py-2">Reason</th>
                <th className="hidden md:table-cell px-3 py-2">Source Path</th>
                <th className="hidden md:table-cell px-3 py-2">Updated</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((job) => (
                <tr
                  key={job.id}
                  className={[
                    'cursor-pointer border-t border-slate-800 text-slate-100',
                    selectedID === job.id ? 'bg-sky-900/30' : 'hover:bg-slate-800/40'
                  ].join(' ')}
                  onClick={() => {
                    setSelectedID(job.id)
                    setApproveWorkID(job.work_id || '')
                    setApproveEditionID(job.edition_id || '')
                  }}
                >
                  <td className="px-3 py-2">{job.id}</td>
                  <td className="px-3 py-2 text-slate-300">{job.work_id || '-'}</td>
                  <td className="px-3 py-2"><StatusBadge label={reasonLabel(job.last_error)} /></td>
                  <td className="hidden md:table-cell max-w-xs truncate px-3 py-2 text-slate-300" title={job.source_path}>{job.source_path}</td>
                  <td className="hidden md:table-cell px-3 py-2 text-slate-300">{new Date(job.updated_at).toLocaleString()}</td>
                </tr>
              ))}
              {filtered.length === 0 ? (
                <tr>
                  <td className="px-3 py-6 text-center text-slate-400" colSpan={5}>
                    No needs_review import jobs.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>

        <div className="rounded border border-slate-800 bg-slate-900/60 p-4">
          <h3 className="text-lg font-semibold text-slate-100">Review Details</h3>
          {!selectedJob ? <p className="mt-2 text-sm text-slate-400">Select a job from the list.</p> : null}

          {selectedJob ? (
            <div className="mt-3 space-y-3 text-sm">
              <p className="text-slate-300">Job #{selectedJob.id}</p>
              <div className="rounded border border-slate-800 bg-slate-900/40 p-2">
                <p className="text-xs uppercase text-slate-400">Naming preview</p>
                <p className="mt-1 text-xs text-slate-300">Final path: {selectedJob.target_path || '(pending)'} </p>
                <pre className="mt-1 overflow-auto text-xs text-slate-200">{JSON.stringify(namingPreview, null, 2)}</pre>
              </div>
              <div className="rounded border border-slate-800 bg-slate-900/40 p-2">
                <p className="text-xs uppercase text-slate-400">Candidate comparison context</p>
                <pre className="mt-1 overflow-auto text-xs text-slate-200">{JSON.stringify(decisionContext, null, 2)}</pre>
              </div>

              <label className="block">
                <span className="text-xs uppercase text-slate-400">Approve Work ID</span>
                <input
                  className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-sm text-slate-100"
                  value={approveWorkID}
                  onChange={(e) => setApproveWorkID(e.target.value)}
                />
              </label>
              <label className="block">
                <span className="text-xs uppercase text-slate-400">Approve Edition ID</span>
                <input
                  className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-sm text-slate-100"
                  value={approveEditionID}
                  onChange={(e) => setApproveEditionID(e.target.value)}
                />
              </label>

              <div className="flex flex-wrap gap-2">
                <button
                  className="rounded bg-emerald-700 px-2 py-1 text-xs font-medium text-white"
                  onClick={() => approveMutation.mutate({ id: selectedJob.id, workID: approveWorkID.trim(), editionID: approveEditionID.trim() })}
                >
                  Approve
                </button>
                <button
                  className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300"
                  onClick={() => retryMutation.mutate(selectedJob.id)}
                >
                  Retry
                </button>
                <button
                  className="rounded border border-red-700 px-2 py-1 text-xs text-red-300"
                  onClick={() => setConfirmSkip(true)}
                >
                  Skip
                </button>
              </div>

              <div className="flex items-center gap-2 border-t border-slate-800 pt-2">
                <select
                  className="rounded border border-slate-700 bg-slate-900 px-2 py-1 text-xs text-slate-100"
                  value={decideAction}
                  onChange={(e) => setDecideAction(e.target.value as (typeof DECISIONS)[number])}
                >
                  {DECISIONS.map((opt) => (
                    <option key={opt} value={opt}>
                      {opt}
                    </option>
                  ))}
                </select>
                <button
                  className="rounded border border-amber-700 px-2 py-1 text-xs text-amber-300"
                  onClick={() => decideMutation.mutate({ id: selectedJob.id, action: decideAction })}
                >
                  Apply Decision
                </button>
              </div>
              <div className="flex flex-wrap gap-2">
                <button className="rounded bg-emerald-700 px-2 py-1 text-xs text-white" onClick={() => decideMutation.mutate({ id: selectedJob.id, action: 'replace_existing' })}>
                  Replace Existing
                </button>
                <button className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300" onClick={() => decideMutation.mutate({ id: selectedJob.id, action: 'keep_both' })}>
                  Keep Both
                </button>
                <button className="rounded border border-red-700 px-2 py-1 text-xs text-red-300" onClick={() => decideMutation.mutate({ id: selectedJob.id, action: 'skip' })}>
                  Skip
                </button>
              </div>
            </div>
          ) : null}
        </div>
      </div>

      {listQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load import list.</div> : null}

      <ConfirmDialog
        open={confirmSkip}
        title="Skip import job"
        description={selectedJob ? `Skip import job #${selectedJob.id}?` : ''}
        onCancel={() => setConfirmSkip(false)}
        onConfirm={() => {
          if (selectedJob) {
            skipMutation.mutate(selectedJob.id)
          }
        }}
      />
    </section>
  )
}
