import {
  CrownOutlined,
  EditOutlined,
  LockOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
  UserOutlined,
} from '@ant-design/icons';
import {
  Avatar,
  Button,
  Card,
  Col,
  Empty,
  Form,
  Input,
  Modal,
  Row,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { api, errorMessage } from '../api/client';
import { useAuth } from '../auth/AuthContext';
import { assignableMerchantRoles, canManageStaffRole } from '../auth/permissions';
import { PageHeading } from '../components/PageHeading';
import type { Staff } from '../types';
import { dateTime, initials } from '../utils/format';

const roles = [
  { key: 'MERCHANT_OWNER', name: '老板', icon: <CrownOutlined />, color: '#9a623b', permissions: ['全部数据', '退款', '员工管理', '门店设置'] },
  { key: 'MERCHANT_MANAGER', name: '店长', icon: <SafetyCertificateOutlined />, color: '#446b8d', permissions: ['订单管理', '商品库存', '打印管理', '经营数据'] },
  { key: 'MERCHANT_STAFF', name: '店员', icon: <UserOutlined />, color: '#4b8060', permissions: ['订单查看', '状态流转', '补打小票'] },
];

interface StaffFormValues {
  name: string;
  phone: string;
  role: string;
  password?: string;
  enabled: boolean;
}

function normalizeStaff(value: Staff): Staff {
  const raw = value as unknown as Record<string, unknown>;
  const role = value.role ?? String(raw.role ?? 'MERCHANT_STAFF');
  return {
    ...value,
    name: value.name ?? String(raw.display_name ?? raw.username ?? ''),
    phone: value.phone ?? String(raw.username ?? ''),
    role,
    roleName: value.roleName ?? roles.find((item) => item.key === role)?.name,
    enabled: value.enabled ?? String(raw.status ?? 'ACTIVE') === 'ACTIVE',
    createdAt: value.createdAt ?? String(raw.created_at ?? ''),
  };
}

function staffPayload(values: StaffFormValues) {
  return {
    username: values.phone,
    password: values.password ?? '',
    display_name: values.name,
    role: values.role,
    status: values.enabled ? 'ACTIVE' : 'DISABLED',
  };
}

export function StaffPage() {
  const { user } = useAuth();
  const [staff, setStaff] = useState<Staff[]>([]);
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<Staff | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm<StaffFormValues>();
  const [messageApi, contextHolder] = message.useMessage();
  const assignableRoles = assignableMerchantRoles(user);
  const roleOptions = roles
    .filter((role) => assignableRoles.includes(role.key))
    .map((role) => ({ label: `${role.name} · ${role.permissions.join('、')}`, value: role.key }));

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setStaff((await api.getList<Staff>('/merchant/staff')).items.map(normalizeStaff));
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [messageApi]);

  useEffect(() => { void load(); }, [load]);

  const openStaff = (item?: Staff) => {
    if (item && !canManageStaffRole(user, item.role)) {
      messageApi.warning('店长只能管理店员账号');
      return;
    }
    setEditing(item ?? null);
    form.setFieldsValue(item ? { name: item.name, phone: item.phone, role: item.role, enabled: item.enabled } : { role: 'MERCHANT_STAFF', enabled: true, name: '', phone: '', password: '' });
    setModalOpen(true);
  };

  const saveStaff = async () => {
    const values = await form.validateFields();
    if (!assignableRoles.includes(values.role) || (editing && !canManageStaffRole(user, editing.role))) {
      messageApi.error('当前账号无权分配或修改该角色');
      return;
    }
    setSaving(true);
    try {
      const saved = editing
        ? await api.put<Staff>(`/merchant/staff/${editing.id}`, staffPayload(values))
        : await api.post<Staff>('/merchant/staff', staffPayload(values));
      const normalized = normalizeStaff(saved);
      setStaff((items) => editing
        ? items.map((item) => item.id === editing.id ? normalized : item)
        : [normalized, ...items]);
      messageApi.success(editing ? '员工信息已更新' : '员工账号已创建');
      setModalOpen(false);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const toggleStaff = async (item: Staff, enabled: boolean) => {
    if (!canManageStaffRole(user, item.role)) {
      messageApi.warning('店长只能管理店员账号');
      return;
    }
    try {
      const updated = normalizeStaff(await api.put<Staff>(`/merchant/staff/${item.id}`, staffPayload({ name: item.name, phone: item.phone, role: item.role, enabled })));
      setStaff((items) => items.map((staffItem) => staffItem.id === item.id ? updated : staffItem));
      messageApi.success(enabled ? '账号已启用' : '账号已停用');
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const roleCounts = useMemo(() => staff.reduce<Record<string, number>>((counts, item) => ({ ...counts, [item.role]: (counts[item.role] ?? 0) + 1 }), {}), [staff]);

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading title="员工与角色" description="按岗位分配最小权限，所有关键操作都会记录操作人" extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => openStaff()}>新增员工</Button>} />
      <Row gutter={[16, 16]} className="role-grid">
        {roles.map((role) => (
          <Col xs={12} xl={6} key={role.key}>
            <Card bordered={false} className="role-card">
              <span className="role-icon" style={{ backgroundColor: `${role.color}18`, color: role.color }}>{role.icon}</span>
              <div className="role-title"><strong>{role.name}</strong><Tag>{roleCounts[role.key] ?? 0} 人</Tag></div>
              <Typography.Paragraph type="secondary" ellipsis={{ rows: 2 }}>{role.permissions.join(' · ')}</Typography.Paragraph>
            </Card>
          </Col>
        ))}
      </Row>
      <Card bordered={false} className="content-card table-card">
        <Table<Staff>
          rowKey="id"
          loading={loading}
          dataSource={staff}
          scroll={{ x: 780 }}
          locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无员工账号" /> }}
          columns={[
            {
              title: '员工', key: 'staff', width: 230,
              render: (_, item) => <Space><Avatar style={{ background: '#d9b18d' }}>{initials(item.name)}</Avatar><div><Typography.Text strong>{item.name}</Typography.Text><div><Typography.Text type="secondary">{item.phone}</Typography.Text></div></div></Space>,
            },
            { title: '角色', dataIndex: 'role', width: 120, render: (value, item) => item.roleName || roles.find((role) => role.key === value)?.name || value },
            { title: '权限范围', dataIndex: 'role', render: (value) => roles.find((role) => role.key === value)?.permissions.slice(0, 3).map((permission) => <Tag key={permission}>{permission}</Tag>) || '--' },
            { title: '最近登录', dataIndex: 'lastLoginAt', width: 180, render: dateTime },
            { title: '状态', dataIndex: 'enabled', width: 100, render: (value: boolean, item) => <Switch checked={value} disabled={item.role === 'MERCHANT_OWNER' || !canManageStaffRole(user, item.role)} onChange={(checked) => void toggleStaff(item, checked)} /> },
            { title: '操作', key: 'action', width: 100, fixed: 'right', render: (_, item) => <Button type="link" icon={<EditOutlined />} disabled={!canManageStaffRole(user, item.role)} onClick={() => openStaff(item)}>编辑</Button> },
          ]}
        />
      </Card>

      <Modal title={editing ? '编辑员工' : '新增员工'} open={modalOpen} onCancel={() => setModalOpen(false)} onOk={() => void saveStaff()} confirmLoading={saving} okText="保存">
        <Form<StaffFormValues> form={form} layout="vertical">
          <Form.Item label="姓名" name="name" rules={[{ required: true, message: '请输入员工姓名' }]}><Input placeholder="真实姓名或店内称呼" /></Form.Item>
          <Form.Item label="手机号 / 登录账号" name="phone" rules={[{ required: true, message: '请输入手机号' }, { pattern: /^1\d{10}$/, message: '请输入正确的手机号' }]}><Input placeholder="员工使用该手机号登录" /></Form.Item>
          {!editing && <Form.Item label="初始密码" name="password" rules={[{ required: true, min: 8, message: '密码至少 8 位' }]}><Input.Password prefix={<LockOutlined />} placeholder="首次登录后建议修改" /></Form.Item>}
          <Form.Item label="岗位角色" name="role" rules={[{ required: true }]}>
            <Select options={roleOptions} />
          </Form.Item>
          <Form.Item label="启用账号" name="enabled" valuePropName="checked"><Switch disabled={editing?.role === 'MERCHANT_OWNER'} /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
