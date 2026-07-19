import { GiftOutlined, PlusOutlined, ReloadOutlined, SafetyCertificateOutlined, WalletOutlined } from '@ant-design/icons';
import {
  Alert,
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
  Select,
  Space,
  Statistic,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import type { Dayjs } from 'dayjs';
import dayjs from 'dayjs';
import { useCallback, useEffect, useRef, useState } from 'react';
import { api, errorMessage } from '../api/client';
import { useAuth } from '../auth/AuthContext';
import { isMerchantOwner } from '../auth/permissions';
import { PageHeading } from '../components/PageHeading';
import type { Customer, MemberSummary, StoredValueRecord, StoredValueRule, StoredValueSettings } from '../member/types';
import '../member/member.css';
import { dateTime, yuan } from '../utils/format';

interface RuleForm {
  name: string;
  recharge: number;
  gift: number;
  gift_growth: number;
  per_customer_limit: number;
  active_range?: [Dayjs, Dayjs];
  description?: string;
  enabled: boolean;
}

interface RecordForm {
  customer_id: string | number;
  rule_id?: string | number;
  principal?: number;
  gift?: number;
  payment_method: string;
  remark: string;
}

function idempotency() {
  return `stored_${Date.now()}_${Math.random().toString(36).slice(2)}`;
}

export function StoredValuePage() {
  const { user } = useAuth();
  const owner = isMerchantOwner(user);
  const [activeTab, setActiveTab] = useState('rules');
  const [rules, setRules] = useState<StoredValueRule[]>([]);
  const [records, setRecords] = useState<StoredValueRecord[]>([]);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [settings, setSettings] = useState<StoredValueSettings>();
  const [settingsReady, setSettingsReady] = useState(false);
  const [settingsLoadError, setSettingsLoadError] = useState('');
  const [summary, setSummary] = useState<MemberSummary>();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [ruleOpen, setRuleOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<StoredValueRule>();
  const [recordOpen, setRecordOpen] = useState(false);
  const [ruleForm] = Form.useForm<RuleForm>();
  const [recordForm] = Form.useForm<RecordForm>();
  const [settingsForm] = Form.useForm<StoredValueSettings>();
  const [messageApi, contextHolder] = message.useMessage();
  const recordKey = useRef('');

  const load = useCallback(async () => {
    setLoading(true);
    setSettingsReady(false);
    setSettingsLoadError('');
    try {
      const [ruleResult, recordResult, customerResult, settingsResult, summaryResult] = await Promise.all([
        api.getList<StoredValueRule>('/merchant/stored-value-rules'),
        api.getList<StoredValueRecord>('/merchant/stored-value-records', { page_size: 100 }),
        api.getList<Customer>('/merchant/customers', { page_size: 100 }),
        api.get<StoredValueSettings>('/merchant/stored-value-settings'),
        api.get<MemberSummary>('/merchant/member-summary'),
      ]);
      setRules(ruleResult.items);
      setRecords(recordResult.items);
      setCustomers(customerResult.items);
      setSettings(settingsResult);
      setSummary(summaryResult);
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

  const openRule = (rule?: StoredValueRule) => {
    setEditingRule(rule);
    ruleForm.setFieldsValue(rule ? {
      name: rule.name,
      recharge: rule.recharge_cents / 100,
      gift: rule.gift_cents / 100,
      gift_growth: rule.gift_growth,
      per_customer_limit: rule.per_customer_limit,
      active_range: rule.starts_at && rule.ends_at ? [dayjs(rule.starts_at), dayjs(rule.ends_at)] : undefined,
      description: String(rule.benefits?.description || ''),
      enabled: rule.status === 'ACTIVE',
    } : { recharge: 100, gift: 0, gift_growth: 0, per_customer_limit: 0, enabled: true });
    setRuleOpen(true);
  };

  const saveRule = async () => {
    const values = await ruleForm.validateFields();
    const payload = {
      name: values.name,
      recharge_cents: Math.round(values.recharge * 100),
      gift_cents: Math.round(values.gift * 100),
      gift_growth: values.gift_growth,
      per_customer_limit: values.per_customer_limit,
      starts_at: values.active_range?.[0]?.toISOString(),
      ends_at: values.active_range?.[1]?.toISOString(),
      benefits: { description: values.description || '' },
      status: values.enabled ? 'ACTIVE' : 'DISABLED',
    };
    setSaving(true);
    try {
      if (editingRule) await api.put(`/merchant/stored-value-rules/${editingRule.id}`, payload);
      else await api.post('/merchant/stored-value-rules', payload);
      setRuleOpen(false);
      messageApi.success(editingRule ? '储值规则已更新' : '储值规则已创建');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const createRecord = async () => {
    const values = await recordForm.validateFields();
    const payload = {
      customer_id: values.customer_id,
      rule_id: values.rule_id,
      principal_cents: values.rule_id ? 0 : Math.round(Number(values.principal || 0) * 100),
      gift_cents: values.rule_id ? 0 : Math.round(Number(values.gift || 0) * 100),
      payment_method: values.payment_method,
      remark: values.remark,
    };
    setSaving(true);
    try {
      if (!recordKey.current) recordKey.current = idempotency();
      await api.postIdempotent('/merchant/stored-value-records', payload, recordKey.current);
      setRecordOpen(false);
      recordKey.current = '';
      recordForm.resetFields();
      messageApi.success('手工储值已确认，并已追加本金/赠送金流水');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const saveSettings = async () => {
    if (!settingsReady) {
      messageApi.error('储值设置尚未成功载入，请刷新后再保存');
      return;
    }
    const values = await settingsForm.validateFields();
    setSaving(true);
    try {
      await api.put('/merchant/stored-value-settings', values);
      messageApi.success('储值设置已保存');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading title="会员储值" description="管理储值档位、手工储值记录和资金规则；小程序真实充值待支付闭环接入后开放" extra={<Button icon={<ReloadOutlined />} onClick={() => void load()}>刷新</Button>} />
      <Alert className="stored-warning" type="warning" showIcon message="V1 资金边界" description="当前仅支持老板录入已在线下收款的储值记录。系统不会发起会生活支付，小程序充值入口被强制关闭。每笔资金变更都会写入不可变余额流水。" />
      <Row gutter={[16, 16]} className="member-summary-grid">
        <Col xs={12} lg={6}><Card className="member-summary-card"><Statistic title="累计储值本金" value={(summary?.stored_value_principal_cents ?? 0) / 100} precision={2} prefix="¥" /></Card></Col>
        <Col xs={12} lg={6}><Card className="member-summary-card"><Statistic title="累计赠送" value={(summary?.stored_value_gift_cents ?? 0) / 100} precision={2} prefix="¥" /></Card></Col>
        <Col xs={12} lg={6}><Card className="member-summary-card"><Statistic title="储值顾客" value={summary?.stored_value_customer_count ?? 0} prefix={<WalletOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card className="member-summary-card"><Statistic title="启用规则" value={summary?.active_stored_value_rule_count ?? 0} prefix={<GiftOutlined />} /></Card></Col>
      </Row>
      <Card bordered={false} className="content-card member-tabs-card">
        <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
          {
            key: 'rules', label: '储值规则', children: <>
              <div className="member-filter-bar"><Typography.Text type="secondary">设置固定充值和赠送档位；0 次表示不限制每位顾客参与次数</Typography.Text><Button type="primary" icon={<PlusOutlined />} onClick={() => openRule()}>新增规则</Button></div>
              <Table<StoredValueRule> rowKey="id" loading={loading} dataSource={rules} columns={[
                { title: '规则', dataIndex: 'name' },
                { title: '储值本金', dataIndex: 'recharge_cents', render: (value) => yuan(Number(value) / 100) },
                { title: '赠送金额', dataIndex: 'gift_cents', render: (value) => <span className="member-positive">{yuan(Number(value) / 100)}</span> },
                { title: '赠成长值（预留）', dataIndex: 'gift_growth', render: (value) => Number(value) ? `${value}（未执行）` : '—' },
                { title: '每人限次', dataIndex: 'per_customer_limit', render: (value) => Number(value) ? `${value} 次` : '不限' },
                { title: '有效时间', render: (_, item) => item.starts_at ? `${dateTime(item.starts_at)} 至 ${item.ends_at ? dateTime(item.ends_at) : '长期'}` : '长期' },
                { title: '状态', render: (_, item) => <Tag color={item.status === 'ACTIVE' ? 'success' : 'default'}>{item.status === 'ACTIVE' ? '启用' : '停用'}</Tag> },
                { title: '操作', render: (_, item) => <Space><Button type="link" onClick={() => openRule(item)}>编辑</Button><Popconfirm title="删除后历史储值记录仍保留规则快照" onConfirm={async () => { try { await api.delete(`/merchant/stored-value-rules/${item.id}`); await load(); } catch (error) { messageApi.error(errorMessage(error)); } }}><Button type="link" danger>删除</Button></Popconfirm></Space> },
              ]} />
            </>,
          },
          {
            key: 'records', label: '储值记录', children: <>
              <div className="member-filter-bar"><Typography.Text type="secondary">资金记录不可编辑和删除</Typography.Text>{owner ? <Button type="primary" icon={<PlusOutlined />} onClick={() => { recordKey.current = idempotency(); recordForm.resetFields(); recordForm.setFieldsValue({ payment_method: 'CASH' }); setRecordOpen(true); }}>录入手工储值</Button> : <Tag color="warning">仅老板可录入</Tag>}</div>
              <Table<StoredValueRecord> rowKey="id" loading={loading} dataSource={records} scroll={{ x: 1050 }} columns={[
                { title: '储值单号', dataIndex: 'record_no', width: 210 },
                { title: '顾客', dataIndex: 'customer_name', width: 140 },
                { title: '规则', dataIndex: 'rule_name', width: 160, render: (value) => value || '自定义金额' },
                { title: '本金', dataIndex: 'principal_cents', width: 120, render: (value) => yuan(Number(value) / 100) },
                { title: '赠送', dataIndex: 'gift_cents', width: 120, render: (value) => <span className="member-positive">{yuan(Number(value) / 100)}</span> },
                { title: '收款记录', dataIndex: 'payment_method', width: 110 },
                { title: '状态', dataIndex: 'status', width: 110, render: (value) => <Tag color={value === 'CONFIRMED' ? 'success' : 'default'}>{value}</Tag> },
                { title: '备注', dataIndex: 'remark', width: 180, render: (value) => value || '—' },
                { title: '时间', dataIndex: 'created_at', width: 170, render: dateTime },
              ]} />
            </>,
          },
          {
            key: 'settings', label: '储值设置', children: <Row gutter={24}>
              <Col xs={24} lg={8}><Card bordered={false} style={{ background: '#faf7f3' }}><SafetyCertificateOutlined style={{ fontSize: 28, color: '#9a623b' }} /><Typography.Title level={4}>资金安全设置</Typography.Title><Typography.Paragraph type="secondary">余额分为本金和赠送金两个账户。扣减顺序只影响未来余额支付；V1 尚未开放余额支付。</Typography.Paragraph><Tag color={settings?.enabled ? 'success' : 'default'}>{settings?.enabled ? '储值管理已启用' : '储值管理未启用'}</Tag></Card></Col>
              <Col xs={24} lg={16}>{settingsLoadError ? <Alert type="error" showIcon message="储值设置加载失败，已禁止保存" description={settingsLoadError} style={{ marginBottom: 16 }} /> : null}<Form form={settingsForm} layout="vertical"><Row gutter={12}><Col span={12}><Form.Item label="启用储值管理" name="enabled" valuePropName="checked"><Switch /></Form.Item></Col><Col span={12}><Form.Item label="小程序展示充值入口" name="show_in_miniapp" valuePropName="checked"><Switch disabled checked={false} /><Typography.Text type="secondary">待真实支付接入</Typography.Text></Form.Item></Col></Row><Row gutter={12}><Col span={8}><Form.Item label="最低单笔" name="min_recharge_cents" rules={[{ required: true }]}><InputNumber min={1} precision={0} addonAfter="分" style={{ width: '100%' }} /></Form.Item></Col><Col span={8}><Form.Item label="最高单笔" name="max_recharge_cents" rules={[{ required: true }]}><InputNumber min={1} precision={0} addonAfter="分" style={{ width: '100%' }} /></Form.Item></Col><Col span={8}><Form.Item label="账户余额上限" name="max_balance_cents" rules={[{ required: true }]}><InputNumber min={1} precision={0} addonAfter="分" style={{ width: '100%' }} /></Form.Item></Col></Row><Row gutter={12}><Col span={12}><Form.Item label="未来余额支付扣减顺序" name="deduction_order"><Select options={[{ value: 'BONUS_FIRST', label: '先赠送金，后本金' }, { value: 'PRINCIPAL_FIRST', label: '先本金，后赠送金' }]} /></Form.Item></Col><Col span={12}><Form.Item label="退款策略" name="refund_policy"><Select options={[{ value: 'MANUAL_REVIEW', label: '人工审核' }, { value: 'REJECT_AFTER_USE', label: '使用后拒绝自动退款' }]} /></Form.Item></Col></Row><Form.Item label="储值协议 URL" name="agreement_url"><Input /></Form.Item><Button type="primary" loading={saving} disabled={!settingsReady} onClick={() => void saveSettings()}>保存设置</Button></Form></Col>
            </Row>,
          },
        ]} />
      </Card>

      <Modal title={editingRule ? '编辑储值规则' : '新增储值规则'} width={680} open={ruleOpen} onCancel={() => setRuleOpen(false)} onOk={() => void saveRule()} confirmLoading={saving}><Form form={ruleForm} layout="vertical"><Form.Item label="规则名称" name="name" rules={[{ required: true }]}><Input placeholder="例如：充 100 送 10" /></Form.Item><Row gutter={12}><Col span={8}><Form.Item label="储值本金" name="recharge" rules={[{ required: true }]}><InputNumber min={0.01} precision={2} prefix="¥" style={{ width: '100%' }} /></Form.Item></Col><Col span={8}><Form.Item label="赠送金额" name="gift" rules={[{ required: true }]}><InputNumber min={0} precision={2} prefix="¥" style={{ width: '100%' }} /></Form.Item></Col><Col span={8}><Form.Item label="赠成长值（预留）" name="gift_growth" extra="待会员成长值闭环后开放"><InputNumber disabled min={0} precision={0} style={{ width: '100%' }} /></Form.Item></Col></Row><Row gutter={12}><Col span={12}><Form.Item label="每位顾客限次（0 不限）" name="per_customer_limit"><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item></Col><Col span={12}><Form.Item label="启用" name="enabled" valuePropName="checked"><Switch /></Form.Item></Col></Row><Form.Item label="生效区间" name="active_range"><DatePicker.RangePicker showTime style={{ width: '100%' }} /></Form.Item><Form.Item label="权益说明" name="description"><Input.TextArea /></Form.Item></Form></Modal>
      <Modal title="录入手工储值" open={recordOpen} maskClosable={!saving} keyboard={!saving} cancelButtonProps={{ disabled: saving }} onCancel={() => { if (saving) return; recordKey.current = ''; recordForm.resetFields(); setRecordOpen(false); }} onOk={() => void createRecord()} confirmLoading={saving} okButtonProps={{ danger: true }}><Alert type="warning" showIcon message="提交前请确认线下款项已经收到" description="本操作不会发起支付，但会立即增加顾客余额并生成不可删除的资金流水。" style={{ marginBottom: 16 }} /><Form form={recordForm} layout="vertical"><Form.Item label="顾客" name="customer_id" rules={[{ required: true }]}><Select showSearch optionFilterProp="label" options={customers.map((item) => ({ value: item.id, label: `${item.name} · 当前余额 ${yuan(item.balance_cents / 100)}` }))} /></Form.Item><Form.Item label="储值规则" name="rule_id"><Select allowClear placeholder="不选择时可自定义金额" options={rules.filter((item) => item.status === 'ACTIVE').map((item) => ({ value: item.id, label: `${item.name}（${yuan(item.recharge_cents / 100)} + 赠 ${yuan(item.gift_cents / 100)}）` }))} /></Form.Item><Form.Item noStyle shouldUpdate={(previous, current) => previous.rule_id !== current.rule_id}>{({ getFieldValue }) => !getFieldValue('rule_id') ? <Row gutter={12}><Col span={12}><Form.Item label="本金金额" name="principal" rules={[{ required: true }]}><InputNumber min={0.01} precision={2} prefix="¥" style={{ width: '100%' }} /></Form.Item></Col><Col span={12}><Form.Item label="赠送金额" name="gift"><InputNumber min={0} precision={2} prefix="¥" style={{ width: '100%' }} /></Form.Item></Col></Row> : null}</Form.Item><Form.Item label="线下收款方式" name="payment_method"><Select options={[{ value: 'CASH', label: '现金' }, { value: 'TRANSFER', label: '转账' }, { value: 'MANUAL', label: '其他人工确认' }]} /></Form.Item><Form.Item label="凭证/原因说明" name="remark" rules={[{ required: true }]}><Input.TextArea rows={3} maxLength={255} showCount /></Form.Item></Form></Modal>
    </div>
  );
}
