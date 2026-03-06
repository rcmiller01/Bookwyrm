import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { fetchJSON } from '../lib/api'

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
  enabled: boolean
  reliability_score: number
  tier: string
}
type IndexerBackendsResponse = { backends: IndexerBackend[] }

function StatCard({ label, value, subtitle }: { label: string; value: string | number; subtitle?: string }) {
  return (
    <div className="rounded border border-slate-800 bg-slate-900/60 p-4">
      <p className="text-xs uppercase tracking-wide text-slate-400">{label}</p>
      <p className="mt-2 text-2xl font-semibold text-slate-100">{value}</p>
      {subtitle ? <p className="mt-1 text-xs text-slate-400">{subtitle}</p> : null}
    </div>
  )
}

function ChecklistItem({ label, ok, detail }: { label: string; ok: boolean; detail?: string }) {
  return (
    <div className="flex items-start justify-between gap-2 rounded border border-slate-800 bg-slate-900/40 px-3 py-2">
      <div>
        <p className="text-sm text-slate-100">{label}</p>
        {detail ? <p className="text-xs text-slate-400">{detail}</p> : null}
      </div>
      <span className={ok ? 'text-xs font-medium text-emerald-400' : 'text-xs font-medium text-amber-400'}>{ok ? 'Ready' : 'Missing'}</span>
    </div>
  )
}

export function DashboardPage() {
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

      {loading ? <p className="text-sm text-slate-400">Loading dashboard data...</p> : null}

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
            />
            <ChecklistItem
              label="Indexer backend enabled"
              ok={enabledIndexerBackends.length > 0}
              detail={`${enabledIndexerBackends.length} enabled backend(s)`}
            />
            <ChecklistItem
              label="Download client enabled"
              ok={enabledDownloadClients.length > 0}
              detail={`${enabledDownloadClients.length} enabled client(s)`}
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

      {importStatsQuery.isError || downloadJobsQuery.isError || importNeedsReviewQuery.isError || downloadClientsQuery.isError || metadataReliabilityQuery.isError || indexerBackendsQuery.isError ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">
          One or more dashboard sources failed to load. Verify backend, metadata, and indexer services are reachable.
        </div>
      ) : null}
    </section>
  )
}
