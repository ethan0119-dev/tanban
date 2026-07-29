import type { OrderStatus, OrderType } from '../../types';

export interface OrderWorkflowAction {
  status: OrderStatus;
  text: string;
}

/**
 * The cashier and order-management workboards must expose the same forward
 * production flow. Payment and settlement actions are handled separately.
 */
export const nextOrderWorkflowAction: Partial<Record<OrderStatus, OrderWorkflowAction>> = {
  PAID: { status: 'PREPARING', text: '开始制作' },
  ACCEPTED: { status: 'PREPARING', text: '开始制作' },
  PREPARING: { status: 'READY', text: '完成制作 / 请取餐' },
  READY: { status: 'COMPLETED', text: '完成订单' },
};

export function orderWorkflowActionText(currentStatus: OrderStatus, orderType?: OrderType): string {
  const action = nextOrderWorkflowAction[currentStatus];
  if (!action) return '';
  if (action.status === 'COMPLETED' && orderType === 'DINE_IN') return '完成订单/清台';
  return action.text;
}

export function canRunOrderWorkflowAction(
  currentStatus: OrderStatus,
  settlementMode?: string,
  paymentStatus?: string,
): boolean {
  const action = nextOrderWorkflowAction[currentStatus];
  if (!action) return false;
  return !(action.status === 'COMPLETED' && settlementMode === 'PAY_AFTER' && paymentStatus === 'UNPAID');
}
