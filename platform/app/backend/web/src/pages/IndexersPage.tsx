import { useMemo } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { FilterBar } from '../components/FilterBar'
import { PageHeader } from '../components/PageHeader'
import { StatusBadge } from '../components/StatusBadge'
import { useToast } from '../components/ToastProvider'
import { useLocalStorageState } from '../hooks/useLocalStorageState'
import { fetchJSON, postJSON, postNoContent } from '../lib/api'
import { errorMessage } from '../lib/errorMessage'

type BackendRecord = {
  id: string
  name: string
  enabled: boolean
  priority: number
  reliability_score: number
  tier: string
  config_json?: Record<string, unknown>
}

type BackendsResponse = { backends: BackendRecord[] }
type ConnectionTestResponse = { status?: string }

function preferred(rec: BackendRecord): boolean {
  return Boolean(rec.config_json?.preferred)
}

export function IndexersPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()
  const [query, setQuery] = useLocalStorageState<string>('settings.indexers.query', '')
  const [stageMinCandidates, setStageMinCandidates] = useLocalStorageState<number>('settings.indexers.stage.minCandidates', 10)
  const [stageScoreThreshold, setStageScoreThreshold] = useLocalStorageState<number>('settings.indexers.stage.score', 0.75)
  const [stageTimeoutSec, setStageTimeoutSec] = useLocalStorageState<number>('settings.indexers.stage.timeout', 45)

  const backendsQuery = useQuery({
    queryKey: ['settings', 'indexers', 'backends'],
    queryFn: () => fetchJSON<BackendsResponse>('/ui-api/indexer/backends/reliability'),
    refetchInterval: 15000
  })

  const mutation = useMutation({
    mutationFn: async (action: { type: 'enable' | 'disable' | 'preferred' | 'priority'; id: string; value?: number | boolean }) => {
      switch (action.type) {
        case 'enable':
          await postNoContent(`/ui-api/indexer/backends/${encodeURIComponent(action.id)}/enable`)
          return
        case 'disable':
          await postNoContent(`/ui-api/indexer/backends/${encodeURIComponent(action.id)}/disable`)
          return
        case 'preferred':
          await postNoContent(`/ui-api/indexer/backends/${encodeURIComponent(action.id)}/preferred`, { preferred: Boolean(action.value) })
          return
        case 'priority':
          await postNoContent(`/ui-api/indexer/backends/${encodeURIComponent(action.id)}/priority`, { priority: Number(action.value) || 100 })
          return
      }
    },
    onSuccess: async () => {
      pushToast('Indexer backend updated')
      await queryClient.invalidateQueries({ queryKey: ['settings', 'indexers', 'backends'] })
      await queryClient.invalidateQueries({ queryKey: ['indexers', 'backends'] })
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const rows = useMemo(() => {
    const lowered = query.trim().toLowerCase()
    return (backendsQuery.data?.backends ?? []).filter((row) => {
      if (!lowered) return true
      return row.name.toLowerCase().includes(lowered) || row.id.toLowerCase().includes(lowered)
    })
  }, [backendsQuery.data?.backends, query])
  const enabledBackends = (backendsQuery.data?.backends ?? []).filter((b) => b.enabled)

  const testConnections = useMutation({
    mutationFn: () => postJSON<ConnectionTestResponse>('/api/v1/system/actions/test-connections', {}),
    onSuccess: () => pushToast('Connection tests completed'),
    onError: (error) => pushToast(errorMessage(error))
  })

  return (
    <section className="space-y-4">
      <PageHeader
        title="Indexers"
        subtitle="Preferred sources, backend reliability, and staged search controls."
        actions={
          <div className="flex gap-2">
            <button className="rounded border border-amber-700 px-3 py-1.5 text-sm text-amber-300" disabled={testConnections.isPending} onClick={() => testConnections.mutate()}>
              {testConnections.isPending ? 'Testing...' : 'Test Connections'}
            </button>
            <button className="rounded border border-slate-700 px-3 py-1.5 text-sm text-slate-200" onClick={() => void backendsQuery.refetch()}>
              Refresh
            </button>
          </div>
        }
      />

      {enabledBackends.length === 0 ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">
          No search backends are enabled. Enable at least one backend to make wanted/search workflows operational.
        </div>
      ) : null}

      <div className="rounded border border-slate-800 bg-slate-900/60 p-3 text-xs text-slate-300">
        Ordering rule: preferred backends run first, then tier/reliability/priority.
      </div>

      <div className="rounded border border-slate-800 bg-slate-900/60 p-4">
        <h3 className="text-sm font-semibold uppercase tracking-wide text-slate-300">Staged Search Controls</h3>
        <div className="mt-3 grid gap-3 md:grid-cols-3">
          <label className="text-sm text-slate-300">
            Min candidates
            <input type="number" min={1} className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={stageMinCandidates} onChange={(e) => setStageMinCandidates(Math.max(1, Number(e.target.value) || 1))} />
          </label>
          <label className="text-sm text-slate-300">
            Score threshold
            <input type="number" min={0} max={1} step="0.01" className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={stageScoreThreshold} onChange={(e) => setStageScoreThreshold(Math.min(1, Math.max(0, Number(e.target.value) || 0)))} />
          </label>
          <label className="text-sm text-slate-300">
            Stage timeout (sec)
            <input type="number" min={5} className="mt-1 w-full rounded border border-slate-700 bg-slate-900 px-2 py-1 text-slate-100" value={stageTimeoutSec} onChange={(e) => setStageTimeoutSec(Math.max(5, Number(e.target.value) || 5))} />
          </label>
        </div>
        <p className="mt-2 text-xs text-slate-400">Example behavior: stop after {stageMinCandidates} results or score {'>='} {stageScoreThreshold.toFixed(2)} within {stageTimeoutSec}s.</p>
      </div>

      <FilterBar>
        <input
          className="w-64 rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100"
          placeholder="Filter backends"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
      </FilterBar>

      <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
        <table className="w-full text-left text-sm">
          <thead className="bg-slate-900 text-slate-300">
            <tr>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Enabled</th>
              <th className="px-3 py-2">Preferred</th>
              <th className="px-3 py-2">Priority</th>
              <th className="px-3 py-2">Reliability</th>
              <th className="px-3 py-2">Tier</th>
              <th className="px-3 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.id} className="border-t border-slate-800 text-slate-100">
                <td className="px-3 py-2">{row.name}</td>
                <td className="px-3 py-2">{row.enabled ? <StatusBadge label="Enabled" /> : <StatusBadge label="Disabled" />}</td>
                <td className="px-3 py-2">{preferred(row) ? 'Yes' : 'No'}</td>
                <td className="px-3 py-2">{row.priority}</td>
                <td className="px-3 py-2">{row.reliability_score.toFixed(2)}</td>
                <td className="px-3 py-2">{row.tier}</td>
                <td className="px-3 py-2">
                  <div className="flex flex-wrap gap-2">
                    <button className="rounded border border-slate-700 px-2 py-1 text-xs text-slate-200" onClick={() => mutation.mutate({ type: row.enabled ? 'disable' : 'enable', id: row.id })}>
                      {row.enabled ? 'Disable' : 'Enable'}
                    </button>
                    <button className="rounded border border-amber-700 px-2 py-1 text-xs text-amber-300" onClick={() => mutation.mutate({ type: 'preferred', id: row.id, value: !preferred(row) })}>
                      {preferred(row) ? 'Unfavorite' : 'Favorite'}
                    </button>
                    <button className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300" onClick={() => mutation.mutate({ type: 'priority', id: row.id, value: Math.max(1, row.priority - 10) })}>
                      Higher
                    </button>
                    <button className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300" onClick={() => mutation.mutate({ type: 'priority', id: row.id, value: row.priority + 10 })}>
                      Lower
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {rows.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-3 py-6 text-center text-slate-400">
                  No indexer backends found.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </section>
  )
}
