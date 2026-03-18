import { ReactNode } from 'react'

export function BulkActionBar({
  count,
  children
}: {
  count: number
  children: ReactNode
}) {
  return (
    <div className="rounded border border-slate-800 bg-slate-900/60 p-3">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-xs uppercase text-slate-400">Bulk Actions ({count})</span>
        {children}
      </div>
    </div>
  )
}
