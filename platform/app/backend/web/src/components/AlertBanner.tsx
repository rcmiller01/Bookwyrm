import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { fetchJSON } from '../lib/api'

type SubsystemCheck = {
  name: string
  status: string
  error?: string
  guidance?: string
}

type HealthDetailResponse = {
  status: string
  checks: SubsystemCheck[]
}

const settingsLinks: Record<string, string> = {
  metadata_service: '/system/status',
  indexer_service: '/settings/indexers',
  'download_client:': '/settings/download-clients'
}

function linkForCheck(name: string): string | null {
  for (const [prefix, path] of Object.entries(settingsLinks)) {
    if (name.startsWith(prefix)) return path
  }
  return null
}

export function AlertBanner() {
  const healthQuery = useQuery({
    queryKey: ['system', 'health-detail'],
    queryFn: () => fetchJSON<HealthDetailResponse>('/api/v1/system/health-detail'),
    refetchInterval: 30000,
    retry: 1
  })

  const degraded = (healthQuery.data?.checks ?? []).filter(
    (c) => c.status !== 'ok' && c.status !== 'unconfigured'
  )

  if (healthQuery.isLoading || degraded.length === 0) return null

  return (
    <div className="space-y-2">
      {degraded.map((check) => {
        const link = linkForCheck(check.name)
        return (
          <div
            key={check.name}
            className="flex items-start gap-3 rounded border border-amber-900/60 bg-amber-950/30 px-3 py-2"
          >
            <span className="mt-0.5 text-amber-400">⚠</span>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium text-amber-200">{check.name.replace(/_/g, ' ')}</p>
              {check.error ? <p className="text-xs text-amber-300/80">{check.error}</p> : null}
              {check.guidance ? <p className="mt-0.5 text-xs text-amber-200/70">{check.guidance}</p> : null}
            </div>
            {link ? (
              <Link
                to={link}
                className="shrink-0 rounded border border-amber-700 px-2 py-0.5 text-xs text-amber-300 hover:bg-amber-900/30"
              >
                Settings
              </Link>
            ) : null}
          </div>
        )
      })}
    </div>
  )
}
