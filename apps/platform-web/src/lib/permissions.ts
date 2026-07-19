import type { CurrentUser } from '../types';

export function canManagePlatformUsers(user: CurrentUser | null | undefined): boolean {
  return String(user?.role ?? '').trim().toUpperCase() === 'PLATFORM_ADMIN';
}
