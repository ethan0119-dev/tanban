export type NotificationCategory = 'SYSTEM_UPDATE' | 'BUG_FIX' | 'NEW_FEATURE' | 'NOTICE' | 'ACTION_REQUIRED';
export type NotificationSeverity = 'INFO' | 'IMPORTANT' | 'URGENT';

export interface MerchantNotification {
  id: string;
  title: string;
  summary: string;
  content: string;
  category: NotificationCategory;
  severity: NotificationSeverity;
  publishedAt: string;
  readAt?: string;
  isRead: boolean;
}
