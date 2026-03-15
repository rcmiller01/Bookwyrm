import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useToast } from '../components/ToastProvider'
import { errorMessage } from '../lib/errorMessage'
import { fetchJSON, postJSON } from '../lib/api'

type ReadinessItem = {
  key: string
  label: string
  ready: boolean
  blocking: boolean
  status: string
  detail?: string
  guidance?: string
  route?: string
}

type ReadinessResponse = {
  status: string
  ready: boolean
  blocking_count: number
  warning_count: number
  items: ReadinessItem[]
  generated_at: string
}

type ActionResponse = {
  status?: string
  retried?: number
  queued?: number
}

function SetupItemRow({ item }: { item: ReadinessItem }) {
  const badge = item.ready
    ? 'bg-emerald-900/40 text-emerald-300 border-emerald-800/60'
    : item.blocking
      ? 'bg-red-900/40 text-red-300 border-red-800/60'
      : 'bg-amber-900/40 text-amber-300 border-amber-800/60'
  const stateLabel = item.ready ? 'Ready' : item.blocking ? 'Required' : 'Warning'

  return (
    <div className="rounded border border-slate-800 bg-slate-900/50 p-3">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <p className="text-sm font-medium text-slate-100">{item.label}</p>
          {item.detail ? <p className="text-xs text-slate-400">{item.detail}</p> : null}
          {!item.ready && item.guidance ? <p className="mt-1 text-xs text-sky-300/90">{item.guidance}</p> : null}
        </div>
        <span className={`rounded border px-2 py-0.5 text-xs ${badge}`}>{stateLabel}</span>
      </div>
      {item.route ? (
        <div className="mt-2">
          <Link className="text-xs text-sky-300 hover:text-sky-200" to={item.route}>
            Open relevant settings
          </Link>
        </div>
      ) : null}
    </div>
  )
}

export function SetupPage() {
  const queryClient = useQueryClient()
  const { pushToast } = useToast()

  const readiness = useQuery({
    queryKey: ['system', 'readiness'],
    queryFn: () => fetchJSON<ReadinessResponse>('/api/v1/system/readiness'),
    refetchInterval: 30000
  })

  const action = useMutation({
    mutationFn: async (path: string) => postJSON<ActionResponse>(path, {}),
    onSuccess: async (data) => {
      const bits = [data.retried ? `${data.retried} retried` : '', data.queued ? `${data.queued} queued` : '']
        .filter(Boolean)
        .join(', ')
      pushToast(bits ? `Action completed (${bits})` : 'Action completed')
      await queryClient.invalidateQueries({ queryKey: ['system', 'readiness'] })
    },
    onError: (err) => pushToast(errorMessage(err))
  })

  const data = readiness.data
  const blocking = data?.items.filter((item) => !item.ready && item.blocking) ?? []
  const warnings = data?.items.filter((item) => !item.ready && !item.blocking) ?? []

  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">Setup Checklist</h2>
        <p className="text-sm text-slate-400">First-run readiness, dependency checks, and guided remediation.</p>
      </header>

      <div className="rounded border border-slate-800 bg-slate-900/60 p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-sm text-slate-300">Overall Status</p>
            <p className="text-lg font-semibold text-slate-100">
              {data?.status ? data.status.replace(/_/g, ' ') : readiness.isLoading ? 'loading' : 'unknown'}
            </p>
            {data?.generated_at ? <p className="text-xs text-slate-500">Last check: {new Date(data.generated_at).toLocaleString()}</p> : null}
          </div>
          <div className="flex gap-2">
            <button
              className="rounded border border-amber-700 px-3 py-1.5 text-xs text-amber-300 hover:bg-amber-900/20 disabled:opacity-60"
              disabled={action.isPending}
              onClick={() => action.mutate('/api/v1/system/actions/test-connections')}
            >
              Test Connections
            </button>
            <button
              className="rounded border border-emerald-700 px-3 py-1.5 text-xs text-emerald-300 hover:bg-emerald-900/20 disabled:opacity-60"
              disabled={action.isPending}
              onClick={() => action.mutate('/api/v1/system/actions/retry-failed-downloads')}
            >
              Retry Failed Downloads
            </button>
            <button
              className="rounded border border-emerald-700 px-3 py-1.5 text-xs text-emerald-300 hover:bg-emerald-900/20 disabled:opacity-60"
              disabled={action.isPending}
              onClick={() => action.mutate('/api/v1/system/actions/retry-failed-imports')}
            >
              Retry Failed Imports
            </button>
          </div>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <div className="space-y-3 rounded border border-slate-800 bg-slate-900/60 p-4">
          <h3 className="text-lg font-semibold text-slate-100">Required Before Beta</h3>
          {blocking.length === 0 ? (
            <p className="text-sm text-emerald-300">All required checks are satisfied.</p>
          ) : (
            blocking.map((item) => <SetupItemRow key={item.key} item={item} />)
          )}
        </div>
        <div className="space-y-3 rounded border border-slate-800 bg-slate-900/60 p-4">
          <h3 className="text-lg font-semibold text-slate-100">Warnings / Recommendations</h3>
          {warnings.length === 0 ? (
            <p className="text-sm text-slate-300">No non-blocking warnings.</p>
          ) : (
            warnings.map((item) => <SetupItemRow key={item.key} item={item} />)
          )}
        </div>
      </div>

      {readiness.isLoading ? <p className="text-sm text-slate-400">Checking readiness...</p> : null}
      {readiness.isError ? (
        <div className="rounded border border-red-900/80 bg-red-950/40 p-3 text-sm text-red-200">
          Failed to load setup readiness.
        </div>
      ) : null}
    </section>
  )
}
