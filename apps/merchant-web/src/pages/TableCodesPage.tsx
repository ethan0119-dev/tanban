import {
  CopyOutlined,
  DownloadOutlined,
  EyeOutlined,
  FileImageOutlined,
  PlusOutlined,
  QrcodeOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  QRCode,
  Row,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { api, errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import {
  TABLE_AREAS_ENDPOINT,
  TABLE_CODES_ENDPOINT,
  normalizeTableArea,
  normalizeTableCode,
} from '../features/storefront/model';
import type { Id, TableArea, TableCode } from '../types';

interface TableCodeFormValues {
  areaId: Id;
  name: string;
  tableCode: string;
  capacity: number;
  remark: string;
  sortOrder: number;
  enabled: boolean;
}

interface TableAreaFormValues {
  name: string;
  sortOrder: number;
  enabled: boolean;
}

function tablePayload(values: TableCodeFormValues) {
  return {
    areaId: values.areaId,
    name: values.name.trim(),
    tableCode: values.tableCode.trim(),
    capacity: Number(values.capacity || 0),
    remark: values.remark?.trim() || '',
    sortOrder: Number(values.sortOrder || 0),
    status: values.enabled ? 'ACTIVE' : 'DISABLED',
  };
}

export function TableCodesPage() {
  const [areas, setAreas] = useState<TableArea[]>([]);
  const [items, setItems] = useState<TableCode[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [areaFilter, setAreaFilter] = useState<Id>();
  const [keyword, setKeyword] = useState('');
  const [editing, setEditing] = useState<TableCode>();
  const [editingArea, setEditingArea] = useState<TableArea>();
  const [previewing, setPreviewing] = useState<TableCode>();
  const [formOpen, setFormOpen] = useState(false);
  const [areaFormOpen, setAreaFormOpen] = useState(false);
  const [form] = Form.useForm<TableCodeFormValues>();
  const [areaForm] = Form.useForm<TableAreaFormValues>();
  const qrContainerRef = useRef<HTMLDivElement>(null);
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [areaResult, tableResult] = await Promise.all([
        api.getList<TableArea>(TABLE_AREAS_ENDPOINT, { page_size: 500 }),
        api.getList<TableCode>(TABLE_CODES_ENDPOINT, { page_size: 500 }),
      ]);
      setAreas(areaResult.items.map(normalizeTableArea));
      setItems(tableResult.items.map(normalizeTableCode));
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [messageApi]);

  useEffect(() => { void load(); }, [load]);

  const filtered = useMemo(() => items.filter((item) => {
    const matchesArea = areaFilter === undefined || String(item.areaId) === String(areaFilter);
    const term = keyword.trim().toLowerCase();
    const matchesKeyword = !term || [item.tableNo, item.tableName, item.scene].some((value) => value.toLowerCase().includes(term));
    return matchesArea && matchesKeyword;
  }), [areaFilter, items, keyword]);

  const openForm = (item?: TableCode) => {
    setEditing(item);
    const firstArea = areas.find((area) => area.status === 'ACTIVE');
    form.setFieldsValue(item ? {
      areaId: item.areaId,
      name: item.tableName,
      tableCode: item.tableNo,
      capacity: item.seats,
      remark: item.remark ?? '',
      sortOrder: item.sortOrder ?? 0,
      enabled: item.status === 'ACTIVE',
    } : { areaId: firstArea?.id, name: '', tableCode: '', capacity: 2, remark: '', sortOrder: items.length, enabled: true });
    setFormOpen(true);
  };

  const save = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      if (editing) await api.put(`${TABLE_CODES_ENDPOINT}/${editing.id}`, tablePayload(values));
      else await api.post(TABLE_CODES_ENDPOINT, tablePayload(values));
      messageApi.success(editing ? '桌位资料已更新' : '桌位及扫码参数已创建');
      setFormOpen(false);
      setEditing(undefined);
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const openAreaForm = (area?: TableArea) => {
    setEditingArea(area);
    areaForm.setFieldsValue(area
      ? { name: area.name, sortOrder: area.sortOrder, enabled: area.status === 'ACTIVE' }
      : { name: '', sortOrder: areas.length, enabled: true });
    setAreaFormOpen(true);
  };

  const saveArea = async () => {
    const values = await areaForm.validateFields();
    setSaving(true);
    try {
      const payload = {
        name: values.name.trim(),
        sortOrder: Number(values.sortOrder || 0),
        status: values.enabled ? 'ACTIVE' : 'DISABLED',
      };
      const saved = normalizeTableArea(editingArea
        ? await api.put<TableArea>(`${TABLE_AREAS_ENDPOINT}/${editingArea.id}`, payload)
        : await api.post<TableArea>(TABLE_AREAS_ENDPOINT, payload));
      await load();
      if (!editingArea && saved.status === 'ACTIVE') form.setFieldValue('areaId', saved.id);
      areaForm.resetFields();
      setAreaFormOpen(false);
      setEditingArea(undefined);
      messageApi.success(editingArea ? '桌台区域已更新' : '桌台区域已创建');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const toggleAreaStatus = async (area: TableArea, enabled: boolean) => {
    try {
      await api.put(`${TABLE_AREAS_ENDPOINT}/${area.id}`, {
        name: area.name,
        sortOrder: area.sortOrder,
        status: enabled ? 'ACTIVE' : 'DISABLED',
      });
      setAreas((current) => current.map((item) => item.id === area.id ? { ...item, status: enabled ? 'ACTIVE' : 'DISABLED' } : item));
      messageApi.success(enabled ? '区域已启用' : '区域已停用；该区域桌码将不能创建新订单');
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const toggleStatus = async (item: TableCode, enabled: boolean) => {
    try {
      await api.put(`${TABLE_CODES_ENDPOINT}/${item.id}`, {
        areaId: item.areaId,
        name: item.tableName,
        tableCode: item.tableNo,
        capacity: item.seats,
        remark: item.remark ?? '',
        sortOrder: item.sortOrder ?? 0,
        status: enabled ? 'ACTIVE' : 'DISABLED',
      });
      setItems((current) => current.map((row) => row.id === item.id ? { ...row, status: enabled ? 'ACTIVE' : 'DISABLED' } : row));
      messageApi.success(enabled ? '桌码已启用' : '桌码已停用，历史订单仍保留桌台快照');
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const copyScene = async (item: TableCode) => {
    try {
      await navigator.clipboard.writeText(item.scene);
      messageApi.success('scene 已复制');
    } catch {
      messageApi.warning(`请手工复制：${item.scene}`);
    }
  };

  const downloadDevelopmentCode = (item: TableCode) => {
    const canvas = qrContainerRef.current?.querySelector('canvas');
    if (!canvas) {
      messageApi.error('二维码尚未渲染完成');
      return;
    }
    const link = document.createElement('a');
    link.download = `${item.areaName}-${item.tableNo}-联调二维码.png`;
    link.href = canvas.toDataURL('image/png');
    link.click();
  };

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading
        title="桌码管理"
        description="按区域维护桌位，为每桌生成独立扫码参数并将桌台快照带入店内订单"
        extra={<Space><Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>刷新</Button><Button onClick={() => openAreaForm()}>新增区域</Button><Button type="primary" icon={<PlusOutlined />} disabled={!areas.some((area) => area.status === 'ACTIVE')} onClick={() => openForm()}>新增桌位</Button></Space>}
      />
      <Alert
        className="table-code-flow-alert"
        type="info"
        showIcon
        message="桌码下单链路"
        description="后端生成不可猜测的 qrScene 和带 scene 参数的小程序路径 → 顾客扫码进入点单页 → 小程序向服务端解析商户、门店、区域和桌位 → 创建订单时服务端再次校验并固化桌台快照。桌码只定位消费场景，不参与价格或支付判断。"
      />
      <Card bordered={false} className="content-card table-card" title="桌台区域" extra={<Button type="link" onClick={() => openAreaForm()}>新增区域</Button>}>
        <Table<TableArea>
          size="small"
          rowKey="id"
          loading={loading}
          dataSource={areas}
          pagination={false}
          locale={{ emptyText: '还没有区域，请先新增区域' }}
          columns={[
            { title: '区域名称', dataIndex: 'name' },
            { title: '桌位数', key: 'tableCount', width: 100, render: (_, area) => items.filter((item) => String(item.areaId) === String(area.id)).length },
            { title: '排序', dataIndex: 'sortOrder', width: 90 },
            { title: '启用', key: 'status', width: 90, render: (_, area) => <Switch checked={area.status === 'ACTIVE'} onChange={(checked) => void toggleAreaStatus(area, checked)} /> },
            { title: '操作', key: 'action', width: 90, render: (_, area) => <Button type="link" onClick={() => openAreaForm(area)}>编辑</Button> },
          ]}
        />
      </Card>
      <Card bordered={false} className="content-card table-card">
        <div className="table-toolbar">
          <Space wrap>
            <Select allowClear value={areaFilter} onChange={setAreaFilter} placeholder="全部区域" style={{ width: 160 }} options={areas.map((area) => ({ value: area.id, label: area.name, disabled: area.status !== 'ACTIVE' }))} />
            <Input.Search allowClear value={keyword} onChange={(event) => setKeyword(event.target.value)} placeholder="桌号、桌名或 scene" style={{ width: 260 }} />
          </Space>
          <Typography.Text type="secondary">{areas.length} 个区域 · {filtered.length} 个桌位</Typography.Text>
        </div>
        <Table<TableCode>
          rowKey="id"
          loading={loading}
          dataSource={filtered}
          scroll={{ x: 900 }}
          locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={areas.length ? '还没有桌位' : '请先创建桌台区域'}><Button type="primary" onClick={() => areas.length ? openForm() : setAreaFormOpen(true)}>{areas.length ? '创建第一张桌码' : '新增区域'}</Button></Empty> }}
          columns={[
            { title: '区域', dataIndex: 'areaName', width: 130 },
            { title: '桌号', dataIndex: 'tableNo', width: 105, render: (value) => <strong className="table-number">{value}</strong> },
            { title: '桌位名称', dataIndex: 'tableName', width: 160 },
            { title: '容量', dataIndex: 'seats', width: 80, render: (value) => value > 0 ? `${value} 人` : '--' },
            {
              title: '扫码参数', dataIndex: 'scene', minWidth: 240,
              render: (value, item) => value ? <Space size={4}><Typography.Text code ellipsis={{ tooltip: value }} style={{ maxWidth: 190 }}>{value}</Typography.Text><Button type="text" size="small" icon={<CopyOutlined />} onClick={() => void copyScene(item)} /></Space> : <Tag color="processing">生成中</Tag>,
            },
            { title: '启用', dataIndex: 'status', width: 80, render: (_, item) => <Switch checked={item.status === 'ACTIVE'} onChange={(checked) => void toggleStatus(item, checked)} /> },
            { title: '操作', key: 'action', width: 145, fixed: 'right', render: (_, item) => <Space size={2}><Button type="link" icon={<EyeOutlined />} onClick={() => setPreviewing(item)}>查看</Button><Button type="link" onClick={() => openForm(item)}>编辑</Button></Space> },
          ]}
        />
      </Card>

      <Modal title={editing ? '编辑桌位' : '新增桌位'} open={formOpen} onCancel={() => setFormOpen(false)} onOk={() => void save()} confirmLoading={saving} okText={editing ? '保存修改' : '创建桌位'}>
        <Form<TableCodeFormValues> form={form} layout="vertical">
          <Form.Item label="桌台区域" required>
            <Space.Compact block>
              <Form.Item name="areaId" noStyle rules={[{ required: true, message: '请选择区域' }]}><Select placeholder="请选择区域" options={areas.filter((area) => area.status === 'ACTIVE').map((area) => ({ value: area.id, label: area.name }))} /></Form.Item>
              <Button onClick={() => openAreaForm()}>新增区域</Button>
            </Space.Compact>
          </Form.Item>
          <Row gutter={12}>
            <Col span={12}><Form.Item label="桌号" name="tableCode" rules={[{ required: true, message: '请输入桌号' }]}><Input placeholder="例如：B02" /></Form.Item></Col>
            <Col span={12}><Form.Item label="桌位名称" name="name" rules={[{ required: true, message: '请输入桌位名称' }]}><Input placeholder="例如：靠窗双人桌" /></Form.Item></Col>
          </Row>
          <Row gutter={12}>
            <Col span={8}><Form.Item label="容量" name="capacity"><InputNumber min={1} max={999} precision={0} addonAfter="人" style={{ width: '100%' }} /></Form.Item></Col>
            <Col span={8}><Form.Item label="排序" name="sortOrder"><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item></Col>
            <Col span={8}><Form.Item label="立即启用" name="enabled" valuePropName="checked"><Switch /></Form.Item></Col>
          </Row>
          <Form.Item label="备注" name="remark"><Input.TextArea rows={2} maxLength={200} showCount placeholder="例如：靠近电源、适合四人" /></Form.Item>
          <Alert type="warning" showIcon message="桌号在同一门店内不能重复" description="创建后由服务端生成随机 publicId、qrScene 和完整小程序路径；前端不会把商户 ID 或桌位主键直接编码进二维码。" />
        </Form>
      </Modal>

      <Modal title={editingArea ? '编辑桌台区域' : '新增桌台区域'} open={areaFormOpen} onCancel={() => { setAreaFormOpen(false); setEditingArea(undefined); }} onOk={() => void saveArea()} confirmLoading={saving} okText={editingArea ? '保存修改' : '创建区域'}>
        <Form<TableAreaFormValues> form={areaForm} layout="vertical">
          <Form.Item label="区域名称" name="name" rules={[{ required: true, message: '请输入区域名称' }]}><Input placeholder="例如：室内、露台、二楼" /></Form.Item>
          <Form.Item label="排序" name="sortOrder" rules={[{ required: true }]}><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item>
          <Form.Item label="立即启用" name="enabled" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>

      <Modal title={previewing ? `${previewing.areaName} · ${previewing.tableName}` : '桌码预览'} width={760} open={Boolean(previewing)} onCancel={() => setPreviewing(undefined)} footer={previewing ? <Space><Button onClick={() => setPreviewing(undefined)}>关闭</Button><Button type="primary" icon={<DownloadOutlined />} onClick={() => downloadDevelopmentCode(previewing)}>下载联调二维码</Button></Space> : null}>
        {previewing && <div className="table-code-preview">
          <Alert type="warning" showIcon message="当前是普通二维码 / 开发联调码" description="二维码内容为后端返回的 miniappPath，并非微信官方无限制小程序码。待配置正式 AppID、AppSecret 并接入微信 getUnlimited 接口后，再切换为官方小程序码。" />
          <div className="table-code-code-grid single-code">
            <div className="table-code-image" ref={qrContainerRef}>
              <QRCode type="canvas" value={previewing.miniappPath} size={230} bordered={false} status={previewing.scene ? 'active' : 'loading'} />
              <strong>普通二维码 · 开发联调</strong>
              <Typography.Text type="secondary" ellipsis={{ tooltip: previewing.miniappPath }}>{previewing.miniappPath}</Typography.Text>
            </div>
            <div className="table-code-image official-placeholder"><FileImageOutlined /><span>微信官方小程序码待接入</span></div>
          </div>
          <Descriptions size="small" column={1} bordered items={[
            { label: '桌号', children: previewing.tableNo },
            { label: '小程序路径', children: <Typography.Text code copyable>{previewing.miniappPath}</Typography.Text> },
            { label: 'qrScene', children: <Typography.Text code copyable>{previewing.scene || '生成中'}</Typography.Text> },
            { label: '状态', children: <Tag color={previewing.status === 'ACTIVE' ? 'success' : 'default'}>{previewing.status === 'ACTIVE' ? '可扫码下单' : '已停用'}</Tag> },
          ]} />
          <Typography.Paragraph type="secondary" className="table-code-help"><QrcodeOutlined /> 顾客扫码后，小程序顶部应持续显示当前桌台；购物车、结算页和订单详情都沿用本次解析得到的桌码上下文。</Typography.Paragraph>
        </div>}
      </Modal>
    </div>
  );
}
