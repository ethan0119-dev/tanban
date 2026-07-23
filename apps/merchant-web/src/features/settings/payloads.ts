import type { MerchantOperationSettings, MerchantSettings } from '../../types';

export function operationSettingsWritePayload(
  settings: MerchantOperationSettings,
  patch: Partial<MerchantOperationSettings>,
): MerchantOperationSettings {
  const definedPatch = Object.fromEntries(
    Object.entries(patch).filter(([, value]) => value !== undefined),
  ) as Partial<MerchantOperationSettings>;
  return { ...settings, ...definedPatch };
}

export function merchantSettingsWritePayload(
  settings: MerchantSettings,
  patch: Partial<MerchantSettings> = {},
) {
  const value = { ...settings, ...patch };
  return {
    storeName: value.storeName,
    logo: value.logo || '',
    phone: value.phone || '',
    address: value.address || '',
    announcement: value.announcement || '',
    businessHours: value.businessHours,
    autoAcceptOrder: value.autoAcceptOrder,
    orderVoiceReminder: value.orderVoiceReminder,
    printTrigger: value.printTrigger,
    autoPrintReceipt: value.autoPrintReceipt,
    autoPrintLabel: value.autoPrintLabel,
    pickupMode: value.pickupMode,
    allowLatePayment: value.allowLatePayment,
    paymentTimeoutMinutes: value.paymentTimeoutMinutes,
  };
}

export function memberLevelOrderWritePayload(values: {
  customer_id: string | number;
  level_id: string | number;
  amount: number;
  payment_method: string;
  status: string;
  remark?: string;
}) {
  return {
    customer_id: values.customer_id,
    level_id: values.level_id,
    amount_cents: Math.round(values.amount * 100),
    payment_method: values.payment_method,
    status: values.status,
    remark: values.remark,
  };
}
