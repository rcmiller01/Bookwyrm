export type StructuredError = {
  error: string
  message?: string
  guidance?: string
  category?: string
}

export function parseStructuredError(error: unknown): StructuredError | null {
  if (!error || typeof error !== 'object') return null
  // Check if this looks like a structured error from the API
  if ('error' in error && typeof (error as Record<string, unknown>).error === 'string') {
    const obj = error as Record<string, unknown>
    return {
      error: obj.error as string,
      message: typeof obj.message === 'string' ? obj.message : undefined,
      guidance: typeof obj.guidance === 'string' ? obj.guidance : undefined,
      category: typeof obj.category === 'string' ? obj.category : undefined
    }
  }
  // Check for Error objects that might carry a response body
  if (error instanceof Error) {
    const cause = (error as Error & { body?: unknown }).body
    if (cause && typeof cause === 'object' && 'error' in cause) {
      return parseStructuredError(cause)
    }
  }
  return null
}

export function errorMessage(error: unknown): string {
  // Try structured error first
  const structured = parseStructuredError(error)
  if (structured) {
    const parts = [structured.message || structured.error]
    if (structured.guidance) {
      parts.push(structured.guidance)
    }
    return parts.join(' — ')
  }
  if (error instanceof Error) return error.message
  return String(error)
}
