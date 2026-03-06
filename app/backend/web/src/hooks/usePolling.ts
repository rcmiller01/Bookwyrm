import { useEffect } from 'react'

export function usePolling(callback: () => void, intervalMs: number, enabled = true) {
  useEffect(() => {
    if (!enabled || intervalMs <= 0) {
      return
    }
    callback()
    const handle = window.setInterval(callback, intervalMs)
    return () => window.clearInterval(handle)
  }, [callback, enabled, intervalMs])
}
