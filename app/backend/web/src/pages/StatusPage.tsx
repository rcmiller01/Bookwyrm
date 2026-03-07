import { useQuery } from '@tanstack/react-query'
import { AlertBanner } from '../components/AlertBanner'
import { fetchJSON } from '../lib/api'

type Health = { status?: string }

type SystemStatusResponse = {
  version?: string
  commit?: string
  built?: string
  go_version?: string
  startup_time?: string
  services?: Record<string, { status?: string; version?: string; commit?: string }>
  library_root?: string
  library_exists?: boolean
  download_clients?: string[]
}

type ImportStats = {
  counts_by_status?: Record<string, number>
  library_root_configured?: boolean
}

type BackendsResponse = {
  backends?: { enabled: boolean; tier: string }[]
}

type DownloadClientsResponse = {
  items?: { enabled: boolean; tier: string }[]
}

function HealthRow({ label, ok, detail }: { label: string; ok: boolean; detail: string }) {
  return (
    <div className="flex items-center justify-between rounded border border-slate-800 bg-slate-900/40 px-3 py-2 text-sm">
      <span className="text-slate-300">{label}</span>
      <span className={ok ? 'font-medium text-emerald-400' : 'font-medium text-red-300'}>
        {ok ? 'ok' : 'degraded'} ({detail})
      </span>
    </div>
  )
}

export function StatusPage() {
  const systemStatus = useQuery({
    queryKey: ['status', 'system-status'],
    queryFn: () => fetchJSON<SystemStatusResponse>('/api/v1/system/status'),
    refetchInterval: 30000
  })

  const backendHealth = useQuery({
    queryKey: ['status', 'backend-health'],
    queryFn: () => fetchJSON<Health>('/api/v1/health'),
    refetchInterval: 15000
  })

  const metadataHealth = useQuery({
    queryKey: ['status', 'metadata-health'],
    queryFn: () => fetchJSON<Health>('/ui-api/metadata/health'),
    refetchInterval: 15000
  })

  const indexerHealth = useQuery({
    queryKey: ['status', 'indexer-health'],
    queryFn: () => fetchJSON<Health>('/ui-api/indexer/health'),
    refetchInterval: 15000
  })

  const importStats = useQuery({
    queryKey: ['status', 'import-stats'],
    queryFn: () => fetchJSON<ImportStats>('/api/v1/import/stats'),
    refetchInterval: 15000
  })

  const backends = useQuery({
    queryKey: ['status', 'indexer-backends'],
    queryFn: () => fetchJSON<BackendsResponse>('/ui-api/indexer/backends/reliability'),
    refetchInterval: 15000
  })

  const clients = useQuery({
    queryKey: ['status', 'download-clients'],
    queryFn: () => fetchJSON<DownloadClientsResponse>('/api/v1/download/clients'),
    refetchInterval: 15000
  })

  const enabledBackends = (backends.data?.backends ?? []).filter((b) => b.enabled)
  const healthyBackends = enabledBackends.filter((b) => b.tier !== 'quarantine')
  const enabledClients = (clients.data?.items ?? []).filter((c) => c.enabled)
  const healthyClients = enabledClients.filter((c) => c.tier !== 'quarantine')

  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">Status</h2>
        <p className="text-sm text-slate-400">Service health and runtime readiness.</p>
      </header>

      <AlertBanner />

      {systemStatus.data?.version ? (
        <div className="rounded border border-slate-800 bg-slate-900/60 p-4">
          <h3 className="text-lg font-semibold text-slate-100">Version Info</h3>
          <div className="mt-3 grid gap-2 md:grid-cols-3 text-sm">
            <div className="rounded border border-slate-800 bg-slate-900/40 px-3 py-2">
              <p className="text-slate-400">Backend</p>
              <p className="text-slate-100 font-medium">
                {systemStatus.data.version}
                {systemStatus.data.commit && systemStatus.data.commit !== 'unknown' ? (
                  <span className="ml-2 text-xs text-slate-400">({systemStatus.data.commit.substring(0, 7)})</span>
                ) : null}
              </p>
            </div>
            {systemStatus.data.services?.metadata_service?.version ? (
              <div className="rounded border border-slate-800 bg-slate-900/40 px-3 py-2">
                <p className="text-slate-400">Metadata Service</p>
                <p className="text-slate-100 font-medium">
                  {systemStatus.data.services.metadata_service.version}
                  {systemStatus.data.services.metadata_service.commit ? (
                    <span className="ml-2 text-xs text-slate-400">({String(systemStatus.data.services.metadata_service.commit).substring(0, 7)})</span>
                  ) : null}
                </p>
              </div>
            ) : null}
            {systemStatus.data.services?.indexer_service?.version ? (
              <div className="rounded border border-slate-800 bg-slate-900/40 px-3 py-2">
                <p className="text-slate-400">Indexer Service</p>
                <p className="text-slate-100 font-medium">
                  {systemStatus.data.services.indexer_service.version}
                  {systemStatus.data.services.indexer_service.commit ? (
                    <span className="ml-2 text-xs text-slate-400">({String(systemStatus.data.services.indexer_service.commit).substring(0, 7)})</span>
                  ) : null}
                </p>
              </div>
            ) : null}
          </div>
        </div>
      ) : null}

      <div className="rounded border border-slate-800 bg-slate-900/60 p-4 space-y-2">
        <HealthRow label="Backend API" ok={backendHealth.data?.status === 'ok'} detail={backendHealth.data?.status || 'unknown'} />
        <HealthRow label="Metadata service" ok={metadataHealth.data?.status === 'ok'} detail={metadataHealth.data?.status || 'unknown'} />
        <HealthRow label="Indexer service" ok={indexerHealth.data?.status === 'ok'} detail={indexerHealth.data?.status || 'unknown'} />
        <HealthRow
          label="Library root configured"
          ok={Boolean(importStats.data?.library_root_configured)}
          detail={importStats.data?.library_root_configured ? 'configured' : 'missing'}
        />
        <HealthRow
          label="Indexer backends ready"
          ok={enabledBackends.length > 0 && healthyBackends.length > 0}
          detail={`${healthyBackends.length}/${enabledBackends.length} healthy tier`}
        />
        <HealthRow
          label="Download clients ready"
          ok={enabledClients.length > 0 && healthyClients.length > 0}
          detail={`${healthyClients.length}/${enabledClients.length} healthy tier`}
        />
      </div>

      <div className="rounded border border-slate-800 bg-slate-900/50 p-4">
        <h3 className="text-lg font-semibold text-slate-100">Import Queue Snapshot</h3>
        <div className="mt-3 grid gap-2 md:grid-cols-3">
          {Object.entries(importStats.data?.counts_by_status ?? {}).map(([status, count]) => (
            <div key={status} className="rounded border border-slate-800 bg-slate-900/40 px-3 py-2 text-sm">
              <p className="text-slate-300">{status}</p>
              <p className="text-lg font-semibold text-slate-100">{count}</p>
            </div>
          ))}
        </div>
      </div>

      {backendHealth.isError || metadataHealth.isError || indexerHealth.isError || importStats.isError || backends.isError || clients.isError ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">
          One or more status checks failed.
        </div>
      ) : null}
    </section>
  )
}
