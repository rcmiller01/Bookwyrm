import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertBanner } from '../components/AlertBanner'
import { useToast } from '../components/ToastProvider'
import { errorMessage } from '../lib/errorMessage'
import { fetchJSON, postJSON } from '../lib/api'

type Health = { status?: string }

type SystemStatusResponse = {
  version?: string
  commit?: string
  built?: string
  go_version?: string
  startup_time?: string
  services?: Record<string, { status?: string; version?: string; commit?: string }>
  dependency_summary?: {
    status?: string
    can_function_now?: boolean
    blocking_count?: number
    warning_count?: number
  }
  migration_status?: {
    status?: string
    ready?: boolean
    detail?: string
    pending_count?: number
  }
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

type ActionResponse = {
  status?: string
  queued?: number
  retried?: number
  failed?: number
}

type LogsLocationResponse = {
  log_dir: string
  exists: boolean
  file_uri: string
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
  const upgradeNotesURL = 'https://github.com/rcmiller01/Bookwyrm/blob/main/docs/upgrading.md'
  const queryClient = useQueryClient()
  const { pushToast } = useToast()

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
  const logsLocation = useQuery({
    queryKey: ['status', 'logs-location'],
    queryFn: () => fetchJSON<LogsLocationResponse>('/api/v1/system/logs-location'),
    refetchInterval: false
  })

  const enabledBackends = (backends.data?.backends ?? []).filter((b) => b.enabled)
  const healthyBackends = enabledBackends.filter((b) => b.tier !== 'quarantine')
  const enabledClients = (clients.data?.items ?? []).filter((c) => c.enabled)
  const healthyClients = enabledClients.filter((c) => c.tier !== 'quarantine')
  const migration = systemStatus.data?.migration_status
  const migrationPending = (migration?.status || '').toLowerCase() === 'pending'
  const migrationFailed = (migration?.status || '').toLowerCase() === 'failed'
  const dependencies = systemStatus.data?.dependency_summary
  const degradedMode = dependencies?.can_function_now === false

  const runAction = useMutation({
    mutationFn: async (path: string) => postJSON<ActionResponse>(path, {}),
    onSuccess: async (data) => {
      const summary = [data.retried ? `${data.retried} retried` : '', data.queued ? `${data.queued} queued` : '']
        .filter(Boolean)
        .join(', ')
      pushToast(summary ? `Action completed (${summary})` : 'Action completed')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['status'] }),
        queryClient.invalidateQueries({ queryKey: ['system', 'tasks'] }),
        queryClient.invalidateQueries({ queryKey: ['dashboard'] })
      ])
    },
    onError: (err) => pushToast(errorMessage(err))
  })

  async function downloadSupportBundle() {
    try {
      const response = await fetch('/api/v1/system/support-bundle', { method: 'GET' })
      if (!response.ok) {
        throw new Error(`Support bundle failed (${response.status})`)
      }
      const blob = await response.blob()
      const disposition = response.headers.get('Content-Disposition') || ''
      const filenameMatch = disposition.match(/filename="?([^"]+)"?/)
      const filename = filenameMatch?.[1] || `bookwyrm-support-${Date.now()}.zip`
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
      pushToast('Support bundle downloaded')
    } catch (err) {
      pushToast(errorMessage(err))
    }
  }

  function openLogsFolder() {
    const uri = logsLocation.data?.file_uri
    if (uri) {
      window.open(uri, '_blank', 'noopener,noreferrer')
      pushToast(`Logs folder: ${logsLocation.data?.log_dir}`)
      return
    }
    pushToast('Logs path unavailable')
  }

  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">Status</h2>
        <p className="text-sm text-slate-400">Service health and runtime readiness.</p>
      </header>

      <AlertBanner />

      {degradedMode ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">
          Degraded mode: {dependencies?.blocking_count ?? 0} blocking dependency issue(s) detected. Resolve setup items in System Setup before normal operation.
        </div>
      ) : null}

      {migrationPending || migrationFailed ? (
        <div className="rounded border border-amber-800/60 bg-amber-950/40 p-3 text-sm text-amber-200">
          <p className="font-medium">{migrationFailed ? 'Migration check failed' : 'Pending migrations detected'}</p>
          <p className="mt-1">{migration?.detail || 'Migration state requires attention before continuing upgrades.'}</p>
        </div>
      ) : null}

      <div className="rounded border border-slate-800 bg-slate-900/60 p-4">
        <h3 className="text-lg font-semibold text-slate-100">Support & Recovery</h3>
        <p className="mt-1 text-sm text-slate-400">Download diagnostics and run safe one-click remediation actions.</p>
        <p className="mt-1 text-xs text-amber-300/90">Backup reminder: take a DB backup before upgrade remediation or cleanup operations.</p>
        <div className="mt-3 flex flex-wrap gap-2">
          <button
            className="rounded border border-sky-700 px-3 py-1.5 text-xs text-sky-300 hover:bg-sky-900/20"
            onClick={downloadSupportBundle}
          >
            Download Support Bundle
          </button>
          <button
            className="rounded border border-sky-700 px-3 py-1.5 text-xs text-sky-300 hover:bg-sky-900/20"
            onClick={openLogsFolder}
          >
            Open Logs Folder
          </button>
          <button
            className="rounded border border-emerald-700 px-3 py-1.5 text-xs text-emerald-300 hover:bg-emerald-900/20 disabled:opacity-60"
            disabled={runAction.isPending}
            onClick={() => runAction.mutate('/api/v1/system/actions/retry-failed-downloads')}
          >
            Retry Failed Downloads
          </button>
          <button
            className="rounded border border-emerald-700 px-3 py-1.5 text-xs text-emerald-300 hover:bg-emerald-900/20 disabled:opacity-60"
            disabled={runAction.isPending}
            onClick={() => runAction.mutate('/api/v1/system/actions/retry-failed-imports')}
          >
            Retry Failed Imports
          </button>
          <button
            className="rounded border border-amber-700 px-3 py-1.5 text-xs text-amber-300 hover:bg-amber-900/20 disabled:opacity-60"
            disabled={runAction.isPending}
            onClick={() => runAction.mutate('/api/v1/system/actions/test-connections')}
          >
            Test Connections
          </button>
          <button
            className="rounded border border-amber-700 px-3 py-1.5 text-xs text-amber-300 hover:bg-amber-900/20 disabled:opacity-60"
            disabled={runAction.isPending}
            onClick={() => runAction.mutate('/api/v1/system/actions/run-cleanup')}
          >
            Run Cleanup
          </button>
          <button
            className="rounded border border-amber-700 px-3 py-1.5 text-xs text-amber-300 hover:bg-amber-900/20 disabled:opacity-60"
            disabled={runAction.isPending}
            onClick={() => runAction.mutate('/api/v1/system/actions/recompute-reliability')}
          >
            Recompute Reliability
          </button>
          <button
            className="rounded border border-violet-700 px-3 py-1.5 text-xs text-violet-300 hover:bg-violet-900/20 disabled:opacity-60"
            disabled={runAction.isPending}
            onClick={() => runAction.mutate('/api/v1/system/actions/rerun-wanted-searches')}
          >
            Rerun Wanted Searches
          </button>
          <button
            className="rounded border border-violet-700 px-3 py-1.5 text-xs text-violet-300 hover:bg-violet-900/20 disabled:opacity-60"
            disabled={runAction.isPending}
            onClick={() => runAction.mutate('/api/v1/system/actions/rerun-enrichment')}
          >
            Rerun Enrichment
          </button>
        </div>
        {logsLocation.data ? (
          <p className="mt-2 text-xs text-slate-400">Logs path: {logsLocation.data.log_dir}</p>
        ) : null}
      </div>

      {systemStatus.data?.version ? (
        <div className="rounded border border-slate-800 bg-slate-900/60 p-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h3 className="text-lg font-semibold text-slate-100">Version Info</h3>
            <a className="text-xs text-sky-300 underline hover:text-sky-200" href={upgradeNotesURL} rel="noreferrer" target="_blank">
              Upgrade notes
            </a>
          </div>
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
