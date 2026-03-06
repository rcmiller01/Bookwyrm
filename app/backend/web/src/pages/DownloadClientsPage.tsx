import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useToast } from '../components/ToastProvider'
import { fetchJSON, patchJSON } from '../lib/api'

type DownloadClient = {
  id: string
  name: string
  client_type: string
  enabled: boolean
  tier: string
  reliability_score: number
  priority: number
}

type DownloadClientsResponse = { items: DownloadClient[] }

export function DownloadClientsPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()

  const clientsQuery = useQuery({
    queryKey: ['settings', 'download-clients'],
    queryFn: () => fetchJSON<DownloadClientsResponse>('/api/v1/download/clients'),
    refetchInterval: 10000
  })

  const updateMutation = useMutation({
    mutationFn: (payload: { id: string; enabled?: boolean; priority?: number }) =>
      patchJSON<DownloadClient>(`/api/v1/download/clients/${encodeURIComponent(payload.id)}`, {
        ...(payload.enabled !== undefined ? { enabled: payload.enabled } : {}),
        ...(payload.priority !== undefined ? { priority: payload.priority } : {})
      }),
    onSuccess: async () => {
      pushToast('Download client updated')
      await queryClient.invalidateQueries({ queryKey: ['settings', 'download-clients'] })
      await queryClient.invalidateQueries({ queryKey: ['dashboard', 'download-clients'] })
    },
    onError: (error) => pushToast((error as Error).message)
  })

  const rows = clientsQuery.data?.items ?? []

  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">Download Clients</h2>
        <p className="text-sm text-slate-400">Enable/disable clients and adjust scheduling priority.</p>
      </header>

      <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
        <table className="w-full text-left text-sm">
          <thead className="bg-slate-900 text-slate-300">
            <tr>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Type</th>
              <th className="px-3 py-2">Enabled</th>
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
                <td className="px-3 py-2">{row.client_type}</td>
                <td className="px-3 py-2">{row.enabled ? 'yes' : 'no'}</td>
                <td className="px-3 py-2">{row.priority}</td>
                <td className="px-3 py-2">{row.reliability_score.toFixed(2)}</td>
                <td className="px-3 py-2">{row.tier}</td>
                <td className="px-3 py-2">
                  <div className="flex flex-wrap gap-2">
                    <button
                      className="rounded border border-slate-700 px-2 py-1 text-xs text-slate-200"
                      onClick={() => updateMutation.mutate({ id: row.id, enabled: !row.enabled })}
                    >
                      {row.enabled ? 'Disable' : 'Enable'}
                    </button>
                    <button
                      className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300"
                      onClick={() => updateMutation.mutate({ id: row.id, priority: Math.max(1, row.priority - 10) })}
                    >
                      -Priority
                    </button>
                    <button
                      className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300"
                      onClick={() => updateMutation.mutate({ id: row.id, priority: row.priority + 10 })}
                    >
                      +Priority
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {rows.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-3 py-6 text-center text-slate-400">
                  No download clients found.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      {clientsQuery.isLoading ? <p className="text-sm text-slate-400">Loading download clients...</p> : null}
      {clientsQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load download clients.</div> : null}
    </section>
  )
}
