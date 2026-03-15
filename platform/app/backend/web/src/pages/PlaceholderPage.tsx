import { DataTable } from '../components/DataTable'

export function PlaceholderPage({ title }: { title: string }) {
  return (
    <section className="space-y-4">
      <header>
        <h2 className="text-2xl font-semibold text-slate-100">{title}</h2>
        <p className="text-sm text-slate-400">Phase 16 Slice B shell placeholder</p>
      </header>
      <DataTable
        columns={[
          { key: 'name', header: 'Name' },
          { key: 'status', header: 'Status' },
          { key: 'updated', header: 'Updated' }
        ]}
        rows={[
          { id: '1', name: `${title} Item A`, status: 'queued', updated: 'just now' },
          { id: '2', name: `${title} Item B`, status: 'running', updated: '1m ago' }
        ]}
      />
    </section>
  )
}
