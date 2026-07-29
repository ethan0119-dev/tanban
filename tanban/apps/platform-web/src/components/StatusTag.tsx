import { Tag } from 'antd';
import type { EntityStatus } from '../types';

const statusMap: Record<string, { color: string; label: string }> = {
  active: { color: 'success', label: '正常' },
  disabled: { color: 'default', label: '已停用' },
  pending: { color: 'processing', label: '待审核' },
  unbound: { color: 'default', label: '未绑定' },
  rejected: { color: 'error', label: '已驳回' },
};

export function StatusTag({ status }: { status?: EntityStatus | string }) {
  const item = statusMap[status || ''] || { color: 'default', label: status || '未知' };
  return <Tag color={item.color}>{item.label}</Tag>;
}
