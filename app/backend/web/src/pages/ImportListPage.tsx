import { useCallback, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CandidateComparisonTable } from '../components/CandidateComparisonTable'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { EventTimeline } from '../components/EventTimeline'
import { PageHeader } from '../components/PageHeader'
import { StatusBadge } from '../components/StatusBadge'
import { useToast } from '../components/ToastProvider'
import { useLocalStorageState } from '../hooks/useLocalStorageState'
import { fetchJSON, postNoContent } from '../lib/api'
import { errorMessage } from '../lib/errorMessage'

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
type ImportEvent = {
  id?: number
  ts?: string
  event_type?: string
  message?: string
  payload?: Record<string, unknown>
}

type ImportJobDetailResponse = { job: ImportJob; events: ImportEvent[] }

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

  const refreshList = useCallback(async () => {
    await queryClient.invalidateQueries({ queryKey: ['activity', 'import-needs-review'] })
    if (selectedID !== null) {
      await queryClient.invalidateQueries({ queryKey: ['activity', 'import-job', selectedID] })
    }
  }, [queryClient, selectedID])

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
    onError: (error) => pushToast(errorMessage(error))
  })

  const retryMutation = useMutation({
    mutationFn: (id: number) => postNoContent(`/api/v1/import/jobs/${id}/retry`),
    onSuccess: async () => {
      pushToast('Import job retried')
      await refreshList()
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const skipMutation = useMutation({
    mutationFn: (id: number) => postNoContent(`/api/v1/import/jobs/${id}/skip`, { reason: 'operator skip from UI' }),
    onSuccess: async () => {
      pushToast('Import job skipped')
      setConfirmSkip(false)
      await refreshList()
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const decideMutation = useMutation({
    mutationFn: (payload: { id: number; action: string }) =>
      postNoContent(`/api/v1/import/jobs/${payload.id}/decide`, { action: payload.action }),
    onSuccess: async () => {
      pushToast('Decision applied')
      await refreshList()
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const [approveRerunning, setApproveRerunning] = useState(false)
  const approveRerunAbort = useRef(false)

  const approveAndRerun = useCallback(async (jobId: number, workID: string, editionID: string) => {
    if (!workID.trim()) {
      pushToast('Work ID is required for approve')
      return
    }
    setApproveRerunning(true)
    approveRerunAbort.current = false
    try {
      await postNoContent(`/api/v1/import/jobs/${jobId}/approve`, {
        work_id: workID.trim(),
        edition_id: editionID.trim()
      })
      // Poll until status leaves needs_review (10s timeout)
      const deadline = Date.now() + 10_000
      while (Date.now() < deadline && !approveRerunAbort.current) {
        await new Promise((r) => setTimeout(r, 500))
        try {
          const detail = await fetchJSON<ImportJobDetailResponse>(`/api/v1/import/jobs/${jobId}`)
          const status = detail.job.status
          if (status !== 'needs_review' && status !== 'queued' && status !== 'running') {
            pushToast(`Import job #${jobId}: ${status}`)
            await refreshList()
            return
          }
        } catch {
          // polling error, continue
        }
      }
      pushToast('Approved — rerun in progress (check back shortly)')
      await refreshList()
    } catch (err) {
      pushToast(errorMessage(err))
    } finally {
      setApproveRerunning(false)
    }
  }, [pushToast, refreshList])

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

  const decisionContext = useMemo(
    () => detailQuery.data?.job.decision_json ?? selectedJob?.decision_json ?? {},
    [detailQuery.data?.job.decision_json, selectedJob?.decision_json]
  )
  const namingPreview = detailQuery.data?.job.naming_result_json ?? selectedJob?.naming_result_json ?? {}

  const candidateRows = useMemo(() => {
    const raw = decisionContext.candidates
    if (!Array.isArray(raw)) return []
    return raw.map((c: Record<string, unknown>) => ({
      work_id: typeof c.work_id === 'string' ? c.work_id : undefined,
      title: typeof c.title === 'string' ? c.title : undefined,
      title_score: typeof c.title_score === 'number' ? c.title_score : undefined,
      author_score: typeof c.author_score === 'number' ? c.author_score : undefined,
      score: typeof c.score === 'number' ? c.score : undefined,
      reason: typeof c.reason === 'string' ? c.reason : undefined
    }))
  }, [decisionContext])

  const collisionInfo = useMemo(() => {
    const col = decisionContext.collision
    if (!col || typeof col !== 'object') return null
    return col as { target_path?: string; reason?: string }
  }, [decisionContext])

  const detailEvents = useMemo(() => {
    return (detailQuery.data?.events ?? []) as ImportEvent[]
  }, [detailQuery.data?.events])

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
                    // Auto-populate from the highest-scoring candidate if available
                    const candidates = Array.isArray(job.decision_json?.candidates) ? job.decision_json.candidates : []
                    const topCandidate = candidates.length > 0
                      ? (candidates as Array<Record<string, unknown>>).reduce((best, c) => {
                          const bs = typeof best.score === 'number' ? best.score : 0
                          const cs = typeof c.score === 'number' ? c.score : 0
                          return cs > bs ? c : best
                        })
                      : null
                    setApproveWorkID(
                      (topCandidate && typeof topCandidate.work_id === 'string' ? topCandidate.work_id : null) || job.work_id || ''
                    )
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
                {Object.keys(namingPreview).length > 0 ? (
                  <details className="mt-1">
                    <summary className="cursor-pointer text-[10px] text-slate-500 hover:text-slate-300">raw naming data</summary>
                    <pre className="mt-1 overflow-auto text-[10px] text-slate-200">{JSON.stringify(namingPreview, null, 2)}</pre>
                  </details>
                ) : null}
              </div>

              <div className="rounded border border-slate-800 bg-slate-900/40 p-2">
                <p className="text-xs uppercase text-slate-400">Candidate comparison</p>
                {candidateRows.length > 0 ? (
                  <CandidateComparisonTable
                    candidates={candidateRows}
                    onApprove={(workId) => {
                      setApproveWorkID(workId)
                      setApproveEditionID('')
                    }}
                  />
                ) : (
                  <p className="mt-1 text-xs text-slate-400">No candidate data in decision context.</p>
                )}
                {collisionInfo ? (
                  <div className="mt-2 rounded border border-amber-900/60 bg-amber-950/30 p-2">
                    <p className="text-xs font-medium text-amber-300">Collision detected</p>
                    <p className="mt-0.5 text-xs text-amber-200/80">Target: {collisionInfo.target_path ?? '(unknown)'}</p>
                    {collisionInfo.reason ? <p className="mt-0.5 text-xs text-amber-200/80">Reason: {collisionInfo.reason}</p> : null}
                  </div>
                ) : null}
                {Object.keys(decisionContext).length > 0 ? (
                  <details className="mt-2">
                    <summary className="cursor-pointer text-[10px] text-slate-500 hover:text-slate-300">raw decision data</summary>
                    <pre className="mt-1 overflow-auto text-[10px] text-slate-200">{JSON.stringify(decisionContext, null, 2)}</pre>
                  </details>
                ) : null}
              </div>

              {detailEvents.length > 0 ? (
                <div className="rounded border border-slate-800 bg-slate-900/40 p-2">
                  <p className="text-xs uppercase text-slate-400 mb-2">Event timeline</p>
                  <EventTimeline events={detailEvents} />
                </div>
              ) : null}

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
                  className="rounded bg-emerald-700/80 px-2 py-1 text-xs font-medium text-white disabled:opacity-40"
                  disabled={approveRerunning}
                  onClick={() => approveAndRerun(selectedJob.id, approveWorkID, approveEditionID)}
                >
                  {approveRerunning ? '⟳ Running...' : 'Approve & Rerun'}
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
