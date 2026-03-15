export function StatusBadge({ label }: { label: string }) {
  const normalized = label.trim().toLowerCase()
  const color =
    normalized === 'monitored' ||
    normalized === 'ready' ||
    normalized === 'healthy' ||
    normalized === 'imported'
      ? 'border-emerald-700 text-emerald-300 bg-emerald-950/30'
      : normalized === 'missing' ||
          normalized === 'needs_review' ||
          normalized === 'cutoff_unmet' ||
          normalized === 'quarantine'
        ? 'border-amber-700 text-amber-300 bg-amber-950/30'
        : normalized === 'failed' || normalized === 'error'
          ? 'border-red-700 text-red-300 bg-red-950/30'
          : 'border-slate-700 text-slate-300 bg-slate-900/40'

  return <span className={`inline-flex rounded border px-2 py-0.5 text-xs ${color}`}>{label}</span>
}
