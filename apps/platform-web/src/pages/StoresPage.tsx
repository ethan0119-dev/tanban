import { EditOutlined, EnvironmentOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
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
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useState } from 'react';
import { PageHeader } from '../components/PageHeader';
import { StatusTag } from '../components/StatusTag';
import { storeService, tenantService } from '../lib/services';
import type { PageMeta, Store, Tenant } from '../types';

interface StoreFormValues {
  tenantId: string;
  name: string;
  code?: string;
  phone?: string;
  address?: string;
  businessHours?: string;
  status: 'active' | 'disabled';
}

export function StoresPage() {
  const [rows, setRows] = useState<Store[]>([]);
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [meta, setMeta] = useState<PageMeta>({ page: 1, pageSize: 20, total: 0 });
  const [keyword, setKeyword] = useState('');
  const [tenantId, setTenantId] = useState<string>();
  const [status, setStatus] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<Store>();
  const [modalOpen, setModalOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm<StoreFormValues>();
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async (page = meta.page, pageSize = meta.pageSize) => {
    setLoading(true);
    try {
      const result = await storeService.list({ page, pageSize, keyword, tenantId, status });
      setRows(result.items);
      setMeta(result.meta);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '门店列表加载失败');
    } finally {
      setLoading(false);
    }
  }, [keyword, messageApi, meta.page, meta.pageSize, status, tenantId]);

  useEffect(() => {
    void load(1);
    tenantService.list({ page: 1, pageSize: 100, status: 'active' })
      .then((result) => setTenants(result.items))
      .catch(() => undefined);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const openEdit = (record: Store) => {
    setEditing(record);
    form.setFieldsValue({
      tenantId: record.tenantId || '',
      name: record.name,
      code: record.code,
      phone: record.phone,
      address: record.address,
      businessHours: record.businessHours,
      status: record.status === 'disabled' ? 'disabled' : 'active',
    });
    setModalOpen(true);
  };

  const saveStore = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      if (!editing) return;
      await storeService.update(editing.tenantId || values.tenantId, editing.id, values);
      messageApi.success('店铺信息已更新');
      setModalOpen(false);
      void load(meta.page);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '门店保存失败');
    } finally {
      setSaving(false);
    }
  };

  const toggleStatus = async (record: Store, enabled: boolean) => {
    if (!record.tenantId) {
      messageApi.error('该门店缺少所属商户信息，无法更新状态');
      return;
    }
    try {
      await storeService.update(record.tenantId, record.id, { ...record, status: enabled ? 'active' : 'disabled' });
      messageApi.success(enabled ? '门店已启用' : '门店已停用');
      void load();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '状态更新失败');
    }
  };

  const columns: ColumnsType<Store> = [
    {
      title: '门店', dataIndex: 'name', key: 'name', fixed: 'left', width: 220,
      render: (value, row) => <div className="entity-name entity-name--store"><span><EnvironmentOutlined /></span><div><strong>{value}</strong><small>{row.code || '未设置门店编号'}</small></div></div>,
    },
    { title: '所属商户', dataIndex: 'tenantName', key: 'tenantName', width: 180, render: (value, row) => value || tenants.find((tenant) => tenant.id === row.tenantId)?.name || '—' },
    { title: '联系电话', dataIndex: 'phone', key: 'phone', width: 140, render: (value) => value || '—' },
    { title: '地址', dataIndex: 'address', key: 'address', ellipsis: true, width: 260, render: (value) => value || '—' },
    { title: '营业时间', dataIndex: 'businessHours', key: 'businessHours', width: 150, render: (value) => value || '未设置' },
    { title: '累计订单', dataIndex: 'orderCount', key: 'orderCount', width: 110, align: 'right', render: (value) => Number(value || 0).toLocaleString('zh-CN') },
    { title: '状态', dataIndex: 'status', key: 'status', width: 100, render: (value) => <StatusTag status={value} /> },
    {
      title: '操作', key: 'actions', fixed: 'right', width: 175,
      render: (_, record) => <Space><Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</Button><Popconfirm title={record.status === 'disabled' ? '启用该门店？' : '停用该门店？'} description={record.status === 'disabled' ? '启用后可恢复接单。' : '停用后顾客将无法在该门店下单。'} onConfirm={() => void toggleStatus(record, record.status === 'disabled')}><Switch size="small" checked={record.status !== 'disabled'} /></Popconfirm></Space>,
    },
  ];

  return (
    <div>
      {contextHolder}
      <PageHeader title="店铺总览" description="每个商户租户对应一家独立店铺；新店请在商户管理中开通新的租户。" />
      <Card bordered={false}>
        <Row gutter={[12, 12]} className="table-toolbar">
          <Col xs={24} md={8} lg={7}><Input allowClear prefix={<SearchOutlined />} placeholder="搜索门店名称、编号或地址" value={keyword} onChange={(event) => setKeyword(event.target.value)} onPressEnter={() => void load(1)} /></Col>
          <Col xs={12} md={6} lg={5}><Select showSearch allowClear optionFilterProp="label" placeholder="全部商户" value={tenantId} onChange={setTenantId} options={tenants.map((tenant) => ({ value: tenant.id, label: tenant.name }))} style={{ width: '100%' }} /></Col>
          <Col xs={12} md={4} lg={4}><Select allowClear placeholder="全部状态" value={status} onChange={setStatus} options={[{ value: 'active', label: '正常' }, { value: 'disabled', label: '已停用' }]} style={{ width: '100%' }} /></Col>
          <Col xs={24} md={6} lg={8}><Space><Button type="primary" icon={<SearchOutlined />} onClick={() => void load(1)}>查询</Button><Button icon={<ReloadOutlined />} onClick={() => { setKeyword(''); setTenantId(undefined); setStatus(undefined); setTimeout(() => void load(1), 0); }}>重置</Button></Space></Col>
        </Row>
        <Table<Store>
          rowKey="id"
          columns={columns}
          dataSource={rows}
          loading={loading}
          scroll={{ x: 1300 }}
          pagination={{ current: meta.page, pageSize: meta.pageSize, total: meta.total, showSizeChanger: true, showTotal: (total) => `共 ${total} 家门店`, onChange: (page, pageSize) => void load(page, pageSize) }}
        />
      </Card>

      <Modal title="编辑店铺" width={640} open={modalOpen} onCancel={() => setModalOpen(false)} onOk={() => void saveStore()} okText="保存" confirmLoading={saving}>
        <Form form={form} layout="vertical" requiredMark={false} className="modal-form">
          <Form.Item label="所属商户" name="tenantId" rules={[{ required: true, message: '请选择所属商户' }]}><Select showSearch optionFilterProp="label" disabled={Boolean(editing)} placeholder="选择商户" options={tenants.map((tenant) => ({ value: tenant.id, label: tenant.name }))} /></Form.Item>
          <Row gutter={12}>
            <Col span={14}><Form.Item label="门店名称" name="name" rules={[{ required: true, message: '请输入门店名称' }]}><Input placeholder="例如：码农咖啡（主门店）" /></Form.Item></Col>
            <Col span={10}><Form.Item label="门店编号" name="code" rules={[{ required: true, message: '请输入门店编号' }, { pattern: /^[A-Za-z0-9_-]+$/, message: '仅支持字母、数字、下划线和短横线' }]}><Input placeholder="例如：STORE001" /></Form.Item></Col>
          </Row>
          <Row gutter={12}>
            <Col span={12}><Form.Item label="联系电话" name="phone"><Input placeholder="门店联系电话" /></Form.Item></Col>
            <Col span={12}><Form.Item label="营业时间" name="businessHours"><Input placeholder="例如：18:00 - 02:00" /></Form.Item></Col>
          </Row>
          <Form.Item label="经营地址" name="address"><Input.TextArea rows={2} placeholder="请输入详细经营地址或摊位位置" /></Form.Item>
          <Form.Item label="门店状态" name="status"><Select options={[{ value: 'active', label: '正常营业' }, { value: 'disabled', label: '暂停营业' }]} /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
