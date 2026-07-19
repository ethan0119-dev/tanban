import { EyeOutlined, PlusOutlined, ReloadOutlined, SearchOutlined, ShopOutlined } from '@ant-design/icons';
import {
  Button,
  Card,
  Col,
  Descriptions,
  Drawer,
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
import { tenantService } from '../lib/services';
import type { PageMeta, Tenant } from '../types';

interface TenantFormValues {
  code: string;
  name: string;
  contactName: string;
  contactPhone: string;
  status: 'active' | 'pending';
  paymentProvider: 'mock' | 'tianque';
  paymentMerchantNo?: string;
  paymentSubAppId?: string;
  initialStoreCode: string;
  initialStoreName: string;
  ownerUsername: string;
  ownerPassword: string;
  ownerDisplayName: string;
}

const paymentStatusText: Record<string, string> = {
  unbound: '未绑定',
  pending: '审核中',
  active: '已开通',
  rejected: '已驳回',
};

const paymentStatusColor: Record<string, string> = {
  unbound: 'default',
  pending: 'processing',
  active: 'success',
  rejected: 'error',
};

export function TenantsPage() {
  const [rows, setRows] = useState<Tenant[]>([]);
  const [meta, setMeta] = useState<PageMeta>({ page: 1, pageSize: 20, total: 0 });
  const [keyword, setKeyword] = useState('');
  const [status, setStatus] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [selected, setSelected] = useState<Tenant>();
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm<TenantFormValues>();
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async (page = meta.page, pageSize = meta.pageSize) => {
    setLoading(true);
    try {
      const result = await tenantService.list({ page, pageSize, keyword, status });
      setRows(result.items);
      setMeta(result.meta);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '商户列表加载失败');
    } finally {
      setLoading(false);
    }
  }, [keyword, messageApi, meta.page, meta.pageSize, status]);

  useEffect(() => { void load(1); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const createTenant = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      await tenantService.create(values);
      messageApi.success('商户已创建，可继续配置门店和支付信息');
      setCreateOpen(false);
      form.resetFields();
      void load(1);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '商户创建失败');
    } finally {
      setSaving(false);
    }
  };

  const toggleStatus = async (record: Tenant, enabled: boolean) => {
    try {
      await tenantService.update(record.id, { ...record, status: enabled ? 'active' : 'disabled' });
      messageApi.success(enabled ? '商户已启用' : '商户已停用');
      void load();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '状态更新失败');
    }
  };

  const columns: ColumnsType<Tenant> = [
    {
      title: '商户名称', dataIndex: 'name', key: 'name', fixed: 'left', width: 230,
      render: (value, row) => <div className="entity-name entity-name--merchant"><span><ShopOutlined /></span><div><strong>{value}</strong><small>{row.code || '—'}</small></div></div>,
    },
    { title: '联系人', key: 'contact', width: 170, render: (_, row) => <div>{row.contactName || '—'}<small className="table-subtext">{row.contactPhone || ''}</small></div> },
    { title: '门店', dataIndex: 'storeCount', key: 'storeCount', width: 90, align: 'right', render: (value) => `${value || 0} 家` },
    { title: '累计订单', dataIndex: 'orderCount', key: 'orderCount', width: 120, align: 'right', render: (value) => Number(value || 0).toLocaleString('zh-CN') },
    { title: '支付接入', dataIndex: 'paymentStatus', key: 'paymentStatus', width: 110, render: (value = 'unbound') => <Tag color={paymentStatusColor[value] || 'default'}>{paymentStatusText[value] || value}</Tag> },
    { title: '状态', dataIndex: 'status', key: 'status', width: 100, render: (value) => <StatusTag status={value} /> },
    { title: '入驻时间', dataIndex: 'createdAt', key: 'createdAt', width: 120, render: (value) => value ? new Date(value).toLocaleDateString('zh-CN') : '—' },
    {
      title: '操作', key: 'actions', fixed: 'right', width: 190,
      render: (_, record) => <Space>
        <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => setSelected(record)}>查看</Button>
        <Popconfirm
          title={record.status === 'disabled' ? '启用该商户？' : '停用该商户？'}
          description={record.status === 'disabled' ? '启用后商户可恢复经营。' : '停用后商户后台与点单服务将受限。'}
          onConfirm={() => void toggleStatus(record, record.status === 'disabled')}
        ><Switch size="small" checked={record.status !== 'disabled'} /></Popconfirm>
      </Space>,
    },
  ];

  return (
    <div>
      {contextHolder}
      <PageHeader title="商户管理" description="管理 SaaS 租户、经营状态及支付接入关系。" extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => { form.resetFields(); form.setFieldsValue({ status: 'active', paymentProvider: 'mock' }); setCreateOpen(true); }}>新增商户</Button>} />
      <Card bordered={false}>
        <Row gutter={[12, 12]} className="table-toolbar">
          <Col xs={24} md={10} lg={8}><Input allowClear prefix={<SearchOutlined />} placeholder="搜索商户名称、编号或联系人" value={keyword} onChange={(event) => setKeyword(event.target.value)} onPressEnter={() => void load(1)} /></Col>
          <Col xs={12} md={6} lg={4}><Select allowClear placeholder="全部状态" value={status} onChange={setStatus} options={[{ value: 'active', label: '正常' }, { value: 'pending', label: '待审核' }, { value: 'disabled', label: '已停用' }]} style={{ width: '100%' }} /></Col>
          <Col xs={12} md={8} lg={12}><Space><Button type="primary" icon={<SearchOutlined />} onClick={() => void load(1)}>查询</Button><Button icon={<ReloadOutlined />} onClick={() => { setKeyword(''); setStatus(undefined); setTimeout(() => void load(1), 0); }}>重置</Button></Space></Col>
        </Row>
        <Table<Tenant>
          rowKey="id"
          columns={columns}
          dataSource={rows}
          loading={loading}
          scroll={{ x: 1220 }}
          pagination={{ current: meta.page, pageSize: meta.pageSize, total: meta.total, showSizeChanger: true, showTotal: (total) => `共 ${total} 家商户`, onChange: (page, pageSize) => void load(page, pageSize) }}
        />
      </Card>

      <Modal title="新增商户并开通账号" open={createOpen} width={760} okText="创建商户" onCancel={() => setCreateOpen(false)} onOk={() => void createTenant()} confirmLoading={saving}>
        <Form form={form} layout="vertical" requiredMark={false} className="modal-form">
          <Row gutter={12}>
            <Col span={15}><Form.Item label="商户名称" name="name" rules={[{ required: true, message: '请输入商户名称' }]}><Input placeholder="例如：码农咖啡" /></Form.Item></Col>
            <Col span={9}><Form.Item label="商户编号" name="code" rules={[{ required: true, message: '请输入商户编号' }, { pattern: /^[A-Za-z0-9_-]+$/, message: '仅支持字母、数字、下划线和短横线' }]}><Input placeholder="例如：MNKF001" /></Form.Item></Col>
          </Row>
          <Row gutter={12}>
            <Col span={12}><Form.Item label="联系人" name="contactName" rules={[{ required: true, message: '请输入联系人' }]}><Input /></Form.Item></Col>
            <Col span={12}><Form.Item label="联系电话" name="contactPhone" rules={[{ required: true, message: '请输入联系电话' }, { pattern: /^1\d{10}$/, message: '请输入有效手机号' }]}><Input /></Form.Item></Col>
          </Row>
          <Row gutter={12}>
            <Col span={12}><Form.Item label="首店名称" name="initialStoreName" rules={[{ required: true, message: '请输入首店名称' }]}><Input placeholder="例如：码农咖啡主门店" /></Form.Item></Col>
            <Col span={12}><Form.Item label="首店点单码" name="initialStoreCode" rules={[{ required: true, message: '请输入点单码' }, { pattern: /^[A-Za-z0-9_-]+$/, message: '仅支持字母、数字、下划线和短横线' }]}><Input placeholder="例如：manong-coffee" /></Form.Item></Col>
          </Row>
          <Row gutter={12}>
            <Col span={8}><Form.Item label="老板登录账号" name="ownerUsername" rules={[{ required: true, message: '请输入老板账号' }]}><Input autoComplete="off" /></Form.Item></Col>
            <Col span={8}><Form.Item label="老板姓名" name="ownerDisplayName" rules={[{ required: true, message: '请输入老板姓名' }]}><Input /></Form.Item></Col>
            <Col span={8}><Form.Item label="初始密码" name="ownerPassword" rules={[{ required: true, message: '请输入初始密码' }, { min: 8, message: '至少 8 位' }]}><Input.Password autoComplete="new-password" /></Form.Item></Col>
          </Row>
          <Row gutter={12}>
            <Col span={8}><Form.Item label="初始状态" name="status" rules={[{ required: true }]}><Select options={[{ value: 'active', label: '正常运营' }, { value: 'pending', label: '待完善资料' }]} /></Form.Item></Col>
            <Col span={8}><Form.Item label="支付适配器" name="paymentProvider" rules={[{ required: true }]}><Select options={[{ value: 'mock', label: '虚拟支付（联调）' }, { value: 'tianque', label: '会生活/天阙' }]} /></Form.Item></Col>
            <Col span={8}><Form.Item label="随行付商户号" name="paymentMerchantNo"><Input placeholder="联调阶段可留空" /></Form.Item></Col>
          </Row>
          <Form.Item label="微信支付子 AppID" name="paymentSubAppId"><Input placeholder="由支付渠道为该商户绑定后填写；联调阶段可留空" /></Form.Item>
        </Form>
      </Modal>

      <Drawer title="商户详情" width={520} open={Boolean(selected)} onClose={() => setSelected(undefined)} extra={selected && <StatusTag status={selected.status} />}>
        {selected && <>
          <div className="drawer-entity"><span><ShopOutlined /></span><div><h2>{selected.name}</h2><p>{selected.code || '暂无商户编号'}</p></div></div>
          <Descriptions column={1} bordered size="small" className="detail-descriptions">
            <Descriptions.Item label="商户编号">{selected.code || '—'}</Descriptions.Item>
            <Descriptions.Item label="联系人">{selected.contactName || '—'} {selected.contactPhone || ''}</Descriptions.Item>
            <Descriptions.Item label="门店数量">{selected.storeCount || 0} 家</Descriptions.Item>
            <Descriptions.Item label="累计订单">{selected.orderCount || 0} 单</Descriptions.Item>
            <Descriptions.Item label="支付状态"><Tag color={paymentStatusColor[selected.paymentStatus || 'unbound']}>{paymentStatusText[selected.paymentStatus || 'unbound']}</Tag></Descriptions.Item>
            <Descriptions.Item label="随行付商户号">{selected.paymentMerchantNo ? `${selected.paymentMerchantNo.slice(0, 4)}****${selected.paymentMerchantNo.slice(-4)}` : '未绑定'}</Descriptions.Item>
            <Descriptions.Item label="入驻时间">{selected.createdAt ? new Date(selected.createdAt).toLocaleString('zh-CN', { hour12: false }) : '—'}</Descriptions.Item>
            <Descriptions.Item label="服务到期">{selected.expiresAt ? new Date(selected.expiresAt).toLocaleDateString('zh-CN') : '未设置'}</Descriptions.Item>
          </Descriptions>
        </>}
      </Drawer>
    </div>
  );
}
