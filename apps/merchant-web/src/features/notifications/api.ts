import { api } from '../../api/client';
import type { ListResult } from '../../types';
import type { MerchantNotification } from './types';

export const NOTIFICATIONS_CHANGED_EVENT = 'tanban:merchant:notifications-changed';

type RawRecord = Record<string, unknown>;

function toNotification(value: unknown): MerchantNotification {
  const item = (value && typeof value === 'object' ? value : {}) as RawRecord;
  return {
    id: String(item.id || ''),
    title: String(item.title || ''),
    summary: String(item.summary || ''),
    content: String(item.content || ''),
    category: String(item.category || 'NOTICE').toUpperCase() as MerchantNotification['category'],
    severity: String(item.severity || 'INFO').toUpperCase() as MerchantNotification['severity'],
    publishedAt: String(item.published_at ?? item.publishedAt ?? ''),
    readAt: String(item.read_at ?? item.readAt ?? '') || undefined,
    isRead: Boolean(item.is_read ?? item.isRead ?? item.read_at ?? item.readAt),
  };
}

function notifyChanged() {
  window.dispatchEvent(new Event(NOTIFICATIONS_CHANGED_EVENT));
}

export const notificationApi = {
  list: async (params: { page: number; page_size: number; unread_only?: boolean }): Promise<ListResult<MerchantNotification>> => {
    const result = await api.getList<RawRecord>('/merchant/notifications', params);
    return { items: result.items.map(toNotification), meta: result.meta };
  },
  unreadCount: async (): Promise<number> => {
    const result = await api.get<{ count: number }>('/merchant/notifications/unread-count');
    return Number(result.count || 0);
  },
  get: async (id: string): Promise<MerchantNotification> => toNotification(await api.get<RawRecord>(`/merchant/notifications/${id}`)),
  markRead: async (id: string): Promise<MerchantNotification> => {
    const result = toNotification(await api.post<RawRecord>(`/merchant/notifications/${id}/read`));
    notifyChanged();
    return result;
  },
  markAllRead: async (): Promise<number> => {
    const result = await api.post<{ read_count: number }>('/merchant/notifications/read-all');
    notifyChanged();
    return Number(result.read_count || 0);
  },
};
