type ConfirmDialogProps = {
  open: boolean
  title: string
  description: string
  onConfirm: () => void
  onCancel: () => void
}

export function ConfirmDialog({ open, title, description, onConfirm, onCancel }: ConfirmDialogProps) {
  if (!open) {
    return null
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
      <div className="w-full max-w-md rounded border border-slate-700 bg-slate-900 p-4 shadow-xl">
        <h3 className="text-lg font-semibold text-slate-100">{title}</h3>
        <p className="mt-2 text-sm text-slate-300">{description}</p>
        <div className="mt-4 flex justify-end gap-2">
          <button className="rounded border border-slate-600 px-3 py-1 text-sm text-slate-200" onClick={onCancel}>
            Cancel
          </button>
          <button className="rounded bg-red-600 px-3 py-1 text-sm font-medium text-white" onClick={onConfirm}>
            Confirm
          </button>
        </div>
      </div>
    </div>
  )
}
