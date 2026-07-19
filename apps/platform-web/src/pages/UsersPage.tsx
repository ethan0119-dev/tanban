import { EditOutlined, KeyOutlined, PlusOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import {
  Button,
  Card,
  Col,
  Form,
  Input,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useState } from 'react';
import { PageHeader } from '../components/PageHeader';
import { StatusTag } from '../components/StatusTag';
import { userService } from '../lib/services';
import type { PageMeta, PlatformUser } from '../types';

interface UserFormValues {
  name: string;
  username: string;
  role: string;
  password?: string;
  status?: 'active' | 'disabled';
}

const roleOptions = [
  { value: 'PLATFORM_ADMIN', label: '超级管理员' },
  { value: 'PLATFORM_OPERATOR', label: '运营管理员' },
];

const roleNames = Object.fromEntries(roleOptions.map((item) => [item.value, item.label]));

export function UsersPage() {
  const [rows, setRows] = useState<PlatformUser[]>([]);
  const [meta, setMeta] = useState<PageMeta>({ page: 1, pageSize: 20, total: 0 });
  const [loading, setLoading] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [status, setStatus] = useState<string>();
  const [editing, setEditing] = useState<PlatformUser>();
  const [editOpen, setEditOpen] = useState(false);
  const [resetTarget, setResetTarget] = useState<PlatformUser>();
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm<UserFormValues>();
  const [passwordForm] = Form.useForm<{ password: string; confirm: string }>();
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async (page = meta.page, pageSize = meta.pageSize) => {
    setLoading(true);
    try {
      const result = await userService.list({ page, pageSize, keyword, status });
      setRows(result.items);
      setMeta(result.meta);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '管理员列表加载失败');
    } finally {
      setLoading(false);
    }
  }, [keyword, messageApi, meta.page, meta.pageSize, status]);

  useEffect(() => { void load(1); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const openCreate = () => {
    setEditing(undefined);
    form.resetFields();
    form.setFieldsValue({ role: 'PLATFORM_OPERATOR', status: 'active' });
    setEditOpen(true);
  };

  const openEdit = (record: PlatformUser) => {
    setEditing(record);
    form.setFieldsValue({
      name: record.name,
      username: record.username,
      role: record.role,
      status: record.status === 'disabled' ? 'disabled' : 'active',
    });
    setEditOpen(true);
  };

  const saveUser = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      if (editing) {
        const update = { ...values };
        delete update.password;
        await userService.update(editing.id, update);
        messageApi.success('管理员信息已更新');
      } else {
        await userService.create(values as UserFormValues & { password: string });
        messageApi.success('管理员已创建');
      }
      setEditOpen(false);
      void load(editing ? meta.page : 1);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const toggleStatus = async (record: PlatformUser, enabled: boolean) => {
    try {
      await userService.update(record.id, { ...record, status: enabled ? 'active' : 'disabled' });
      messageApi.success(enabled ? '管理员已启用' : '管理员已停用');
      void load();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '状态更新失败');
    }
  };

  const resetPassword = async () => {
    if (!resetTarget) return;
    const { password } = await passwordForm.validateFields();
    setSaving(true);
    try {
      await userService.resetPassword(resetTarget, password);
      messageApi.success('密码已重置');
      setResetTarget(undefined);
      passwordForm.resetFields();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '密码重置失败');
    } finally {
      setSaving(false);
    }
  };

  const columns: ColumnsType<PlatformUser> = [
    {
      title: '管理员', dataIndex: 'name', key: 'name', fixed: 'left', width: 210,
      render: (value, row) => <div className="entity-name"><span>{String(value || row.username).slice(0, 1)}</span><div><strong>{value || '未命名'}</strong><small>@{row.username}</small></div></div>,
    },
    { title: '角色', dataIndex: 'role', key: 'role', width: 140, render: (value) => <Tag color={value === 'PLATFORM_ADMIN' ? 'volcano' : 'blue'}>{roleNames[value] || value}</Tag> },
    { title: '状态', dataIndex: 'status', key: 'status', width: 100, render: (value) => <StatusTag status={value} /> },
    { title: '创建时间', dataIndex: 'createdAt', key: 'createdAt', width: 180, render: (value) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—' },
    {
      title: '操作', key: 'actions', fixed: 'right', width: 260,
      render: (_, record) => (
        <Space size="middle">
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</Button>
          <Button type="link" size="small" icon={<KeyOutlined />} onClick={() => { setResetTarget(record); passwordForm.resetFields(); }}>重置密码</Button>
          <Popconfirm
            title={record.status === 'disabled' ? '启用该管理员？' : '停用该管理员？'}
            description={record.status === 'disabled' ? '启用后可重新登录管理端。' : '停用后该账户将无法登录。'}
            onConfirm={() => void toggleStatus(record, record.status === 'disabled')}
          >
            <Switch size="small" checked={record.status !== 'disabled'} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      {contextHolder}
      <PageHeader
        title="管理员用户"
        description="创建平台管理员并按岗位分配访问权限。"
        extra={<Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建管理员</Button>}
      />
      <Card bordered={false}>
        <Row gutter={[12, 12]} className="table-toolbar">
          <Col xs={24} md={10} lg={8}><Input allowClear prefix={<SearchOutlined />} placeholder="搜索姓名、账号或手机号" value={keyword} onChange={(event) => setKeyword(event.target.value)} onPressEnter={() => void load(1)} /></Col>
          <Col xs={12} md={6} lg={4}><Select allowClear placeholder="全部状态" value={status} onChange={setStatus} options={[{ value: 'active', label: '正常' }, { value: 'disabled', label: '已停用' }]} style={{ width: '100%' }} /></Col>
          <Col xs={12} md={8} lg={12}><Space><Button type="primary" icon={<SearchOutlined />} onClick={() => void load(1)}>查询</Button><Button icon={<ReloadOutlined />} onClick={() => { setKeyword(''); setStatus(undefined); setTimeout(() => void load(1), 0); }}>重置</Button></Space></Col>
        </Row>
        <Table<PlatformUser>
          rowKey="id"
          columns={columns}
          dataSource={rows}
          loading={loading}
          scroll={{ x: 1100 }}
          pagination={{ current: meta.page, pageSize: meta.pageSize, total: meta.total, showSizeChanger: true, showTotal: (total) => `共 ${total} 位管理员`, onChange: (page, pageSize) => void load(page, pageSize) }}
        />
      </Card>

      <Modal title={editing ? '编辑管理员' : '新建管理员'} open={editOpen} onCancel={() => setEditOpen(false)} onOk={() => void saveUser()} confirmLoading={saving} okText="保存">
        <Form form={form} layout="vertical" requiredMark={false} className="modal-form">
          <Row gutter={12}>
            <Col span={12}><Form.Item label="姓名" name="name" rules={[{ required: true, message: '请输入姓名' }]}><Input placeholder="管理员姓名" /></Form.Item></Col>
            <Col span={12}><Form.Item label="登录账号" name="username" rules={[{ required: true, message: '请输入账号' }, { min: 4, message: '至少 4 个字符' }]}><Input placeholder="登录用户名" disabled={Boolean(editing)} /></Form.Item></Col>
          </Row>
          <Form.Item label="角色" name="role" rules={[{ required: true, message: '请选择角色' }]}><Select options={roleOptions} /></Form.Item>
          {!editing && <Form.Item label="初始密码" name="password" rules={[{ required: true, message: '请设置初始密码' }, { min: 8, message: '密码至少 8 位' }]}><Input.Password placeholder="至少 8 位" /></Form.Item>}
          <Form.Item label="账户状态" name="status"><Select options={[{ value: 'active', label: '正常' }, { value: 'disabled', label: '停用' }]} /></Form.Item>
        </Form>
      </Modal>

      <Modal title={`重置密码 · ${resetTarget?.name || ''}`} open={Boolean(resetTarget)} onCancel={() => setResetTarget(undefined)} onOk={() => void resetPassword()} confirmLoading={saving} okText="确认重置">
        <Form form={passwordForm} layout="vertical" requiredMark={false}>
          <Form.Item label="新密码" name="password" rules={[{ required: true, message: '请输入新密码' }, { min: 8, message: '密码至少 8 位' }]}><Input.Password placeholder="至少 8 位，建议包含字母和数字" /></Form.Item>
          <Form.Item label="确认新密码" name="confirm" dependencies={['password']} rules={[{ required: true, message: '请再次输入密码' }, ({ getFieldValue }) => ({ validator(_, value) { return !value || getFieldValue('password') === value ? Promise.resolve() : Promise.reject(new Error('两次输入的密码不一致')); } })]}><Input.Password /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
