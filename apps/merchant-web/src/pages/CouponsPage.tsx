import {
  EditOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
  TagsOutlined,
} from '@ant-design/icons';
import {
  Button,
  Card,
  Col,
  DatePicker,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Row,
  Segmented,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import type { Dayjs } from 'dayjs';
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { errorMessage } from '../api/client';
import { FeatureAvailabilityNotice } from '../components/FeatureAvailabilityNotice';
import { PageHeading } from '../components/PageHeading';
import { marketingApi } from '../features/marketing/api';
import type {
  CouponCampaign,
  CouponCampaignPayload,
  CouponDistributionChannel,
  CouponType,
  CouponValidityMode,
  MarketingCampaignStatus,
} from '../features/marketing/types';
import { dateTime, yuan } from '../utils/format';
import './marketing.css';

interface CouponFormValues {
  name: string;
  description: string;
  type: CouponType;
  threshold: number;
  discount: number;
  distribution_mode: CouponDistributionChannel;
  total_stock: number;
  per_subject_limit: number;
  claim_range?: [Dayjs, Dayjs];
  validity_mode: CouponValidityMode;
  validity_range?: [Dayjs, Dayjs];
  valid_days: number;
  order_types: string[];
}

const statusMeta: Record<MarketingCampaignStatus, { text: string; color: string }> = {
  DRAFT: { text: '草稿', color: 'default' },
  ACTIVE: { text: '进行中', color: 'success' },
  PAUSED: { text: '已暂停', color: 'warning' },
  ENDED: { text: '已结束', color: 'default' },
};

function CampaignStatusTag({ status }: { status: MarketingCampaignStatus }) {
  const meta = statusMeta[status] || statusMeta.DRAFT;
  return <Tag color={meta.color}>{meta.text}</Tag>;
}

const typeText: Record<CouponType, string> = { CASH: '代金券', FULL_REDUCTION: '满减券' };

export function CouponsPage() {
  const [items, setItems] = useState<CouponCampaign[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [statusFilter, setStatusFilter] = useState<MarketingCampaignStatus | 'ALL'>('ALL');
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<CouponCampaign>();
  const [form] = Form.useForm<CouponFormValues>();
  const [messageApi, holder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setItems(await marketingApi.listCoupons());
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [messageApi]);

  useEffect(() => { void load(); }, [load]);

  const rows = useMemo(() => statusFilter === 'ALL' ? items : items.filter((item) => item.status === statusFilter), [items, statusFilter]);
  const activeCount = items.filter((item) => item.status === 'ACTIVE').length;
  const stock = items.reduce((sum, item) => sum + Math.max(item.totalStock - item.issuedCount, 0), 0);
  const redeemed = items.reduce((sum, item) => sum + item.redeemedCount, 0);

  const openEditor = (item?: CouponCampaign) => {
    setEditing(item);
    form.resetFields();
    form.setFieldsValue(item ? {
      name: item.name,
      description: item.description,
      type: item.type,
      threshold: item.thresholdCents / 100,
      discount: item.discountCents / 100,
      distribution_mode: item.distributionChannel,
      total_stock: item.totalStock,
      per_subject_limit: item.perSubjectLimit,
      claim_range: item.claimStartAt && item.claimEndAt ? [dayjs(item.claimStartAt), dayjs(item.claimEndAt)] : undefined,
      validity_mode: item.validityMode,
      validity_range: item.validFrom && item.validTo ? [dayjs(item.validFrom), dayjs(item.validTo)] : undefined,
      valid_days: item.validDays,
      order_types: item.orderTypes,
    } : {
      type: 'FULL_REDUCTION',
      threshold: 30,
      discount: 5,
      distribution_mode: 'PUBLIC_CLAIM',
      total_stock: 100,
      per_subject_limit: 1,
      validity_mode: 'RELATIVE_DAYS',
      valid_days: 30,
      order_types: ['DINE_IN', 'TAKEOUT'],
    });
    setEditorOpen(true);
  };

  const save = async () => {
    const values = await form.validateFields();
    const thresholdCents = Math.round(Number(values.threshold || 0) * 100);
    const discountCents = Math.round(Number(values.discount || 0) * 100);
    if (values.type === 'FULL_REDUCTION' && discountCents > thresholdCents) {
      messageApi.error('满减券的优惠金额不能高于使用门槛');
      return;
    }
    const payload: CouponCampaignPayload = {
      name: values.name.trim(),
      description: values.description?.trim() || '',
      coupon_type: values.type,
      distribution_mode: values.distribution_mode,
      threshold_cents: values.type === 'CASH' ? 0 : thresholdCents,
      discount_cents: discountCents,
      total_stock: Number(values.total_stock),
      per_subject_limit: Number(values.per_subject_limit),
      claim_start_at: values.claim_range?.[0].toISOString(),
      claim_end_at: values.claim_range?.[1].toISOString(),
      validity_mode: values.validity_mode,
      valid_from: values.validity_mode === 'FIXED_RANGE' ? values.validity_range?.[0].toISOString() : undefined,
      valid_to: values.validity_mode === 'FIXED_RANGE' ? values.validity_range?.[1].toISOString() : undefined,
      valid_days: values.validity_mode === 'RELATIVE_DAYS' ? Number(values.valid_days) : 0,
      order_types: values.order_types,
    };
    setSaving(true);
    try {
      if (editing) await marketingApi.updateCoupon(editing.id, payload);
      else await marketingApi.createCoupon(payload);
      setEditorOpen(false);
      messageApi.success(editing ? '优惠券已更新' : '优惠券草稿已创建');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const transition = async (item: CouponCampaign, action: 'activate' | 'pause') => {
    try {
      if (action === 'activate') await marketingApi.activateCoupon(item.id);
      else await marketingApi.pauseCoupon(item.id);
      messageApi.success(action === 'activate' ? '优惠券已启用' : '优惠券已暂停');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  return (
    <div className="page-shell marketing-page">
      {holder}
      <PageHeading title="优惠券" description="统一维护代金券和满减券的券面、库存、领取时间与适用订单" extra={<Space><Button icon={<ReloadOutlined />} onClick={() => void load()}>刷新</Button><Button type="primary" icon={<PlusOutlined />} onClick={() => openEditor()}>新建优惠券</Button></Space>} />
      <FeatureAvailabilityNotice className="marketing-alert" type="warning" feature="COUPON_REDEMPTION" />
      <Row gutter={[16, 16]} className="marketing-summary-grid">
        <Col xs={12} lg={6}><Card bordered={false}><Statistic title="优惠券活动" value={items.length} prefix={<TagsOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card bordered={false}><Statistic title="进行中" value={activeCount} /></Card></Col>
        <Col xs={12} lg={6}><Card bordered={false}><Statistic title="剩余可领取" value={stock} /></Card></Col>
        <Col xs={12} lg={6}><Card bordered={false}><Statistic title="累计核销" value={redeemed} /></Card></Col>
      </Row>
      <Card bordered={false} className="content-card marketing-table-card">
        <div className="marketing-toolbar">
          <Segmented value={statusFilter} onChange={(value) => setStatusFilter(value as typeof statusFilter)} options={[{ label: '全部', value: 'ALL' }, { label: '草稿', value: 'DRAFT' }, { label: '进行中', value: 'ACTIVE' }, { label: '已暂停', value: 'PAUSED' }, { label: '已结束', value: 'ENDED' }]} />
          <Typography.Text type="secondary">共 {rows.length} 个活动</Typography.Text>
        </div>
        <Table<CouponCampaign> rowKey="id" loading={loading} dataSource={rows} scroll={{ x: 1160 }} columns={[
          { title: '活动', width: 220, render: (_, item) => <Space direction="vertical" size={0}><Typography.Text strong>{item.name}</Typography.Text><Typography.Text type="secondary">{typeText[item.type]} · {item.distributionChannel === 'PUBLIC_CLAIM' ? '公开领取' : item.distributionChannel === 'MANUAL_ONLY' ? '仅人工发放' : '仅抽奖发放'}</Typography.Text></Space> },
          { title: '优惠规则', width: 180, render: (_, item) => item.type === 'FULL_REDUCTION' ? `满 ${yuan(item.thresholdCents / 100)} 减 ${yuan(item.discountCents / 100)}` : `${yuan(item.discountCents / 100)} 代金券` },
          { title: '库存 / 已领 / 已核销', width: 190, render: (_, item) => `${item.totalStock} / ${item.issuedCount} / ${item.redeemedCount}` },
          { title: '每人限领', dataIndex: 'perSubjectLimit', width: 110, render: (value) => value > 0 ? `${value} 张` : '不限' },
          { title: '领取时间', width: 210, render: (_, item) => item.claimStartAt ? <><div>{dateTime(item.claimStartAt)}</div><Typography.Text type="secondary">至 {item.claimEndAt ? dateTime(item.claimEndAt) : '长期'}</Typography.Text></> : '长期' },
          { title: '券有效期', width: 160, render: (_, item) => item.validityMode === 'RELATIVE_DAYS' ? `领取后 ${item.validDays} 天` : `${dateTime(item.validFrom)} 至 ${dateTime(item.validTo)}` },
          { title: '状态', dataIndex: 'status', width: 100, render: (status) => <CampaignStatusTag status={status} /> },
          { title: '操作', fixed: 'right', width: 210, render: (_, item) => <Space size={4}><Button type="link" icon={<EditOutlined />} disabled={item.status === 'ACTIVE' || item.status === 'ENDED'} onClick={() => openEditor(item)}>编辑</Button>{item.status === 'ACTIVE' ? <Popconfirm title="暂停后顾客暂时无法领取，已领取券不受影响" onConfirm={() => void transition(item, 'pause')}><Button type="link" icon={<PauseCircleOutlined />}>暂停</Button></Popconfirm> : item.status !== 'ENDED' ? <Popconfirm title="启用前会再次核对库存和时间规则" onConfirm={() => void transition(item, 'activate')}><Button type="link" icon={<PlayCircleOutlined />}>启用</Button></Popconfirm> : null}</Space> },
        ]} />
      </Card>

      <Modal title={editing ? '编辑优惠券' : '新建优惠券'} width={760} open={editorOpen} onCancel={() => setEditorOpen(false)} onOk={() => void save()} confirmLoading={saving} destroyOnHidden>
        <Form form={form} layout="vertical">
          <Row gutter={16}><Col span={15}><Form.Item label="活动名称" name="name" rules={[{ required: true, whitespace: true, message: '请输入活动名称' }, { max: 100 }]}><Input placeholder="例如：新客满 30 减 5" /></Form.Item></Col><Col span={9}><Form.Item label="优惠券类型" name="type" rules={[{ required: true }]}><Select options={[{ value: 'CASH', label: '代金券' }, { value: 'FULL_REDUCTION', label: '满减券' }]} /></Form.Item></Col></Row>
          <Form.Item label="活动说明" name="description" rules={[{ max: 500 }]}><Input.TextArea rows={2} placeholder="面向商户和顾客的简短说明" /></Form.Item>
          <Form.Item noStyle shouldUpdate={(previous, current) => previous.type !== current.type}>{({ getFieldValue }) => <Row gutter={16}>{getFieldValue('type') === 'FULL_REDUCTION' ? <Col span={8}><Form.Item label="使用门槛" name="threshold" rules={[{ required: true }]}><InputNumber min={0.01} precision={2} prefix="¥" style={{ width: '100%' }} /></Form.Item></Col> : null}<Col span={8}><Form.Item label="优惠金额" name="discount" rules={[{ required: true }]}><InputNumber min={0.01} precision={2} prefix="¥" style={{ width: '100%' }} /></Form.Item></Col><Col span={8}><Form.Item label="总库存" name="total_stock" rules={[{ required: true }]}><InputNumber min={1} precision={0} style={{ width: '100%' }} /></Form.Item></Col></Row>}</Form.Item>
          <Row gutter={16}><Col span={8}><Form.Item label="发放方式" name="distribution_mode" rules={[{ required: true }]}><Select options={[{ value: 'PUBLIC_CLAIM', label: '顾客公开领取' }, { value: 'MANUAL_ONLY', label: '仅商户人工发放' }, { value: 'LOTTERY_ONLY', label: '仅作为抽奖奖品' }]} /></Form.Item></Col><Col span={8}><Form.Item label="每位顾客限领" name="per_subject_limit"><InputNumber min={1} precision={0} style={{ width: '100%' }} /></Form.Item></Col><Col span={8}><Form.Item label="适用订单" name="order_types" rules={[{ required: true }]}><Select mode="multiple" options={[{ value: 'DINE_IN', label: '桌码堂食' }, { value: 'TAKEOUT', label: '快餐自取' }, { value: 'DELIVERY', label: '外卖（暂未开放）', disabled: true }]} /></Form.Item></Col></Row>
          <Form.Item label="领取时间" name="claim_range"><DatePicker.RangePicker showTime style={{ width: '100%' }} /></Form.Item>
          <Row gutter={16}><Col span={10}><Form.Item label="有效期方式" name="validity_mode"><Select options={[{ value: 'RELATIVE_DAYS', label: '领取后若干天' }, { value: 'FIXED_RANGE', label: '固定日期区间' }]} /></Form.Item></Col><Col span={14}><Form.Item noStyle shouldUpdate={(previous, current) => previous.validity_mode !== current.validity_mode}>{({ getFieldValue }) => getFieldValue('validity_mode') === 'FIXED_RANGE' ? <Form.Item label="固定有效期" name="validity_range" rules={[{ required: true, message: '请选择固定有效期' }]}><DatePicker.RangePicker showTime style={{ width: '100%' }} /></Form.Item> : <Form.Item label="领取后有效天数" name="valid_days" rules={[{ required: true }]}><InputNumber min={1} max={3650} precision={0} addonAfter="天" style={{ width: '100%' }} /></Form.Item>}</Form.Item></Col></Row>
        </Form>
      </Modal>
    </div>
  );
}
