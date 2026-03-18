interface ErrorAlertProps {
  message?: string
}

export function ErrorAlert({ message = 'Failed to load data.' }: ErrorAlertProps) {
  return (
    <div className="rounded border border-red-800/50 bg-red-950/30 px-4 py-3 text-sm text-red-300">
      {message}
    </div>
  )
}
