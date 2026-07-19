import { describe, expect, it } from 'vitest';
import type { CurrentUser } from '../types';
import { canManagePlatformUsers } from './permissions';

function user(role: string): CurrentUser {
  return { id: '1', name: '测试用户', username: 'tester', role };
}

describe('platform permissions', () => {
  it('allows platform administrators to manage users', () => {
    expect(canManagePlatformUsers(user('PLATFORM_ADMIN'))).toBe(true);
    expect(canManagePlatformUsers(user(' platform_admin '))).toBe(true);
  });

  it('denies operators and unknown users', () => {
    expect(canManagePlatformUsers(user('PLATFORM_OPERATOR'))).toBe(false);
    expect(canManagePlatformUsers(null)).toBe(false);
  });
});
