import { useQuery } from '@tanstack/react-query'
import { fetchJSON } from '../lib/api'

type ImportStatsResponse = {
  keep_incoming?: boolean
  keep_incoming_source?: string
  library_root?: string
  library_root_configured?: boolean
  next_runnable_at?: string | null
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between rounded border border-slate-800 bg-slate-900/40 px-3 py-2 text-sm">
      <span className="text-slate-300">{label}</span>
      <span className="font-medium text-slate-100">{value}</span>
    </div>
  )
}

export function MediaManagementPage() {
  const statsQuery = useQuery({
    queryKey: ['settings', 'media-management'],
    queryFn: () => fetchJSON<ImportStatsResponse>('/api/v1/import/stats'),
    refetchInterval: 15000
  })

  const stats = statsQuery.data

  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">Media Management</h2>
        <p className="text-sm text-slate-400">Current import and file placement configuration surfaced from backend runtime settings.</p>
      </header>

      <div className="rounded border border-slate-800 bg-slate-900/50 p-4 space-y-2">
        <Row label="Library root configured" value={stats?.library_root_configured ? 'yes' : 'no'} />
        <Row label="Library root" value={stats?.library_root?.trim() ? stats.library_root : '-'} />
        <Row label="Keep incoming files" value={stats?.keep_incoming ? 'yes' : 'no'} />
        <Row label="Keep incoming source" value={stats?.keep_incoming_source?.trim() ? stats.keep_incoming_source : 'default'} />
        <Row
          label="Next runnable import"
          value={stats?.next_runnable_at ? new Date(stats.next_runnable_at).toLocaleString() : 'none scheduled'}
        />
      </div>

      <div className="rounded border border-slate-800 bg-slate-900/40 p-3 text-xs text-slate-300">
        This page is currently read-only. Update media-management values via backend environment/config, then reload.
      </div>

      {statsQuery.isLoading ? <p className="text-sm text-slate-400">Loading media-management settings...</p> : null}
      {statsQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load media-management settings.</div> : null}
    </section>
  )
}
