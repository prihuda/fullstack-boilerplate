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

// ── Response handling ──────────────────────────────────────────────

function buildHeaders(): HeadersInit {
  return {
    'Content-Type': 'application/json',
  };
}

async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let errorData: ApiError;
    try {
      errorData = (await response.json()) as ApiError;
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

  return response.json() as Promise<T>;
}

export async function request<T>(
  method: string,
  path: string,
  body?: unknown,
  signal?: AbortSignal | null,
): Promise<T> {
  const url = `${API_BASE_URL}${path}`;
  const options: RequestInit = {
    method,
    headers: buildHeaders(),
    credentials: 'include',
    signal: signal ?? undefined,
  };

  if (body !== undefined) {
    options.body = JSON.stringify(body);
  }

  let response = await fetch(url, options);

  // Auto-refresh on 401: attempt to rotate tokens, then retry once
  if (response.status === 401) {
    const refreshed = await tryRefreshToken();
    if (refreshed) {
      response = await fetch(url, options);
    }
  }

  return handleResponse<T>(response);
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
