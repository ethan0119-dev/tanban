import axios, { AxiosError, type AxiosRequestConfig } from 'axios';
import type { ListResult, PageMeta } from '../types';

export const TOKEN_KEY = 'tanban_merchant_token';
export const AUTH_UNAUTHORIZED_EVENT = 'tanban:merchant:unauthorized';

const baseURL = (import.meta.env.VITE_API_BASE_URL || 'https://tbapi.666qwe.cn/api/v1').replace(/\/$/, '');

export interface ApiEnvelope<T> {
  data?: T;
  meta?: PageMeta;
  error?: string | { message?: string; code?: string } | null;
  message?: string;
  code?: string | number;
}

export class ApiError extends Error {
  code?: string | number;
  status?: number;

  constructor(message: string, options?: { code?: string | number; status?: number }) {
    super(message);
    this.name = 'ApiError';
    this.code = options?.code;
    this.status = options?.status;
  }
}

export function unwrapEnvelope<T>(payload: T | ApiEnvelope<T>): T {
  if (payload && typeof payload === 'object' && !Array.isArray(payload)) {
    const envelope = payload as ApiEnvelope<T>;
    if ('error' in envelope && envelope.error) {
      const message = typeof envelope.error === 'string'
        ? envelope.error
        : envelope.error.message || '请求失败';
      throw new ApiError(message, { code: typeof envelope.error === 'object' ? envelope.error.code : undefined });
    }
    if ('data' in envelope) return envelope.data as T;
  }
  return payload as T;
}

export function toListResult<T>(payload: unknown, meta?: PageMeta): ListResult<T> {
  const value = unwrapEnvelope(payload as never) as unknown;
  if (Array.isArray(value)) {
    return { items: value as T[], meta: meta ?? { total: value.length } };
  }
  if (value && typeof value === 'object') {
    const record = value as Record<string, unknown>;
    const items = (record.items ?? record.list ?? record.records ?? record.rows ?? []) as T[];
    const embeddedMeta = (record.meta ?? record.pagination ?? {}) as PageMeta;
    return {
      items: Array.isArray(items) ? items : [],
      meta: {
        ...meta,
        ...embeddedMeta,
        total: Number(embeddedMeta.total ?? record.total ?? meta?.total ?? (Array.isArray(items) ? items.length : 0)),
      },
    };
  }
  return { items: [], meta: meta ?? { total: 0 } };
}

const http = axios.create({
  baseURL,
  timeout: 15_000,
  headers: { 'Content-Type': 'application/json' },
});

http.interceptors.request.use((config) => {
  const token = localStorage.getItem(TOKEN_KEY);
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

http.interceptors.response.use(
  (response) => {
    const payload = response.data as ApiEnvelope<unknown>;
    if (payload && typeof payload === 'object' && payload.error) {
      const message = typeof payload.error === 'string'
        ? payload.error
        : payload.error.message || payload.message || '请求失败';
      throw new ApiError(message, {
        code: typeof payload.error === 'object' ? payload.error.code : payload.code,
        status: response.status,
      });
    }
    return response;
  },
  (error: AxiosError<ApiEnvelope<unknown>>) => {
    if (error.response?.status === 401) {
      localStorage.removeItem(TOKEN_KEY);
      window.dispatchEvent(new Event(AUTH_UNAUTHORIZED_EVENT));
    }
    const payload = error.response?.data;
    const serverError = payload?.error;
    const message = typeof serverError === 'string'
      ? serverError
      : serverError?.message || payload?.message || error.message || '网络异常，请稍后重试';
    return Promise.reject(new ApiError(message, {
      code: serverError && typeof serverError === 'object' ? serverError.code : payload?.code,
      status: error.response?.status,
    }));
  },
);

async function request<T>(config: AxiosRequestConfig): Promise<T> {
  const response = await http.request<ApiEnvelope<T> | T>(config);
  return unwrapEnvelope(response.data);
}

async function requestList<T>(url: string, params?: unknown): Promise<ListResult<T>> {
  const response = await http.request<ApiEnvelope<T[]> | T[]>({ method: 'GET', url, params });
  const payload = response.data;
  if (payload && typeof payload === 'object' && !Array.isArray(payload) && 'data' in payload) {
    const envelope = payload as ApiEnvelope<T[]>;
    const rawMeta = (envelope.meta ?? {}) as PageMeta & { page_size?: number };
    return toListResult<T>(envelope.data ?? [], { ...rawMeta, pageSize: rawMeta.pageSize ?? rawMeta.page_size });
  }
  return toListResult<T>(payload);
}

export const api = {
  get: <T>(url: string, params?: unknown) => request<T>({ method: 'GET', url, params }),
  getList: <T>(url: string, params?: unknown) => requestList<T>(url, params),
  post: <T>(url: string, data?: unknown) => request<T>({ method: 'POST', url, data }),
  postIdempotent: <T>(url: string, data: unknown, idempotencyKey: string) => request<T>({ method: 'POST', url, data, headers: { 'Idempotency-Key': idempotencyKey } }),
  put: <T>(url: string, data?: unknown) => request<T>({ method: 'PUT', url, data }),
  patch: <T>(url: string, data?: unknown) => request<T>({ method: 'PATCH', url, data }),
  delete: <T>(url: string) => request<T>({ method: 'DELETE', url }),
};

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : '操作失败，请稍后重试';
}
