import { useCallback, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useToast } from '../components/ToastProvider'
import { usePolling } from '../hooks/usePolling'
import { fetchJSON, postJSON, postNoContent } from '../lib/api'
import { errorMessage } from '../lib/errorMessage'

type EnqueueSearchResponse = {
  search_request_id: number
  status: string
}

type SearchRequestRecord = {
  id: number
  status: string
  last_error?: string
  updated_at: string
}

type SearchRequestResponse = {
  request: SearchRequestRecord
}

type Candidate = {
  candidate_id: string
  title: string
  protocol?: string
  score?: number
  seeders?: number
  size_bytes?: number
  source_pipeline?: string
  source_backend_id?: string
  reasons?: { code: string; message?: string; weight?: number }[]
}

type CandidateRecord = {
  id: number
  candidate: Candidate
}

type CandidatesResponse = { items: CandidateRecord[] }

type GrabResponse = {
  grab: {
    id: number
  }
}

type BackendRecord = {
  id: string
  name: string
  enabled: boolean
  config_json?: Record<string, unknown>
}

type BackendsResponse = { backends: BackendRecord[] }

function parseCSV(value: string): string[] {
  return value
    .split(',')
    .map((v) => v.trim())
    .filter(Boolean)
}

function isPreferred(backend: BackendRecord): boolean {
  return Boolean(backend.config_json?.preferred)
}

