import { ReactNode } from 'react'

export function FilterBar({ children }: { children: ReactNode }) {
  return (
    <div className="rounded border border-slate-800 bg-slate-900/60 p-3">
      <div className="flex flex-wrap items-center gap-2">{children}</div>
    </div>
  )
}
