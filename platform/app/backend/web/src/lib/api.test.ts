import { fetchJSON, postJSON, deleteNoContent } from './api'

let fetchCallCount = 0
let fetchResponses: Array<{ status: number; body?: string; networkError?: boolean }> = []

const originalFetch = globalThis.fetch

beforeEach(() => {
  fetchCallCount = 0
  fetchResponses = []
  globalThis.fetch = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => {
    const idx = fetchCallCount++
    const spec = fetchResponses[idx] ?? fetchResponses[fetchResponses.length - 1]
    if (spec?.networkError) throw new TypeError('Failed to fetch')
    return new Response(spec?.body ?? '{}', {
      status: spec?.status ?? 200,
      headers: { 'Content-Type': 'application/json' },
    })
  }) as typeof fetch
})

afterEach(() => {
  globalThis.fetch = originalFetch
})

describe('fetchJSON', () => {
  it('returns parsed JSON on success', async () => {
    fetchResponses = [{ status: 200, body: '{"ok":true}' }]
    const result = await fetchJSON<{ ok: boolean }>('/test')
    expect(result).toEqual({ ok: true })
    expect(fetchCallCount).toBe(1)
  })

  it('retries on 502 and succeeds', async () => {
    fetchResponses = [
      { status: 502, body: 'bad gateway' },
      { status: 200, body: '{"recovered":true}' },
    ]
    const result = await fetchJSON<{ recovered: boolean }>('/test')
    expect(result).toEqual({ recovered: true })
    expect(fetchCallCount).toBe(2)
  })

  it('retries on 503 up to max retries then throws', async () => {
    fetchResponses = [
      { status: 503, body: 'unavailable' },
      { status: 503, body: 'unavailable' },
      { status: 503, body: 'unavailable' },
    ]
    await expect(fetchJSON('/test')).rejects.toThrow('Request failed (503)')
    expect(fetchCallCount).toBe(3) // 1 initial + 2 retries
  })

  it('does not retry on 4xx', async () => {
    fetchResponses = [{ status: 404, body: 'not found' }]
    await expect(fetchJSON('/test')).rejects.toThrow('Request failed (404)')
    expect(fetchCallCount).toBe(1)
  })

  it('retries on network error then succeeds', async () => {
    fetchResponses = [
      { status: 0, networkError: true },
      { status: 200, body: '{"ok":true}' },
    ]
    const result = await fetchJSON<{ ok: boolean }>('/test')
    expect(result).toEqual({ ok: true })
    expect(fetchCallCount).toBe(2)
  })

  it('respects abort signal', async () => {
    const controller = new AbortController()
    controller.abort()
    await expect(fetchJSON('/test', { signal: controller.signal })).rejects.toThrow()
    expect(fetchCallCount).toBe(0)
  })
})

describe('postJSON', () => {
  it('does not retry by default', async () => {
    fetchResponses = [{ status: 503, body: 'unavailable' }]
    await expect(postJSON('/test', { data: 1 })).rejects.toThrow('Request failed (503)')
    expect(fetchCallCount).toBe(1)
  })
})

describe('deleteNoContent', () => {
  it('resolves on success', async () => {
    fetchResponses = [{ status: 200 }]
    await expect(deleteNoContent('/test')).resolves.toBeUndefined()
  })

  it('throws on failure', async () => {
    fetchResponses = [{ status: 500, body: 'internal error' }]
    await expect(deleteNoContent('/test')).rejects.toThrow('Request failed (500)')
  })
})
