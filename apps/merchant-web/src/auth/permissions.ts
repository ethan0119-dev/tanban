import type { MerchantUser } from '../types';

const STAFF_ROLES = new Set(['STAFF', 'MERCHANT_STAFF']);
const MANAGER_ROLES = new Set(['MANAGER', 'MERCHANT_MANAGER']);
const OWNER_ROLES = new Set(['OWNER', 'MERCHANT_OWNER']);

export function primaryMerchantRole(user: MerchantUser | null | undefined): string {
  return String(user?.roles?.[0] ?? '').trim().toUpperCase();
}

export function isMerchantStaff(user: MerchantUser | null | undefined): boolean {
  return STAFF_ROLES.has(primaryMerchantRole(user));
}

export function canManageMerchant(user: MerchantUser | null | undefined): boolean {
  const role = primaryMerchantRole(user);
  return OWNER_ROLES.has(role) || MANAGER_ROLES.has(role);
}

export function canViewMerchantFinancials(user: MerchantUser | null | undefined): boolean {
  return canManageMerchant(user);
}

export function isMerchantManager(user: MerchantUser | null | undefined): boolean {
  return MANAGER_ROLES.has(primaryMerchantRole(user));
}

export function isMerchantOwner(user: MerchantUser | null | undefined): boolean {
  return OWNER_ROLES.has(primaryMerchantRole(user));
}

export function assignableMerchantRoles(user: MerchantUser | null | undefined): string[] {
  return isMerchantManager(user)
    ? ['MERCHANT_STAFF']
    : ['MERCHANT_OWNER', 'MERCHANT_MANAGER', 'MERCHANT_STAFF'];
}

export function canManageStaffRole(user: MerchantUser | null | undefined, targetRole: string): boolean {
  if (isMerchantManager(user)) return STAFF_ROLES.has(String(targetRole).trim().toUpperCase());
  return isMerchantOwner(user);
}
