import { describe, expect, it } from 'vitest';
import type { MerchantUser } from '../types';
import {
  assignableMerchantRoles,
  canManageMerchant,
  canManageStaffRole,
  canViewMerchantFinancials,
  isMerchantStaff,
  primaryMerchantRole,
} from './permissions';

function user(role?: string): MerchantUser {
  return { id: 1, name: '测试用户', roles: role ? [role] : undefined };
}

describe('merchant role permissions', () => {
  it.each(['STAFF', 'MERCHANT_STAFF', 'merchant_staff'])('recognizes %s as a staff role', (role) => {
    expect(isMerchantStaff(user(role))).toBe(true);
    expect(canManageMerchant(user(role))).toBe(false);
    expect(canViewMerchantFinancials(user(role))).toBe(false);
  });

  it('limits managers to creating and editing staff accounts', () => {
    const manager = user('MERCHANT_MANAGER');
    expect(assignableMerchantRoles(manager)).toEqual(['MERCHANT_STAFF']);
    expect(canManageStaffRole(manager, 'MERCHANT_STAFF')).toBe(true);
    expect(canManageStaffRole(manager, 'MERCHANT_MANAGER')).toBe(false);
    expect(canManageStaffRole(manager, 'MERCHANT_OWNER')).toBe(false);
  });

  it('keeps owner employee-management options', () => {
    const owner = user('MERCHANT_OWNER');
    expect(assignableMerchantRoles(owner)).toEqual(['MERCHANT_OWNER', 'MERCHANT_MANAGER', 'MERCHANT_STAFF']);
    expect(canManageStaffRole(owner, 'MERCHANT_MANAGER')).toBe(true);
  });

  it.each(['MERCHANT_OWNER', 'MERCHANT_MANAGER'])('keeps management access for %s', (role) => {
    expect(isMerchantStaff(user(role))).toBe(false);
    expect(canManageMerchant(user(role))).toBe(true);
  });

  it('normalizes the primary role without changing legacy users with no role', () => {
    expect(primaryMerchantRole(user(' merchant_manager '))).toBe('MERCHANT_MANAGER');
    expect(canManageMerchant(user())).toBe(true);
  });
});
