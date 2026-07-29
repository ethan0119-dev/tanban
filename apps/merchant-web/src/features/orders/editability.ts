import type { Order, OrderStatus, TableBoardTable } from '../../types';

const payAfterEditableStatuses = new Set<OrderStatus>(['PAID', 'ACCEPTED', 'PREPARING', 'READY']);

type EditableOrder = Pick<Order, 'orderType' | 'settlementMode' | 'paymentStatus' | 'paidAmount' | 'status' | 'canAddItems'>;

function locallyAllowsItemChanges(order: EditableOrder): boolean {
  if (order.orderType !== 'DINE_IN' || order.paymentStatus !== 'UNPAID' || Number(order.paidAmount || 0) > 0) {
    return false;
  }
  if (order.settlementMode === 'PAY_BEFORE') {
    return order.status === 'PENDING_PAYMENT';
  }
  return order.settlementMode === 'PAY_AFTER' && payAfterEditableStatuses.has(order.status);
}

export function canAddItemsToOrder(order?: EditableOrder | null): boolean {
  if (!order) return false;
  if (order.canAddItems !== undefined) return order.canAddItems;
  return locallyAllowsItemChanges(order);
}

export function canReturnItemsFromOrder(order?: EditableOrder | null): boolean {
  return canAddItemsToOrder(order);
}

export function canOperateUnpaidPayAfterOrder(order?: EditableOrder | null): boolean {
  return Boolean(order && order.settlementMode === 'PAY_AFTER' && canAddItemsToOrder(order));
}

export function isMergeableTableBill(table: TableBoardTable, selectedOrderID?: string | number): boolean {
  return Boolean(
    table.orderId
    && String(table.orderId) !== String(selectedOrderID ?? '')
    && table.settlementMode === 'PAY_AFTER'
    && table.paymentStatus === 'UNPAID'
    && Number(table.paidCents || 0) === 0
    && !table.paymentLocked
    && ['PAID', 'ACCEPTED', 'PREPARING', 'READY'].includes(String(table.orderStatus || '')),
  );
}

export function addItemsBlockedReason(order?: EditableOrder | null): string {
  if (!order) return '请先选择堂食订单';
  if (order.orderType !== 'DINE_IN') return '带走订单每次独立结算，如需追加请新建带走单';
  if (order.paymentStatus === 'PENDING') return '顾客正在支付，请确认支付结果后再操作';
  if (order.paymentStatus !== 'UNPAID' || Number(order.paidAmount || 0) > 0) {
    return '订单已开始收款；如需调整已付款商品，请通过退款流程处理';
  }
  if (order.canAddItems === false && locallyAllowsItemChanges(order)) {
    return '订单已经进入支付流程，请确认支付结果后再操作';
  }
  if (order.settlementMode === 'PAY_BEFORE') {
    return order.status === 'PENDING_PAYMENT'
      ? ''
      : '先结账订单付款后不能加菜，如需追加请重新下单';
  }
  if (order.settlementMode === 'PAY_AFTER' && !payAfterEditableStatuses.has(order.status)) {
    return '订单已经完成或关闭，不能继续加菜';
  }
  return '当前订单状态不能继续加菜';
}

export function dineInOperationBlockedReason(
  operation: 'TRANSFER' | 'MERGE' | 'RETURN',
  order?: EditableOrder | null,
): string {
  const label = operation === 'TRANSFER' ? '转台' : operation === 'MERGE' ? '并台' : '退菜';
  if (!order) return `请先选择需要${label}的堂食订单`;
  if (order.orderType !== 'DINE_IN') return `带走订单不支持${label}`;
  if (operation === 'RETURN') return addItemsBlockedReason(order).replaceAll('加菜', '退菜');
  if (order.settlementMode !== 'PAY_AFTER') return `${label}仅支持“先用餐后结账”的堂食订单`;
  if (order.paymentStatus !== 'UNPAID' || Number(order.paidAmount || 0) > 0) {
    return `订单已开始收款，不能${label}`;
  }
  if (order.canAddItems === false) return `订单已经进入支付流程，请确认支付结果后再${label}`;
  if (!payAfterEditableStatuses.has(order.status)) return `订单已经完成或关闭，不能${label}`;
  return '';
}
