import { useMemo } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { AlertBanner } from '../components/AlertBanner'
import { fetchJSON } from '../lib/api'

type SystemStatusResponse = {
  version?: string
  commit?: string
}

type ImportStatsResponse = {
  library_root?: string
  library_root_configured?: boolean
}

type DownloadJob = { status: string }
type DownloadJobsResponse = { items: DownloadJob[] }

type ImportJobsResponse = { items: unknown[] }

type DownloadClient = {
  id: string
  enabled: boolean
  reliability_score: number
  tier: string
}
type DownloadClientsResponse = { items: DownloadClient[] }

type MetadataReliabilityProvider = {
  name: string
  score: number
  status: string
}
type MetadataReliabilityResponse = { providers: MetadataReliabilityProvider[] }

type IndexerBackend = {
  id: string
  name: string
  backend_type?: string
  enabled: boolean
  reliability_score: number
  tier: string
}
type IndexerBackendsResponse = { backends: IndexerBackend[] }
type ReadinessResponse = { status: string; ready: boolean; blocking_count: number; warning_count: number }

function StatCard({ label, value, subtitle }: { label: string; value: string | number; subtitle?: string }) {
  return (
    <div className="rounded border border-slate-800 bg-slate-900/60 p-4">
      <p className="text-xs uppercase tracking-wide text-slate-400">{label}</p>
      <p className="mt-2 text-2xl font-semibold text-slate-100">{value}</p>
      {subtitle ? <p className="mt-1 text-xs text-slate-400">{subtitle}</p> : null}
    </div>
  )
}

function ChecklistItem({ label, ok, detail, tip, to }: { label: string; ok: boolean; detail?: string; tip?: string; to?: string }) {
  const inner = (
    <div className="flex items-start justify-between gap-2 rounded border border-slate-800 bg-slate-900/40 px-3 py-2 transition-colors hover:bg-slate-800/60">
      <div>
        <p className="text-sm text-slate-100">{label}</p>
        {detail ? <p className="text-xs text-slate-400">{detail}</p> : null}
        {!ok && tip ? <p className="mt-0.5 text-xs text-sky-400/80">{tip}</p> : null}
      </div>
      <span className={ok ? 'text-xs font-medium text-emerald-400' : 'text-xs font-medium text-amber-400'}>{ok ? 'Ready' : 'Missing'}</span>
    </div>
  )
  if (to) {
    return <Link to={to}>{inner}</Link>
  }
  return inner
}

