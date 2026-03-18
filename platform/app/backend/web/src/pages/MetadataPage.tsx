import { useMemo } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { FilterBar } from '../components/FilterBar'
import { PageHeader } from '../components/PageHeader'
import { StatusBadge } from '../components/StatusBadge'
import { useToast } from '../components/ToastProvider'
import { useLocalStorageState } from '../hooks/useLocalStorageState'
import { fetchJSON, postJSON, postNoContent } from '../lib/api'
import { errorMessage } from '../lib/errorMessage'

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
type ConnectionTestResponse = { status?: string }

function tierFromReliability(provider: ProviderInfo): 'gold' | 'silver' | 'bronze' {
  if (provider.failure_count === 0 && provider.avg_latency_ms <= 500) return 'gold'
  if (provider.failure_count <= 2 && provider.avg_latency_ms <= 1200) return 'silver'
  return 'bronze'
}

export function MetadataPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()
  const [query, setQuery] = useLocalStorageState<string>('settings.metadata.query', '')

  const providersQuery = useQuery({
    queryKey: ['settings', 'metadata', 'providers'],
    queryFn: () => fetchJSON<ProvidersResponse>('/ui-api/metadata/providers'),
    refetchInterval: 15000
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
    onError: (error) => pushToast(errorMessage(error))
  })

  const rows = useMemo(() => {
    const lowered = query.trim().toLowerCase()
    return (providersQuery.data?.providers ?? []).filter((row) => {
      if (!lowered) return true
      return row.name.toLowerCase().includes(lowered) || row.status.toLowerCase().includes(lowered)
    })
  }, [providersQuery.data?.providers, query])
  const enabledProviders = (providersQuery.data?.providers ?? []).filter((provider) => provider.enabled)

  const testConnections = useMutation({
    mutationFn: () => postJSON<ConnectionTestResponse>('/api/v1/system/actions/test-connections', {}),
    onSuccess: () => pushToast('Connection tests completed'),
    onError: (error) => pushToast(errorMessage(error))
  })

  return (
    <section className="space-y-4">
      <PageHeader
        title="Metadata Providers"
        subtitle="Provider reliability, tiering, and routing hints for metadata enrichment."
        actions={
          <div className="flex gap-2">
            <button className="rounded border border-amber-700 px-3 py-1.5 text-sm text-amber-300" disabled={testConnections.isPending} onClick={() => testConnections.mutate()}>
              {testConnections.isPending ? 'Testing...' : 'Test Connections'}
            </button>
            <button className="rounded border border-slate-700 px-3 py-1.5 text-sm text-slate-200" onClick={() => void providersQuery.refetch()}>
              Refresh
            </button>
          </div>
        }
      />

      {enabledProviders.length === 0 ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">
          No metadata providers are enabled. Enable at least one provider to fetch book metadata and recommendations.
        </div>
      ) : null}

      <FilterBar>
        <input
          className="w-64 rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100"
          placeholder="Filter providers"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
      </FilterBar>

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        {rows.map((row) => {
          const tier = tierFromReliability(row)
          return (
            <article key={row.name} className="rounded border border-slate-800 bg-slate-900/50 p-3">
              <div className="flex items-start justify-between gap-3">
                <h3 className="text-sm font-semibold text-slate-100">{row.name}</h3>
                {row.enabled ? <StatusBadge label="Enabled" /> : <StatusBadge label="Disabled" />}
              </div>
              <div className="mt-2 space-y-1 text-xs text-slate-300">
                <p>Tier: {tier}</p>
                <p>Reliability: {row.failure_count} failures, {row.avg_latency_ms}ms avg latency</p>
                <p>Priority: {row.priority}</p>
                <p>Rate limit: {row.rate_limit}/min</p>
                <p>Timeout: {row.timeout_sec}s</p>
                <p title="Provider chosen by reliability, latency, and configured priority.">Why this provider: weighted by tier, health, and priority.</p>
              </div>
              <div className="mt-3 flex flex-wrap gap-2">
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
            </article>
          )
        })}
      </div>

      {rows.length === 0 ? (
        <div className="rounded border border-slate-800 bg-slate-900/50 p-6 text-center text-slate-400">No metadata providers found.</div>
      ) : null}

      {providersQuery.isLoading ? <p className="text-sm text-slate-400">Loading metadata providers...</p> : null}
      {providersQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load metadata providers.</div> : null}
    </section>
  )
}
