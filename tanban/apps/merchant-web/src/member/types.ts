import type { Id } from '../types';

export interface CustomerTag {
  id: Id;
  name: string;
  color: string;
  description?: string;
  status: string;
  customer_count?: number;
}

export interface Customer {
  id: Id;
  public_id: string;
  name: string;
  phone?: string;
  phone_masked?: string;
  avatar_url?: string;
  source: string;
  status: string;
  remark?: string;
  source_store_name?: string;
  source_store_id?: Id;
  member_id?: Id;
  member_no?: string;
  member_status?: string;
  level_id?: Id;
  level_name?: string;
  growth_value?: number;
  principal_cents: number;
  bonus_cents: number;
  balance_cents: number;
  order_count: number;
  net_spent_cents: number;
  refunded_cents?: number;
  registered_at: string;
  last_seen_at?: string;
  joined_at?: string;
  expires_at?: string;
  tags?: CustomerTag[];
}

export interface MemberSummary {
  customer_count: number;
  member_count: number;
  balance_cents: number;
  blocked_customer_count: number;
  stored_value_principal_cents: number;
  stored_value_gift_cents: number;
  stored_value_customer_count: number;
  active_stored_value_rule_count: number;
}

export interface MemberLevel {
  id: Id;
  name: string;
  rank_no: number;
  acquire_type: 'FREE' | 'GROWTH' | 'PAID';
  growth_threshold: number;
  price_cents: number;
  valid_days: number;
  benefits: Record<string, unknown>;
  upgrade_gift: Record<string, unknown>;
  is_default: boolean;
  status: string;
  member_count?: number;
}

export interface MembershipSettings {
  enabled: boolean;
  card_name: string;
  card_color: string;
  card_image_url?: string;
  auto_enroll: boolean;
  default_level_id?: Id;
  growth_per_yuan: number;
  agreement_url?: string;
  show_balance: boolean;
}

export interface CardIssuance {
  id: Id;
  issue_no: string;
  customer_id: Id;
  customer_name: string;
  member_no: string;
  level_name?: string;
  issue_source: string;
  status: string;
  valid_from: string;
  valid_to?: string;
  created_at: string;
}

export interface MemberLevelOrder {
  id: Id;
  order_no: string;
  customer_id: Id;
  customer_name: string;
  level_name: string;
  amount_cents: number;
  payment_method: string;
  payment_status: string;
  status: string;
  remark?: string;
  created_at: string;
}

export interface BalanceLedger {
  id: Id;
  customer_id: Id;
  customer_name: string;
  bucket: 'PRINCIPAL' | 'BONUS';
  delta_cents: number;
  balance_before_cents: number;
  balance_after_cents: number;
  entry_type: string;
  business_type: string;
  business_no?: string;
  remark?: string;
  operator_user_id?: Id;
  created_at: string;
}

export interface StoredValueRule {
  id: Id;
  name: string;
  recharge_cents: number;
  gift_cents: number;
  gift_growth: number;
  benefits: Record<string, unknown>;
  per_customer_limit: number;
  starts_at?: string;
  ends_at?: string;
  status: string;
}

export interface StoredValueSettings {
  enabled: boolean;
  min_recharge_cents: number;
  max_recharge_cents: number;
  max_balance_cents: number;
  deduction_order: 'BONUS_FIRST' | 'PRINCIPAL_FIRST';
  refund_policy: 'MANUAL_REVIEW' | 'REJECT_AFTER_USE';
  agreement_url?: string;
  show_in_miniapp: boolean;
}

export interface StoredValueRecord {
  id: Id;
  record_no: string;
  customer_id: Id;
  customer_name: string;
  rule_name?: string;
  principal_cents: number;
  gift_cents: number;
  payment_method: string;
  status: string;
  remark?: string;
  created_at: string;
}
