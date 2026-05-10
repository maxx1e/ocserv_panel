export class ApiError extends Error {
  status: number;
  code?: string;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

type RequestOptions = Omit<RequestInit, 'body' | 'credentials'> & {
  body?: unknown;
  timeout?: number; // ms, default 15000
};

export async function apiFetch<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { timeout = 15000, ...rest } = options;
  const headers = new Headers(rest.headers);
  let body: BodyInit | undefined;

  if (rest.body !== undefined) {
    headers.set('content-type', 'application/json');
    body = JSON.stringify(rest.body);
  }

  // Set up abort signal with timeout. Use AbortSignal.timeout when available,
  // otherwise fall back to AbortController + setTimeout.
  let signal: AbortSignal;
  let fallbackTimer: ReturnType<typeof setTimeout> | undefined;
  if (typeof AbortSignal.timeout === 'function') {
    signal = AbortSignal.timeout(timeout);
  } else {
    const controller = new AbortController();
    signal = controller.signal;
    fallbackTimer = setTimeout(() => controller.abort(), timeout);
  }

  let response: Response;
  try {
    response = await fetch(path, {
      ...rest,
      headers,
      body,
      credentials: 'same-origin',
      signal
    });
  } catch (err: unknown) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new ApiError('Request timed out — the backend may be unreachable.', 0);
    }
    // Newer browsers throw a TimeoutError for AbortSignal.timeout
    if (err instanceof DOMException && err.name === 'TimeoutError') {
      throw new ApiError('Request timed out — the backend may be unreachable.', 0);
    }
    throw new ApiError('Network error — unable to reach the backend.', 0);
  } finally {
    if (fallbackTimer !== undefined) {
      clearTimeout(fallbackTimer);
    }
  }

  if (!response.ok) {
    const structuredError = await safeStructuredError(response);
    throw new ApiError(
      structuredError?.message ?? safeErrorMessage(path, response.status),
      response.status,
      structuredError?.code
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

type StructuredError = {
  message: string;
  code?: string;
};

async function safeStructuredError(response: Response): Promise<StructuredError | null> {
  const contentType = response.headers.get('content-type') ?? '';
  if (!contentType.toLowerCase().includes('application/json')) return null;

  try {
    const payload: unknown = await response.json();
    if (!isPlainRecord(payload)) return null;

    const parts = ['error', 'message', 'details']
      .map((field) => safeErrorField(payload[field]))
      .filter((value): value is string => Boolean(value));
    const code = safeErrorField(payload.code);

    if (parts.length === 0 && !code) return null;
    const message = parts.length > 0 ? parts.join(' ') : `Backend error code: ${code}`;
    return code ? { message, code } : { message };
  } catch {
    return null;
  }
}

function isPlainRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function safeErrorField(value: unknown): string | null {
  if (typeof value !== 'string') return null;

  const trimmed = value.trim();
  if (!trimmed || trimmed.length > 300) return null;
  if (/password|token|secret|private\s*key|credential|hash/i.test(trimmed)) return null;

  return trimmed;
}

function safeErrorMessage(path: string, status: number): string {
  if (status === 401) return 'Sign in is required or the credentials were not accepted.';
  if (status === 403) return 'Your role is not allowed to perform this action.';
  if (status === 404) return 'The requested backend endpoint was not found.';
  if (status >= 500) return 'The backend could not complete the request. Check service logs for details.';
  if (path.includes('/history/purge')) return 'History purge validation failed. Check the retention window and confirmation.';
  if (path.includes('/setup/owner')) return 'Owner setup failed. Check the username and password requirements.';
  if (path.includes('/auth/login')) return 'Login failed. Check the username and password.';
  return 'The backend rejected the request.';
}
