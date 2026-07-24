import { BankOutlined, CalendarOutlined, CopyOutlined, EyeOutlined, FileImageOutlined, KeyOutlined, PlusOutlined, ReloadOutlined, SearchOutlined, ShopOutlined, UploadOutlined, UserOutlined } from '@ant-design/icons';
import {
  Button,
  Alert,
  Card,
  Col,
  Descriptions,
  DatePicker,
  Drawer,
  Empty,
  Form,
  Image,
  Input,
  Modal,
  Popconfirm,
  Row,
  Radio,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import type { UploadProps } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import dayjs, { type Dayjs } from 'dayjs';
import { PageHeader } from '../components/PageHeader';
import { StatusTag } from '../components/StatusTag';
import { generateInitialPassword, generatedOwnerUsername, type OwnerUsernameMode } from '../features/tenants/credentials';
import { tenantService } from '../lib/services';
import type { PageMeta, Tenant, TenantPaymentSettings } from '../types';
import { formatBeijingDate, formatBeijingDateTime } from '../utils/datetime';

interface TenantFormValues {
  code: string;
  name: string;
  contactName: string;
  contactPhone: string;
  status: 'active' | 'pending';
  paymentProvider: 'mock' | 'tianque' | 'wechat_partner';
  paymentMerchantNo?: string;
  paymentSubAppId?: string;
  initialStoreCode: string;
  ownerUsername: string;
  ownerPassword: string;
  ownerDisplayName: string;
  ownerUsernameMode: OwnerUsernameMode;
  ownerAccountMode: 'CREATE' | 'EXISTING';
  serviceExpiresAt: Dayjs;
}

interface OwnerFormValues {
  username: string;
  displayName: string;
  password: string;
  usernameMode: OwnerUsernameMode;
  accountMode: 'CREATE' | 'EXISTING';
}

interface ProvisioningResult {
  tenant: Tenant;
  username: string;
  password?: string;
  storeName?: string;
  storeCode?: string;
}

const merchantPortalUrl = 'https://mysales.666qwe.cn';

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
  const [documentUploading, setDocumentUploading] = useState<string>();
  const [provisioningResult, setProvisioningResult] = useState<ProvisioningResult>();
  const [ownerOpen, setOwnerOpen] = useState(false);
  const [expirationOpen, setExpirationOpen] = useState(false);
  const [paymentOpen, setPaymentOpen] = useState(false);
  const [paymentLoading, setPaymentLoading] = useState(false);
  const [form] = Form.useForm<TenantFormValues>();
  const [ownerForm] = Form.useForm<OwnerFormValues>();
  const [expirationForm] = Form.useForm<{ expiresAt?: Dayjs }>();
  const [paymentForm] = Form.useForm<TenantPaymentSettings>();
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
      const tenant = await tenantService.create({ ...values, expiresAt: values.serviceExpiresAt?.format('YYYY-MM-DD') });
      setProvisioningResult({ tenant, username: values.ownerUsername, password: values.ownerAccountMode === 'CREATE' ? values.ownerPassword : undefined, storeName: values.name, storeCode: values.initialStoreCode });
      messageApi.success(values.ownerAccountMode === 'EXISTING' ? '新店已开通，并关联到已有老板账号' : '商户、店铺和老板账号已一并创建');
      setCreateOpen(false);
      form.resetFields();
      void load(1);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '商户创建失败');
    } finally {
      setSaving(false);
    }
  };

  const syncCreateUsername = (mode: OwnerUsernameMode) => {
    form.setFieldValue('ownerUsernameMode', mode);
    if (mode === 'CUSTOM') return;
    const values = form.getFieldsValue();
    form.setFieldValue('ownerUsername', generatedOwnerUsername(mode, values.name || '', values.contactPhone || '', values.code || 'shop'));
  };

  const openCreateTenant = () => {
    form.resetFields();
    form.setFieldsValue({ status: 'active', paymentProvider: 'mock', ownerAccountMode: 'CREATE', ownerUsernameMode: 'PHONE', ownerPassword: generateInitialPassword(), serviceExpiresAt: dayjs().add(1, 'year') });
    setCreateOpen(true);
  };

  const openOwnerAccount = () => {
    if (!selected) return;
    const mode: OwnerUsernameMode = selected.contactPhone ? 'PHONE' : 'PINYIN';
    ownerForm.resetFields();
    ownerForm.setFieldsValue({
      usernameMode: mode,
      username: generatedOwnerUsername(mode, selected.name, selected.contactPhone || '', selected.code || 'shop'),
      displayName: selected.contactName || '',
      password: generateInitialPassword(),
      accountMode: 'CREATE',
    });
    setOwnerOpen(true);
  };

  const syncOwnerUsername = (mode: OwnerUsernameMode) => {
    if (!selected) return;
    ownerForm.setFieldValue('usernameMode', mode);
    if (mode === 'CUSTOM') return;
    ownerForm.setFieldValue('username', generatedOwnerUsername(mode, selected.name, selected.contactPhone || '', selected.code || 'shop'));
  };

  const copy = async (value: string, label: string) => {
    await navigator.clipboard.writeText(value);
    messageApi.success(`${label}已复制`);
  };

  const createOwner = async () => {
    if (!selected) return;
    const values = await ownerForm.validateFields();
    setSaving(true);
    try {
      await tenantService.createOwner(selected.id, values);
      const updatedTenant = { ...selected, hasOwner: true, ownerUsername: values.username, ownerDisplayName: values.displayName, ownerStatus: 'active' as const };
      messageApi.success(values.accountMode === 'EXISTING' ? '已有老板账号已关联' : '老板账号已创建，请立即保存初始凭据');
      setOwnerOpen(false);
      ownerForm.resetFields();
      setSelected(updatedTenant);
      setProvisioningResult({ tenant: updatedTenant, username: values.username, password: values.accountMode === 'CREATE' ? values.password : undefined });
      void load();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '老板账号创建失败');
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

  const renewOneYear = async (record: Tenant) => {
    setSaving(true);
    try {
      const updated = await tenantService.renewOneYear(record.id);
      setSelected((current) => current?.id === updated.id ? updated : current);
      messageApi.success(`已续期至 ${formatBeijingDate(updated.expiresAt)}`);
      await load();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '续期失败');
    } finally {
      setSaving(false);
    }
  };

  const openExpiration = (record: Tenant) => {
    setSelected(record);
    expirationForm.setFieldsValue({ expiresAt: record.expiresAt ? dayjs(record.expiresAt) : undefined });
    setExpirationOpen(true);
  };

  const saveExpiration = async () => {
    if (!selected) return;
    const values = await expirationForm.validateFields();
    setSaving(true);
    try {
      const updated = await tenantService.updateServiceExpiration(selected.id, values.expiresAt?.format('YYYY-MM-DD'));
      setSelected(updated);
      setExpirationOpen(false);
      messageApi.success(values.expiresAt ? '商户有效期已更新' : '已设为长期有效');
      await load();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '有效期更新失败');
    } finally {
      setSaving(false);
    }
  };

  const openPaymentSettings = async (record: Tenant) => {
    setSelected(record);
    setPaymentOpen(true);
    setPaymentLoading(true);
    paymentForm.resetFields();
    try {
      paymentForm.setFieldsValue(await tenantService.getPaymentSettings(record.id));
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '支付配置加载失败');
      setPaymentOpen(false);
    } finally {
      setPaymentLoading(false);
    }
  };

  const savePaymentSettings = async () => {
    if (!selected) return;
    const values = await paymentForm.validateFields();
    setSaving(true);
    try {
      const updated = await tenantService.updatePaymentSettings(selected.id, values);
      setSelected({ ...selected, paymentProvider: updated.provider, paymentMerchantNo: updated.merchantNo, paymentSubAppId: updated.subAppId, paymentStatus: updated.onboardingStatus === 'ACTIVE' && updated.productAuthorizationStatus === 'AUTHORIZED' ? 'active' : updated.onboardingStatus === 'REJECTED' ? 'rejected' : updated.onboardingStatus === 'NOT_APPLIED' ? 'unbound' : 'pending' });
      setPaymentOpen(false);
      messageApi.success('商户支付配置已保存');
      await load();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '支付配置保存失败');
    } finally {
      setSaving(false);
    }
  };

  const uploadDocument = async (type: 'business-license' | 'food-business-license', file: File) => {
    if (!selected) return;
    if (!['image/jpeg', 'image/png', 'image/gif'].includes(file.type)) {
      const error = new Error('仅支持 JPG、PNG 或 GIF 图片');
      messageApi.error(error.message);
      throw error;
    }
    if (file.size > 8 * 1024 * 1024) {
      const error = new Error('证照图片不能超过 8 MiB');
      messageApi.error(error.message);
      throw error;
    }
    setDocumentUploading(type);
    try {
      const updated = await tenantService.uploadDocument(selected.id, type, file);
      setSelected(updated);
      setRows((current) => current.map((item) => item.id === updated.id ? updated : item));
      messageApi.success(type === 'business-license' ? '营业执照已保存' : '食品经营许可证已保存');
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '证照上传失败');
      throw error;
    } finally {
      setDocumentUploading(undefined);
    }
  };

  const documentRequest = (type: 'business-license' | 'food-business-license'): UploadProps['customRequest'] => ({ file, onSuccess, onError }) => {
    if (!(file instanceof File)) {
      onError?.(new Error('无效文件'));
      return;
    }
    void uploadDocument(type, file).then(() => onSuccess?.({})).catch((error: unknown) => onError?.(error instanceof Error ? error : new Error('证照上传失败')));
  };

  const columns: ColumnsType<Tenant> = [
    {
      title: '商户名称', dataIndex: 'name', key: 'name', fixed: 'left', width: 230,
      render: (value, row) => <div className="entity-name entity-name--merchant"><span><ShopOutlined /></span><div><strong>{value}</strong><small>{row.code || '—'}</small></div></div>,
    },
    { title: '联系人', key: 'contact', width: 170, render: (_, row) => <div>{row.contactName || '—'}<small className="table-subtext">{row.contactPhone || ''}</small></div> },
    { title: '点单码', dataIndex: 'storeCode', key: 'storeCode', width: 150, render: (value) => value || '—' },
    { title: '老板账号', key: 'owner', width: 150, render: (_, row) => row.hasOwner ? <div>{row.ownerUsername}<small className="table-subtext">{row.ownerDisplayName || '老板'}</small></div> : <Tag color="warning">待创建</Tag> },
    { title: '累计订单', dataIndex: 'orderCount', key: 'orderCount', width: 120, align: 'right', render: (value) => Number(value || 0).toLocaleString('zh-CN') },
    { title: '支付接入', dataIndex: 'paymentStatus', key: 'paymentStatus', width: 110, render: (value = 'unbound') => <Tag color={paymentStatusColor[value] || 'default'}>{paymentStatusText[value] || value}</Tag> },
    { title: '经营证照', key: 'documents', width: 110, render: (_, row) => <Tag color={row.businessLicenseUrl && row.foodBusinessLicenseUrl ? 'success' : 'warning'}>{Number(Boolean(row.businessLicenseUrl)) + Number(Boolean(row.foodBusinessLicenseUrl))}/2</Tag> },
    { title: '服务有效期', key: 'expiration', width: 140, render: (_, row) => row.expiresAt ? <div>{formatBeijingDate(row.expiresAt)}<small className="table-subtext">{row.serviceExpired ? <Tag color="error">已到期</Tag> : '有效'}</small></div> : <Tag color="success">长期有效</Tag> },
    { title: '状态', dataIndex: 'status', key: 'status', width: 100, render: (value) => <StatusTag status={value} /> },
    { title: '入驻时间', dataIndex: 'createdAt', key: 'createdAt', width: 180, render: formatBeijingDateTime },
    {
      title: '操作', key: 'actions', fixed: 'right', width: 300,
      render: (_, record) => <Space>
        <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => setSelected(record)}>查看</Button>
        <Button type="link" size="small" icon={<CalendarOutlined />} onClick={() => openExpiration(record)}>有效期</Button>
        <Popconfirm title="确认续期 1 年？" description="从当前到期日或今天（取较晚者）起顺延一年。" onConfirm={() => void renewOneYear(record)}>
          <Button type="link" size="small" loading={saving}>续期1年</Button>
        </Popconfirm>
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
      <PageHeader title="商户管理" description="一个商户租户对应一家独立店铺；同一老板账号可以关联多个店铺。" extra={<Button type="primary" icon={<PlusOutlined />} onClick={openCreateTenant}>新增商户 / 开通店铺</Button>} />
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
          scroll={{ x: 1720 }}
          pagination={{ current: meta.page, pageSize: meta.pageSize, total: meta.total, showSizeChanger: true, showTotal: (total) => `共 ${total} 家商户`, onChange: (page, pageSize) => void load(page, pageSize) }}
        />
      </Card>

      <Modal title="新增商户并开通独立店铺" open={createOpen} width={760} okText="创建并开通" onCancel={() => setCreateOpen(false)} onOk={() => void createTenant()} confirmLoading={saving}>
        <Form form={form} layout="vertical" requiredMark={false} className="modal-form">
          <Row gutter={12}>
            <Col span={15}><Form.Item label="店铺 / 商户名称" name="name" rules={[{ required: true, message: '请输入店铺名称' }]}><Input placeholder="例如：码农咖啡鼓楼店" onChange={(event) => { if (form.getFieldValue('ownerUsernameMode') === 'PINYIN') form.setFieldValue('ownerUsername', generatedOwnerUsername('PINYIN', event.target.value, '', form.getFieldValue('code') || 'shop')); }} /></Form.Item></Col>
            <Col span={9}><Form.Item label="商户编号" name="code" rules={[{ required: true, message: '请输入商户编号' }, { pattern: /^[A-Za-z0-9_-]+$/, message: '仅支持字母、数字、下划线和短横线' }]}><Input placeholder="例如：MNKF001" /></Form.Item></Col>
          </Row>
          <Form.Item label="商户有效期" name="serviceExpiresAt" rules={[{ required: true, message: '请选择商户有效期' }]} extra="到期日当天仍可正常使用，次日自动进入服务暂停阶段。">
            <DatePicker format="YYYY-MM-DD" style={{ width: '100%' }} />
          </Form.Item>
          <Row gutter={12}>
            <Col span={12}><Form.Item label="联系人" name="contactName" rules={[{ required: true, message: '请输入联系人' }]}><Input onChange={(event) => form.setFieldValue('ownerDisplayName', event.target.value)} /></Form.Item></Col>
            <Col span={12}><Form.Item label="联系电话" name="contactPhone" rules={[{ required: true, message: '请输入联系电话' }, { pattern: /^1\d{10}$/, message: '请输入有效手机号' }]}><Input onChange={(event) => { if (form.getFieldValue('ownerUsernameMode') === 'PHONE') form.setFieldValue('ownerUsername', event.target.value.replace(/\D/g, '')); }} /></Form.Item></Col>
          </Row>
          <Form.Item label="店铺点单码" name="initialStoreCode" rules={[{ required: true, message: '请输入点单码' }, { pattern: /^[A-Za-z0-9_-]+$/, message: '仅支持字母、数字、下划线和短横线' }]}><Input placeholder="例如：manong-coffee-gulou" /></Form.Item>
          <Form.Item label="老板账号" name="ownerAccountMode" rules={[{ required: true }]}>
            <Radio.Group optionType="button" buttonStyle="solid" options={[{ value: 'CREATE', label: '创建新老板账号' }, { value: 'EXISTING', label: '关联已有老板账号' }]} />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(previous, current) => previous.ownerAccountMode !== current.ownerAccountMode}>
            {({ getFieldValue }) => getFieldValue('ownerAccountMode') === 'EXISTING' ? (
              <><Alert type="info" showIcon style={{ marginBottom: 16 }} message="已有老板登录后可在多个独立店铺之间切换；员工仍只进入所属店铺。" /><Form.Item label="已有老板登录账号" name="ownerUsername" rules={[{ required: true, message: '请输入已有老板账号' }]}><Input prefix={<UserOutlined />} autoComplete="off" /></Form.Item></>
            ) : (<>
              <Form.Item label="老板账号生成方式" name="ownerUsernameMode" rules={[{ required: true }]}><Radio.Group optionType="button" buttonStyle="solid" onChange={(event) => syncCreateUsername(event.target.value)} options={[{ value: 'PHONE', label: '联系人手机号' }, { value: 'PINYIN', label: '店铺名称拼音' }, { value: 'CUSTOM', label: '自定义' }]} /></Form.Item>
              <Row gutter={12}>
                <Col span={8}><Form.Item label="老板登录账号" name="ownerUsername" rules={[{ required: true, message: '请输入老板账号' }, { pattern: /^[A-Za-z0-9_.@-]+$/, message: '仅支持字母、数字及 . _ @ -' }]}><Input autoComplete="off" onChange={() => { if (form.getFieldValue('ownerUsernameMode') !== 'CUSTOM') form.setFieldValue('ownerUsernameMode', 'CUSTOM'); }} /></Form.Item></Col>
                <Col span={8}><Form.Item label="老板姓名" name="ownerDisplayName" rules={[{ required: true, message: '请输入老板姓名' }]}><Input /></Form.Item></Col>
                <Col span={8}><Form.Item label="初始密码" name="ownerPassword" rules={[{ required: true, message: '请输入初始密码' }, { min: 8, max: 72, message: '须为 8 至 72 位' }]}><Input.Password autoComplete="new-password" addonAfter={<Button type="text" size="small" icon={<ReloadOutlined />} onClick={() => form.setFieldValue('ownerPassword', generateInitialPassword())}>随机</Button>} /></Form.Item></Col>
              </Row>
            </>)}
          </Form.Item>
          <Row gutter={12}>
            <Col span={8}><Form.Item label="初始状态" name="status" rules={[{ required: true }]}><Select options={[{ value: 'active', label: '正常运营' }, { value: 'pending', label: '待完善资料' }]} /></Form.Item></Col>
            <Col span={8}><Form.Item label="支付适配器" name="paymentProvider" rules={[{ required: true }]}><Select options={[{ value: 'mock', label: '虚拟支付（联调）' }, { value: 'tianque', label: '会生活/天阙' }, { value: 'wechat_partner', label: '微信支付（普通服务商）' }]} /></Form.Item></Col>
            <Col span={8}><Form.Item label="支付商户号" name="paymentMerchantNo"><Input placeholder="可创建后在支付配置中维护" /></Form.Item></Col>
          </Row>
          <Form.Item label="独立子 AppID（可选）" name="paymentSubAppId" extra="微信普通服务商共用摊伴小程序时留空；只有商户使用自己的小程序时才填写 sub_appid。"><Input placeholder="wx..." /></Form.Item>
        </Form>
      </Modal>

      <Drawer title="商户详情" width={520} open={Boolean(selected)} onClose={() => setSelected(undefined)} extra={selected && <StatusTag status={selected.status} />}>
        {selected && <>
          <div className="drawer-entity"><span><ShopOutlined /></span><div><h2>{selected.name}</h2><p>{selected.code || '暂无商户编号'}</p></div></div>
          <Descriptions column={1} bordered size="small" className="detail-descriptions">
            <Descriptions.Item label="商户编号">{selected.code || '—'}</Descriptions.Item>
            <Descriptions.Item label="联系人">{selected.contactName || '—'} {selected.contactPhone || ''}</Descriptions.Item>
            <Descriptions.Item label="门店 ID">{selected.storeId || '—'}</Descriptions.Item>
            <Descriptions.Item label="点单码">{selected.storeCode || '—'}</Descriptions.Item>
            <Descriptions.Item label="老板账号">{selected.hasOwner ? <Space><UserOutlined />{selected.ownerUsername}<Button type="link" size="small" icon={<CopyOutlined />} onClick={() => void copy(selected.ownerUsername || '', '账号')}>复制</Button></Space> : <Button type="primary" size="small" icon={<UserOutlined />} onClick={openOwnerAccount}>创建首个老板账号</Button>}</Descriptions.Item>
            {selected.hasOwner && <Descriptions.Item label="账号姓名 / 状态">{selected.ownerDisplayName || '—'} · <StatusTag status={selected.ownerStatus || 'active'} /></Descriptions.Item>}
            <Descriptions.Item label="累计订单">{selected.orderCount || 0} 单</Descriptions.Item>
            <Descriptions.Item label="支付状态"><Tag color={paymentStatusColor[selected.paymentStatus || 'unbound']}>{paymentStatusText[selected.paymentStatus || 'unbound']}</Tag></Descriptions.Item>
            <Descriptions.Item label="支付商户号">{selected.paymentMerchantNo ? `${selected.paymentMerchantNo.slice(0, 4)}****${selected.paymentMerchantNo.slice(-4)}` : '未绑定'}</Descriptions.Item>
            <Descriptions.Item label="入驻时间">{formatBeijingDateTime(selected.createdAt)}</Descriptions.Item>
            <Descriptions.Item label="服务到期">{selected.expiresAt ? formatBeijingDate(selected.expiresAt) : '未设置'}</Descriptions.Item>
            <Descriptions.Item label="服务状态">{selected.serviceExpired ? <Tag color="error">欠费暂停</Tag> : <Tag color="success">正常</Tag>}</Descriptions.Item>
          </Descriptions>
          <Space style={{ marginTop: 16, marginBottom: 8 }}>
            <Button icon={<CalendarOutlined />} onClick={() => openExpiration(selected)}>设置有效期</Button>
            <Button icon={<BankOutlined />} onClick={() => void openPaymentSettings(selected)}>支付配置</Button>
            <Popconfirm title="确认续期 1 年？" onConfirm={() => void renewOneYear(selected)}><Button type="primary">续期 1 年</Button></Popconfirm>
          </Space>
          <Typography.Title level={5} className="tenant-document-title"><FileImageOutlined /> 商户经营证照</Typography.Title>
          <Typography.Paragraph type="secondary">仅平台管理员可上传或更换；商户后台只能查看，不可删除或修改。</Typography.Paragraph>
          <Row gutter={[12, 12]}>
            {([
              { type: 'business-license' as const, title: '营业执照', url: selected.businessLicenseUrl },
              { type: 'food-business-license' as const, title: '食品经营许可证', url: selected.foodBusinessLicenseUrl },
            ]).map((document) => (
              <Col span={12} key={document.type}>
                <Card size="small" className="tenant-document-card" title={document.title}>
                  <div className="tenant-document-preview">
                    {document.url ? <Image src={document.url} alt={document.title} /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="尚未上传" />}
                  </div>
                  <Upload
                    accept="image/jpeg,image/png,image/gif"
                    showUploadList={false}
                    customRequest={documentRequest(document.type)}
                  >
                    <Button block icon={<UploadOutlined />} loading={documentUploading === document.type}>{document.url ? '更换图片' : '上传图片'}</Button>
                  </Upload>
                </Card>
              </Col>
            ))}
          </Row>
        </>}
      </Drawer>

      <Modal title={`支付配置 · ${selected?.name || ''}`} open={paymentOpen} width={680} okText="保存支付配置" onCancel={() => setPaymentOpen(false)} onOk={() => void savePaymentSettings()} confirmLoading={saving} okButtonProps={{ disabled: paymentLoading }}>
        <Form form={paymentForm} layout="vertical" requiredMark={false}>
          <Alert type="info" showIcon message="商户端无需填写支付密钥" description="服务商证书和 APIv3 密钥由平台统一保管。商户这里只绑定微信支付特约商户号，并记录进件、产品授权和退款授权状态。" style={{ marginBottom: 16 }} />
          {paymentForm.getFieldValue('onboardingApplication') && <Alert
            type="info"
            showIcon
            message={`商户已提交微信支付预审：${paymentForm.getFieldValue(['onboardingApplication', 'merchantShortName']) || '未命名商户'}`}
            description={
              <Space direction="vertical" size={2}>
                <span>主体：{{ MICRO: '小微商户', INDIVIDUAL: '个体工商户', ENTERPRISE: '企业' }[paymentForm.getFieldValue(['onboardingApplication', 'subjectType']) as string] || '—'}；经营者：{paymentForm.getFieldValue(['onboardingApplication', 'operatorName']) || '—'}</span>
                <span>申请状态：{paymentForm.getFieldValue(['onboardingApplication', 'applicationStatus']) || '—'}；提交时间：{paymentForm.getFieldValue(['onboardingApplication', 'submittedAt']) || '—'}</span>
              </Space>
            }
            style={{ marginBottom: 16 }}
          />}
          <Form.Item label="支付适配器" name="provider" rules={[{ required: true }]}><Select options={[{ value: 'mock', label: '虚拟支付（联调）' }, { value: 'tianque', label: '会生活/天阙' }, { value: 'wechat_partner', label: '微信支付（普通服务商）' }]} /></Form.Item>
          <Form.Item noStyle shouldUpdate={(previous, current) => previous.provider !== current.provider}>
            {({ getFieldValue }) => getFieldValue('provider') === 'wechat_partner' ? <>
              <Form.Item label="微信支付特约商户号（sub_mchid）" name="merchantNo" rules={[{ pattern: /^\d{8,32}$/, message: '请输入 8 至 32 位数字' }]}><Input placeholder="进件通过后由微信支付分配" /></Form.Item>
              <Form.Item label="商户独立小程序 AppID（sub_appid，可选）" name="subAppId" rules={[{ pattern: /^$|^wx[a-zA-Z0-9]{16}$/, message: 'AppID 格式不正确' }]} extra="商户共用摊伴小程序时留空；不要重复填写平台的 sp_appid。"><Input placeholder="共用摊伴小程序时留空" /></Form.Item>
              <Row gutter={12}>
                <Col xs={24} md={12}><Form.Item label="特约商户进件状态" name="onboardingStatus" rules={[{ required: true }]}><Select options={[{ value: 'NOT_APPLIED', label: '未进件' }, { value: 'REVIEWING', label: '审核中' }, { value: 'PENDING_SIGNING', label: '待商户签约' }, { value: 'ACTIVE', label: '已开通' }, { value: 'REJECTED', label: '已驳回' }]} /></Form.Item></Col>
                <Col xs={24} md={12}><Form.Item label="小程序支付产品授权" name="productAuthorizationStatus" rules={[{ required: true }]}><Select options={[{ value: 'NOT_AUTHORIZED', label: '未授权' }, { value: 'PENDING', label: '授权处理中' }, { value: 'AUTHORIZED', label: '已授权' }, { value: 'REVOKED', label: '已撤销' }]} /></Form.Item></Col>
              </Row>
              <Form.Item label="服务商 API 退款授权" name="refundAuthorized" valuePropName="checked"><Switch checkedChildren="已授权" unCheckedChildren="未授权" /></Form.Item>
            </> : <>
              <Form.Item label="支付商户号" name="merchantNo"><Input /></Form.Item>
              <Form.Item label="子 AppID" name="subAppId"><Input /></Form.Item>
            </>}
          </Form.Item>
        </Form>
      </Modal>

      <Modal title={`设置有效期 · ${selected?.name || ''}`} open={expirationOpen} okText="保存" onCancel={() => setExpirationOpen(false)} onOk={() => void saveExpiration()} confirmLoading={saving}>
        <Alert type="info" showIcon message="到期日当天仍可正常使用，次日自动暂停服务；清空日期表示长期有效。" style={{ marginBottom: 16 }} />
        <Form form={expirationForm} layout="vertical">
          <Form.Item label="服务到期日" name="expiresAt"><DatePicker allowClear format="YYYY-MM-DD" style={{ width: '100%' }} /></Form.Item>
        </Form>
      </Modal>

      <Modal title="创建首个老板账号" open={ownerOpen} okText="创建账号" onCancel={() => setOwnerOpen(false)} onOk={() => void createOwner()} confirmLoading={saving}>
        <Form form={ownerForm} layout="vertical" requiredMark={false}>
          <Form.Item label="老板账号" name="accountMode" rules={[{ required: true }]}><Radio.Group optionType="button" buttonStyle="solid" options={[{ value: 'CREATE', label: '创建新账号' }, { value: 'EXISTING', label: '关联已有账号' }]} /></Form.Item>
          <Form.Item noStyle shouldUpdate={(previous, current) => previous.accountMode !== current.accountMode}>
            {({ getFieldValue }) => getFieldValue('accountMode') === 'EXISTING' ? (
              <><Alert type="info" showIcon style={{ marginBottom: 16 }} message="请输入已有老板的登录账号，关联后可使用同一账号管理本店。" /><Form.Item label="已有老板登录账号" name="username" rules={[{ required: true, message: '请输入登录账号' }]}><Input prefix={<UserOutlined />} autoComplete="off" /></Form.Item></>
            ) : (<>
              <Form.Item label="账号生成方式" name="usernameMode" rules={[{ required: true }]}><Radio.Group optionType="button" buttonStyle="solid" onChange={(event) => syncOwnerUsername(event.target.value)} options={[{ value: 'PHONE', label: '联系人手机号' }, { value: 'PINYIN', label: '商户名称拼音' }, { value: 'CUSTOM', label: '自定义' }]} /></Form.Item>
              <Form.Item label="登录账号" name="username" rules={[{ required: true, message: '请输入登录账号' }]}><Input prefix={<UserOutlined />} autoComplete="off" /></Form.Item>
              <Form.Item label="老板姓名" name="displayName" rules={[{ required: true, message: '请输入老板姓名' }]}><Input /></Form.Item>
              <Form.Item label="初始密码" name="password" rules={[{ required: true, message: '请输入初始密码' }, { min: 8, max: 72, message: '须为 8 至 72 位' }]}><Input.Password prefix={<KeyOutlined />} autoComplete="new-password" addonAfter={<Button type="text" size="small" icon={<ReloadOutlined />} onClick={() => ownerForm.setFieldValue('password', generateInitialPassword())}>随机</Button>} /></Form.Item>
            </>)}
          </Form.Item>
        </Form>
      </Modal>

      <Modal title="商户开通完成" open={Boolean(provisioningResult)} footer={<Button type="primary" onClick={() => setProvisioningResult(undefined)}>我已保存</Button>} closable={false} maskClosable={false}>
        {provisioningResult && <>
          <div className="drawer-entity"><span><ShopOutlined /></span><div><h2>{provisioningResult.tenant.name}</h2><p>{provisioningResult.storeName ? '首门店、老板账号已创建，可直接登录运营后台' : '老板账号已创建，可直接登录运营后台'}</p></div></div>
          <Descriptions column={1} bordered size="small" className="detail-descriptions">
            <Descriptions.Item label="运营后台"><Space>{merchantPortalUrl}<Button type="link" size="small" icon={<CopyOutlined />} onClick={() => void copy(merchantPortalUrl, '登录地址')}>复制</Button></Space></Descriptions.Item>
            <Descriptions.Item label="登录账号"><Space>{provisioningResult.username}<Button type="link" size="small" icon={<CopyOutlined />} onClick={() => void copy(provisioningResult.username, '账号')}>复制</Button></Space></Descriptions.Item>
            {provisioningResult.password && <Descriptions.Item label="初始密码"><Space><Typography.Text code>{provisioningResult.password}</Typography.Text><Button type="link" size="small" icon={<CopyOutlined />} onClick={() => void copy(provisioningResult.password || '', '初始密码')}>复制</Button></Space></Descriptions.Item>}
            {provisioningResult.storeName && <Descriptions.Item label="首门店">{provisioningResult.storeName}{provisioningResult.storeCode ? `（${provisioningResult.storeCode}）` : ''}</Descriptions.Item>}
          </Descriptions>
          {provisioningResult.password ? <Alert style={{ marginTop: 16 }} type="warning" showIcon message="初始密码仅在此处展示一次" description="请现在复制并安全交付给商户；系统只保存密码哈希，关闭后无法查看原密码。" /> : <Alert style={{ marginTop: 16 }} type="success" showIcon message="已关联已有老板账号" description="老板使用原账号和原密码登录，进入后可选择要管理的店铺。" />}
        </>}
      </Modal>
    </div>
  );
}
