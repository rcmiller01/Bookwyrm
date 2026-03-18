const DEFAULT_TIMEOUT_MS = 15_000
const RETRY_COUNT = 2
const RETRYABLE_STATUSES = new Set([502, 503, 504])

function isRetryable(error: unknown): boolean {
  if (error instanceof DOMException && error.name === 'AbortError') return false
  if (error instanceof Response) return RETRYABLE_STATUSES.has(error.status)
  return true // network errors are retryable
}

function backoffMs(attempt: number): number {
  const base = Math.min(1000 * 2 ** attempt, 8000)
  return base + Math.random() * base * 0.25
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) { reject(signal.reason); return }
    const timer = setTimeout(resolve, ms)
    signal?.addEventListener('abort', () => { clearTimeout(timer); reject(signal.reason) }, { once: true })
  })
}

async function fetchWithTimeout(input: RequestInfo, init?: RequestInit & { timeoutMs?: number }): Promise<Response> {
  const timeoutMs = init?.timeoutMs ?? DEFAULT_TIMEOUT_MS
  const controller = new AbortController()
  const outerSignal = init?.signal

  if (outerSignal?.aborted) throw outerSignal.reason

  const onOuterAbort = () => controller.abort(outerSignal!.reason)
  outerSignal?.addEventListener('abort', onOuterAbort, { once: true })

  const timer = setTimeout(() => controller.abort(new DOMException('Request timed out', 'TimeoutError')), timeoutMs)
  try {
    return await fetch(input, { ...init, signal: controller.signal })
  } finally {
    clearTimeout(timer)
    outerSignal?.removeEventListener('abort', onOuterAbort)
  }
}

async function request(path: string, init?: RequestInit & { timeoutMs?: number; retries?: number }): Promise<Response> {
  const maxRetries = init?.retries ?? 0
  let lastError: unknown

  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      const response = await fetchWithTimeout(path, init)
      if (!response.ok) {
        if (attempt < maxRetries && RETRYABLE_STATUSES.has(response.status)) {
          await sleep(backoffMs(attempt), init?.signal ?? undefined)
          continue
        }
        const text = await response.text()
        throw new Error(`Request failed (${response.status}): ${text || response.statusText}`)
      }
      return response
    } catch (error) {
      lastError = error
      if (error instanceof Error && error.message.startsWith('Request failed')) throw error
      if (attempt < maxRetries && isRetryable(error)) {
        await sleep(backoffMs(attempt), init?.signal ?? undefined)
        continue
      }
      throw error
    }
  }
  throw lastError
}

export async function fetchJSON<T>(path: string, opts?: { signal?: AbortSignal; timeoutMs?: number }): Promise<T> {
  const response = await request(path, {
    headers: { Accept: 'application/json' },
    signal: opts?.signal,
    timeoutMs: opts?.timeoutMs,
    retries: RETRY_COUNT,
  })
  return (await response.json()) as T
}

export async function postJSON<T>(path: string, body: unknown, opts?: { signal?: AbortSignal }): Promise<T> {
  const response = await request(path, {
    method: 'POST',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    signal: opts?.signal,
  })
  return (await response.json()) as T
}

export async function postNoContent(path: string, body?: unknown, opts?: { signal?: AbortSignal }): Promise<void> {
  await request(path, {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      ...(body ? { 'Content-Type': 'application/json' } : {}),
    },
    ...(body ? { body: JSON.stringify(body) } : {}),
    signal: opts?.signal,
  })
}

export async function patchJSON<T>(path: string, body: unknown, opts?: { signal?: AbortSignal }): Promise<T> {
  const response = await request(path, {
    method: 'PATCH',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    signal: opts?.signal,
  })
  return (await response.json()) as T
}

export async function deleteNoContent(path: string, opts?: { signal?: AbortSignal }): Promise<void> {
  await request(path, {
    method: 'DELETE',
    headers: { Accept: 'application/json' },
    signal: opts?.signal,
  })
}
