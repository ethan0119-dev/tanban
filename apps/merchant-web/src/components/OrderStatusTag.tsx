import { Tag } from 'antd';
import type { OrderStatus } from '../types';

export const orderStatusMap: Record<OrderStatus, { text: string; color: string }> = {
  PENDING_PAYMENT: { text: '待付款', color: 'default' },
  PAID: { text: '已付款', color: 'processing' },
  ACCEPTED: { text: '已接单', color: 'blue' },
  PREPARING: { text: '制作中', color: 'warning' },
  READY: { text: '待取餐', color: 'cyan' },
  COMPLETED: { text: '已完成', color: 'success' },
  CLOSED: { text: '已关闭', color: 'error' },
  REFUNDED: { text: '已退款', color: 'purple' },
  PAYMENT_EXCEPTION: { text: '关单后到账异常', color: 'magenta' },
};

export function OrderStatusTag({ status }: { status: OrderStatus }) {
  const config = orderStatusMap[status] ?? { text: status, color: 'default' };
  return <Tag color={config.color}>{config.text}</Tag>;
}
