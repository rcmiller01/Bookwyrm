import { useMemo } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { FilterBar } from '../components/FilterBar'
import { PageHeader } from '../components/PageHeader'
import { StatusBadge } from '../components/StatusBadge'
import { useToast } from '../components/ToastProvider'
import { useLocalStorageState } from '../hooks/useLocalStorageState'
import { useState } from 'react'
import { fetchJSON, patchJSON, postJSON } from '../lib/api'
import { errorMessage } from '../lib/errorMessage'

type DownloadClient = {
  id: string
  name: string
  client_type: string
  enabled: boolean
  tier: string
  reliability_score: number
  priority: number
}

type DownloadClientsResponse = { items: DownloadClient[] }

export function DownloadClientsPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()
  const [query, setQuery] = useLocalStorageState<string>('settings.clients.query', '')

  const clientsQuery = useQuery({
    queryKey: ['settings', 'download-clients'],
    queryFn: () => fetchJSON<DownloadClientsResponse>('/api/v1/download/clients'),
    refetchInterval: 15000
  })

  const updateMutation = useMutation({
    mutationFn: (payload: { id: string; enabled?: boolean; priority?: number }) =>
      patchJSON<DownloadClient>(`/api/v1/download/clients/${encodeURIComponent(payload.id)}`, {
        ...(payload.enabled !== undefined ? { enabled: payload.enabled } : {}),
        ...(payload.priority !== undefined ? { priority: payload.priority } : {})
      }),
    onSuccess: async () => {
      pushToast('Download client updated')
      await queryClient.invalidateQueries({ queryKey: ['settings', 'download-clients'] })
      await queryClient.invalidateQueries({ queryKey: ['dashboard', 'download-clients'] })
    },
    onError: (error) => pushToast(errorMessage(error))
  })

  const [testing, setTesting] = useState<string | null>(null)
  const testConnection = async (clientID: string) => {
    setTesting(clientID)
    try {
      const result = await postJSON<{ ok: boolean; error?: string }>(`/api/v1/test-connection/download-client/${encodeURIComponent(clientID)}`, {})
      if (result.ok) {
        pushToast(`${clientID}: connection successful`)
      } else {
        pushToast(`${clientID}: ${result.error ?? 'test failed'}`)
      }
    } catch (error) {
      pushToast(`${clientID}: ${errorMessage(error)}`)
    } finally {
      setTesting(null)
    }
  }

  const rows = useMemo(() => {
    const lowered = query.trim().toLowerCase()
    return (clientsQuery.data?.items ?? []).filter((row) => {
      if (!lowered) return true
      return row.name.toLowerCase().includes(lowered) || row.client_type.toLowerCase().includes(lowered)
    })
  }, [clientsQuery.data?.items, query])

  return (
    <section className="space-y-4">
      <PageHeader
        title="Download Clients"
        subtitle="Enable clients, tune priority, and verify runtime health signals."
        actions={
          <button className="rounded border border-slate-700 px-3 py-1.5 text-sm text-slate-200" onClick={() => void clientsQuery.refetch()}>
            Test/Refresh
          </button>
        }
      />

      <FilterBar>
        <input
          className="w-64 rounded border border-slate-700 bg-slate-900 px-2 py-1.5 text-sm text-slate-100"
          placeholder="Filter clients"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
      </FilterBar>

      <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
        <table className="w-full text-left text-sm">
          <thead className="bg-slate-900 text-slate-300">
            <tr>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Type</th>
              <th className="px-3 py-2">Enabled</th>
              <th className="px-3 py-2">Priority</th>
              <th className="px-3 py-2">Reliability</th>
              <th className="px-3 py-2">Tier</th>
              <th className="px-3 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.id} className="border-t border-slate-800 text-slate-100">
                <td className="px-3 py-2">{row.name}</td>
                <td className="px-3 py-2">{row.client_type}</td>
                <td className="px-3 py-2">{row.enabled ? <StatusBadge label="Enabled" /> : <StatusBadge label="Disabled" />}</td>
                <td className="px-3 py-2">{row.priority}</td>
                <td className="px-3 py-2">{row.reliability_score.toFixed(2)}</td>
                <td className="px-3 py-2">{row.tier}</td>
                <td className="px-3 py-2">
                  <div className="flex flex-wrap gap-2">
                    <button
                      className="rounded border border-emerald-700 px-2 py-1 text-xs text-emerald-300"
                      disabled={testing === row.id}
                      onClick={() => void testConnection(row.id)}
                    >
                      {testing === row.id ? 'Testing...' : 'Test'}
                    </button>
                    <button
                      className="rounded border border-slate-700 px-2 py-1 text-xs text-slate-200"
                      onClick={() => updateMutation.mutate({ id: row.id, enabled: !row.enabled })}
                    >
                      {row.enabled ? 'Disable' : 'Enable'}
                    </button>
                    <button
                      className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300"
                      onClick={() => updateMutation.mutate({ id: row.id, priority: Math.max(1, row.priority - 10) })}
                    >
                      Higher
                    </button>
                    <button
                      className="rounded border border-sky-700 px-2 py-1 text-xs text-sky-300"
                      onClick={() => updateMutation.mutate({ id: row.id, priority: row.priority + 10 })}
                    >
                      Lower
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {rows.length === 0 ? (
              <tr>
                <td colSpan={7} className="px-3 py-6 text-center text-slate-400">
                  No download clients found.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      {clientsQuery.isLoading ? <p className="text-sm text-slate-400">Loading download clients...</p> : null}
      {clientsQuery.isError ? <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">Failed to load download clients.</div> : null}
    </section>
  )
}
