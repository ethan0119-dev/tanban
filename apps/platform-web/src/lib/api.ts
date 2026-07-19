import { AUTH_UNAUTHORIZED_EVENT, clearToken, getToken } from './auth-storage';
import type { PageMeta, PageResult, QueryParams } from '../types';

const DEFAULT_API_BASE_URL = 'https://tbapi.666qwe.cn/api/v1';
export const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || DEFAULT_API_BASE_URL).replace(/\/$/, '');

export interface ApiEnvelope<T> {
  data?: T;
  meta?: Partial<PageMeta> & Record<string, unknown>;
  error?: string | { code?: string; message?: string };
  message?: string;
  code?: number | string;
}

export class ApiError extends Error {
  status: number;
  code?: string;
  details?: unknown;

  constructor(message: string, status = 0, code?: string, details?: unknown) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

function toQuery(params?: QueryParams): string {
  if (!params) return '';
  const search = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== '' && value !== null) search.set(key, String(value));
  });
  const text = search.toString();
  return text ? `?${text}` : '';
}

function getErrorMessage(payload: ApiEnvelope<unknown> | undefined, fallback: string): string {
  if (!payload) return fallback;
  if (typeof payload.error === 'string') return payload.error;
  if (payload.error?.message) return payload.error.message;
  if (payload.message) return payload.message;
  return fallback;
}

export async function request<T>(
  path: string,
  options: RequestInit & { params?: QueryParams } = {},
): Promise<{ data: T; meta?: ApiEnvelope<T>['meta'] }> {
  const token = getToken();
  const headers = new Headers(options.headers);
  headers.set('Accept', 'application/json');
  if (options.body && !(options.body instanceof FormData)) headers.set('Content-Type', 'application/json');
  if (token) headers.set('Authorization', `Bearer ${token}`);

  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}${toQuery(options.params)}`, {
      ...options,
      headers,
    });
  } catch (error) {
    throw new ApiError('网络连接失败，请检查服务状态后重试', 0, 'NETWORK_ERROR', error);
  }

  let payload: ApiEnvelope<T> | T | undefined;
  if (response.status !== 204) {
    try {
      payload = (await response.json()) as ApiEnvelope<T> | T;
    } catch {
      payload = undefined;
    }
  }

  if (response.status === 401) {
    clearToken();
    window.dispatchEvent(new Event(AUTH_UNAUTHORIZED_EVENT));
  }

  const envelope = payload as ApiEnvelope<T> | undefined;
  if (!response.ok || envelope?.error) {
    const errorCode = typeof envelope?.error === 'object' ? envelope.error.code : undefined;
    throw new ApiError(getErrorMessage(envelope, `请求失败（${response.status}）`), response.status, errorCode, payload);
  }

  if (payload && typeof payload === 'object' && 'data' in payload) {
    return { data: (payload as ApiEnvelope<T>).data as T, meta: (payload as ApiEnvelope<T>).meta };
  }
  return { data: payload as T };
}

export function normalizePage<T>(
  data: unknown,
  meta?: Partial<PageMeta> & Record<string, unknown>,
  fallbackPage = 1,
  fallbackPageSize = 20,
): PageResult<T> {
  const record = data && typeof data === 'object' ? (data as Record<string, unknown>) : {};
  const items = Array.isArray(data)
    ? data
    : ([record.items, record.list, record.records, record.rows].find(Array.isArray) as T[] | undefined) || [];
  const embeddedMeta = (record.meta || record.pagination || {}) as Record<string, unknown>;
  const total = Number(meta?.total ?? embeddedMeta.total ?? record.total ?? items.length);
  const page = Number(meta?.page ?? embeddedMeta.page ?? record.page ?? fallbackPage);
  const pageSize = Number(
    meta?.pageSize ?? meta?.page_size ?? embeddedMeta.pageSize ?? embeddedMeta.page_size ?? embeddedMeta.perPage ?? record.pageSize ?? record.page_size ?? fallbackPageSize,
  );
  return { items: items as T[], meta: { page, pageSize, total } };
}

export const http = {
  get: <T>(path: string, params?: QueryParams) => request<T>(path, { method: 'GET', params }),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: body === undefined ? undefined : JSON.stringify(body) }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PUT', body: body === undefined ? undefined : JSON.stringify(body) }),
  patch: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PATCH', body: body === undefined ? undefined : JSON.stringify(body) }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
};
