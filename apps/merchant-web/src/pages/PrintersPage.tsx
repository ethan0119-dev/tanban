import {
  ApiOutlined,
  CheckCircleOutlined,
  CloudServerOutlined,
  ExperimentOutlined,
  PlusOutlined,
  PrinterOutlined,
  RedoOutlined,
  ThunderboltOutlined,
  WifiOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Badge,
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
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { api, errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import type { PrintJob, Printer } from '../types';
import { dateTime } from '../utils/format';
import { canRetryPrintJob, printJobStatusView } from '../utils/print-job';

const printerStatus = {
  ONLINE: { text: '在线', status: 'success' as const },
  OFFLINE: { text: '离线', status: 'default' as const },
  PAPER_OUT: { text: '缺纸', status: 'error' as const },
  DISABLED: { text: '已停用', status: 'warning' as const },
};

interface PrinterFormValues {
  name: string;
  vendor: string;
  model: string;
  sn: string;
  type: Printer['type'];
  enabled: boolean;
}

function normalizePrinter(value: Printer): Printer {
  const raw = value as unknown as Record<string, unknown>;
  const provider = String(raw.provider ?? value.vendor ?? 'mock');
  const rawStatus = String(raw.status ?? value.status ?? 'ACTIVE');
  return {
    ...value,
    vendor: value.vendor ?? provider,
    provider,
    type: value.type ?? (provider === 'mock' ? 'VIRTUAL' : String(raw.model ?? '').toLowerCase().includes('label') ? 'LABEL' : 'RECEIPT'),
    enabled: value.enabled ?? rawStatus === 'ACTIVE',
    status: rawStatus === 'ACTIVE' ? 'ONLINE' : rawStatus === 'PAPER_OUT' ? 'PAPER_OUT' : rawStatus === 'DISABLED' ? 'DISABLED' : 'OFFLINE',
    paperWidth: value.paperWidth ?? Number(raw.paper_width ?? 58),
    printTrigger: value.printTrigger ?? (raw.print_trigger as Printer['printTrigger']) ?? 'PAYMENT_SUCCESS',
    templateText: value.templateText ?? String(raw.template_text ?? ''),
  };
}

function normalizeJob(value: PrintJob): PrintJob {
  const raw = value as unknown as Record<string, unknown>;
  return {
    ...value,
    orderNo: value.orderNo ?? `订单 #${raw.order_id ?? '--'}`,
    printerName: value.printerName ?? `打印机 #${raw.printer_id ?? '--'}`,
    type: value.type ?? (raw.is_reprint ? 'REPRINT' : 'RECEIPT'),
    retryCount: value.retryCount ?? Number(raw.attempts ?? 0),
    errorMessage: value.errorMessage ?? String(raw.error_message ?? ''),
    createdAt: value.createdAt ?? String(raw.created_at ?? ''),
  };
}

function printerPayload(values: PrinterFormValues | Printer, enabled = values.enabled) {
  const type = values.type;
  return {
    name: values.name,
    provider: type === 'VIRTUAL' ? 'mock' : String(('provider' in values && values.provider) || values.vendor || 'xpyun').toLowerCase(),
    model: values.model ?? (type === 'VIRTUAL' ? 'Mock Printer' : ''),
    sn: values.sn,
    paper_width: 'paperWidth' in values ? values.paperWidth ?? 58 : 58,
    print_trigger: 'printTrigger' in values ? values.printTrigger ?? 'PAYMENT_SUCCESS' : 'PAYMENT_SUCCESS',
    template_text: 'templateText' in values && values.templateText ? values.templateText : '订单 {{order_no}}\n{{items}}\n合计：{{total_cents}} 分\n{{remark}}',
    status: enabled ? 'ACTIVE' : 'DISABLED',
  };
}

export function PrintersPage({ jobsOnly = false }: { jobsOnly?: boolean }) {
  const [printers, setPrinters] = useState<Printer[]>([]);
  const [jobs, setJobs] = useState<PrintJob[]>([]);
  const [loading, setLoading] = useState(false);
  const [binding, setBinding] = useState(false);
  const [saving, setSaving] = useState(false);
  const [activeTab, setActiveTab] = useState(jobsOnly ? 'jobs' : 'devices');
  const [form] = Form.useForm<PrinterFormValues>();
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

  const openBind = (virtual = false) => {
    form.setFieldsValue(virtual ? {
      name: '开发调试虚拟打印机', vendor: 'TANBAN', model: 'Mock Printer', sn: `MOCK-${Date.now()}`, type: 'VIRTUAL', enabled: true,
    } : { vendor: '芯烨', model: 'T58H', type: 'LABEL', enabled: true, name: '出单打印机', sn: '' });
    setBinding(true);
  };

  const savePrinter = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const saved = normalizePrinter(await api.post<Printer>('/merchant/printers', printerPayload(values)));
      setPrinters((items) => [saved, ...items]);
      setBinding(false);
      messageApi.success(values.type === 'VIRTUAL' ? '虚拟打印机已启用' : '打印机绑定成功');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const testPrint = async (printer: Printer) => {
    try {
      await api.post(`/merchant/printers/${printer.id}/test`, { content: '摊伴打印测试页', requestedAt: new Date().toISOString() });
      messageApi.success(`测试任务已发送至 ${printer.name}`);
      setActiveTab('jobs');
      await load();
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
    today: jobs.filter((item) => new Date(item.createdAt).toDateString() === new Date().toDateString()).length,
  }), [jobs, printers]);

  const printJobsTable = (
    <Table<PrintJob>
      rowKey="id"
      loading={loading}
      dataSource={jobs}
      scroll={{ x: 920 }}
      locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无打印任务" /> }}
      columns={[
        { title: '任务 ID', dataIndex: 'id', width: 120, ellipsis: true },
        { title: '订单号', dataIndex: 'orderNo', width: 190 },
        { title: '打印机', dataIndex: 'printerName', width: 160, render: (value) => value || '--' },
        { title: '类型', dataIndex: 'type', width: 110, render: (value) => value === 'LABEL' ? '商品标签' : value === 'RECEIPT' ? '订单小票' : value },
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
        description="按打印机 SN 绑定云打印设备；硬件接入前可使用虚拟打印机联调任务链路"
        extra={<Space><Button icon={<ExperimentOutlined />} onClick={() => openBind(true)}>添加虚拟打印机</Button><Button type="primary" icon={<PlusOutlined />} onClick={() => openBind()}>绑定打印机</Button></Space>}
      />
      <Alert className="printer-tip" type="info" showIcon message="当前阶段建议使用 Mock 虚拟打印机" description="虚拟打印机会完整记录打印任务和补打动作，但不会向真实硬件发送内容。接入芯烨云打印后，只需替换打印供应商适配器。" />
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
                          <div className="printer-info"><span>品牌型号</span><strong>{printer.vendor || '--'} {printer.model || ''}</strong><span>设备 SN</span><code>{printer.sn}</code><span>设备类型</span><strong>{printer.type === 'VIRTUAL' ? 'Mock 虚拟机' : printer.type === 'LABEL' ? '标签打印机' : '小票打印机'}</strong><span>最后在线</span><strong>{dateTime(printer.lastSeenAt)}</strong></div>
                          <Button block icon={<PrinterOutlined />} disabled={!printer.enabled} onClick={() => void testPrint(printer)}>测试打印</Button>
                        </Card>
                      </Col>
                    );
                  })}
                </Row>
              ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有打印机"><Button type="primary" onClick={() => openBind(true)}>先添加虚拟打印机</Button></Empty>,
            },
            {
              key: 'jobs', label: <>打印任务 {stats.failed > 0 && <Tag color="error">{stats.failed} 个异常</Tag>}</>,
              children: printJobsTable,
            },
          ]}
        />
      </Card>

      <Modal title="绑定打印机" open={binding} onCancel={() => setBinding(false)} onOk={() => void savePrinter()} confirmLoading={saving} okText="确认绑定">
        <Form<PrinterFormValues> form={form} layout="vertical">
          <Form.Item label="打印机名称" name="name" rules={[{ required: true, message: '请输入名称' }]}><Input placeholder="例如：咖啡出单机" /></Form.Item>
          <Row gutter={12}>
            <Col span={12}><Form.Item label="品牌" name="vendor" rules={[{ required: true, message: '请输入品牌' }]}><Input placeholder="芯烨" /></Form.Item></Col>
            <Col span={12}><Form.Item label="型号" name="model" rules={[{ required: true, message: '请输入型号' }]}><Input placeholder="T58H" /></Form.Item></Col>
          </Row>
          <Form.Item label="设备 SN" name="sn" rules={[{ required: true, message: '请输入打印机底部的 SN' }]}><Input placeholder="打印机底部标签中的序列号" /></Form.Item>
          <Form.Item label="设备类型" name="type" rules={[{ required: true }]}><Select options={[{ label: '虚拟打印机（开发调试）', value: 'VIRTUAL' }, { label: '标签打印机', value: 'LABEL' }, { label: '小票打印机', value: 'RECEIPT' }]} /></Form.Item>
          <Form.Item label="绑定后立即启用" name="enabled" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