export function DashboardPage() {
  const systemStatusQuery = useQuery({
    queryKey: ['dashboard', 'system-status'],
    queryFn: () => fetchJSON<SystemStatusResponse>('/api/v1/system/status'),
    refetchInterval: 60000
  })

  const importStatsQuery = useQuery({
    queryKey: ['dashboard', 'import-stats'],
    queryFn: () => fetchJSON<ImportStatsResponse>('/api/v1/import/stats'),
    refetchInterval: 15000
  })

  const downloadJobsQuery = useQuery({
    queryKey: ['dashboard', 'download-jobs'],
    queryFn: () => fetchJSON<DownloadJobsResponse>('/api/v1/download/jobs?limit=200'),
    refetchInterval: 5000
  })

  const importNeedsReviewQuery = useQuery({
    queryKey: ['dashboard', 'import-needs-review'],
    queryFn: () => fetchJSON<ImportJobsResponse>('/api/v1/import/jobs?status=needs_review&limit=200'),
    refetchInterval: 5000
  })

  const downloadClientsQuery = useQuery({
    queryKey: ['dashboard', 'download-clients'],
    queryFn: () => fetchJSON<DownloadClientsResponse>('/api/v1/download/clients'),
    refetchInterval: 10000
  })

  const metadataReliabilityQuery = useQuery({
    queryKey: ['dashboard', 'metadata-reliability'],
    queryFn: () => fetchJSON<MetadataReliabilityResponse>('/ui-api/metadata/providers/reliability'),
    refetchInterval: 15000
  })

  const indexerBackendsQuery = useQuery({
    queryKey: ['dashboard', 'indexer-backends'],
    queryFn: () => fetchJSON<IndexerBackendsResponse>('/ui-api/indexer/backends/reliability'),
    refetchInterval: 10000
  })

  const readinessQuery = useQuery({
    queryKey: ['system', 'readiness'],
    queryFn: () => fetchJSON<ReadinessResponse>('/api/v1/system/readiness'),
    refetchInterval: 30000
  })

  const downloadsInProgress = useMemo(() => {
    const statuses = new Set(['queued', 'submitted', 'downloading', 'repairing', 'unpacking'])
    return (downloadJobsQuery.data?.items ?? []).filter((item) => statuses.has((item.status || '').toLowerCase())).length
  }, [downloadJobsQuery.data])

  const importsNeedsReview = (importNeedsReviewQuery.data?.items ?? []).length

  const enabledIndexerBackends = (indexerBackendsQuery.data?.backends ?? []).filter((b) => b.enabled)
  const enabledDownloadClients = (downloadClientsQuery.data?.items ?? []).filter((c) => c.enabled)

  const metadataHealthy = (metadataReliabilityQuery.data?.providers ?? []).filter((p) => p.status === 'healthy').length
  const metadataTotal = (metadataReliabilityQuery.data?.providers ?? []).length
  const indexerHealthy = (indexerBackendsQuery.data?.backends ?? []).filter((b) => b.enabled && b.tier !== 'quarantine').length
  const indexerTotal = enabledIndexerBackends.length
  const downloadHealthy = enabledDownloadClients.filter((c) => c.tier !== 'quarantine').length
  const downloadTotal = enabledDownloadClients.length

  const prowlarrConnected = (indexerBackendsQuery.data?.backends ?? []).some((b) => b.backend_type === 'prowlarr' && b.enabled)

  const checklistItems = [
    Boolean(importStatsQuery.data?.library_root_configured),
    enabledIndexerBackends.length > 0,
    enabledDownloadClients.length > 0,
    prowlarrConnected
  ]
  const checklistIncomplete = checklistItems.filter((ok) => !ok).length

  const loading =
    importStatsQuery.isLoading ||
    downloadJobsQuery.isLoading ||
    importNeedsReviewQuery.isLoading ||
    downloadClientsQuery.isLoading ||
    metadataReliabilityQuery.isLoading ||
    indexerBackendsQuery.isLoading

  return (
    <section className="space-y-6">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">Dashboard</h2>
        <p className="text-sm text-slate-400">Setup checklist, current activity, and subsystem health.</p>
      </header>

      {!loading && checklistIncomplete >= 2 ? (
        <div className="rounded-lg border border-sky-800/60 bg-sky-950/30 p-4">
          <h3 className="text-lg font-semibold text-sky-200">Welcome to Bookwyrm</h3>
          <p className="mt-1 text-sm text-sky-300/80">
            Complete the setup checklist below to get started. {checklistIncomplete} item{checklistIncomplete > 1 ? 's' : ''} remaining.
          </p>
          <p className="mt-2">
            <Link className="text-sm text-sky-300 underline hover:text-sky-200" to="/system/setup">
              Open full setup checklist
            </Link>
          </p>
        </div>
      ) : null}
      {readinessQuery.data && !readinessQuery.data.ready ? (
        <div className="rounded border border-amber-800/60 bg-amber-950/30 p-3 text-sm text-amber-200">
          Setup is not complete ({readinessQuery.data.blocking_count} blocking issue(s)).
          <Link className="ml-2 underline hover:text-amber-100" to="/system/setup">
            Review setup checklist
          </Link>
        </div>
      ) : null}

      {loading ? <p className="text-sm text-slate-400">Loading dashboard data...</p> : null}

      <AlertBanner />

      <div className="grid gap-4 md:grid-cols-3">
        <StatCard label="Downloads In Progress" value={downloadsInProgress} subtitle="queued + active download states" />
        <StatCard label="Imports Needs Review" value={importsNeedsReview} subtitle="operator decisions required" />
        <StatCard label="Enabled Indexers" value={enabledIndexerBackends.length} subtitle="backends available for search" />
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <div className="rounded border border-slate-800 bg-slate-900/60 p-4">
          <h3 className="text-lg font-semibold text-slate-100">Setup Checklist</h3>
          <div className="mt-3 space-y-2">
            <ChecklistItem
              label="Library root configured"
              ok={Boolean(importStatsQuery.data?.library_root_configured)}
              detail={importStatsQuery.data?.library_root || 'Set LIBRARY_ROOT in backend environment'}
              tip="Set the LIBRARY_ROOT env variable to your book directory path"
              to="/settings/media-management"
            />
            <ChecklistItem
              label="Prowlarr connected"
              ok={prowlarrConnected}
              detail={prowlarrConnected ? 'Prowlarr backend enabled' : 'No Prowlarr backend detected'}
              tip="Set PROWLARR_BASE_URL and PROWLARR_API_KEY on the indexer service"
              to="/settings/indexers"
            />
            <ChecklistItem
              label="Indexer backend enabled"
              ok={enabledIndexerBackends.length > 0}
              detail={`${enabledIndexerBackends.length} enabled backend(s)`}
              tip="Enable at least one search backend in indexer settings"
              to="/settings/indexers"
            />
            <ChecklistItem
              label="Download client enabled"
              ok={enabledDownloadClients.length > 0}
              detail={`${enabledDownloadClients.length} enabled client(s)`}
              tip="Configure SABnzbd, qBittorrent, or NZBGet in download client settings"
              to="/settings/download-clients"
            />
          </div>
        </div>

        <div className="rounded border border-slate-800 bg-slate-900/60 p-4">
          <h3 className="text-lg font-semibold text-slate-100">Health Quick View</h3>
          <div className="mt-3 space-y-2 text-sm">
            <div className="flex items-center justify-between rounded border border-slate-800 bg-slate-900/40 px-3 py-2">
              <span className="text-slate-300">Metadata providers</span>
              <span className="font-medium text-slate-100">{metadataHealthy}/{metadataTotal} healthy</span>
            </div>
            <div className="flex items-center justify-between rounded border border-slate-800 bg-slate-900/40 px-3 py-2">
              <span className="text-slate-300">Indexer backends</span>
              <span className="font-medium text-slate-100">{indexerHealthy}/{indexerTotal} healthy tier</span>
            </div>
            <div className="flex items-center justify-between rounded border border-slate-800 bg-slate-900/40 px-3 py-2">
              <span className="text-slate-300">Download clients</span>
              <span className="font-medium text-slate-100">{downloadHealthy}/{downloadTotal} healthy tier</span>
            </div>
          </div>
        </div>
      </div>

      {importStatsQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load import stats. Check backend API connectivity.</div> : null}
      {downloadJobsQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load download jobs. Check backend API connectivity.</div> : null}
      {importNeedsReviewQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load import review queue. Check backend API connectivity.</div> : null}
      {downloadClientsQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load download clients. Check backend API connectivity.</div> : null}
      {metadataReliabilityQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load metadata reliability. Verify METADATA_SERVICE_URL is correct.</div> : null}
      {indexerBackendsQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load indexer backends. Verify INDEXER_SERVICE_URL is correct.</div> : null}

      {systemStatusQuery.data?.version ? (
        <p className="text-center text-xs text-slate-500">
          Bookwyrm {systemStatusQuery.data.version}
          {systemStatusQuery.data.commit && systemStatusQuery.data.commit !== 'unknown'
            ? ` (${systemStatusQuery.data.commit.substring(0, 7)})`
            : ''}
        </p>
      ) : null}
    </section>
  )
}
