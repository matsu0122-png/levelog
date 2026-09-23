import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { apiGet } from '../api/client'
import { AuthProvider, useAuth } from './AuthContext'

const meUser = { id: 'u1', email: 'a@b.com', timezone: 'Asia/Tokyo' }

function jsonResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(JSON.stringify(body)),
  } as Response
}

/** Routes fetch calls by whichever URL substring matches first, so each test only needs to describe the endpoints it cares about. */
function installFetchMock(routes: Record<string, { status: number; body: unknown }>) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string | URL) => {
      const href = String(url)
      for (const [substring, res] of Object.entries(routes)) {
        if (href.includes(substring)) return jsonResponse(res.status, res.body)
      }
      throw new Error(`unmocked fetch call: ${href}`)
    }),
  )
}

function renderAuth() {
  return renderHook(() => useAuth(), { wrapper: AuthProvider })
}

describe('AuthContext', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('resolves to null user when the initial /api/me check is unauthenticated', async () => {
    installFetchMock({ '/api/me': { status: 401, body: { error: { message: '認証が必要です' } } } })

    const { result } = renderAuth()
    expect(result.current.loading).toBe(true)

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.user).toBeNull()
    expect(result.current.sessionExpired).toBe(false)
  })

  it('resolves to the user when already logged in (valid session cookie)', async () => {
    installFetchMock({ '/api/me': { status: 200, body: meUser } })

    const { result } = renderAuth()
    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.user).toEqual(meUser)
  })

  it('login sets the user and clears any prior sessionExpired flag', async () => {
    installFetchMock({
      '/api/me': { status: 401, body: {} },
      '/api/auth/login': { status: 200, body: meUser },
    })

    const { result } = renderAuth()
    await waitFor(() => expect(result.current.loading).toBe(false))

    await act(async () => {
      await result.current.login('a@b.com', 'password123')
    })

    expect(result.current.user).toEqual(meUser)
    expect(result.current.sessionExpired).toBe(false)
  })

  it('a 401 from any later API call clears the session and flags it as expired, once a user was logged in', async () => {
    installFetchMock({ '/api/me': { status: 200, body: meUser } })
    const { result } = renderAuth()
    await waitFor(() => expect(result.current.user).toEqual(meUser))

    // Simulate a page's own data fetch (e.g. GET /api/missions) hitting a
    // session that expired mid-use — this is exactly the scenario the
    // global unauthorized handler exists for (see api/client.ts).
    installFetchMock({ '/api/missions': { status: 401, body: { error: { message: 'セッションが無効です' } } } })
    await act(async () => {
      await apiGet('/api/missions').catch(() => {})
    })

    await waitFor(() => expect(result.current.user).toBeNull())
    expect(result.current.sessionExpired).toBe(true)
  })

  it('does not flag sessionExpired when a 401 arrives while already logged out', async () => {
    installFetchMock({ '/api/me': { status: 401, body: {} } })
    const { result } = renderAuth()
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.user).toBeNull()

    installFetchMock({ '/api/missions': { status: 401, body: {} } })
    await act(async () => {
      await apiGet('/api/missions').catch(() => {})
    })

    expect(result.current.sessionExpired).toBe(false)
  })

  it('logout clears the user', async () => {
    installFetchMock({
      '/api/me': { status: 200, body: meUser },
      '/api/auth/logout': { status: 200, body: { ok: true } },
    })
    const { result } = renderAuth()
    await waitFor(() => expect(result.current.user).toEqual(meUser))

    await act(async () => {
      await result.current.logout()
    })

    expect(result.current.user).toBeNull()
  })

  it('dismissSessionExpired clears the flag without touching the user', async () => {
    installFetchMock({ '/api/me': { status: 200, body: meUser } })
    const { result } = renderAuth()
    await waitFor(() => expect(result.current.user).toEqual(meUser))

    installFetchMock({ '/api/missions': { status: 401, body: {} } })
    await act(async () => {
      await apiGet('/api/missions').catch(() => {})
    })
    await waitFor(() => expect(result.current.sessionExpired).toBe(true))

    act(() => result.current.dismissSessionExpired())

    expect(result.current.sessionExpired).toBe(false)
  })
})
