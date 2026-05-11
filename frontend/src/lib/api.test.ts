import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

describe('API client', () => {
  let get: typeof import('@/lib/api').get;
  let post: typeof import('@/lib/api').post;

  beforeEach(async () => {
    vi.restoreAllMocks();
    vi.resetModules();
    // Dynamic import after resetModules ensures fresh module state
    const mod = await import('@/lib/api');
    get = mod.get;
    post = mod.post;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('get makes GET request with correct URL', async () => {
    const mockResponse = {
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          success: true,
          data: { id: '1', name: 'Test' },
        }),
    };
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(mockResponse as Response);

    const result = await get<{ id: string; name: string }>('/test');

    expect(fetch).toHaveBeenCalledTimes(1);
    const [url, options] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toContain('/api/v1/test');
    expect(options.method).toBe('GET');
    expect(options.credentials).toBe('include');
    expect(result).toEqual({ id: '1', name: 'Test' });
  });

  it('post sends JSON body with Content-Type header', async () => {
    const mockResponse = {
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          success: true,
          data: { token: 'abc' },
        }),
    };
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(mockResponse as Response);

    const body = { email: 'test@example.com', password: 'pass' };
    await post('/auth/login', body);

    const [, options] = (fetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(options.method).toBe('POST');
    expect(options.headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(options.body as string)).toEqual(body);
  });

  it('request returns unwrapped data from envelope', async () => {
    const mockResponse = {
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          success: true,
          data: { items: [1, 2, 3] },
        }),
    };
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(mockResponse as Response);

    const result = await get<{ items: number[] }>('/items');
    expect(result).toEqual({ items: [1, 2, 3] });
  });

  it('401 triggers token refresh and retry', async () => {
    const retryResponse = {
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          success: true,
          data: { refreshed: true },
        }),
    };
    const failResponse = {
      ok: false,
      status: 401,
      json: () =>
        Promise.resolve({
          error: { code: 'UNAUTHORIZED', message: 'Token expired' },
        }),
    };
    const refreshResponse = { ok: true, status: 200 };

    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(failResponse as Response) // GET /data → 401
      .mockResolvedValueOnce(refreshResponse as Response) // POST /auth/refresh → 200
      .mockResolvedValueOnce(retryResponse as Response); // GET /data retry → 200

    const result = await get<{ refreshed: boolean }>('/data');

    expect(fetchSpy).toHaveBeenCalledTimes(3);
    const [retryUrl] = (fetchSpy as ReturnType<typeof vi.fn>).mock.calls[2];
    expect(retryUrl).toContain('/api/v1/data');
    expect(result).toEqual({ refreshed: true });
  });

  it('network error throws NETWORK_ERROR ApiClientError', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new TypeError('Failed to fetch'));

    await expect(get('/fail-not-timeout')).rejects.toMatchObject({
      code: 'NETWORK_ERROR',
    });
  });

  it('throws TIMEOUT when mutation request is aborted', async () => {
    const abortError = new Error('The operation was aborted');
    abortError.name = 'AbortError';
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(abortError);

    await expect(post('/slow-endpoint', { data: 'test' })).rejects.toMatchObject({
      code: 'TIMEOUT',
    });
  });



  it('GET dedup works for same URL without signal', async () => {
    const mockResponse = {
      ok: true,
      status: 200,
      json: () =>
        Promise.resolve({
          success: true,
          data: { items: [1] },
        }),
    };
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(mockResponse as Response);

    // Fire two concurrent GETs for the same URL
    const p1 = get('/dedup');
    const p2 = get('/dedup');

    const [r1, r2] = await Promise.all([p1, p2]);

    // Only one fetch call should have been made
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(r1).toEqual({ items: [1] });
    expect(r2).toEqual({ items: [1] });
  });
});
