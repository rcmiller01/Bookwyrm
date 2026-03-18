import { ReactNode, useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'

export function VirtualizedList<T>({
  rows,
  estimateSize = 52,
  overscan = 10,
  maxHeight = 600,
  empty,
  header,
  renderRow,
  rowKey
}: {
  rows: T[]
  estimateSize?: number
  overscan?: number
  maxHeight?: number
  empty: ReactNode
  header: ReactNode
  renderRow: (row: T, index: number) => ReactNode
  rowKey: (row: T, index: number) => string
}) {
  const parentRef = useRef<HTMLDivElement>(null)
  const rowVirtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => estimateSize,
    overscan
  })

  return (
    <div className="overflow-hidden rounded border border-slate-800 bg-slate-900/50">
      {header}
      {rows.length === 0 ? (
        <div className="px-3 py-6 text-center text-slate-400">{empty}</div>
      ) : (
        <div ref={parentRef} className="overflow-auto" style={{ maxHeight }}>
          <div style={{ height: rowVirtualizer.getTotalSize(), position: 'relative' }}>
            {rowVirtualizer.getVirtualItems().map((virtualItem) => {
              const row = rows[virtualItem.index]
              return (
                <div
                  key={rowKey(row, virtualItem.index)}
                  className="absolute left-0 top-0 w-full border-t border-slate-800 text-slate-100"
                  style={{
                    transform: `translateY(${virtualItem.start}px)`,
                    height: virtualItem.size
                  }}
                >
                  {renderRow(row, virtualItem.index)}
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
