import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, dash, request } from './client'
import * as storage from '../auth/storage'

describe('ApiError', () => {
  it('captures status, message, body', () => {
    const err = new ApiError(400, 'Bad', { error: 'Bad' })
    expect(err).toBeInstanceOf(Error)
    expect(err.status).toBe(400)
    expect(err.message).toBe('Bad')
    expect(err.body).toEqual({ error: 'Bad' })
  })
})

describe('request', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    storage.clearSession()
  })

  it('attaches bearer token and act-as tenant', async () => {
    storage.setToken('t')
    storage.setActAsTenant(7)
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), { status: 200 }),
    )
    await request('/test')
    expect(fetchSpy).toHaveBeenCalledTimes(1)
    const [, init] = fetchSpy.mock.calls[0]
    expect(init?.headers).toMatchObject({
      Authorization: 'Bearer t',
      'X-Tenant-ID': '7',
    })
  })

  it('omits headers when not present', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), { status: 200 }),
    )
    await request('/test')
    expect(fetchSpy).toHaveBeenCalledTimes(1)
    const [, init] = fetchSpy.mock.calls[0]
    expect(init?.headers).toEqual({})
  })

  it('can skip act-as and override token', async () => {
    storage.setToken('t')
    storage.setActAsTenant(7)
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), { status: 200 }),
    )
    await request('/test', { noActAs: true, token: null })
    const [, init] = fetchSpy.mock.calls[0]
    expect(init?.headers).toEqual({})
  })

  it('throws ApiError with parsed error body', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'Invalid username or password.' }), { status: 401 }),
    )
    await expect(request('/dash/login')).rejects.toMatchObject({
      name: 'ApiError',
      status: 401,
      message: 'Invalid username or password.',
      body: { error: 'Invalid username or password.' },
    })
  })

  it('supports formData and JSON body', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response('{}', { status: 200 }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 }))
    await request('/a', { formData: new FormData() })
    let call = vi.mocked(globalThis.fetch).mock.calls[0]
    expect(call[1]?.headers).toEqual({})
    await request('/b', { body: { x: 1 } })
    call = vi.mocked(globalThis.fetch).mock.calls[1]
    expect(call[1]?.headers).toMatchObject({ 'Content-Type': 'application/json' })
    expect(call[1]?.body).toBe(JSON.stringify({ x: 1 }))
  })
})

describe('dash client', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    storage.clearSession()
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response('{}', { status: 200 }))
  })

  it('builds query strings correctly', async () => {
    await dash.listUsers({ status: 'ACTIVE', q: 'alice', page: 2 })
    const [url] = vi.mocked(globalThis.fetch).mock.calls[0]
    expect(url).toContain('status=ACTIVE')
    expect(url).toContain('q=alice')
    expect(url).toContain('page=2')
  })

  it('omits empty query params', async () => {
    await dash.listUsers({ status: '', q: undefined })
    const [url] = vi.mocked(globalThis.fetch).mock.calls[0]
    expect(url).toBe('/api/v1/dash/users')
  })

  it('login sends token null and posts body', async () => {
    storage.setToken('existing')
    await dash.login('alice', 'pw')
    const [url, init] = vi.mocked(globalThis.fetch).mock.calls[0]
    expect(url).toBe('/api/v1/dash/login')
    expect(init?.method).toBe('POST')
    expect(init?.headers).toMatchObject({ 'Content-Type': 'application/json' })
    expect(init?.body).toBe(JSON.stringify({ username: 'alice', password: 'pw' }))
  })

  it('uses noActAs for platform-admin paths', async () => {
    storage.setToken('t')
    storage.setActAsTenant(5)
    await dash.listTenants()
    const [, init] = vi.mocked(globalThis.fetch).mock.calls[0]
    expect(init?.headers).toEqual({ Authorization: 'Bearer t' })
  })

  it('passes subject_user_id and page for audit', async () => {
    await dash.listAudit(10, 3)
    const [url] = vi.mocked(globalThis.fetch).mock.calls[0]
    expect(url).toContain('subject_user_id=10')
    expect(url).toContain('page=3')
  })
})
