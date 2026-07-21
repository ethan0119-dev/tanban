import { CopyOutlined, PlusOutlined, QrcodeOutlined, ReloadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Form, Input, InputNumber, Modal, QRCode, Space, Switch, Table, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { api, errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import { DeveloperOnlyNote } from '../components/DeveloperOnlyNote';
import { merchantFeatureCopy } from '../features/availability/copy';
import type { Id } from '../types';

const ENDPOINT = '/merchant/fast-food-plates';

interface FastFoodPlate {
  id: Id;
  plateName: string;
  plateCode: string;
  publicId: string;
  qrScene: string;
  miniappPath: string;
  remark?: string;
  sortOrder: number;
  status: 'ACTIVE' | 'DISABLED';
}

interface FastFoodPlateForm {
  plateName: string;
  plateCode: string;
  remark?: string;
  sortOrder: number;
  enabled: boolean;
}

function payload(values: FastFoodPlateForm) {
  return {
    plateName: values.plateName.trim(),
    plateCode: values.plateCode.trim(),
    remark: values.remark?.trim() || '',
    sortOrder: Number(values.sortOrder || 0),
    status: values.enabled ? 'ACTIVE' : 'DISABLED',
  };
}

export function FastFoodPlatesPage() {
  const [items, setItems] = useState<FastFoodPlate[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState<FastFoodPlate>();
  const [previewing, setPreviewing] = useState<FastFoodPlate>();
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm<FastFoodPlateForm>();
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const result = await api.getList<FastFoodPlate>(ENDPOINT, { page_size: 500 });
      setItems(result.items);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [messageApi]);

  useEffect(() => { void load(); }, [load]);

  const openForm = (item?: FastFoodPlate) => {
    setEditing(item);
    form.setFieldsValue(item ? {
      plateName: item.plateName,
      plateCode: item.plateCode,
      remark: item.remark,
      sortOrder: item.sortOrder,
      enabled: item.status === 'ACTIVE',
    } : { plateName: '', plateCode: '', remark: '', sortOrder: items.length, enabled: true });
    setModalOpen(true);
  };

  const save = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      if (editing) await api.put(`${ENDPOINT}/${editing.id}`, payload(values));
      else await api.post(ENDPOINT, payload(values));
      messageApi.success(editing ? '码牌已更新' : '码牌已创建');
      setModalOpen(false);
      setEditing(undefined);
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const toggle = async (item: FastFoodPlate, enabled: boolean) => {
    try {
      await api.put(`${ENDPOINT}/${item.id}`, payload({ plateName: item.plateName, plateCode: item.plateCode, remark: item.remark, sortOrder: item.sortOrder, enabled }));
      setItems((current) => current.map((row) => row.id === item.id ? { ...row, status: enabled ? 'ACTIVE' : 'DISABLED' } : row));
      messageApi.success(enabled ? '码牌已启用' : '码牌已停用；历史订单快照不受影响');
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const remove = (item: FastFoodPlate) => Modal.confirm({
    title: `删除码牌 ${item.plateCode}？`,
    content: '删除后二维码不能再创建新订单，历史订单仍保留码牌快照。',
    okText: '删除',
    okButtonProps: { danger: true },
    cancelText: '取消',
    onOk: async () => {
      await api.delete(`${ENDPOINT}/${item.id}`);
      messageApi.success('码牌已删除');
      await load();
    },
  });

  const activeCount = useMemo(() => items.filter((item) => item.status === 'ACTIVE').length, [items]);

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading
        title="快餐码牌"
        description="维护到店自取场景的桌面码牌；扫码下单后订单固化码牌，并按营业日生成稳定取餐号"
        extra={<Space><Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>刷新</Button><Button type="primary" icon={<PlusOutlined />} onClick={() => openForm()}>新增码牌</Button></Space>}
      />
      <Alert className="table-code-flow-alert" type="info" showIcon message={`已启用 ${activeCount} 个码牌`} description="顾客扫码后会自动绑定当前取餐码牌，商家可按码牌和取餐号安排放餐。" />
      <DeveloperOnlyNote style={{ marginBottom: 16 }}>码牌使用随机公共标识生成扫码参数；下单时还会校验码牌所属门店和启用状态。</DeveloperOnlyNote>
      <Card bordered={false} className="content-card table-card">
        <Table<FastFoodPlate>
          rowKey="id"
          loading={loading}
          dataSource={items}
          pagination={false}
          columns={[
            { title: '排序', dataIndex: 'sortOrder', width: 76 },
            { title: '码牌', key: 'plate', render: (_, item) => <Space direction="vertical" size={0}><Typography.Text strong>{item.plateName}</Typography.Text><Typography.Text type="secondary">编号 {item.plateCode}</Typography.Text></Space> },
            { title: '码牌识别码', dataIndex: 'publicId', render: (value: string) => <Typography.Text code copyable>{value}</Typography.Text> },
            { title: '状态', dataIndex: 'status', width: 100, render: (value: string) => value === 'ACTIVE' ? <Tag color="success">启用</Tag> : <Tag>停用</Tag> },
            { title: '启用', key: 'enabled', width: 84, render: (_, item) => <Switch checked={item.status === 'ACTIVE'} onChange={(checked) => void toggle(item, checked)} /> },
            { title: '操作', key: 'actions', width: 220, render: (_, item) => <Space><Button type="link" icon={<QrcodeOutlined />} onClick={() => setPreviewing(item)}>二维码</Button><Button type="link" onClick={() => openForm(item)}>编辑</Button><Button type="link" danger onClick={() => remove(item)}>删除</Button></Space> },
          ]}
        />
      </Card>

      <Modal title={editing ? '编辑快餐码牌' : '新增快餐码牌'} open={modalOpen} confirmLoading={saving} onOk={() => void save()} onCancel={() => setModalOpen(false)} okText="保存">
        <Form form={form} layout="vertical">
          <Form.Item label="展示名称" name="plateName" rules={[{ required: true }, { max: 80 }]}><Input placeholder="例如：咖啡摊 01 号牌" /></Form.Item>
          <Form.Item label="码牌编号" name="plateCode" rules={[{ required: true }, { max: 64 }, { pattern: /^\S+$/, message: '编号不能包含空格' }]}><Input placeholder="例如：P01" /></Form.Item>
          <Form.Item label="备注" name="remark"><Input.TextArea maxLength={255} showCount rows={2} /></Form.Item>
          <Space size="large">
            <Form.Item label="排序" name="sortOrder"><InputNumber min={0} precision={0} /></Form.Item>
            <Form.Item label="启用" name="enabled" valuePropName="checked"><Switch /></Form.Item>
          </Space>
        </Form>
      </Modal>

      <Modal title={previewing ? `${previewing.plateName} · ${previewing.plateCode}` : '码牌二维码'} open={Boolean(previewing)} footer={null} onCancel={() => setPreviewing(undefined)}>
        {previewing && <Space direction="vertical" align="center" style={{ width: '100%' }} size="large">
          {import.meta.env.DEV ? <QRCode value={`${previewing.miniappPath}?scene=${encodeURIComponent(previewing.qrScene)}`} size={240} /> : <div className="official-code-placeholder"><QrcodeOutlined /><span>{merchantFeatureCopy.OFFICIAL_MINIAPP_CODE.title}</span></div>}
          <Typography.Text>码牌识别码：<Typography.Text code>{previewing.publicId}</Typography.Text></Typography.Text>
          <Button icon={<CopyOutlined />} onClick={() => void navigator.clipboard.writeText(previewing.publicId).then(() => messageApi.success('码牌识别码已复制'))}>复制识别码</Button>
          {!import.meta.env.DEV && <Alert type="info" showIcon message={merchantFeatureCopy.OFFICIAL_MINIAPP_CODE.title} description={merchantFeatureCopy.OFFICIAL_MINIAPP_CODE.description} />}
          <DeveloperOnlyNote>开发环境扫码参数：{previewing.qrScene}</DeveloperOnlyNote>
        </Space>}
      </Modal>
    </div>
  );
}
