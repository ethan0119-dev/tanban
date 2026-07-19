import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  TagsOutlined,
  UserOutlined,
  WalletOutlined,
} from '@ant-design/icons';
import {
  Avatar,
  Button,
  Card,
  Col,
  Descriptions,
  Drawer,
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
import { useCallback, useEffect, useRef, useState } from 'react';
import { api, errorMessage } from '../api/client';
import { useAuth } from '../auth/AuthContext';
import { isMerchantOwner } from '../auth/permissions';
import { PageHeading } from '../components/PageHeading';
import type { BalanceLedger, Customer, CustomerTag, MemberSummary } from '../member/types';
import '../member/member.css';
import { dateTime, initials, yuan } from '../utils/format';

interface CustomerForm {
  name: string;
  phone?: string;
  source: string;
  status: string;
  remark?: string;
}

interface TagForm {
  name: string;
  color: string;
  description?: string;
  enabled: boolean;
}

function newKey(prefix: string) {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2)}`;
}

export function CustomersPage() {
  const { user } = useAuth();
  const owner = isMerchantOwner(user);
  const [activeTab, setActiveTab] = useState('customers');
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [tags, setTags] = useState<CustomerTag[]>([]);
  const [ledger, setLedger] = useState<BalanceLedger[]>([]);
  const [summary, setSummary] = useState<MemberSummary>();
  const [loading, setLoading] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [status, setStatus] = useState<string>();
  const [selected, setSelected] = useState<Customer>();
  const [editing, setEditing] = useState<Customer>();
  const [customerOpen, setCustomerOpen] = useState(false);
  const [tagOpen, setTagOpen] = useState(false);
  const [editingTag, setEditingTag] = useState<CustomerTag>();
  const [assignmentOpen, setAssignmentOpen] = useState(false);
  const [adjustOpen, setAdjustOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [customerForm] = Form.useForm<CustomerForm>();
  const [tagForm] = Form.useForm<TagForm>();
  const [assignmentForm] = Form.useForm<{ tag_ids: Array<string | number> }>();
  const [adjustForm] = Form.useForm<{ bucket: string; direction: string; amount: number; remark: string }>();
  const [messageApi, contextHolder] = message.useMessage();
  const adjustmentKey = useRef('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [customerResult, tagResult, ledgerResult, summaryResult] = await Promise.all([
        api.getList<Customer>('/merchant/customers', { keyword: keyword || undefined, status, page_size: 100 }),
        api.getList<CustomerTag>('/merchant/customer-tags'),
        api.getList<BalanceLedger>('/merchant/balance-ledger', { page_size: 100 }),
        api.get<MemberSummary>('/merchant/member-summary'),
      ]);
      setCustomers(customerResult.items);
      setTags(tagResult.items);
      setLedger(ledgerResult.items);
      setSummary(summaryResult);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [keyword, messageApi, status]);

  useEffect(() => { void load(); }, [load]);

  const openCustomer = async (item?: Customer) => {
    let detail = item;
    if (item) {
      try {
        detail = await api.get<Customer>(`/merchant/customers/${item.id}`);
      } catch (error) {
        messageApi.error(errorMessage(error));
        return;
      }
    }
    setEditing(detail);
    customerForm.setFieldsValue(detail ? {
      name: detail.name,
      phone: detail.phone,
      source: detail.source,
      status: detail.status,
      remark: detail.remark,
    } : { source: 'MANUAL', status: 'ACTIVE' });
    setCustomerOpen(true);
  };

  const saveCustomer = async () => {
    const values = await customerForm.validateFields();
    setSaving(true);
    try {
      if (editing) await api.put(`/merchant/customers/${editing.id}`, values);
      else await api.post('/merchant/customers', values);
      setCustomerOpen(false);
      messageApi.success(editing ? '顾客资料已更新' : '顾客已创建');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const viewCustomer = async (item: Customer): Promise<Customer | undefined> => {
    try {
      const detail = await api.get<Customer>(`/merchant/customers/${item.id}`);
      setSelected(detail);
      return detail;
    } catch (error) {
      messageApi.error(errorMessage(error));
      return undefined;
    }
  };

  const archiveCustomer = async (item: Customer) => {
    try {
      await api.delete(`/merchant/customers/${item.id}`);
      messageApi.success('顾客已归档，历史交易与流水仍保留');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const openTag = (item?: CustomerTag) => {
    setEditingTag(item);
    tagForm.setFieldsValue(item ? { name: item.name, color: item.color, description: item.description, enabled: item.status === 'ACTIVE' } : { color: 'blue', enabled: true });
    setTagOpen(true);
  };

  const saveTag = async () => {
    const values = await tagForm.validateFields();
    setSaving(true);
    try {
      const payload = { name: values.name, color: values.color, description: values.description || '', status: values.enabled ? 'ACTIVE' : 'DISABLED' };
      if (editingTag) await api.put(`/merchant/customer-tags/${editingTag.id}`, payload);
      else await api.post('/merchant/customer-tags', payload);
      setTagOpen(false);
      messageApi.success(editingTag ? '标签已更新' : '标签已创建');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const openAssignments = (item: Customer) => {
    setSelected(item);
    assignmentForm.setFieldsValue({ tag_ids: item.tags?.map((tag) => tag.id) || [] });
    setAssignmentOpen(true);
  };

  const saveAssignments = async () => {
    if (!selected) return;
    const values = await assignmentForm.validateFields();
    setSaving(true);
    try {
      await api.put(`/merchant/customers/${selected.id}/tags`, values);
      setAssignmentOpen(false);
      messageApi.success('顾客标签已更新');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const adjustBalance = async () => {
    if (!selected) return;
    const values = await adjustForm.validateFields();
    setSaving(true);
    try {
      if (!adjustmentKey.current) adjustmentKey.current = newKey('balance');
      await api.postIdempotent(`/merchant/customers/${selected.id}/balance-adjustments`, {
        bucket: values.bucket,
        direction: values.direction,
        amount_cents: Math.round(values.amount * 100),
        remark: values.remark,
      }, adjustmentKey.current);
      setAdjustOpen(false);
      adjustmentKey.current = '';
      adjustForm.resetFields();
      messageApi.success('已追加余额调账流水');
      await load();
      await viewCustomer(selected);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading title="用户管理" description="顾客、标签、会员身份与余额流水统一管理；资金变更仅通过不可变流水完成" extra={<Button icon={<ReloadOutlined />} onClick={() => void load()}>刷新</Button>} />
      <Row gutter={[16, 16]} className="member-summary-grid">
        <Col xs={12} lg={6}><Card className="member-summary-card"><Statistic title="顾客数" value={summary?.customer_count ?? 0} prefix={<UserOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card className="member-summary-card"><Statistic title="会员数" value={summary?.member_count ?? 0} prefix={<TagsOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card className="member-summary-card"><Statistic title="储值余额" value={(summary?.balance_cents ?? 0) / 100} precision={2} prefix="¥" /></Card></Col>
        <Col xs={12} lg={6}><Card className="member-summary-card"><Statistic title="已拉黑" value={summary?.blocked_customer_count ?? 0} /></Card></Col>
      </Row>
      <Card bordered={false} className="content-card member-tabs-card">
        <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
          {
            key: 'customers', label: '用户列表', children: <>
              <div className="member-filter-bar">
                <Space wrap>
                  <Input allowClear prefix={<SearchOutlined />} value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="姓名、手机号、会员号" style={{ width: 260 }} />
                  <Select allowClear value={status} onChange={setStatus} placeholder="全部状态" style={{ width: 130 }} options={[{ value: 'ACTIVE', label: '正常' }, { value: 'BLOCKED', label: '已拉黑' }, { value: 'DISABLED', label: '已停用' }]} />
                </Space>
                <Button type="primary" icon={<PlusOutlined />} onClick={() => openCustomer()}>新增顾客</Button>
              </div>
              <Table<Customer> rowKey="id" loading={loading} dataSource={customers} scroll={{ x: 1160 }} columns={[
                { title: '顾客', width: 220, render: (_, item) => <Space><Avatar src={item.avatar_url} className="member-avatar">{initials(item.name)}</Avatar><div><Button type="link" onClick={() => void viewCustomer(item)}>{item.name}</Button><div><Typography.Text type="secondary">{item.phone_masked || item.phone || '未留手机号'}</Typography.Text></div></div></Space> },
                { title: '会员', width: 150, render: (_, item) => item.member_id ? <div><Tag color="gold">{item.level_name || '普通会员'}</Tag><small>{item.member_no}</small></div> : <Typography.Text type="secondary">未开卡</Typography.Text> },
                { title: '标签', width: 190, render: (_, item) => <Space wrap size={[2, 2]}>{item.tags?.map((tag) => <Tag key={String(tag.id)} color={tag.color}>{tag.name}</Tag>)}<Button size="small" type="text" icon={<TagsOutlined />} onClick={() => void viewCustomer(item).then((detail) => { if (detail) openAssignments(detail); })} /></Space> },
                { title: '余额', width: 120, render: (_, item) => <span className="member-balance">{yuan(item.balance_cents / 100)}</span> },
                { title: '消费', width: 150, render: (_, item) => <div>{yuan(item.net_spent_cents / 100)}<div><Typography.Text type="secondary">{item.order_count} 笔订单</Typography.Text></div></div> },
                { title: '来源', dataIndex: 'source_store_name', width: 130, render: (value) => value || '手工录入' },
                { title: '状态', width: 100, render: (_, item) => <Tag color={item.status === 'ACTIVE' ? 'success' : item.status === 'BLOCKED' ? 'error' : 'default'}>{item.status === 'ACTIVE' ? '正常' : item.status === 'BLOCKED' ? '已拉黑' : '已停用'}</Tag> },
                { title: '注册时间', dataIndex: 'registered_at', width: 170, render: dateTime },
                { title: '操作', fixed: 'right', width: 150, render: (_, item) => <Space><Button type="link" icon={<EditOutlined />} onClick={() => void openCustomer(item)}>编辑</Button><Popconfirm title="归档后不再出现在顾客列表，资金流水仍会保留" onConfirm={() => void archiveCustomer(item)}><Button type="link" danger icon={<DeleteOutlined />} /></Popconfirm></Space> },
              ]} />
            </>,
          },
          {
            key: 'tags', label: '标签管理', children: <>
              <div className="member-filter-bar"><Typography.Text type="secondary">标签用于商户内部筛选，不会展示给顾客</Typography.Text><Button type="primary" icon={<PlusOutlined />} onClick={() => openTag()}>新增标签</Button></div>
              <Table<CustomerTag> rowKey="id" loading={loading} dataSource={tags} columns={[
                { title: '标签', render: (_, item) => <Tag color={item.color}>{item.name}</Tag> },
                { title: '说明', dataIndex: 'description', render: (value) => value || '—' },
                { title: '顾客数', dataIndex: 'customer_count' },
                { title: '状态', render: (_, item) => <Tag color={item.status === 'ACTIVE' ? 'success' : 'default'}>{item.status === 'ACTIVE' ? '启用' : '停用'}</Tag> },
                { title: '操作', render: (_, item) => <Space><Button type="link" onClick={() => openTag(item)}>编辑</Button><Popconfirm title="删除标签会解除全部顾客关联" onConfirm={async () => { try { await api.delete(`/merchant/customer-tags/${item.id}`); await load(); } catch (error) { messageApi.error(errorMessage(error)); } }}><Button type="link" danger>删除</Button></Popconfirm></Space> },
              ]} />
            </>,
          },
          {
            key: 'ledger', label: '余额流水', children: <>
              <Typography.Paragraph type="secondary">余额流水不可编辑或删除；发现错误时请由老板追加反向调账流水。</Typography.Paragraph>
              <Table<BalanceLedger> rowKey="id" loading={loading} dataSource={ledger} scroll={{ x: 1000 }} columns={[
                { title: '顾客', dataIndex: 'customer_name', width: 150 },
                { title: '账户', dataIndex: 'bucket', width: 100, render: (value) => value === 'PRINCIPAL' ? '本金' : '赠送金' },
                { title: '变动', dataIndex: 'delta_cents', width: 120, render: (value: number) => <span className={value >= 0 ? 'member-positive' : 'member-negative'}>{value >= 0 ? '+' : ''}{yuan(value / 100)}</span> },
                { title: '变动前', dataIndex: 'balance_before_cents', width: 120, render: (value) => yuan(Number(value) / 100) },
                { title: '变动后', dataIndex: 'balance_after_cents', width: 120, render: (value) => yuan(Number(value) / 100) },
                { title: '类型', dataIndex: 'entry_type', width: 140 },
                { title: '业务号', dataIndex: 'business_no', width: 190, render: (value) => value || '—' },
                { title: '说明', dataIndex: 'remark', width: 220, render: (value) => value || '—' },
                { title: '时间', dataIndex: 'created_at', width: 170, render: dateTime },
              ]} />
            </>,
          },
        ]} />
      </Card>

      <Drawer title="顾客详情" width={620} open={Boolean(selected)} onClose={() => setSelected(undefined)} extra={selected && owner ? <Button icon={<WalletOutlined />} onClick={() => { adjustmentKey.current = newKey('balance'); adjustForm.resetFields(); adjustForm.setFieldsValue({ bucket: 'PRINCIPAL', direction: 'CREDIT' }); setAdjustOpen(true); }}>余额调账</Button> : undefined}>
        {selected && <>
          <Space size={14} style={{ marginBottom: 20 }}><Avatar size={58} src={selected.avatar_url} className="member-avatar">{initials(selected.name)}</Avatar><div><Typography.Title level={4} style={{ margin: 0 }}>{selected.name}</Typography.Title><Typography.Text type="secondary">{selected.phone_masked || selected.phone || '未留手机号'}</Typography.Text></div></Space>
          <div className="member-detail-grid">
            <div className="member-detail-item"><small>会员等级</small><strong>{selected.level_name || '未开卡'}</strong></div>
            <div className="member-detail-item"><small>会员号</small><strong>{selected.member_no || '—'}</strong></div>
            <div className="member-detail-item"><small>本金余额</small><strong>{yuan(selected.principal_cents / 100)}</strong></div>
            <div className="member-detail-item"><small>赠送余额</small><strong>{yuan(selected.bonus_cents / 100)}</strong></div>
            <div className="member-detail-item"><small>累计消费</small><strong>{yuan(selected.net_spent_cents / 100)}</strong></div>
            <div className="member-detail-item"><small>订单数</small><strong>{selected.order_count}</strong></div>
          </div>
          <Descriptions column={1} bordered size="small" items={[
            { key: 'source', label: '来源', children: selected.source_store_name || selected.source },
            { key: 'registered', label: '注册时间', children: dateTime(selected.registered_at) },
            { key: 'remark', label: '备注', children: selected.remark || '—' },
            { key: 'tags', label: '标签', children: <Space wrap>{selected.tags?.map((tag) => <Tag key={String(tag.id)} color={tag.color}>{tag.name}</Tag>)}</Space> },
          ]} />
        </>}
      </Drawer>

      <Modal title={editing ? '编辑顾客' : '新增顾客'} open={customerOpen} onCancel={() => setCustomerOpen(false)} onOk={() => void saveCustomer()} confirmLoading={saving}>
        <Form form={customerForm} layout="vertical"><Row gutter={12}><Col span={12}><Form.Item label="姓名" name="name" rules={[{ required: true }]}><Input /></Form.Item></Col><Col span={12}><Form.Item label="手机号" name="phone"><Input /></Form.Item></Col></Row><Row gutter={12}><Col span={12}><Form.Item label="来源" name="source"><Select options={[{ value: 'MANUAL', label: '手工录入' }, { value: 'MINIPROGRAM', label: '微信小程序' }, { value: 'IMPORT', label: '批量导入' }]} /></Form.Item></Col><Col span={12}><Form.Item label="状态" name="status"><Select options={[{ value: 'ACTIVE', label: '正常' }, { value: 'BLOCKED', label: '拉黑' }, { value: 'DISABLED', label: '停用' }]} /></Form.Item></Col></Row><Form.Item label="备注" name="remark"><Input.TextArea rows={3} maxLength={500} showCount /></Form.Item></Form>
      </Modal>
      <Modal title={editingTag ? '编辑标签' : '新增标签'} open={tagOpen} onCancel={() => setTagOpen(false)} onOk={() => void saveTag()} confirmLoading={saving}><Form form={tagForm} layout="vertical"><Form.Item label="标签名称" name="name" rules={[{ required: true }]}><Input /></Form.Item><Form.Item label="颜色" name="color"><Select options={['blue', 'green', 'gold', 'orange', 'red', 'purple', 'cyan'].map((value) => ({ value, label: <Tag color={value}>{value}</Tag> }))} /></Form.Item><Form.Item label="说明" name="description"><Input.TextArea /></Form.Item><Form.Item label="启用" name="enabled" valuePropName="checked"><Switch /></Form.Item></Form></Modal>
      <Modal title={`设置 ${selected?.name || ''} 的标签`} open={assignmentOpen} onCancel={() => setAssignmentOpen(false)} onOk={() => void saveAssignments()} confirmLoading={saving}><Form form={assignmentForm} layout="vertical"><Form.Item name="tag_ids" label="顾客标签"><Select mode="multiple" allowClear options={tags.filter((tag) => tag.status === 'ACTIVE').map((tag) => ({ value: tag.id, label: tag.name }))} /></Form.Item></Form></Modal>
      <Modal title={`为 ${selected?.name || '顾客'} 追加余额调账流水`} open={adjustOpen} maskClosable={!saving} keyboard={!saving} cancelButtonProps={{ disabled: saving }} onCancel={() => { if (saving) return; adjustmentKey.current = ''; adjustForm.resetFields(); setAdjustOpen(false); }} onOk={() => void adjustBalance()} confirmLoading={saving} okButtonProps={{ danger: true }}><Typography.Paragraph type="warning">本次调账对象：<strong>{selected?.name || '—'}</strong>。调账不会覆盖历史余额，提交后将追加一条不可删除的流水，并记录操作者与原因。</Typography.Paragraph><Form form={adjustForm} layout="vertical"><Row gutter={12}><Col span={12}><Form.Item label="账户" name="bucket" rules={[{ required: true }]}><Select options={[{ value: 'PRINCIPAL', label: '本金' }, { value: 'BONUS', label: '赠送金' }]} /></Form.Item></Col><Col span={12}><Form.Item label="方向" name="direction" rules={[{ required: true }]}><Select options={[{ value: 'CREDIT', label: '增加' }, { value: 'DEBIT', label: '扣减' }]} /></Form.Item></Col></Row><Form.Item label="金额" name="amount" rules={[{ required: true }]}><InputNumber min={0.01} precision={2} prefix="¥" style={{ width: '100%' }} /></Form.Item><Form.Item label="调账原因" name="remark" rules={[{ required: true, message: '必须填写原因' }]}><Input.TextArea rows={3} maxLength={255} /></Form.Item></Form></Modal>
    </div>
  );
}
