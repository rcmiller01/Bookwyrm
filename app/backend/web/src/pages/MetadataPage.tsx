import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useToast } from '../components/ToastProvider'
import { fetchJSON, postNoContent } from '../lib/api'

type ProviderInfo = {
  name: string
  enabled: boolean
  priority: number
  timeout_sec: number
  rate_limit: number
  status: string
  failure_count: number
  avg_latency_ms: number
}

type ProvidersResponse = { providers: ProviderInfo[] }

export function MetadataPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()

  const providersQuery = useQuery({
    queryKey: ['settings', 'metadata', 'providers'],
    queryFn: () => fetchJSON<ProvidersResponse>('/ui-api/metadata/providers'),
    refetchInterval: 10000
  })

  const updateMutation = useMutation({
    mutationFn: async (action: { name: string; enabled?: boolean; priority?: number }) => {
      await postNoContent(`/ui-api/metadata/providers/${encodeURIComponent(action.name)}`, {
        ...(action.enabled !== undefined ? { enabled: action.enabled } : {}),
        ...(action.priority !== undefined ? { priority: action.priority } : {})
      })
    },
    onSuccess: async () => {
      pushToast('Metadata provider updated')
      await queryClient.invalidateQueries({ queryKey: ['settings', 'metadata', 'providers'] })
      await queryClient.invalidateQueries({ queryKey: ['dashboard', 'metadata-reliability'] })
    },
    onError: (error) => pushToast((error as Error).message)
  })

  const rows = providersQuery.data?.providers ?? []

  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">Metadata</h2>
        <p className="text-sm text-slate-400">Manage provider enablement and priority for enrichment and metadata fetches.</p>
      </header>

      <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
        <table className="w-full text-left text-sm">
          <thead className="bg-slate-900 text-slate-300">
            <tr>
              <th className="px-3 py-2">Provider</th>
              <th className="px-3 py-2">Enabled</th>
              <th className="px-3 py-2">Priority</th>
              <th className="px-3 py-2">Rate Limit</th>
              <th className="px-3 py-2">Timeout</th>
              <th className="px-3 py-2">Status</th>
              <th className="px-3 py-2">Failures</th>
              <th className="px-3 py-2">Latency</th>
              <th className="px-3 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.name} className="border-t border-slate-800 text-slate-100">
                <td className="px-3 py-2">{row.name}</td>
                <td className="px-3 py-2">{row.enabled ? 'yes' : 'no'}</td>
                <td className="px-3 py-2">{row.priority}</td>
                <td className="px-3 py-2">{row.rate_limit}</td>
                <td className="px-3 py-2">{row.timeout_sec}s</td>
                <td className="px-3 py-2">{row.status}</td>
                <td className="px-3 py-2">{row.failure_count}</td>
                <td className="px-3 py-2">{row.avg_latency_ms}ms</td>
                <td className="px-3 py-2">
                  <div className="flex flex-wrap gap-2">
                    <button
                      className="rounded border border-slate-700 px-2 py-1 text-xs text-slate-200"
                      onClick={() => updateMutation.mutate({ name: row.name, enabled: !row.enabled })}
                    >
                      {row.enabled ? 'Disable' : 'Enable'}
                    </button>
                    <button
                      className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300"
                      onClick={() => updateMutation.mutate({ name: row.name, priority: Math.max(1, row.priority - 1) })}
                    >
                      Higher
                    </button>
                    <button
                      className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300"
                      onClick={() => updateMutation.mutate({ name: row.name, priority: row.priority + 1 })}
                    >
                      Lower
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {rows.length === 0 ? (
              <tr>
                <td colSpan={9} className="px-3 py-6 text-center text-slate-400">
                  No metadata providers found.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      {providersQuery.isLoading ? <p className="text-sm text-slate-400">Loading metadata providers...</p> : null}
      {providersQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load metadata providers.</div> : null}
    </section>
  )
}
