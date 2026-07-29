import { describe, expect, it } from 'vitest';
import type { MerchantUser } from '../types';
import {
  assignableMerchantRoles,
  canManageMerchant,
  canManageStaffRole,
  canViewMerchantFinancials,
  capabilitiesForMerchantRole,
  hasMerchantCapability,
  isMerchantStaff,
  MERCHANT_ROLE_CAPABILITIES,
  normalizeMerchantRole,
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

  it('grants refunds to owners and managers but not staff', () => {
    expect(hasMerchantCapability(user('MERCHANT_OWNER'), 'CREATE_REFUNDS')).toBe(true);
    expect(hasMerchantCapability(user('MERCHANT_MANAGER'), 'CREATE_REFUNDS')).toBe(true);
    expect(hasMerchantCapability(user('MERCHANT_STAFF'), 'CREATE_REFUNDS')).toBe(false);
  });

  it('keeps sensitive customer money operations owner-only', () => {
    const ownerOnlyCapabilities = [
      'ARCHIVE_CUSTOMERS',
      'ADJUST_CUSTOMER_BALANCE',
      'CREATE_STORED_VALUE_RECORD',
    ] as const;

    for (const capability of ownerOnlyCapabilities) {
      expect(hasMerchantCapability(user('MERCHANT_OWNER'), capability)).toBe(true);
      expect(hasMerchantCapability(user('MERCHANT_MANAGER'), capability)).toBe(false);
      expect(hasMerchantCapability(user('MERCHANT_STAFF'), capability)).toBe(false);
    }
  });

  it('uses the centralized matrix for the role permission list', () => {
    expect(capabilitiesForMerchantRole('MERCHANT_MANAGER')).toBe(MERCHANT_ROLE_CAPABILITIES.MERCHANT_MANAGER);
    expect(capabilitiesForMerchantRole('manager')).toBe(MERCHANT_ROLE_CAPABILITIES.MERCHANT_MANAGER);
    expect(capabilitiesForMerchantRole('UNKNOWN')).toEqual([]);
  });

  it('uses capabilities returned by the API as the authority', () => {
    const managerWithRestrictedCapabilities = {
      ...user('MERCHANT_MANAGER'),
      capabilities: ['VIEW_DASHBOARD'],
    };
    expect(hasMerchantCapability(managerWithRestrictedCapabilities, 'VIEW_DASHBOARD')).toBe(true);
    expect(hasMerchantCapability(managerWithRestrictedCapabilities, 'CREATE_REFUNDS')).toBe(false);
    expect(canManageMerchant(managerWithRestrictedCapabilities)).toBe(false);
  });

  it('normalizes the primary role and fails closed when role is missing or unknown', () => {
    expect(primaryMerchantRole(user(' merchant_manager '))).toBe('MERCHANT_MANAGER');
    expect(normalizeMerchantRole(' manager ')).toBe('MERCHANT_MANAGER');
    expect(canManageMerchant(user())).toBe(false);
    expect(canManageMerchant(user('UNKNOWN'))).toBe(false);
    expect(canViewMerchantFinancials(user('UNKNOWN'))).toBe(false);
    expect(hasMerchantCapability(user('UNKNOWN'), 'MANAGE_ORDERS')).toBe(false);
  });
});
