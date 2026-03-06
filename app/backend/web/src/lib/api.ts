export async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(path, {
    headers: {
      Accept: 'application/json'
    }
  })
  if (!response.ok) {
    const text = await response.text()
    throw new Error(`Request failed (${response.status}): ${text || response.statusText}`)
  }
  return (await response.json()) as T
}

export async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const response = await fetch(path, {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(body)
  })
  if (!response.ok) {
    const text = await response.text()
    throw new Error(`Request failed (${response.status}): ${text || response.statusText}`)
  }
  return (await response.json()) as T
}

export async function postNoContent(path: string, body?: unknown): Promise<void> {
  const response = await fetch(path, {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      ...(body ? { 'Content-Type': 'application/json' } : {})
    },
    ...(body ? { body: JSON.stringify(body) } : {})
  })
  if (!response.ok) {
    const text = await response.text()
    throw new Error(`Request failed (${response.status}): ${text || response.statusText}`)
  }
}

export async function patchJSON<T>(path: string, body: unknown): Promise<T> {
  const response = await fetch(path, {
    method: 'PATCH',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(body)
  })
  if (!response.ok) {
    const text = await response.text()
    throw new Error(`Request failed (${response.status}): ${text || response.statusText}`)
  }
  return (await response.json()) as T
}
