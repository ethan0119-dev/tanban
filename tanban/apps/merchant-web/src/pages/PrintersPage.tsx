import {
  ApiOutlined,
  CheckCircleOutlined,
  CloudServerOutlined,
  EditOutlined,
  ExperimentOutlined,
  PlusOutlined,
  PrinterOutlined,
  RedoOutlined,
  ThunderboltOutlined,
  WifiOutlined,
} from '@ant-design/icons';
import {
  Badge,
  Button,
  Card,
  Col,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
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
import { useCallback, useEffect, useMemo, useState } from 'react';
import { api, errorMessage } from '../api/client';
import { DeveloperOnlyNote } from '../components/DeveloperOnlyNote';
import { PageHeading } from '../components/PageHeading';
import type { PrintCopyRole, PrintJob, Printer } from '../types';
import { beijingNowDateTime, beijingTodayKey, dateTime } from '../utils/format';
import { canRetryPrintJob, printJobStatusView } from '../utils/print-job';

const printerStatus = {
  ONLINE: { text: '在线', status: 'success' as const },
  OFFLINE: { text: '离线', status: 'default' as const },
  PAPER_OUT: { text: '缺纸', status: 'error' as const },
  UNREACHABLE: { text: '无法连接', status: 'error' as const },
  SIMULATED: { text: '模拟设备', status: 'processing' as const },
  DISABLED: { text: '已停用', status: 'warning' as const },
};

const printCopyRoleName: Record<NonNullable<PrintJob['copyRole']>, string> = {
  MERCHANT: '商家联',
  CUSTOMER: '顾客联',
  KITCHEN: '后厨联',
  ITEM: '商品标签',
};

interface PrinterFormValues {
  name: string;
  vendor: string;
  model: string;
  sn: string;
  type: Printer['type'];
  outputType: NonNullable<Printer['outputType']>;
  copyRoles: PrintCopyRole[];
  paperWidth: 58 | 80;
  labelWidthMM?: number;
  labelHeightMM?: number;
  enabled: boolean;
}

function normalizedPrinterCopyRoles(rawValue: unknown, outputType: NonNullable<Printer['outputType']>): PrintCopyRole[] {
  const source = Array.isArray(rawValue) ? rawValue : typeof rawValue === 'string' ? rawValue.split(',') : [];
  const allowed: PrintCopyRole[] = outputType === 'LABEL' ? ['ITEM'] : ['MERCHANT', 'CUSTOMER', 'KITCHEN'];
  const selected = new Set(source.map((value) => String(value).trim().toUpperCase()).filter((value): value is PrintCopyRole => allowed.includes(value as PrintCopyRole)));
  const result = allowed.filter((role) => selected.has(role));
  return result.length ? result : outputType === 'LABEL' ? ['ITEM'] : ['MERCHANT'];
}

function normalizePrinter(value: Printer): Printer {
  const raw = value as unknown as Record<string, unknown>;
  const provider = String(raw.provider ?? value.vendor ?? 'mock');
  const configuredStatus = String(raw.status ?? 'ACTIVE');
  const connectionStatus = String(raw.connection_status ?? value.connectionStatus ?? (provider === 'mock' ? 'SIMULATED' : 'UNREACHABLE'));
  const outputType = value.outputType ?? (raw.output_type as Printer['outputType']) ?? (value.type === 'LABEL' ? 'LABEL' : 'RECEIPT');
  return {
    ...value,
    vendor: value.vendor ?? provider,
    provider,
    type: value.type ?? (provider === 'mock' ? 'VIRTUAL' : outputType === 'LABEL' ? 'LABEL' : 'RECEIPT'),
    enabled: value.enabled ?? configuredStatus === 'ACTIVE',
    status: configuredStatus === 'DISABLED' ? 'DISABLED' : connectionStatus as Printer['status'],
    connectionStatus: connectionStatus as Printer['connectionStatus'],
    connectionMessage: String(raw.connection_message ?? value.connectionMessage ?? ''),
    statusCheckedAt: String(raw.status_checked_at ?? value.statusCheckedAt ?? ''),
    lastSeenAt: String(raw.last_seen_at ?? value.lastSeenAt ?? ''),
    paperWidth: Number(value.paperWidth ?? raw.paper_width ?? 58) === 80 ? 80 : 58,
    labelWidthMM: Number(value.labelWidthMM ?? raw.label_width_mm ?? 0) || undefined,
    labelHeightMM: Number(value.labelHeightMM ?? raw.label_height_mm ?? 0) || undefined,
    printTrigger: value.printTrigger ?? (raw.print_trigger as Printer['printTrigger']) ?? 'PAYMENT_SUCCESS',
    outputType,
    copyRoles: normalizedPrinterCopyRoles(value.copyRoles ?? raw.copyRoles ?? raw.copy_roles, outputType),
    templateText: value.templateText ?? String(raw.template_text ?? ''),
  };
}

function normalizeJob(value: PrintJob): PrintJob {
  const raw = value as unknown as Record<string, unknown>;
  const paperWidth = Number(value.paperWidth ?? raw.paper_width ?? 58);
  return {
    ...value,
    orderNo: value.orderNo ?? `订单 #${raw.order_id ?? '--'}`,
    printerName: value.printerName ?? `打印机 #${raw.printer_id ?? '--'}`,
    type: value.type ?? (raw.is_reprint ? 'REPRINT' : 'RECEIPT'),
    templateId: value.templateId ?? raw.template_id as PrintJob['templateId'],
    copyRole: (value.copyRole ?? raw.copy_role) as PrintJob['copyRole'],
    paperWidth: paperWidth === 80 ? 80 : 58,
    retryCount: value.retryCount ?? Number(raw.attempts ?? 0),
    errorMessage: value.errorMessage ?? String(raw.error_message ?? ''),
    createdAt: value.createdAt ?? String(raw.created_at ?? ''),
  };
}

function printerPayload(values: PrinterFormValues | Printer, enabled = values.enabled) {
  const type = values.type;
  const outputType = values.outputType ?? (type === 'LABEL' ? 'LABEL' : 'RECEIPT');
  const copyRoles = normalizedPrinterCopyRoles(values.copyRoles, outputType);
  return {
    name: values.name,
    provider: type === 'VIRTUAL' ? 'mock' : String(('provider' in values && values.provider) || values.vendor || 'xpyun').toLowerCase(),
    model: values.model ?? (type === 'VIRTUAL' ? 'Mock Printer' : ''),
    sn: values.sn,
    paper_width: values.paperWidth === 80 ? 80 : 58,
    label_width_mm: outputType === 'LABEL' ? Number(values.labelWidthMM ?? 0) : null,
    label_height_mm: outputType === 'LABEL' ? Number(values.labelHeightMM ?? 0) : null,
    print_trigger: 'printTrigger' in values ? values.printTrigger ?? 'PAYMENT_SUCCESS' : 'PAYMENT_SUCCESS',
    output_type: outputType,
    copyRoles,
    copy_roles: copyRoles,
    template_text: 'templateText' in values && values.templateText ? values.templateText : '订单 {{order_no}}\n{{items}}\n合计：{{total_cents}} 分\n{{remark}}',
    status: enabled ? 'ACTIVE' : 'DISABLED',
  };
}

export function PrintersPage({ jobsOnly = false }: { jobsOnly?: boolean }) {
  const [printers, setPrinters] = useState<Printer[]>([]);
  const [jobs, setJobs] = useState<PrintJob[]>([]);
  const [loading, setLoading] = useState(false);
  const [binding, setBinding] = useState(false);
  const [editingPrinter, setEditingPrinter] = useState<Printer | null>(null);
  const [saving, setSaving] = useState(false);
  const [activeTab, setActiveTab] = useState(jobsOnly ? 'jobs' : 'devices');
  const [form] = Form.useForm<PrinterFormValues>();
  const formOutputType = Form.useWatch('outputType', form);
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      if (jobsOnly) {
        const jobResult = await api.getList<PrintJob>('/merchant/print-jobs', { page_size: 100 });
        setJobs(jobResult.items.map(normalizeJob));
        setPrinters([]);
        return;
      }
      const [printerResult, jobResult] = await Promise.all([
        api.getList<Printer>('/merchant/printers'),
        api.getList<PrintJob>('/merchant/print-jobs', { page_size: 100 }),
      ]);
      setPrinters(printerResult.items.map(normalizePrinter));
      setJobs(jobResult.items.map(normalizeJob));
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [jobsOnly, messageApi]);

  useEffect(() => { void load(); }, [load]);

  useEffect(() => {
    if (jobsOnly) return;
    const timer = window.setInterval(() => {
      void api.getList<Printer>('/merchant/printers')
        .then((result) => setPrinters(result.items.map(normalizePrinter)))
        .catch(() => undefined);
    }, 30_000);
    return () => window.clearInterval(timer);
  }, [jobsOnly]);

  useEffect(() => {
    if (!binding || !formOutputType) return;
    const current = form.getFieldValue('copyRoles') ?? [];
    if (formOutputType === 'LABEL') {
      form.setFieldValue('type', 'LABEL');
      if (!form.getFieldValue('labelWidthMM')) form.setFieldValue('labelWidthMM', 40);
      if (!form.getFieldValue('labelHeightMM')) form.setFieldValue('labelHeightMM', 30);
      if (current.length !== 1 || current[0] !== 'ITEM') form.setFieldValue('copyRoles', ['ITEM']);
      return;
    }
    if (form.getFieldValue('type') === 'LABEL') form.setFieldValue('type', 'RECEIPT');
    if (!current.length || current.includes('ITEM')) form.setFieldValue('copyRoles', ['MERCHANT']);
  }, [binding, form, formOutputType]);

  const openBind = (virtual = false) => {
    setEditingPrinter(null);
    form.resetFields();
    form.setFieldsValue(virtual ? {
      name: '模拟打印机（测试）', vendor: 'TANBAN', model: 'Test Printer', sn: `MOCK-${Date.now()}`, type: 'VIRTUAL', outputType: 'RECEIPT', copyRoles: ['MERCHANT'], paperWidth: 58, enabled: true,
    } : { vendor: '芯烨', model: 'XP-T271U', type: 'LABEL', outputType: 'LABEL', copyRoles: ['ITEM'], paperWidth: 58, labelWidthMM: 40, labelHeightMM: 30, enabled: true, name: '杯贴打印机', sn: '' });
    setBinding(true);
  };

  const openEdit = (printer: Printer) => {
    setEditingPrinter(printer);
    form.resetFields();
    form.setFieldsValue({
      name: printer.name,
      vendor: printer.vendor ?? printer.provider ?? '',
      model: printer.model ?? '',
      sn: printer.sn,
      type: printer.type,
      outputType: printer.outputType ?? (printer.type === 'LABEL' ? 'LABEL' : 'RECEIPT'),
      copyRoles: normalizedPrinterCopyRoles(printer.copyRoles, printer.outputType ?? (printer.type === 'LABEL' ? 'LABEL' : 'RECEIPT')),
      paperWidth: printer.paperWidth === 80 ? 80 : 58,
      labelWidthMM: printer.labelWidthMM,
      labelHeightMM: printer.labelHeightMM,
      enabled: printer.enabled,
    });
    setBinding(true);
  };

  const closeBinding = () => {
    if (saving) return;
    setBinding(false);
    setEditingPrinter(null);
    form.resetFields();
  };

  const savePrinter = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const saved = normalizePrinter(editingPrinter
        ? await api.put<Printer>(`/merchant/printers/${editingPrinter.id}`, printerPayload(values))
        : await api.post<Printer>('/merchant/printers', printerPayload(values)));
      setPrinters((items) => editingPrinter
        ? items.map((item) => item.id === editingPrinter.id ? saved : item)
        : [saved, ...items]);
      setBinding(false);
      setEditingPrinter(null);
      form.resetFields();
      messageApi.success(editingPrinter ? '打印机配置已更新' : values.type === 'VIRTUAL' ? '模拟打印机已启用' : '打印机绑定成功');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const testPrint = async (printer: Printer) => {
    try {
      await api.post(`/merchant/printers/${printer.id}/test`, { content: '摊伴打印测试页', requestedAt: beijingNowDateTime() });
      messageApi.success(`测试页已直接发送至 ${printer.name}；测试页不计入订单打印任务`);
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const togglePrinter = async (printer: Printer, enabled: boolean) => {
    try {
      const updated = normalizePrinter(await api.put<Printer>(`/merchant/printers/${printer.id}`, printerPayload(printer, enabled)));
      setPrinters((items) => items.map((item) => item.id === printer.id ? updated : item));
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const retryJob = async (job: PrintJob) => {
    try {
      await api.post(`/merchant/print-jobs/${job.id}/retry`, { markAsReprint: true });
      messageApi.success('补打任务已提交');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const stats = useMemo(() => ({
    online: printers.filter((item) => item.enabled && item.status === 'ONLINE').length,
    failed: jobs.filter((item) => item.status === 'FAILED' || item.status === 'UNKNOWN').length,
    today: jobs.filter((item) => dateTime(item.createdAt).slice(0, 10) === beijingTodayKey()).length,
  }), [jobs, printers]);

  const printJobsTable = (
    <Table<PrintJob>
      rowKey="id"
      loading={loading}
      dataSource={jobs}
      scroll={{ x: 1120 }}
      locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无打印任务" /> }}
      columns={[
        { title: '任务 ID', dataIndex: 'id', width: 120, ellipsis: true },
        { title: '订单号', dataIndex: 'orderNo', width: 190 },
        { title: '打印机', dataIndex: 'printerName', width: 160, render: (value) => value || '--' },
        { title: '类型', dataIndex: 'type', width: 110, render: (value) => value === 'LABEL' ? '商品标签' : value === 'RECEIPT' ? '订单小票' : value },
        { title: '联次', dataIndex: 'copyRole', width: 100, render: (value: PrintJob['copyRole']) => value ? printCopyRoleName[value] : '--' },
        { title: '纸宽', dataIndex: 'paperWidth', width: 80, render: (value) => value ? `${value}mm` : '--' },
        { title: '状态', dataIndex: 'status', width: 125, render: (value: PrintJob['status']) => { const state = printJobStatusView(value); return <Tag icon={value === 'SUCCESS' ? <CheckCircleOutlined /> : value === 'PENDING' ? <CloudServerOutlined /> : undefined} color={state.color}>{state.text}</Tag>; } },
        { title: '重试', dataIndex: 'retryCount', width: 80, render: (value) => value ?? 0 },
        { title: '失败原因', dataIndex: 'errorMessage', ellipsis: true, render: (value) => value || '--' },
        { title: '创建时间', dataIndex: 'createdAt', width: 180, render: dateTime },
        { title: '操作', key: 'action', width: 100, fixed: 'right', render: (_, job) => <Button type="link" icon={<RedoOutlined />} disabled={!canRetryPrintJob(job.status)} onClick={() => void retryJob(job)}>补打</Button> },
      ]}
    />
  );

  if (jobsOnly) {
    return (
      <div className="page-shell">
        {contextHolder}
        <PageHeading
          title="打印任务"
          description="查看当前门店的打印结果；失败或需要重打时可提交补打任务"
          extra={<Button icon={<RedoOutlined />} loading={loading} onClick={() => void load()}>刷新任务</Button>}
        />
        <Row gutter={[16, 16]} className="printer-stats">
          <Col xs={12}><Card bordered={false}><strong>{stats.today}</strong><span>今日任务</span><ThunderboltOutlined /></Card></Col>
          <Col xs={12}><Card bordered={false}><strong className={stats.failed ? 'danger' : ''}>{stats.failed}</strong><span>异常任务</span><ApiOutlined /></Card></Col>
        </Row>
        <Card bordered={false} className="content-card">{printJobsTable}</Card>
      </div>
    );
  }

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading
        title="打印中心"
        description="按打印机 SN 绑定设备，并查看打印任务、异常状态和补打记录"
        extra={<Space>{import.meta.env.DEV ? <Button icon={<ExperimentOutlined />} onClick={() => openBind(true)}>添加模拟打印机</Button> : null}<Button type="primary" icon={<PlusOutlined />} onClick={() => openBind()}>绑定打印机</Button></Space>}
      />
      <DeveloperOnlyNote className="printer-tip">模拟打印机只记录任务和补打动作，不会向实体设备发送内容。</DeveloperOnlyNote>
      <Row gutter={[16, 16]} className="printer-stats">
        <Col xs={8}><Card bordered={false}><strong>{stats.online}</strong><span>在线设备</span><WifiOutlined /></Card></Col>
        <Col xs={8}><Card bordered={false}><strong>{stats.today}</strong><span>今日任务</span><ThunderboltOutlined /></Card></Col>
        <Col xs={8}><Card bordered={false}><strong className={stats.failed ? 'danger' : ''}>{stats.failed}</strong><span>异常任务</span><ApiOutlined /></Card></Col>
      </Row>
      <Card bordered={false} className="content-card">
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'devices', label: '打印机设备',
              children: printers.length ? (
                <Row gutter={[16, 16]}>
                  {printers.map((printer) => {
                    const state = printerStatus[printer.enabled ? printer.status : 'DISABLED'] ?? printerStatus.OFFLINE;
                    return (
                      <Col xs={24} md={12} xl={8} key={printer.id}>
                        <Card className="printer-card" bordered>
                          <div className="printer-card-head">
                            <span className={`printer-device-icon ${printer.type.toLowerCase()}`}><PrinterOutlined /></span>
                            <div><Typography.Title level={5}>{printer.name}</Typography.Title><Badge status={state.status} text={state.text} /></div>
                            <Switch checked={printer.enabled} onChange={(checked) => void togglePrinter(printer, checked)} />
                          </div>
                          <div className="printer-info"><span>品牌型号</span><strong>{printer.vendor || '--'} {printer.model || ''}</strong><span>设备 SN</span><code>{printer.sn}</code><span>接入类型</span><strong>{printer.type === 'VIRTUAL' ? '模拟设备（测试）' : '云打印机'}</strong><span>输出类型</span><strong>{printer.outputType === 'LABEL' ? '商品标签' : '订单小票'}</strong><span>{printer.outputType === 'LABEL' ? '标签尺寸' : '纸张宽度'}</span><strong>{printer.outputType === 'LABEL' ? `${printer.labelWidthMM || '--'} × ${printer.labelHeightMM || '--'}mm` : printer.paperWidth === 80 ? '80mm' : '58mm'}</strong><span>打印联次</span><strong>{(printer.copyRoles ?? []).map((role) => printCopyRoleName[role]).join(' / ') || '--'}</strong><span>状态说明</span><strong>{printer.connectionMessage || '--'}</strong><span>状态检查</span><strong>{dateTime(printer.statusCheckedAt)}</strong><span>最后在线</span><strong>{dateTime(printer.lastSeenAt)}</strong></div>
                          <Row gutter={8}>
                            <Col span={10}><Button block icon={<EditOutlined />} onClick={() => openEdit(printer)}>编辑配置</Button></Col>
                            <Col span={14}><Button block icon={<PrinterOutlined />} disabled={!printer.enabled} onClick={() => void testPrint(printer)}>测试打印</Button></Col>
                          </Row>
                        </Card>
                      </Col>
                    );
                  })}
                </Row>
              ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有打印机"><Button type="primary" onClick={() => openBind(false)}>绑定第一台打印机</Button></Empty>,
            },
            {
              key: 'jobs', label: <>打印任务 {stats.failed > 0 && <Tag color="error">{stats.failed} 个异常</Tag>}</>,
              children: printJobsTable,
            },
          ]}
        />
      </Card>

      <Modal title={editingPrinter ? '编辑打印机' : '绑定打印机'} open={binding} onCancel={closeBinding} onOk={() => void savePrinter()} confirmLoading={saving} okText={editingPrinter ? '保存配置' : '确认绑定'} cancelButtonProps={{ disabled: saving }} maskClosable={!saving} keyboard={!saving}>
        <Form<PrinterFormValues> form={form} layout="vertical">
          <Form.Item label="打印机名称" name="name" rules={[{ required: true, message: '请输入名称' }]}><Input placeholder="例如：咖啡出单机" /></Form.Item>
          <Row gutter={12}>
            <Col span={12}><Form.Item label="品牌" name="vendor" rules={[{ required: true, message: '请输入品牌' }]}><Input placeholder="芯烨" /></Form.Item></Col>
            <Col span={12}><Form.Item label="型号" name="model" rules={[{ required: true, message: '请输入型号' }]}><Input placeholder="T58H" /></Form.Item></Col>
          </Row>
          <Form.Item label="设备 SN" name="sn" rules={[{ required: true, message: '请输入打印机底部的 SN' }]}><Input placeholder="打印机底部标签中的序列号" /></Form.Item>
          <Form.Item label="设备类型" name="type" rules={[{ required: true }]}><Select options={[...(import.meta.env.DEV ? [{ label: '模拟打印机（测试）', value: 'VIRTUAL' as const }] : []), { label: '标签打印机', value: 'LABEL' }, { label: '小票打印机', value: 'RECEIPT' }]} /></Form.Item>
          <Form.Item label="输出类型" name="outputType" rules={[{ required: true, message: '请选择输出类型' }]}><Select options={[{ label: '订单小票', value: 'RECEIPT' }, { label: '商品标签 / 杯贴', value: 'LABEL' }]} /></Form.Item>
          {formOutputType === 'LABEL' ? (
            <Row gutter={12}>
              <Col span={12}>
                <Form.Item label="标签宽度（mm）" name="labelWidthMM" rules={[{ required: true, message: '请输入标签宽度' }, { type: 'number', min: 20, max: 110, message: '请输入 20–110mm' }]}>
                  <InputNumber min={20} max={110} precision={0} style={{ width: '100%' }} placeholder="40" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="标签高度（mm）" name="labelHeightMM" rules={[{ required: true, message: '请输入标签高度' }, { type: 'number', min: 20, max: 200, message: '请输入 20–200mm' }]}>
                  <InputNumber min={20} max={200} precision={0} style={{ width: '100%' }} placeholder="30" />
                </Form.Item>
              </Col>
              <Col span={24}><Typography.Text type="secondary">XP-T271U 当前使用 40 × 30mm 标签；尺寸按单张标签面材填写，不包含背纸。</Typography.Text></Col>
            </Row>
          ) : (
            <Form.Item label="纸张宽度" name="paperWidth" rules={[{ required: true, message: '请选择打印机实际纸宽' }]} extra="任务会按这台设备的实际宽度重新排版，避免 80mm 模板发送到 58mm 设备后截断。"><Select options={[{ label: '58mm 热敏纸', value: 58 }, { label: '80mm 热敏纸', value: 80 }]} /></Form.Item>
          )}
          <Form.Item label="打印联次" name="copyRoles" rules={[{ required: true, type: 'array', min: 1, message: '请至少选择一个打印联次' }]}>
            <Select
              mode="multiple"
              disabled={formOutputType === 'LABEL'}
              placeholder="选择这台打印机负责的联次"
              options={formOutputType === 'LABEL'
                ? [{ label: '商品标签', value: 'ITEM' }]
                : [{ label: '商家联', value: 'MERCHANT' }, { label: '顾客联', value: 'CUSTOMER' }, { label: '后厨联', value: 'KITCHEN' }]}
            />
          </Form.Item>
          <Form.Item label="绑定后立即启用" name="enabled" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
