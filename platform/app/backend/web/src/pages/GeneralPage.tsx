import { useQuery } from '@tanstack/react-query'
import { fetchJSON } from '../lib/api'

type HealthResponse = { status?: string }

type EnrichmentStatsResponse = {
  enabled?: boolean
  worker_count?: number
  queue_depth?: Record<string, number>
  next_runnable_at?: string | null
}

function HealthBadge({ label, ok }: { label: string; ok: boolean }) {
  return (
    <div className="flex items-center justify-between rounded border border-slate-800 bg-slate-900/40 px-3 py-2 text-sm">
      <span className="text-slate-300">{label}</span>
      <span className={ok ? 'font-medium text-emerald-400' : 'font-medium text-red-300'}>{ok ? 'ok' : 'degraded'}</span>
    </div>
  )
}

export function GeneralPage() {
  const backendHealthQuery = useQuery({
    queryKey: ['settings', 'general', 'backend-health'],
    queryFn: () => fetchJSON<HealthResponse>('/api/v1/health'),
    refetchInterval: 15000
  })

  const metadataHealthQuery = useQuery({
    queryKey: ['settings', 'general', 'metadata-health'],
    queryFn: () => fetchJSON<HealthResponse>('/ui-api/metadata/health'),
    refetchInterval: 15000
  })

  const indexerHealthQuery = useQuery({
    queryKey: ['settings', 'general', 'indexer-health'],
    queryFn: () => fetchJSON<HealthResponse>('/ui-api/indexer/health'),
    refetchInterval: 15000
  })

  const enrichmentStatsQuery = useQuery({
    queryKey: ['settings', 'general', 'enrichment-stats'],
    queryFn: () => fetchJSON<EnrichmentStatsResponse>('/ui-api/metadata/enrichment/stats'),
    refetchInterval: 15000
  })

  const queueDepth = enrichmentStatsQuery.data?.queue_depth ?? {}
  const totalQueueDepth = Object.values(queueDepth).reduce((sum, value) => sum + value, 0)

  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">General</h2>
        <p className="text-sm text-slate-400">Runtime health and task processing overview.</p>
      </header>

      <div className="rounded border border-slate-800 bg-slate-900/50 p-4 space-y-2">
        <HealthBadge label="Backend API" ok={backendHealthQuery.data?.status === 'ok'} />
        <HealthBadge label="Metadata service" ok={metadataHealthQuery.data?.status === 'ok'} />
        <HealthBadge label="Indexer service" ok={indexerHealthQuery.data?.status === 'ok'} />
      </div>

      <div className="rounded border border-slate-800 bg-slate-900/50 p-4">
        <h3 className="text-lg font-semibold text-slate-100">Enrichment Engine</h3>
        <div className="mt-3 grid gap-2 text-sm">
          <div className="flex items-center justify-between rounded border border-slate-800 bg-slate-900/40 px-3 py-2">
            <span className="text-slate-300">Enabled</span>
            <span className="font-medium text-slate-100">{enrichmentStatsQuery.data?.enabled ? 'yes' : 'no'}</span>
          </div>
          <div className="flex items-center justify-between rounded border border-slate-800 bg-slate-900/40 px-3 py-2">
            <span className="text-slate-300">Worker count</span>
            <span className="font-medium text-slate-100">{enrichmentStatsQuery.data?.worker_count ?? 0}</span>
          </div>
          <div className="flex items-center justify-between rounded border border-slate-800 bg-slate-900/40 px-3 py-2">
            <span className="text-slate-300">Queue depth</span>
            <span className="font-medium text-slate-100">{totalQueueDepth}</span>
          </div>
          <div className="flex items-center justify-between rounded border border-slate-800 bg-slate-900/40 px-3 py-2">
            <span className="text-slate-300">Next runnable</span>
            <span className="font-medium text-slate-100">
              {enrichmentStatsQuery.data?.next_runnable_at
                ? new Date(enrichmentStatsQuery.data.next_runnable_at).toLocaleString()
                : 'none scheduled'}
            </span>
          </div>
        </div>
      </div>

      {(backendHealthQuery.isError || metadataHealthQuery.isError || indexerHealthQuery.isError || enrichmentStatsQuery.isError) ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">
          One or more general status sources failed to load.
        </div>
      ) : null}
    </section>
  )
}
