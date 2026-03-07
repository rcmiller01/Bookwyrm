type CandidateRow = {
  work_id?: string
  title?: string
  title_score?: number
  author_score?: number
  score?: number
  reason?: string
}

function pct(v: number | undefined): string {
  if (v === undefined || v === null) return '-'
  return `${Math.round(v * 100)}%`
}

function ScoreBar({ value, color }: { value: number | undefined; color: string }) {
  const width = typeof value === 'number' ? Math.max(0, Math.min(100, Math.round(value * 100))) : 0
  return (
    <div className="h-2 w-full rounded bg-slate-800">
      <div className={`h-full rounded ${color}`} style={{ width: `${width}%` }} />
    </div>
  )
}

export function CandidateComparisonTable({
  candidates,
  onApprove
}: {
  candidates: CandidateRow[]
  onApprove?: (workId: string) => void
}) {
  if (!candidates || candidates.length === 0) {
    return <p className="text-xs text-slate-400">No candidate data available.</p>
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-xs">
        <thead className="text-slate-400">
          <tr>
            <th className="px-2 py-1">Work</th>
            <th className="px-2 py-1">Title</th>
            <th className="px-2 py-1 w-20">Title</th>
            <th className="px-2 py-1 w-20">Author</th>
            <th className="px-2 py-1 w-20">Score</th>
            <th className="px-2 py-1">Reason</th>
            {onApprove ? <th className="px-2 py-1" /> : null}
          </tr>
        </thead>
        <tbody>
          {candidates.map((c, idx) => (
            <tr
              key={c.work_id ?? idx}
              className={[
                'border-t border-slate-800 text-slate-200',
                idx === 0 ? 'bg-emerald-900/20' : ''
              ].join(' ')}
            >
              <td className="px-2 py-1.5 font-mono text-slate-300">{c.work_id ?? '-'}</td>
              <td className="px-2 py-1.5 max-w-[200px] truncate" title={c.title}>{c.title ?? '-'}</td>
              <td className="px-2 py-1.5">
                <div className="flex items-center gap-1">
                  <span className="w-8 text-right">{pct(c.title_score)}</span>
                  <ScoreBar value={c.title_score} color="bg-sky-500" />
                </div>
              </td>
              <td className="px-2 py-1.5">
                <div className="flex items-center gap-1">
                  <span className="w-8 text-right">{pct(c.author_score)}</span>
                  <ScoreBar value={c.author_score} color="bg-violet-500" />
                </div>
              </td>
              <td className="px-2 py-1.5">
                <div className="flex items-center gap-1">
                  <span className="w-8 text-right">{pct(c.score)}</span>
                  <ScoreBar value={c.score} color="bg-emerald-500" />
                </div>
              </td>
              <td className="px-2 py-1.5 text-slate-400">{c.reason ?? ''}</td>
              {onApprove ? (
                <td className="px-2 py-1.5">
                  {c.work_id ? (
                    <button
                      className="rounded border border-emerald-700 px-1.5 py-0.5 text-[10px] text-emerald-300"
                      onClick={() => onApprove(c.work_id!)}
                    >
                      Approve as this
                    </button>
                  ) : null}
                </td>
              ) : null}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
