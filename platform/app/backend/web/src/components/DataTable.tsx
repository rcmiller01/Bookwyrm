import { useMemo, useState } from 'react'

type Row = { id: string } & Record<string, string>

type Column = {
  key: string
  header: string
}

export function DataTable({ columns, rows }: { columns: Column[]; rows: Row[] }) {
  const [query, setQuery] = useState('')
  const [sortKey, setSortKey] = useState(columns[0]?.key ?? '')

  const visibleRows = useMemo(() => {
    const lowered = query.trim().toLowerCase()
    return rows
      .filter((row) =>
        lowered === '' ? true : Object.values(row).some((value) => value.toLowerCase().includes(lowered))
      )
      .sort((a, b) => (a[sortKey] ?? '').localeCompare(b[sortKey] ?? ''))
  }, [query, rows, sortKey])

  return (
    <div className="rounded border border-slate-800 bg-slate-900/50">
      <div className="flex items-center justify-between border-b border-slate-800 px-3 py-2">
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Filter..."
          className="w-56 rounded border border-slate-700 bg-slate-900 px-2 py-1 text-sm text-slate-100 outline-none focus:border-sky-500"
        />
        <select
          value={sortKey}
          onChange={(e) => setSortKey(e.target.value)}
          className="rounded border border-slate-700 bg-slate-900 px-2 py-1 text-sm text-slate-100"
        >
          {columns.map((col) => (
            <option key={col.key} value={col.key}>
              Sort: {col.header}
            </option>
          ))}
        </select>
      </div>
      <table className="w-full text-left text-sm">
        <thead className="bg-slate-900 text-slate-300">
          <tr>
            <th className="px-3 py-2">
              <input type="checkbox" aria-label="Select all" />
            </th>
            {columns.map((col) => (
              <th key={col.key} className="px-3 py-2">
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {visibleRows.map((row) => (
            <tr key={row.id} className="border-t border-slate-800 text-slate-100">
              <td className="px-3 py-2">
                <input type="checkbox" aria-label={`Select row ${row.id}`} />
              </td>
              {columns.map((col) => (
                <td key={col.key} className="px-3 py-2">
                  {row[col.key]}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
