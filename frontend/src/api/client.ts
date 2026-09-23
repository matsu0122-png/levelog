const API_BASE = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? 'http://localhost:8080'

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// AuthContext registers a handler here on mount so that a 401 from *any*
// API call (not just the initial session check) clears the stale session
// and lets the router redirect to /login, instead of every page having to
// special-case session expiry itself.
type UnauthorizedHandler = () => void
let unauthorizedHandler: UnauthorizedHandler | null = null
export function setUnauthorizedHandler(handler: UnauthorizedHandler | null) {
  unauthorizedHandler = handler
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: 'include',
    headers: {
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...options.headers,
    },
  })

  if (!res.ok) {
    let message = `リクエストに失敗しました (${res.status})`
    try {
      const body = (await res.json()) as { error?: { message?: string } }
      if (body?.error?.message) message = body.error.message
    } catch {
      // ignore non-JSON error bodies
    }
    if (res.status === 401) {
      unauthorizedHandler?.()
    }
    throw new ApiError(res.status, message)
  }

  if (res.status === 204) {
    return undefined as T
  }
  const text = await res.text()
  return text ? (JSON.parse(text) as T) : (undefined as T)
}

export const apiGet = <T,>(path: string) => request<T>(path)

export const apiPost = <T,>(path: string, body?: unknown) =>
  request<T>(path, { method: 'POST', body: body !== undefined ? JSON.stringify(body) : undefined })

export const apiPut = <T,>(path: string, body?: unknown) =>
  request<T>(path, { method: 'PUT', body: body !== undefined ? JSON.stringify(body) : undefined })

export const apiPatch = <T,>(path: string, body?: unknown) =>
  request<T>(path, { method: 'PATCH', body: body !== undefined ? JSON.stringify(body) : undefined })

export const apiDelete = <T,>(path: string) => request<T>(path, { method: 'DELETE' })
