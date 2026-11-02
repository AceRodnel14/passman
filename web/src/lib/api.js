export class ApiError extends Error {
  constructor(message, status, data, retryAfter = 0) {
    super(message);
    this.status = status;
    this.data = data;
    this.retryAfter = retryAfter; // seconds, from the Retry-After header (0 when absent)
  }
}

let unauthorizedHandler = () => {};
export function onUnauthorized(fn) {
  unauthorizedHandler = fn;
}

export async function api(method, url, body) {
  const opts = { method, headers: {}, credentials: 'same-origin' };
  if (body instanceof FormData) {
    opts.body = body;
  } else if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  let res;
  try {
    res = await fetch(url, opts);
  } catch {
    throw new ApiError('Cannot reach the passman server.', 0, null);
  }
  if (res.status === 204) return null;
  const isJSON = (res.headers.get('content-type') || '').includes('application/json');
  const data = isJSON ? await res.json() : await res.blob();
  if (!res.ok) {
    if (res.status === 401 && !url.startsWith('/api/login')) unauthorizedHandler();
    throw new ApiError(data?.error || `Request failed (${res.status})`, res.status, data, Number(res.headers.get('Retry-After')) || 0);
  }
  return data;
}
