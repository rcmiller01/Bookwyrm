import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { fetchJSON } from '../lib/api'

type LibraryItem = {
  work_id: string
}

type LibraryItemsResponse = { items: LibraryItem[] }

type SeriesEntry = {
  series_id?: string
  series_name?: string
}

type WorkPayload = {
  title?: string
  series_entries?: SeriesEntry[]
}

type WorkIntelligenceResponse = {
  work?: WorkPayload
}

type SeriesRow = {
  key: string
  series: string
  works: number
}

function parseSeries(workID: string, work?: WorkPayload): { title: string; entries: SeriesEntry[] } {
  return {
    title: work?.title?.trim() || workID,
    entries: work?.series_entries ?? []
  }
}

export function SeriesPage() {
  const libraryItemsQuery = useQuery({
    queryKey: ['library', 'series', 'library-items'],
    queryFn: () => fetchJSON<LibraryItemsResponse>('/api/v1/library/items?limit=500'),
    refetchInterval: 30000
  })

  const workIDs = useMemo(() => {
    const ids = new Set<string>()
    for (const item of libraryItemsQuery.data?.items ?? []) {
      if (item.work_id?.trim()) {
        ids.add(item.work_id.trim())
      }
    }
    return Array.from(ids).slice(0, 150)
  }, [libraryItemsQuery.data])

  const workDetailsQuery = useQuery({
    queryKey: ['library', 'series', 'work-details', workIDs.join(',')],
    enabled: workIDs.length > 0,
    queryFn: async () => {
      const results = await Promise.all(
        workIDs.map(async (workID) => {
          try {
            const payload = await fetchJSON<WorkIntelligenceResponse>(`/api/v1/works/${encodeURIComponent(workID)}/intelligence`)
            return parseSeries(workID, payload.work)
          } catch {
            return parseSeries(workID)
          }
        })
      )
      return results
    }
  })

  const rows = useMemo<SeriesRow[]>(() => {
    const aggregate = new Map<string, SeriesRow>()
    for (const work of workDetailsQuery.data ?? []) {
      const seen = new Set<string>()
      for (const entry of work.entries) {
        const key = entry.series_id?.trim() || entry.series_name?.trim()
        if (!key || seen.has(key)) {
          continue
        }
        seen.add(key)
        const existing = aggregate.get(key)
        if (existing) {
          existing.works += 1
          continue
        }
        aggregate.set(key, {
          key,
          series: entry.series_name?.trim() || key,
          works: 1
        })
      }
    }
    return Array.from(aggregate.values()).sort((a, b) => a.series.localeCompare(b.series))
  }, [workDetailsQuery.data])

  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">Series</h2>
        <p className="text-sm text-slate-400">Series relationships inferred from metadata attached to imported works.</p>
      </header>

      <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
        <table className="w-full text-left text-sm">
          <thead className="bg-slate-900 text-slate-300">
            <tr>
              <th className="px-3 py-2">Series</th>
              <th className="px-3 py-2">Works in Library</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.key} className="border-t border-slate-800 text-slate-100">
                <td className="px-3 py-2">{row.series}</td>
                <td className="px-3 py-2">{row.works}</td>
              </tr>
            ))}
            {rows.length === 0 ? (
              <tr>
                <td colSpan={2} className="px-3 py-6 text-center text-slate-400">
                  No series metadata found yet.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      {libraryItemsQuery.isLoading || workDetailsQuery.isLoading ? <p className="text-sm text-slate-400">Loading series...</p> : null}
      {libraryItemsQuery.isError || workDetailsQuery.isError ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">
          Failed to load series data.
        </div>
      ) : null}
    </section>
  )
}
