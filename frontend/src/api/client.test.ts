import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiGet, apiPost, ApiError, setUnauthorizedHandler } from './client'

function mockFetchOnce(status: number, body: unknown, ok = status >= 200 && status < 300) {
  const response = {
    ok,
    status,
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(JSON.stringify(body)),
  } as Response
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue(response),
  )
  return response
}

describe('api/client', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    setUnauthorizedHandler(null)
  })

  it('returns the decoded JSON body on success', async () => {
    mockFetchOnce(200, { id: 'u1', email: 'a@b.com' })

    const result = await apiGet<{ id: string; email: string }>('/api/me')

    expect(result).toEqual({ id: 'u1', email: 'a@b.com' })
  })

  it('always sends credentials so the session cookie is included', async () => {
    mockFetchOnce(200, { ok: true })
    await apiGet('/api/me')

    const calledInit = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0][1] as RequestInit
    expect(calledInit.credentials).toBe('include')
  })

  it('throws ApiError with the server message on a non-2xx response', async () => {
    mockFetchOnce(400, { error: { code: 'BAD_REQUEST', message: 'メールアドレスが不正です' } })

    await expect(apiPost('/api/auth/register', { email: 'bad' })).rejects.toMatchObject({
      status: 400,
      message: 'メールアドレスが不正です',
    })
  })

  it('falls back to a generic message when the error body has no message', async () => {
    mockFetchOnce(500, {})

    await expect(apiGet('/api/missions')).rejects.toThrow(/500/)
  })

  it('falls back to a generic message when the error body is not JSON', async () => {
    const response = {
      ok: false,
      status: 502,
      json: () => Promise.reject(new Error('not json')),
    } as unknown as Response
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response))

    await expect(apiGet('/api/missions')).rejects.toThrow(/502/)
  })

  it('invokes the registered unauthorized handler exactly once on a 401', async () => {
    const handler = vi.fn()
    setUnauthorizedHandler(handler)
    mockFetchOnce(401, { error: { code: 'UNAUTHORIZED', message: '認証が必要です' } })

    await expect(apiGet('/api/me')).rejects.toBeInstanceOf(ApiError)
    expect(handler).toHaveBeenCalledTimes(1)
  })

  it('does not invoke the unauthorized handler for non-401 errors', async () => {
    const handler = vi.fn()
    setUnauthorizedHandler(handler)
    mockFetchOnce(403, { error: { code: 'FORBIDDEN', message: '権限がありません' } })

    await expect(apiGet('/api/missions/other-users-mission')).rejects.toBeInstanceOf(ApiError)
    expect(handler).not.toHaveBeenCalled()
  })

  it('returns undefined for a 204 No Content response', async () => {
    mockFetchOnce(204, undefined)
    const result = await apiPost('/api/daily-missions/x/complete')
    expect(result).toBeUndefined()
  })
})

describe('api/client request body encoding', () => {
  beforeEach(() => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200, text: () => Promise.resolve('') }))
  })
  afterEach(() => vi.unstubAllGlobals())

  it('sets Content-Type only when a body is present', async () => {
    await apiPost('/api/missions', { title: 'test' })
    const [, init] = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit]
    expect((init.headers as Record<string, string>)['Content-Type']).toBe('application/json')
    expect(init.body).toBe(JSON.stringify({ title: 'test' }))
  })

  it('omits Content-Type when there is no body', async () => {
    await apiGet('/api/missions')
    const [, init] = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string, RequestInit]
    expect((init.headers as Record<string, string> | undefined)?.['Content-Type']).toBeUndefined()
  })
})
