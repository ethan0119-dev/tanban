import { CrownOutlined, PlusOutlined, ReloadOutlined, SettingOutlined, UserAddOutlined } from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Col,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { api, errorMessage } from '../api/client';
import { FeatureAvailabilityNotice } from '../components/FeatureAvailabilityNotice';
import { PageHeading } from '../components/PageHeading';
import { ImagePickerField } from '../components/media/ImagePickerField';
import { MediaLibraryModal } from '../components/media/MediaLibraryModal';
import type { CardIssuance, Customer, MemberLevel, MemberLevelOrder, MembershipSettings } from '../member/types';
import { memberLevelOrderWritePayload } from '../features/settings/payloads';
import '../member/member.css';
import { dateTime, yuan } from '../utils/format';

interface LevelForm {
  name: string;
  rank_no: number;
  acquire_type: string;
  growth_threshold: number;
  price: number;
  valid_days: number;
  discount?: number;
  description?: string;
  is_default: boolean;
  enabled: boolean;
}

interface SettingsForm {
  enabled: boolean;
  card_name: string;
  card_color: string;
  card_image_url?: string;
  auto_enroll: boolean;
  default_level_id?: string | number;
  growth_per_yuan: number;
  agreement_url?: string;
  show_balance: boolean;
}

function idempotency(prefix: string) {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2)}`;
}

export function MembershipPage() {
  const [activeTab, setActiveTab] = useState('levels');
  const [levels, setLevels] = useState<MemberLevel[]>([]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [issuances, setIssuances] = useState<CardIssuance[]>([]);
  const [orders, setOrders] = useState<MemberLevelOrder[]>([]);
  const [settings, setSettings] = useState<MembershipSettings>();
  const [settingsReady, setSettingsReady] = useState(false);
  const [settingsLoadError, setSettingsLoadError] = useState('');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [levelOpen, setLevelOpen] = useState(false);
  const [editingLevel, setEditingLevel] = useState<MemberLevel>();
  const [issueOpen, setIssueOpen] = useState(false);
  const [orderOpen, setOrderOpen] = useState(false);
  const [cardLibraryOpen, setCardLibraryOpen] = useState(false);
  const [levelForm] = Form.useForm<LevelForm>();
  const [issueForm] = Form.useForm<{ customer_id: string | number; level_id?: string | number; issue_source: string }>();
  const [orderForm] = Form.useForm<{ customer_id: string | number; level_id: string | number; amount: number; payment_method: string; status: string; remark?: string }>();
  const [settingsForm] = Form.useForm<SettingsForm>();
  const [messageApi, contextHolder] = message.useMessage();
  const issuanceKey = useRef('');
  const levelOrderKey = useRef('');

  const load = useCallback(async () => {
    setLoading(true);
    setSettingsReady(false);
    setSettingsLoadError('');
    try {
      const [levelResult, customerResult, issuanceResult, orderResult, settingsResult] = await Promise.all([
        api.getList<MemberLevel>('/merchant/member-levels'),
        api.getList<Customer>('/merchant/customers', { page_size: 100 }),
        api.getList<CardIssuance>('/merchant/member-card-issuances', { page_size: 100 }),
        api.getList<MemberLevelOrder>('/merchant/member-level-orders', { page_size: 100 }),
        api.get<MembershipSettings>('/merchant/membership-settings'),
      ]);
      setLevels(levelResult.items);
      setCustomers(customerResult.items);
      setIssuances(issuanceResult.items);
      setOrders(orderResult.items);
      setSettings(settingsResult);
      settingsForm.setFieldsValue(settingsResult);
      setSettingsReady(true);
    } catch (error) {
      const detail = errorMessage(error);
      setSettingsLoadError(detail);
      messageApi.error(detail);
    } finally {
      setLoading(false);
    }
  }, [messageApi, settingsForm]);

  useEffect(() => { void load(); }, [load]);

  const openLevel = (level?: MemberLevel) => {
    setEditingLevel(level);
    const benefit = level?.benefits || {};
    levelForm.setFieldsValue(level ? {
      name: level.name,
      rank_no: level.rank_no,
      acquire_type: level.acquire_type,
      growth_threshold: level.growth_threshold,
      price: level.price_cents / 100,
      valid_days: level.valid_days,
      discount: Number(benefit.discount || 100),
      description: String(benefit.description || ''),
      is_default: level.is_default,
      enabled: level.status === 'ACTIVE',
    } : { rank_no: levels.length + 1, acquire_type: 'PAID', growth_threshold: 0, price: 0, valid_days: 0, discount: 100, is_default: levels.length === 0, enabled: true });
    setLevelOpen(true);
  };

  const saveLevel = async () => {
    const values = await levelForm.validateFields();
    const payload = {
      name: values.name,
      rank_no: values.rank_no,
      acquire_type: values.acquire_type,
      growth_threshold: values.growth_threshold,
      price_cents: Math.round(values.price * 100),
      valid_days: values.valid_days,
      benefits: { discount: values.discount, description: values.description || '' },
      upgrade_gift: {},
      is_default: values.is_default,
      status: values.enabled ? 'ACTIVE' : 'DISABLED',
    };
    setSaving(true);
    try {
      if (editingLevel) await api.put(`/merchant/member-levels/${editingLevel.id}`, payload);
      else await api.post('/merchant/member-levels', payload);
      setLevelOpen(false);
      messageApi.success(editingLevel ? '等级已更新' : '等级已创建');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const issueCard = async () => {
    const values = await issueForm.validateFields();
    setSaving(true);
    try {
      if (!issuanceKey.current) issuanceKey.current = idempotency('issue');
      await api.postIdempotent('/merchant/member-card-issuances', values, issuanceKey.current);
      setIssueOpen(false);
      issuanceKey.current = '';
      issueForm.resetFields();
      messageApi.success('会员卡已开通，开卡记录已留存');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const createOrder = async () => {
    const values = await orderForm.validateFields();
    setSaving(true);
    try {
      if (!levelOrderKey.current) levelOrderKey.current = idempotency('level_order');
      await api.postIdempotent('/merchant/member-level-orders', memberLevelOrderWritePayload(values), levelOrderKey.current);
      setOrderOpen(false);
      levelOrderKey.current = '';
      orderForm.resetFields();
      messageApi.success(values.status === 'COMPLETED' ? '充值已入账，会员等级已生效' : '等级充值记录已创建');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const saveSettings = async () => {
    if (!settingsReady) {
      messageApi.error('会员卡设置尚未成功载入，请刷新后再保存');
      return;
    }
    const values = await settingsForm.validateFields();
    setSaving(true);
    try {
      await api.put('/merchant/membership-settings', values);
      messageApi.success('会员卡设置已保存');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const memberCount = useMemo(() => customers.filter((item) => item.member_id).length, [customers]);
  const cardImageURL = Form.useWatch('card_image_url', settingsForm);

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading title="会员管理" description="配置只向上升级的会员身份与折扣；等级开通金额进入本金余额，但不参与储值赠送活动" extra={<Button icon={<ReloadOutlined />} onClick={() => void load()}>刷新</Button>} />
      <Card bordered={false} className="content-card member-tabs-card">
        <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
          {
            key: 'levels', label: '会员卡等级', children: <>
              <Alert type="info" showIcon message="等级按排序值从低到高，只允许向上升级" description="顾客端会隐藏当前及更低等级；“充值开通”金额进入本金余额，但不会叠加会员储值中的赠送金额。" style={{ marginBottom: 16 }} />
              <div className="member-filter-bar"><Typography.Text type="secondary">共 {levels.length} 个等级，覆盖 {memberCount} 位会员</Typography.Text><Button type="primary" icon={<PlusOutlined />} onClick={() => openLevel()}>新增等级</Button></div>
              <Table<MemberLevel> rowKey="id" loading={loading} dataSource={levels} columns={[
                { title: '等级', render: (_, item) => <Space><CrownOutlined style={{ color: '#b77a3d' }} /><strong>{item.name}</strong>{item.is_default && <Tag color="gold">默认</Tag>}</Space> },
                { title: '获得方式', dataIndex: 'acquire_type', render: (value) => ({ FREE: '免费开卡', GROWTH: '成长值升级', PAID: '充值开通' }[String(value)] || value) },
                { title: '充值 / 条件', render: (_, item) => item.acquire_type === 'PAID' ? yuan(item.price_cents / 100) : item.acquire_type === 'GROWTH' ? `${item.growth_threshold} 成长值` : '无门槛' },
                { title: '会员折扣', render: (_, item) => Number(item.benefits?.discount || 100) < 100 ? `${Number((Number(item.benefits?.discount) / 10).toFixed(1))} 折` : '无折扣' },
                { title: '会员数', dataIndex: 'member_count' },
                { title: '有效期', dataIndex: 'valid_days', render: (value) => Number(value) ? `${value} 天` : '永久' },
                { title: '状态', render: (_, item) => <Tag color={item.status === 'ACTIVE' ? 'success' : 'default'}>{item.status === 'ACTIVE' ? '启用' : '停用'}</Tag> },
                { title: '操作', render: (_, item) => <Space><Button type="link" onClick={() => openLevel(item)}>编辑</Button><Popconfirm title={item.member_count ? '已有会员使用该等级，只能先停用' : '确认删除等级？'} disabled={Boolean(item.member_count)} onConfirm={async () => { try { await api.delete(`/merchant/member-levels/${item.id}`); await load(); } catch (error) { messageApi.error(errorMessage(error)); } }}><Button type="link" danger disabled={Boolean(item.member_count)}>删除</Button></Popconfirm></Space> },
              ]} />
            </>,
          },
          {
            key: 'issuances', label: '开卡记录', children: <>
              <div className="member-filter-bar"><Typography.Text type="secondary">开卡记录只追加，不支持删除</Typography.Text><Button type="primary" icon={<UserAddOutlined />} onClick={() => { issuanceKey.current = idempotency('issue'); issueForm.resetFields(); issueForm.setFieldsValue({ issue_source: 'MANUAL' }); setIssueOpen(true); }}>开通会员卡</Button></div>
              <Table<CardIssuance> rowKey="id" loading={loading} dataSource={issuances} scroll={{ x: 980 }} columns={[
                { title: '开卡单号', dataIndex: 'issue_no', width: 210 },
                { title: '顾客', dataIndex: 'customer_name', width: 150 },
                { title: '会员号', dataIndex: 'member_no', width: 190 },
                { title: '等级', dataIndex: 'level_name', width: 130, render: (value) => value || '普通会员' },
                { title: '来源', dataIndex: 'issue_source', width: 110 },
                { title: '有效期', width: 250, render: (_, item) => `${dateTime(item.valid_from)} 至 ${item.valid_to ? dateTime(item.valid_to) : '永久'}` },
                { title: '时间', dataIndex: 'created_at', width: 170, render: dateTime },
              ]} />
            </>,
          },
          {
            key: 'settings', label: '会员卡设置', children: <Row gutter={24}>
              <Col xs={24} lg={9}><div className="member-card-preview" style={{ background: settings?.card_color || '#8b5635', ...(cardImageURL ? { backgroundImage: `linear-gradient(120deg, rgba(38,28,22,.18), rgba(38,28,22,.44)), url(${cardImageURL})`, backgroundSize: 'cover', backgroundPosition: 'center' } : {}) }}><small>码农咖啡</small><strong>{Form.useWatch('card_name', settingsForm) || '会员卡'}</strong><span>NO. 00000001</span></div></Col>
              <Col xs={24} lg={15}><FeatureAvailabilityNotice feature="MEMBERSHIP_AUTOMATION" style={{ marginBottom: 16 }} />{settingsLoadError ? <Alert type="error" showIcon message="会员卡设置加载失败，已禁止保存" description={settingsLoadError} style={{ marginBottom: 16 }} /> : null}<Form form={settingsForm} layout="vertical"><Row gutter={12}><Col span={12}><Form.Item label="会员功能" name="enabled" valuePropName="checked"><Switch /></Form.Item></Col><Col span={12}><Form.Item label="新顾客自动开卡" name="auto_enroll" valuePropName="checked" extra="下单时按默认等级建立会员身份；付费等级仍需完成对应充值后才生效。"><Switch /></Form.Item></Col></Row><Row gutter={12}><Col span={12}><Form.Item label="卡名称" name="card_name" rules={[{ required: true }]}><Input /></Form.Item></Col><Col span={12}><Form.Item label="主题色" name="card_color"><Input type="color" /></Form.Item></Col></Row><Form.Item label="默认等级" name="default_level_id"><Select allowClear options={levels.filter((item) => item.status === 'ACTIVE').map((item) => ({ value: item.id, label: item.name }))} /></Form.Item><Form.Item label="每消费 1 元获得成长值（暂未开放）" name="growth_per_yuan"><InputNumber disabled min={0} precision={0} style={{ width: '100%' }} /></Form.Item><Form.Item label="卡面图片" name="card_image_url"><ImagePickerField alt="会员卡面" hint="用于会员中心的会员卡背景" onOpenLibrary={() => setCardLibraryOpen(true)} /></Form.Item><Form.Item label="会员协议 URL" name="agreement_url"><Input /></Form.Item><Form.Item label="小程序展示余额" name="show_balance" valuePropName="checked"><Switch /></Form.Item><Button type="primary" icon={<SettingOutlined />} loading={saving} disabled={!settingsReady} onClick={() => void saveSettings()}>保存设置</Button></Form></Col>
            </Row>,
          },
          {
            key: 'orders', label: '等级充值记录', children: <>
              <Alert type="info" showIcon message="完成状态会同时入账余额并更新会员等级" description="金额必须与所选等级配置的一次充值金额一致；现金或转账到账后再选择“已完成”。" style={{ marginBottom: 16 }} />
              <div className="member-filter-bar"><Typography.Text type="secondary">共 {orders.length} 条记录</Typography.Text><Button type="primary" icon={<PlusOutlined />} onClick={() => { levelOrderKey.current = idempotency('level_order'); orderForm.resetFields(); orderForm.setFieldsValue({ payment_method: 'CASH', status: 'COMPLETED' }); setOrderOpen(true); }}>登记等级充值</Button></div>
              <Table<MemberLevelOrder> rowKey="id" loading={loading} dataSource={orders} scroll={{ x: 950 }} columns={[
                { title: '订单号', dataIndex: 'order_no', width: 210 },
                { title: '顾客', dataIndex: 'customer_name', width: 140 },
                { title: '等级', dataIndex: 'level_name', width: 130 },
                { title: '金额', dataIndex: 'amount_cents', width: 120, render: (value) => yuan(Number(value) / 100) },
                { title: '收款方式', dataIndex: 'payment_method', width: 110 },
                { title: '状态', dataIndex: 'status', width: 110, render: (value) => <Tag color={value === 'COMPLETED' ? 'success' : 'default'}>{value}</Tag> },
                { title: '备注', dataIndex: 'remark', width: 180, render: (value) => value || '—' },
                { title: '创建时间', dataIndex: 'created_at', width: 170, render: dateTime },
              ]} />
            </>,
          },
        ]} />
      </Card>

      <Modal title={editingLevel ? '编辑会员等级' : '新增会员等级'} width={720} open={levelOpen} onCancel={() => setLevelOpen(false)} onOk={() => void saveLevel()} confirmLoading={saving}>
        <Form form={levelForm} layout="vertical"><Row gutter={12}><Col span={12}><Form.Item label="等级名称" name="name" rules={[{ required: true }]}><Input /></Form.Item></Col><Col span={12}><Form.Item label="排序" name="rank_no" rules={[{ required: true }]}><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item></Col></Row><Row gutter={12}><Col span={8}><Form.Item label="获得方式" name="acquire_type"><Select options={[{ value: 'FREE', label: '免费开卡' }, { value: 'GROWTH', label: '成长值升级' }, { value: 'PAID', label: '充值开通' }]} /></Form.Item></Col><Col span={8}><Form.Item label="成长值门槛" name="growth_threshold"><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item></Col><Col span={8}><Form.Item label="一次充值金额" name="price" extra="选择充值开通时，该金额会全部进入会员本金余额"><InputNumber min={0} precision={2} prefix="¥" style={{ width: '100%' }} /></Form.Item></Col></Row><Row gutter={12}><Col span={8}><Form.Item label="有效天数（0 为永久）" name="valid_days"><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item></Col><Col span={8}><Form.Item label="会员折扣（例如 88 表示 8.8 折）" name="discount"><InputNumber min={1} max={100} precision={0} addonAfter="%" style={{ width: '100%' }} /></Form.Item></Col><Col span={4}><Form.Item label="默认等级" name="is_default" valuePropName="checked"><Switch /></Form.Item></Col><Col span={4}><Form.Item label="启用" name="enabled" valuePropName="checked"><Switch /></Form.Item></Col></Row><Form.Item label="权益说明" name="description"><Input.TextArea rows={3} /></Form.Item></Form>
      </Modal>
      <Modal title="开通会员卡" open={issueOpen} maskClosable={!saving} keyboard={!saving} cancelButtonProps={{ disabled: saving }} onCancel={() => { if (saving) return; issuanceKey.current = ''; issueForm.resetFields(); setIssueOpen(false); }} onOk={() => void issueCard()} confirmLoading={saving}><Form form={issueForm} layout="vertical"><Form.Item label="顾客" name="customer_id" rules={[{ required: true }]}><Select showSearch optionFilterProp="label" options={customers.map((item) => ({ value: item.id, label: `${item.name} ${item.phone_masked || ''}` }))} /></Form.Item><Form.Item label="会员等级" name="level_id"><Select allowClear options={levels.filter((item) => item.status === 'ACTIVE').map((item) => ({ value: item.id, label: item.name }))} /></Form.Item><Form.Item label="开卡来源" name="issue_source"><Select options={[{ value: 'MANUAL', label: '商户手工开卡' }, { value: 'FREE', label: '免费开卡' }, { value: 'IMPORT', label: '导入' }]} /></Form.Item></Form></Modal>
      <Modal title="登记等级充值" open={orderOpen} maskClosable={!saving} keyboard={!saving} cancelButtonProps={{ disabled: saving }} onCancel={() => { if (saving) return; levelOrderKey.current = ''; orderForm.resetFields(); setOrderOpen(false); }} onOk={() => void createOrder()} confirmLoading={saving}><Form form={orderForm} layout="vertical"><Form.Item label="顾客" name="customer_id" rules={[{ required: true }]}><Select showSearch optionFilterProp="label" options={customers.map((item) => ({ value: item.id, label: item.name }))} /></Form.Item><Form.Item label="等级" name="level_id" rules={[{ required: true }]}><Select options={levels.filter((item) => item.status === 'ACTIVE').map((item) => ({ value: item.id, label: `${item.name} · 充值 ${yuan(item.price_cents / 100)}` }))} onChange={(value) => orderForm.setFieldValue('amount', levels.find((item) => String(item.id) === String(value))?.price_cents ? Number(levels.find((item) => String(item.id) === String(value))!.price_cents) / 100 : 0)} /></Form.Item><Form.Item label="充值金额" name="amount" rules={[{ required: true }]} extra="金额由等级配置带入，服务端会再次核对"><InputNumber min={0} precision={2} prefix="¥" readOnly style={{ width: '100%' }} /></Form.Item><Form.Item label="收款方式" name="payment_method"><Select options={[{ value: 'CASH', label: '现金' }, { value: 'TRANSFER', label: '转账 / 系统外支付' }, { value: 'MANUAL', label: '其他人工确认' }]} /></Form.Item><Form.Item label="状态" name="status"><Select options={[{ value: 'COMPLETED', label: '已收款并完成（余额立即入账）' }, { value: 'RECORDED', label: '仅记录，暂不入账' }, { value: 'CANCELLED', label: '已取消' }]} /></Form.Item><Form.Item label="备注" name="remark"><Input.TextArea /></Form.Item></Form></Modal>
      <MediaLibraryModal open={cardLibraryOpen} title="选择会员卡面" onCancel={() => setCardLibraryOpen(false)} onConfirm={(selected) => { if (selected[0]) settingsForm.setFieldValue('card_image_url', selected[0].url); setCardLibraryOpen(false); }} />
    </div>
  );
}
