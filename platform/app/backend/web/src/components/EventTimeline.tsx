import { useState } from 'react'

type TimelineEvent = {
  id?: number
  ts?: string
  event_type?: string
  message?: string
  payload?: Record<string, unknown>
}

const badgeColors: Record<string, string> = {
  queued: 'bg-slate-700 text-slate-300',
  started: 'bg-sky-900 text-sky-300',
  running: 'bg-sky-900 text-sky-300',
  warning: 'bg-amber-900 text-amber-300',
  imported: 'bg-emerald-900 text-emerald-300',
  completed: 'bg-emerald-900 text-emerald-300',
  failed: 'bg-red-900 text-red-300',
  error: 'bg-red-900 text-red-300',
  needs_review: 'bg-amber-900 text-amber-300',
  skipped: 'bg-slate-700 text-slate-300'
}

function EventBadge({ eventType }: { eventType: string }) {
  const color = badgeColors[eventType] ?? 'bg-slate-700 text-slate-300'
  return (
    <span className={`inline-block rounded px-1.5 py-0.5 text-[10px] font-medium ${color}`}>
      {eventType}
    </span>
  )
}

function PayloadDetails({ payload }: { payload: Record<string, unknown> }) {
  const [expanded, setExpanded] = useState(false)
  const keys = Object.keys(payload)
  if (keys.length === 0) return null
  return (
    <div className="mt-1">
      <button
        className="text-[10px] text-slate-500 hover:text-slate-300"
        onClick={() => setExpanded(!expanded)}
      >
        {expanded ? '▾ hide payload' : `▸ ${keys.length} fields`}
      </button>
      {expanded && (
        <pre className="mt-1 overflow-auto rounded bg-slate-950/60 p-1.5 text-[10px] text-slate-300">
          {JSON.stringify(payload, null, 2)}
        </pre>
      )}
    </div>
  )
}

export function EventTimeline({ events }: { events: TimelineEvent[] }) {
  if (!events || events.length === 0) {
    return <p className="text-xs text-slate-400">No events recorded.</p>
  }

  const sorted = [...events].sort((a, b) => {
    const ta = a.ts ? new Date(a.ts).getTime() : 0
    const tb = b.ts ? new Date(b.ts).getTime() : 0
    return ta - tb
  })

  return (
    <div className="relative space-y-0">
      {sorted.map((event, idx) => (
        <div key={event.id ?? idx} className="flex gap-3 pb-3">
          <div className="flex flex-col items-center">
            <div className="mt-1 h-2 w-2 rounded-full bg-slate-500" />
            {idx < sorted.length - 1 && <div className="w-px flex-1 bg-slate-700" />}
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              {event.event_type && <EventBadge eventType={event.event_type} />}
              {event.ts && (
                <span className="text-[10px] text-slate-500">
                  {new Date(event.ts).toLocaleString()}
                </span>
              )}
            </div>
            {event.message && (
              <p className="mt-0.5 text-xs text-slate-300">{event.message}</p>
            )}
            {event.payload && Object.keys(event.payload).length > 0 && (
              <PayloadDetails payload={event.payload} />
            )}
          </div>
        </div>
      ))}
    </div>
  )
}
