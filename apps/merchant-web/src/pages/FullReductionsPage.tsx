import { EditOutlined, PauseCircleOutlined, PlayCircleOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { Button, Card, Col, DatePicker, Form, Input, InputNumber, Modal, Popconfirm, Row, Select, Space, Table, Tag, Typography, message } from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useCallback, useEffect, useState } from 'react';
import { errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import { marketingApi } from '../features/marketing/api';
import type { FullReductionCampaign, FullReductionPayload } from '../features/marketing/types';
import { toBeijingRFC3339 } from '../utils/format';

interface FormValues {
  name: string;
  description?: string;
  threshold: number;
  discount: number;
  order_types: string[];
  active_range?: [Dayjs, Dayjs];
}

const statusText = { DRAFT: '草稿', ACTIVE: '进行中', PAUSED: '已暂停', ENDED: '已结束' } as const;
const statusColor = { DRAFT: 'default', ACTIVE: 'success', PAUSED: 'warning', ENDED: 'default' } as const;
const yuan = (cents: number) => (cents / 100).toFixed(2);

export function FullReductionsPage() {
  const [items, setItems] = useState<FullReductionCampaign[]>([]);
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<FullReductionCampaign | null>(null);
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm<FormValues>();
  const [messageApi, holder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try { setItems(await marketingApi.listFullReductions()); }
    catch (error) { messageApi.error(errorMessage(error)); }
    finally { setLoading(false); }
  }, [messageApi]);

  useEffect(() => { void load(); }, [load]);

  const edit = (item?: FullReductionCampaign) => {
    setEditing(item || null);
    form.setFieldsValue(item ? {
      name: item.name, description: item.description, threshold: item.thresholdCents / 100,
      discount: item.discountCents / 100, order_types: item.orderTypes,
      active_range: item.activeFrom && item.activeTo ? [dayjs(item.activeFrom), dayjs(item.activeTo)] : undefined,
    } : { name: '', description: '', threshold: 30, discount: 5, order_types: ['DINE_IN', 'TAKEOUT'], active_range: undefined });
    setOpen(true);
  };

  const save = async () => {
    const values = await form.validateFields();
    if (values.discount > values.threshold) {
      messageApi.error('减免金额不能大于使用门槛');
      return;
    }
    const payload: FullReductionPayload = {
      name: values.name.trim(), description: values.description?.trim() || '',
      threshold_cents: Math.round(values.threshold * 100), discount_cents: Math.round(values.discount * 100),
      order_types: values.order_types,
      active_from: toBeijingRFC3339(values.active_range?.[0]),
      active_to: toBeijingRFC3339(values.active_range?.[1]),
    };
    setSaving(true);
    try {
      if (editing) await marketingApi.updateFullReduction(editing.id, payload);
      else await marketingApi.createFullReduction(payload);
      messageApi.success(editing ? '活动已更新' : '活动已创建');
      setOpen(false);
      await load();
    } catch (error) { messageApi.error(errorMessage(error)); }
    finally { setSaving(false); }
  };

  const transition = async (item: FullReductionCampaign, action: 'activate' | 'pause') => {
    try {
      if (action === 'activate') await marketingApi.activateFullReduction(item.id);
      else await marketingApi.pauseFullReduction(item.id);
      messageApi.success(action === 'activate' ? '活动已启用' : '活动已暂停');
      await load();
    } catch (error) { messageApi.error(errorMessage(error)); }
  };

  return <div className="page-shell">
    {holder}
    <PageHeading title="满额立减" description="顾客订单达到门槛后自动减免；结算页允许取消，并可与一张优惠券叠加"
      extra={<Space><Button icon={<ReloadOutlined />} onClick={() => void load()}>刷新</Button><Button type="primary" icon={<PlusOutlined />} onClick={() => edit()}>新建活动</Button></Space>} />
    <Card bordered={false}>
      <Table rowKey="id" loading={loading} dataSource={items} pagination={false} scroll={{ x: 960 }} columns={[
        { title: '活动', width: 240, render: (_, item) => <Space direction="vertical" size={0}><Typography.Text strong>{item.name}</Typography.Text><Typography.Text type="secondary">{item.description || '达到门槛自动减免'}</Typography.Text></Space> },
        { title: '优惠规则', width: 180, render: (_, item) => `满 ¥${yuan(item.thresholdCents)} 减 ¥${yuan(item.discountCents)}` },
        { title: '适用订单', width: 160, render: (_, item) => item.orderTypes.map((type) => type === 'DINE_IN' ? '堂食' : '自取').join('、') },
        { title: '活动时间', width: 220, render: (_, item) => item.activeFrom ? `${item.activeFrom} 至 ${item.activeTo || '长期'}` : '长期有效' },
        { title: '状态', width: 100, render: (_, item) => <Tag color={statusColor[item.status]}>{statusText[item.status]}</Tag> },
        { title: '操作', fixed: 'right' as const, width: 210, render: (_, item) => <Space size={4}>
          <Button type="link" icon={<EditOutlined />} disabled={item.status === 'ACTIVE'} onClick={() => edit(item)}>编辑</Button>
          {item.status === 'ACTIVE'
            ? <Popconfirm title="暂停后新订单将不再享受该满减" onConfirm={() => void transition(item, 'pause')}><Button type="link" icon={<PauseCircleOutlined />}>暂停</Button></Popconfirm>
            : <Popconfirm title="启用后符合门槛的订单将自动减免" onConfirm={() => void transition(item, 'activate')}><Button type="link" icon={<PlayCircleOutlined />}>启用</Button></Popconfirm>}
        </Space> },
      ]} />
    </Card>
    <Modal title={editing ? '编辑满额立减' : '新建满额立减'} open={open} confirmLoading={saving} onOk={() => void save()} onCancel={() => setOpen(false)} destroyOnHidden>
      <Form form={form} layout="vertical">
        <Form.Item label="活动名称" name="name" rules={[{ required: true, whitespace: true, message: '请输入活动名称' }]}><Input placeholder="例如：满 50 减 8" maxLength={100} /></Form.Item>
        <Form.Item label="活动说明" name="description"><Input.TextArea rows={2} maxLength={500} /></Form.Item>
        <Row gutter={16}>
          <Col span={12}><Form.Item label="订单门槛" name="threshold" rules={[{ required: true }]}><InputNumber min={0.01} precision={2} prefix="¥" style={{ width: '100%' }} /></Form.Item></Col>
          <Col span={12}><Form.Item label="减免金额" name="discount" rules={[{ required: true }]}><InputNumber min={0.01} precision={2} prefix="¥" style={{ width: '100%' }} /></Form.Item></Col>
        </Row>
        <Form.Item label="适用订单" name="order_types" rules={[{ required: true, message: '请选择适用订单' }]}><Select mode="multiple" options={[{ value: 'DINE_IN', label: '桌码堂食' }, { value: 'TAKEOUT', label: '门店自取' }]} /></Form.Item>
        <Form.Item label="活动时间（不选则长期）" name="active_range"><DatePicker.RangePicker showTime format="YYYY-MM-DD HH:mm:ss" style={{ width: '100%' }} /></Form.Item>
      </Form>
    </Modal>
  </div>;
}