export function ManualSearchPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()

  const [workID, setWorkID] = useState('')
  const [title, setTitle] = useState('')
  const [author, setAuthor] = useState('')
  const [formats, setFormats] = useState('epub')
  const [languages, setLanguages] = useState('en')
  const [limit, setLimit] = useState(50)
  const [timeoutSec, setTimeoutSec] = useState(15)
  const [minCandidates, setMinCandidates] = useState(3)
  const [minScoreThreshold, setMinScoreThreshold] = useState(0.7)
  const [stageTimeoutSec, setStageTimeoutSec] = useState(45)
  const [autoHandoff, setAutoHandoff] = useState(true)
  const [searchRequestID, setSearchRequestID] = useState<number | null>(null)
  const [searchStartedAt, setSearchStartedAt] = useState<number | null>(null)

  const backendsQuery = useQuery({
    queryKey: ['indexers', 'backends'],
    queryFn: () => fetchJSON<BackendsResponse>('/ui-api/indexer/backends'),
    refetchInterval: 10000
  })

  const requestQuery = useQuery({
    queryKey: ['manual-search', 'request', searchRequestID],
    queryFn: () => fetchJSON<SearchRequestResponse>(`/ui-api/indexer/search/${searchRequestID}`),
    enabled: searchRequestID !== null
  })

  const candidatesQuery = useQuery({
    queryKey: ['manual-search', 'candidates', searchRequestID],
    queryFn: () => fetchJSON<CandidatesResponse>(`/ui-api/indexer/candidates/${searchRequestID}?limit=${limit}`),
    enabled: searchRequestID !== null
  })

  const refreshSearch = useCallback(() => {
    if (searchRequestID === null) {
      return
    }
    void requestQuery.refetch()
    void candidatesQuery.refetch()
  }, [candidatesQuery, requestQuery, searchRequestID])

  const shouldPoll = useMemo(() => {
    if (!searchStartedAt || !requestQuery.data?.request) {
      return false
    }
    const status = requestQuery.data.request.status?.toLowerCase() ?? ''
    if (status === 'succeeded' || status === 'failed') {
      return false
    }
    return Date.now() - searchStartedAt < stageTimeoutSec * 1000
  }, [requestQuery.data?.request, searchStartedAt, stageTimeoutSec])

  usePolling(refreshSearch, 2000, shouldPoll)

  const enqueueMutation = useMutation({
    mutationFn: () =>
      postJSON<EnqueueSearchResponse>(`/ui-api/indexer/search/work/${encodeURIComponent(workID.trim())}`, {
        title: title.trim(),
        author: author.trim(),
        formats: parseCSV(formats),
        languages: parseCSV(languages),
        limit,
        timeout_sec: timeoutSec
      }),
    onSuccess: (payload) => {
      setSearchRequestID(payload.search_request_id)
      setSearchStartedAt(Date.now())
      pushToast(`Search queued (#${payload.search_request_id})`)
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const grabMutation = useMutation({
    mutationFn: async (candidateID: number) => {
      const grabbed = await postJSON<GrabResponse>(`/ui-api/indexer/grab/${candidateID}`, {})
      if (autoHandoff) {
        await postJSON<{ job: { id: number } }>(`/api/v1/download/from-grab/${grabbed.grab.id}`, {})
      }
      return grabbed
    },
    onSuccess: () => pushToast(autoHandoff ? 'Candidate grabbed and handed off to downloader' : 'Candidate grabbed'),
    onError: (error) => pushToast(errorMessage(error))
  })

  const preferredMutation = useMutation({
    mutationFn: (payload: { id: string; preferred: boolean }) =>
      postNoContent(`/ui-api/indexer/backends/${encodeURIComponent(payload.id)}/preferred`, { preferred: payload.preferred }),
    onSuccess: async () => {
      pushToast('Preferred source updated')
      await queryClient.invalidateQueries({ queryKey: ['indexers', 'backends'] })
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const candidateRows = candidatesQuery.data?.items ?? []
  const aboveThresholdCount = candidateRows.filter((row) => (row.candidate.score ?? 0) >= minScoreThreshold).length
  const stopConditionMet = aboveThresholdCount >= minCandidates

  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">Books</h2>
        <p className="text-sm text-slate-400">Manual staged search and candidate grab flow.</p>
      </header>

      <div className="rounded border border-slate-800 bg-slate-900/60 p-4">
        <h3 className="text-lg font-semibold text-slate-100">Staged Search</h3>
        <div className="mt-3 grid gap-3 md:grid-cols-3">
          <label className="text-sm text-slate-300">
            Work ID
            <input className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={workID} onChange={(e) => setWorkID(e.target.value)} />
          </label>
          <label className="text-sm text-slate-300">
            Title
            <input className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={title} onChange={(e) => setTitle(e.target.value)} />
          </label>
          <label className="text-sm text-slate-300">
            Author
            <input className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={author} onChange={(e) => setAuthor(e.target.value)} />
          </label>
          <label className="text-sm text-slate-300">
            Formats (csv)
            <input className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={formats} onChange={(e) => setFormats(e.target.value)} />
          </label>
          <label className="text-sm text-slate-300">
            Languages (csv)
            <input className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={languages} onChange={(e) => setLanguages(e.target.value)} />
          </label>
          <label className="text-sm text-slate-300">
            Max candidates
            <input type="number" min={1} className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={limit} onChange={(e) => setLimit(Number(e.target.value) || 50)} />
          </label>
          <label className="text-sm text-slate-300">
            Backend timeout (sec)
            <input type="number" min={1} className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={timeoutSec} onChange={(e) => setTimeoutSec(Number(e.target.value) || 15)} />
          </label>
          <label className="text-sm text-slate-300">
            Stop min candidates
            <input type="number" min={1} className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={minCandidates} onChange={(e) => setMinCandidates(Number(e.target.value) || 1)} />
          </label>
          <label className="text-sm text-slate-300">
            Stop min score
            <input type="number" min={0} max={1} step="0.05" className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={minScoreThreshold} onChange={(e) => setMinScoreThreshold(Number(e.target.value) || 0)} />
          </label>
          <label className="text-sm text-slate-300">
            Stage timeout (sec)
            <input type="number" min={5} className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={stageTimeoutSec} onChange={(e) => setStageTimeoutSec(Number(e.target.value) || 45)} />
          </label>
          <label className="flex items-center gap-2 text-sm text-slate-300">
            <input type="checkbox" checked={autoHandoff} onChange={(e) => setAutoHandoff(e.target.checked)} />
            Auto handoff to downloader
          </label>
        </div>

        <div className="mt-4 flex items-center gap-3">
          <button
            className="rounded bg-sky-700 px-3 py-1 text-sm font-medium text-white disabled:opacity-50"
            disabled={!workID.trim() || enqueueMutation.isPending}
            onClick={() => enqueueMutation.mutate()}
          >
            {enqueueMutation.isPending ? 'Searching...' : 'Run Manual Search'}
          </button>
          {searchRequestID ? <span className="text-sm text-slate-300">Request #{searchRequestID}</span> : null}
          {requestQuery.data?.request ? (
            <span className="text-sm text-slate-300">Status: {requestQuery.data.request.status}</span>
          ) : null}
        </div>

        {searchRequestID ? (
          <p className="mt-2 text-xs text-slate-400">
            Stop condition: {aboveThresholdCount} candidate(s) {">="} {minScoreThreshold.toFixed(2)} / required {minCandidates} ({stopConditionMet ? 'met' : 'not met'})
          </p>
        ) : null}
      </div>

      <div className="rounded border border-slate-800 bg-slate-900/50">
        <div className="border-b border-slate-800 px-3 py-2">
          <h3 className="text-lg font-semibold text-slate-100">Candidates</h3>
          <p className="text-xs text-slate-400">Preferred (hearted) sources are dispatched before non-preferred sources.</p>
        </div>
        <div className="overflow-auto">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-900 text-slate-300">
              <tr>
                <th className="px-3 py-2">Title</th>
                <th className="px-3 py-2">Source</th>
                <th className="px-3 py-2">Protocol</th>
                <th className="px-3 py-2">Size</th>
                <th className="px-3 py-2">Seeders</th>
                <th className="px-3 py-2">Score</th>
                <th className="px-3 py-2">Why</th>
                <th className="px-3 py-2">Actions</th>
              </tr>
            </thead>
            <tbody>
              {candidateRows.map((row) => {
                const candidate = row.candidate
                const backend = (backendsQuery.data?.backends ?? []).find((b) => b.id === candidate.source_backend_id)
                const preferred = backend ? isPreferred(backend) : false
                return (
                  <tr key={row.id} className="border-t border-slate-800 text-slate-100">
                    <td className="px-3 py-2">{candidate.title}</td>
                    <td className="px-3 py-2">
                      <div className="flex items-center gap-2">
                        <span className="rounded border border-slate-700 px-1.5 py-0.5 text-xs text-slate-300">
                          {candidate.source_pipeline === 'prowlarr' ? 'Prowlarr' : `MCP:${candidate.source_backend_id || 'unknown'}`}
                        </span>
                        {backend ? (
                          <button
                            className={[
                              'rounded px-1.5 py-0.5 text-xs',
                              preferred ? 'bg-amber-500/20 text-amber-300' : 'border border-slate-700 text-slate-400'
                            ].join(' ')}
                            onClick={() => preferredMutation.mutate({ id: backend.id, preferred: !preferred })}
                            title="Toggle preferred source"
                          >
                            Fav
                          </button>
                        ) : null}
                      </div>
                    </td>
                    <td className="px-3 py-2">{candidate.protocol || '-'}</td>
                    <td className="px-3 py-2">{candidate.size_bytes ? Math.round(candidate.size_bytes / (1024 * 1024)) + ' MB' : '-'}</td>
                    <td className="px-3 py-2">{candidate.seeders ?? '-'}</td>
                    <td className="px-3 py-2">{(candidate.score ?? 0).toFixed(2)}</td>
                    <td className="px-3 py-2">
                      <details>
                        <summary className="cursor-pointer text-xs text-sky-300">reasons</summary>
                        <ul className="mt-1 space-y-1 text-xs text-slate-300">
                          {(candidate.reasons ?? []).map((reason, idx) => (
                            <li key={`${row.id}-reason-${idx}`}>
                              {reason.code}{reason.message ? `: ${reason.message}` : ''}
                            </li>
                          ))}
                        </ul>
                      </details>
                    </td>
                    <td className="px-3 py-2">
                      <button
                        className="rounded border border-emerald-700 px-2 py-1 text-xs text-emerald-300"
                        onClick={() => grabMutation.mutate(row.id)}
                      >
                        Grab
                      </button>
                    </td>
                  </tr>
                )
              })}
              {candidateRows.length === 0 ? (
                <tr>
                  <td className="px-3 py-6 text-center text-slate-400" colSpan={8}>
                    No candidates yet. Run a manual search.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </div>

      {requestQuery.data?.request?.last_error ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">{requestQuery.data.request.last_error}</div>
      ) : null}
    </section>
  )
}


