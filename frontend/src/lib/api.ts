import { API_BASE_URL } from '@/lib/constants';
import type { ApiError } from '@/types/auth';

class ApiClientError extends Error {
  code: string;
  details?: Record<string, string>;

  constructor(code: string, message: string, details?: Record<string, string>) {
    super(message);
    this.name = 'ApiClientError';
    this.code = code;
    this.details = details;
  }
}

// ── Token refresh ──────────────────────────────────────────────────

let refreshPromise: Promise<boolean> | null = null;

/**
 * Attempt to refresh the access token via HttpOnly refresh_token cookie.
 * The browser auto-sends the refresh_token cookie on this endpoint;
 * the backend rotates it and sets new access_token + refresh_token cookies.
 *
 * Uses a module-level promise to deduplicate concurrent refresh attempts.
 */
async function tryRefreshToken(): Promise<boolean> {
  if (refreshPromise) return refreshPromise;

  refreshPromise = (async () => {
    try {
      const res = await fetch(`${API_BASE_URL}/auth/refresh`, {
        method: 'POST',
        credentials: 'include',
      });
      return res.ok;
    } catch {
      return false;
    } finally {
      refreshPromise = null;
    }
  })();

  return refreshPromise;
}

// ── GET deduplication ─────────────────────────────────────────────

const pendingRequests = new Map<string, Promise<unknown>>();

// ── Response handling ──────────────────────────────────────────────

const MUTATION_TIMEOUT = 30_000; // 30 seconds

export async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  signal?: AbortSignal | null,
): Promise<T> {
  const url = `${API_BASE_URL}${path}`;
  const key = `${method}:${url}`;

  // Deduplicate concurrent GET requests to the same URL
  if (method === 'GET' && pendingRequests.has(key)) {
    return pendingRequests.get(key) as Promise<T>;
  }

  const promise = doRequest<T>(method, url, body, signal);

  if (method === 'GET') {
    pendingRequests.set(key, promise);
    promise.finally(() => pendingRequests.delete(key));
  }

  return promise;
}

async function doRequest<T>(
  method: string,
  url: string,
  body?: unknown,
  signal?: AbortSignal | null,
): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
  }

  // Add AbortController timeout for mutations (POST, PUT, DELETE)
  let controller: AbortController | undefined;
  let timeoutId: ReturnType<typeof setTimeout> | undefined;

  if (method !== 'GET') {
    controller = new AbortController();
    timeoutId = setTimeout(() => controller!.abort(), MUTATION_TIMEOUT);
  }

  const signals = [signal, controller?.signal].filter((s): s is AbortSignal => s != null);
  const combinedSignal = signals.length > 0
    ? AbortSignal.any(signals)
    : undefined;

  try {
    let response = await fetch(url, {
      method,
      headers,
      credentials: 'include',
      body: body !== undefined ? JSON.stringify(body) : undefined,
      signal: combinedSignal,
    });

    // Auto-refresh on 401: attempt to rotate tokens, then retry once
    if (response.status === 401) {
      const refreshed = await tryRefreshToken();
      if (refreshed) {
        response = await fetch(url, {
          method,
          headers,
          credentials: 'include',
          body: body !== undefined ? JSON.stringify(body) : undefined,
          signal: combinedSignal,
        });
      }
    }

    return handleResponse<T>(response);
  } finally {
    if (timeoutId !== undefined) {
      clearTimeout(timeoutId);
    }
  }
}

async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let errorData: ApiError;
    try {
      const body = await response.json();
      errorData = body.error ?? body as ApiError;
    } catch {
      errorData = {
        code: 'UNKNOWN',
        message: `Request failed with status ${response.status}`,
      };
    }
    throw new ApiClientError(
      errorData.code,
      errorData.message,
      errorData.details,
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const body = await response.json();
  // Unwrap from { success, data } envelope
  if (body && typeof body === 'object' && 'data' in body) {
    return body.data as T;
  }
  return body as T;
}

export function get<T>(path: string, signal?: AbortSignal | null): Promise<T> {
  return request<T>('GET', path, undefined, signal);
}

export function post<T>(path: string, body?: unknown, signal?: AbortSignal | null): Promise<T> {
  return request<T>('POST', path, body, signal);
}

export function put<T>(path: string, body?: unknown, signal?: AbortSignal | null): Promise<T> {
  return request<T>('PUT', path, body, signal);
}

export function del<T>(path: string, signal?: AbortSignal | null): Promise<T> {
  return request<T>('DELETE', path, undefined, signal);
}
