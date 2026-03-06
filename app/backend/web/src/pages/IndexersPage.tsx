import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useToast } from '../components/ToastProvider'
import { fetchJSON, postNoContent } from '../lib/api'

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

function preferred(rec: BackendRecord): boolean {
  return Boolean(rec.config_json?.preferred)
}

export function IndexersPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()

  const backendsQuery = useQuery({
    queryKey: ['settings', 'indexers', 'backends'],
    queryFn: () => fetchJSON<BackendsResponse>('/ui-api/indexer/backends/reliability'),
    refetchInterval: 10000
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
    onError: (error) => pushToast((error as Error).message)
  })

  const rows = backendsQuery.data?.backends ?? []

  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">Indexers</h2>
        <p className="text-sm text-slate-400">Manage backend state, priority, reliability, and preferred source hearting.</p>
      </header>

      <div className="rounded border border-slate-800 bg-slate-900/60 p-3 text-xs text-slate-300">
        Ordering rule: preferred (hearted) backends are searched before non-preferred backends, then tier/reliability/priority applies.
      </div>

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
                <td className="px-3 py-2">{row.enabled ? 'yes' : 'no'}</td>
                <td className="px-3 py-2">{preferred(row) ? '♥' : '-'}</td>
                <td className="px-3 py-2">{row.priority}</td>
                <td className="px-3 py-2">{row.reliability_score.toFixed(2)}</td>
                <td className="px-3 py-2">{row.tier}</td>
                <td className="px-3 py-2">
                  <div className="flex flex-wrap gap-2">
                    <button className="rounded border border-slate-700 px-2 py-1 text-xs text-slate-200" onClick={() => mutation.mutate({ type: row.enabled ? 'disable' : 'enable', id: row.id })}>
                      {row.enabled ? 'Disable' : 'Enable'}
                    </button>
                    <button className="rounded border border-amber-700 px-2 py-1 text-xs text-amber-300" onClick={() => mutation.mutate({ type: 'preferred', id: row.id, value: !preferred(row) })}>
                      {preferred(row) ? 'Unheart' : 'Heart'}
                    </button>
                    <button className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300" onClick={() => mutation.mutate({ type: 'priority', id: row.id, value: row.priority + 10 })}>
                      +Priority
                    </button>
                    <button className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300" onClick={() => mutation.mutate({ type: 'priority', id: row.id, value: Math.max(1, row.priority - 10) })}>
                      -Priority
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
