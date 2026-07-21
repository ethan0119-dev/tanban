import { BellOutlined, CheckOutlined, ReloadOutlined } from '@ant-design/icons';
import { Badge, Button, Card, Drawer, Empty, List, Segmented, Space, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import { notificationApi } from '../features/notifications/api';
import type { MerchantNotification } from '../features/notifications/types';
import { dateTime } from '../utils/format';

const categoryText: Record<string, string> = {
  SYSTEM_UPDATE: '系统迭代', BUG_FIX: '问题修复', NEW_FEATURE: '新功能', NOTICE: '注意事项', ACTION_REQUIRED: '待办提醒',
};
const categoryColor: Record<string, string> = {
  SYSTEM_UPDATE: 'blue', BUG_FIX: 'green', NEW_FEATURE: 'purple', NOTICE: 'default', ACTION_REQUIRED: 'orange',
};
const severityText: Record<string, string> = { INFO: '普通', IMPORTANT: '重要', URGENT: '紧急' };
const severityColor: Record<string, string> = { INFO: 'blue', IMPORTANT: 'orange', URGENT: 'red' };

function displayTime(value: string) {
  return value ? dateTime(value) : '—';
}

export function NotificationsPage() {
  const [rows, setRows] = useState<MerchantNotification[]>([]);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(15);
  const [total, setTotal] = useState(0);
  const [mode, setMode] = useState<'all' | 'unread'>('all');
  const [loading, setLoading] = useState(false);
  const [selected, setSelected] = useState<MerchantNotification>();
  const [messageApi, holder] = message.useMessage();

  const load = useCallback(async (targetPage = page) => {
    setLoading(true);
    try {
      const result = await notificationApi.list({ page: targetPage, page_size: pageSize, unread_only: mode === 'unread' || undefined });
      setRows(result.items);
      setTotal(Number(result.meta.total || 0));
      setPage(targetPage);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [messageApi, mode, page, pageSize]);

  useEffect(() => { void load(1); }, [mode]); // eslint-disable-line react-hooks/exhaustive-deps

  const openNotification = async (item: MerchantNotification) => {
    setSelected(item);
    if (item.isRead) return;
    try {
      const updated = await notificationApi.markRead(item.id);
      setSelected(updated);
      setRows((current) => mode === 'unread' ? current.filter((row) => row.id !== item.id) : current.map((row) => row.id === item.id ? updated : row));
      if (mode === 'unread') setTotal((value) => Math.max(0, value - 1));
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const markAllRead = async () => {
    try {
      const count = await notificationApi.markAllRead();
      messageApi.success(count ? `已将 ${count} 条通知标记为已读` : '当前没有未读通知');
      void load(1);
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  return <div className="page-shell notification-page">
    {holder}
    <PageHeading title="通知收件箱" description="查看摊伴平台发布的系统迭代、问题修复、新功能说明和重要注意事项。" extra={<Space><Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>刷新</Button><Button icon={<CheckOutlined />} onClick={() => void markAllRead()}>全部已读</Button></Space>} />
    <Card bordered={false} className="content-card notification-inbox-card">
      <div className="notification-toolbar"><Segmented value={mode} onChange={(value) => setMode(value as 'all' | 'unread')} options={[{ value: 'all', label: '全部通知' }, { value: 'unread', label: '只看未读' }]} /><Typography.Text type="secondary">共 {total} 条</Typography.Text></div>
      <List
        loading={loading}
        dataSource={rows}
        locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={mode === 'unread' ? '没有未读通知' : '暂时没有平台通知'} /> }}
        pagination={total > pageSize ? { current: page, pageSize, total, showSizeChanger: false, onChange: (targetPage) => void load(targetPage) } : false}
        renderItem={(item) => <List.Item className={`notification-item ${item.isRead ? 'is-read' : 'is-unread'}`} onClick={() => void openNotification(item)} actions={[<Button key="open" type="link" onClick={(event) => { event.stopPropagation(); void openNotification(item); }}>查看详情</Button>]}>
          <List.Item.Meta
            avatar={<Badge dot={!item.isRead} offset={[-2, 3]}><span className={`notification-icon severity-${item.severity.toLowerCase()}`}><BellOutlined /></span></Badge>}
            title={<Space wrap><Typography.Text strong={!item.isRead}>{item.title}</Typography.Text><Tag color={categoryColor[item.category]}>{categoryText[item.category] || item.category}</Tag>{item.severity !== 'INFO' && <Tag color={severityColor[item.severity]}>{severityText[item.severity]}</Tag>}</Space>}
            description={<div><Typography.Paragraph ellipsis={{ rows: 2 }}>{item.summary || item.content}</Typography.Paragraph><Typography.Text type="secondary">{displayTime(item.publishedAt)}</Typography.Text></div>}
          />
        </List.Item>}
      />
    </Card>
    <Drawer title="通知详情" width={600} open={Boolean(selected)} onClose={() => setSelected(undefined)}>
      {selected && <article className="notification-detail"><Space wrap><Tag color={categoryColor[selected.category]}>{categoryText[selected.category]}</Tag><Tag color={severityColor[selected.severity]}>{severityText[selected.severity]}</Tag>{selected.isRead && <Tag color="success">已读</Tag>}</Space><Typography.Title level={3}>{selected.title}</Typography.Title><Typography.Text type="secondary">发布于 {displayTime(selected.publishedAt)}</Typography.Text>{selected.summary && <Typography.Paragraph className="notification-summary">{selected.summary}</Typography.Paragraph>}<Typography.Paragraph className="notification-content">{selected.content}</Typography.Paragraph></article>}
    </Drawer>
  </div>;
}
