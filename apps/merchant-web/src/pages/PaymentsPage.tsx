import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  DollarOutlined,
  RedoOutlined,
  SearchOutlined,
  UndoOutlined,
} from '@ant-design/icons';
import {
  Button,
  Card,
  Col,
  DatePicker,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Row,
  Statistic,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import type { Dayjs } from 'dayjs';
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { api, ApiError, errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import type { OfflineReconciliation, PaymentRecord, RefundRecord } from '../types';
import { dateTime, toBeijingRFC3339, yuan } from '../utils/format';
import { paymentProviderName } from '../utils/payment';

const { RangePicker } = DatePicker;
const paymentStatus: Record<string, { text: string; color: string }> = {
  SUCCESS: { text: '支付成功', color: 'success' },
  PAID: { text: '支付成功', color: 'success' },
  PENDING: { text: '支付中', color: 'processing' },
  PAYING: { text: '支付中', color: 'processing' },
  CREATING: { text: '创建支付中', color: 'processing' },
  FAILED: { text: '支付失败', color: 'error' },
  CLOSED: { text: '已关闭', color: 'default' },
  REFUNDED: { text: '已全额退款', color: 'purple' },
  PARTIAL_REFUNDED: { text: '部分退款', color: 'orange' },
  PARTIALLY_REFUNDED: { text: '部分退款', color: 'orange' },
};

function normalizePayment(value: PaymentRecord): PaymentRecord {
  const raw = value as unknown as Record<string, unknown>;
  const paidCents = raw.amount_cents ?? raw.paid_cents;
  const refundedCents = Number(raw.refunded_cents ?? 0);
  return {
    ...value,
    orderId: value.orderId ?? (raw.order_id as string | number),
    orderNo: value.orderNo ?? String(raw.order_no ?? ''),
    paymentNo: value.paymentNo ?? String(raw.payment_no ?? raw.provider_order_no ?? ''),
    provider: value.provider ?? String(raw.provider ?? ''),
    amount: paidCents !== undefined ? Number(paidCents) / 100 : Number(value.amount ?? 0),
    refundableAmount: value.refundableAmount ?? (paidCents !== undefined ? (Number(paidCents) - refundedCents) / 100 : Number(value.amount ?? 0)),
    method: paymentProviderName(value.method ?? raw.provider),
    status: value.status ?? String(raw.payment_status ?? raw.status ?? ''),
    paidAt: value.paidAt ?? (raw.paid_at ? String(raw.paid_at) : undefined),
  };
}

function normalizeRefund(value: RefundRecord, orderNumbers: Map<string, string>): RefundRecord {
  const raw = value as unknown as Record<string, unknown>;
  const orderId = String(raw.order_id ?? '');
  return {
    ...value,
    refundNo: value.refundNo ?? String(raw.refund_no ?? ''),
    orderNo: value.orderNo ?? orderNumbers.get(orderId) ?? `订单 ID ${orderId}`,
    amount: raw.amount_cents !== undefined ? Number(raw.amount_cents) / 100 : Number(value.amount ?? 0),
    operatorName: value.operatorName ?? (raw.created_by ? `员工 #${raw.created_by}` : undefined),
    createdAt: value.createdAt ?? String(raw.created_at ?? ''),
  };
}

function normalizeReconciliation(value: Record<string, unknown>): OfflineReconciliation {
  return {
    id: value.id as string | number,
    businessDate: String(value.business_date ?? value.businessDate ?? ''),
    expectedCash: Number(value.expected_cash_cents ?? 0) / 100,
    expectedExternal: Number(value.expected_external_cents ?? 0) / 100,
    actualCash: value.actual_cash_cents === undefined ? undefined : Number(value.actual_cash_cents) / 100,
    actualExternal: value.actual_external_cents === undefined ? undefined : Number(value.actual_external_cents) / 100,
    discrepancy: value.discrepancy_cents === undefined ? undefined : Number(value.discrepancy_cents) / 100,
    cashCount: Number(value.cash_count ?? 0),
    externalCount: Number(value.external_count ?? 0),
    note: String(value.note ?? ''),
    status: String(value.status ?? 'UNCONFIRMED') as OfflineReconciliation['status'],
    confirmedBy: value.confirmed_by as string | number,
    confirmedAt: String(value.confirmed_at ?? ''),
  };
}

export function PaymentsPage() {
  const [activeTab, setActiveTab] = useState('payments');
  const [payments, setPayments] = useState<PaymentRecord[]>([]);
  const [refunds, setRefunds] = useState<RefundRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [dates, setDates] = useState<[Dayjs | null, Dayjs | null] | null>(null);
  const [refundTarget, setRefundTarget] = useState<PaymentRecord | null>(null);
  const [refundIdempotencyKey, setRefundIdempotencyKey] = useState('');
  const [refundSaving, setRefundSaving] = useState(false);
  const [refundForm] = Form.useForm<{ amount: number; reason: string }>();
  const [reconciliationDate, setReconciliationDate] = useState<Dayjs>(dayjs());
  const [reconciliation, setReconciliation] = useState<OfflineReconciliation | null>(null);
  const [reconciliationLoading, setReconciliationLoading] = useState(false);
  const [reconciliationForm] = Form.useForm<{ actualCash: number; actualExternal: number; note: string }>();
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    const params = {
      keyword: keyword || undefined,
      startAt: toBeijingRFC3339(dates?.[0]?.startOf('day')),
      endAt: toBeijingRFC3339(dates?.[1]?.endOf('day')),
      page_size: 100,
    };
    try {
      let paymentItems: PaymentRecord[];
      try {
        const paymentResult = await api.getList<PaymentRecord>('/merchant/payments', params);
        paymentItems = paymentResult.items.map(normalizePayment);
      } catch (error) {
        if (!(error instanceof ApiError) || error.status !== 404) throw error;
        const orders = await api.getList<Record<string, unknown>>('/merchant/orders', params);
        paymentItems = orders.items
          .filter((order) => Number(order.paid_cents ?? 0) > 0)
          .map((order) => normalizePayment({
            id: order.id as string | number,
            orderId: order.id as string | number,
            orderNo: String(order.order_no ?? ''),
            amount: Number(order.paid_cents ?? 0) / 100,
            refundableAmount: (Number(order.paid_cents ?? 0) - Number(order.refunded_cents ?? 0)) / 100,
            status: String(order.payment_status ?? order.status ?? ''),
            paidAt: order.paid_at ? String(order.paid_at) : undefined,
          }));
      }
      const refundResult = await api.getList<RefundRecord>('/merchant/refunds', params);
      const orderNumbers = new Map(paymentItems.map((payment) => [String(payment.orderId), payment.orderNo]));
      setPayments(paymentItems);
      setRefunds(refundResult.items.map((refund) => normalizeRefund(refund, orderNumbers)));
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [dates, keyword, messageApi]);

  useEffect(() => { void load(); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const loadReconciliation = useCallback(async (date = reconciliationDate) => {
    setReconciliationLoading(true);
    try {
      const report = normalizeReconciliation(await api.get<Record<string, unknown>>('/merchant/offline-reconciliation', { business_date: date.format('YYYY-MM-DD') }));
      setReconciliation(report);
      reconciliationForm.setFieldsValue({
        actualCash: report.actualCash ?? report.expectedCash,
        actualExternal: report.actualExternal ?? report.expectedExternal,
        note: report.note || '',
      });
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setReconciliationLoading(false);
    }
  }, [messageApi, reconciliationDate, reconciliationForm]);

  useEffect(() => {
    if (activeTab === 'reconciliation') void loadReconciliation();
  }, [activeTab]); // eslint-disable-line react-hooks/exhaustive-deps

  const confirmReconciliation = async () => {
    const values = await reconciliationForm.validateFields();
    setReconciliationLoading(true);
    try {
      const report = normalizeReconciliation(await api.post<Record<string, unknown>>('/merchant/offline-reconciliation/confirm', {
        business_date: reconciliationDate.format('YYYY-MM-DD'),
        actual_cash_cents: Math.round(values.actualCash * 100),
        actual_external_cents: Math.round(values.actualExternal * 100),
        note: values.note || '',
      }));
      setReconciliation(report);
      messageApi.success('线下支付日结已确认');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setReconciliationLoading(false);
    }
  };

  const openRefund = (payment: PaymentRecord) => {
    setRefundTarget(payment);
    setRefundIdempotencyKey(globalThis.crypto?.randomUUID?.() || `refund-${Date.now()}-${Math.random().toString(16).slice(2)}`);
    refundForm.setFieldsValue({ amount: payment.refundableAmount ?? payment.amount, reason: '' });
  };

  const submitRefund = async () => {
    if (!refundTarget) return;
    const values = await refundForm.validateFields();
    setRefundSaving(true);
    try {
      await api.postIdempotent('/merchant/refunds', {
        order_id: Number(refundTarget.orderId),
        amount_cents: Math.round(values.amount * 100),
        reason: values.reason,
      }, refundIdempotencyKey);
      messageApi.success('退款申请已提交，将以支付机构最终结果为准');
      setRefundTarget(null);
      setRefundIdempotencyKey('');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setRefundSaving(false);
    }
  };

  const summary = useMemo(() => {
    const successful = payments.filter((item) => ['SUCCESS', 'PAID', 'PARTIAL_REFUNDED', 'PARTIALLY_REFUNDED', 'REFUNDED'].includes(item.status));
    return {
      received: successful.reduce((sum, item) => sum + Number(item.amount ?? 0), 0),
      paymentCount: successful.length,
      refunding: refunds.filter((item) => ['PENDING', 'PROCESSING', 'REFUNDING'].includes(item.status)).length,
      refunded: refunds.filter((item) => ['SUCCESS', 'REFUNDED'].includes(item.status)).reduce((sum, item) => sum + Number(item.amount ?? 0), 0),
    };
  }, [payments, refunds]);

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading title="支付与退款" description="本页展示订单支付状态；实际资金由支付机构直接结算至商户银行卡" extra={<Button icon={<RedoOutlined />} loading={loading} onClick={() => void load()}>刷新记录</Button>} />
      <Row gutter={[16, 16]}>
        <Col xs={12} xl={6}><Card bordered={false} className="metric-card compact"><Statistic title="支付总额" value={summary.received} prefix="¥" precision={2} valueStyle={{ color: '#3b2a20' }} /><DollarOutlined className="summary-watermark" /></Card></Col>
        <Col xs={12} xl={6}><Card bordered={false} className="metric-card compact"><Statistic title="成功笔数" value={summary.paymentCount} suffix="笔" /><CheckCircleOutlined className="summary-watermark green" /></Card></Col>
        <Col xs={12} xl={6}><Card bordered={false} className="metric-card compact"><Statistic title="退款中" value={summary.refunding} suffix="笔" /><ClockCircleOutlined className="summary-watermark orange" /></Card></Col>
        <Col xs={12} xl={6}><Card bordered={false} className="metric-card compact"><Statistic title="已退款" value={summary.refunded} prefix="¥" precision={2} /><UndoOutlined className="summary-watermark red" /></Card></Col>
      </Row>
      <Card bordered={false} className="content-card payment-record-card">
        <div className="record-filter">
          <Input allowClear prefix={<SearchOutlined />} value={keyword} onChange={(event) => setKeyword(event.target.value)} onPressEnter={() => void load()} placeholder="搜索订单号或支付单号" />
          <RangePicker format="YYYY-MM-DD" value={dates} onChange={(value) => setDates(value)} />
          <Button type="primary" onClick={() => void load()}>查询</Button>
        </div>
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'payments', label: `支付记录 ${payments.length ? `(${payments.length})` : ''}`,
              children: (
                <Table<PaymentRecord>
                  rowKey="id"
                  loading={loading}
                  dataSource={payments}
                  scroll={{ x: 980 }}
                  locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无支付记录" /> }}
                  columns={[
                    { title: '订单号', dataIndex: 'orderNo', width: 190 },
                    { title: '支付单号', dataIndex: 'paymentNo', width: 210, render: (value, record) => value || record.providerOrderNo || '--' },
                    { title: '支付方式', dataIndex: 'method', width: 120, render: (value) => paymentProviderName(value) },
                    { title: '支付金额', dataIndex: 'amount', width: 125, render: (value) => <strong>{yuan(value)}</strong> },
                    { title: '可退金额', dataIndex: 'refundableAmount', width: 125, render: (value, record) => yuan(value ?? record.amount) },
                    { title: '状态', dataIndex: 'status', width: 120, render: (value: string) => { const info = paymentStatus[value] ?? { text: value, color: 'default' }; return <Tag color={info.color}>{info.text}</Tag>; } },
                    { title: '支付时间', dataIndex: 'paidAt', width: 180, render: dateTime },
                    {
                      title: '操作', key: 'action', width: 100, fixed: 'right',
                      render: (_, record) => <Button type="link" disabled={['offline_cash', 'external'].includes(String(record.provider)) || !['SUCCESS', 'PAID', 'PARTIAL_REFUNDED', 'PARTIALLY_REFUNDED'].includes(record.status) || Number(record.refundableAmount ?? record.amount) <= 0} onClick={() => openRefund(record)}>退款</Button>,
                    },
                  ]}
                />
              ),
            },
            {
              key: 'refunds', label: `退款记录 ${refunds.length ? `(${refunds.length})` : ''}`,
              children: (
                <Table<RefundRecord>
                  rowKey="id"
                  loading={loading}
                  dataSource={refunds}
                  scroll={{ x: 900 }}
                  locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无退款记录" /> }}
                  columns={[
                    { title: '退款单号', dataIndex: 'refundNo', width: 210 },
                    { title: '订单号', dataIndex: 'orderNo', width: 190 },
                    { title: '退款金额', dataIndex: 'amount', width: 130, render: (value) => <strong className="refund-amount">-{yuan(value)}</strong> },
                    { title: '原因', dataIndex: 'reason', ellipsis: true, render: (value) => value || '--' },
                    { title: '操作人', dataIndex: 'operatorName', width: 120, render: (value) => value || '--' },
                    { title: '状态', dataIndex: 'status', width: 120, render: (value: string) => <Tag color={['SUCCESS', 'REFUNDED'].includes(value) ? 'success' : ['FAILED'].includes(value) ? 'error' : 'processing'}>{['SUCCESS', 'REFUNDED'].includes(value) ? '退款成功' : value === 'FAILED' ? '退款失败' : '退款中'}</Tag> },
                    { title: '申请时间', dataIndex: 'createdAt', width: 180, render: dateTime },
                  ]}
                />
              ),
            },
            {
              key: 'reconciliation',
              label: '线下支付日结',
              children: (
                <div>
                  <Row gutter={[16, 16]} align="middle" style={{ marginBottom: 18 }}>
                    <Col><Typography.Text strong>营业日期</Typography.Text></Col>
                    <Col>
                      <DatePicker
                        value={reconciliationDate}
                        allowClear={false}
                        format="YYYY-MM-DD"
                        onChange={(value) => {
                          if (!value) return;
                          setReconciliationDate(value);
                          void loadReconciliation(value);
                        }}
                      />
                    </Col>
                    <Col><Button loading={reconciliationLoading} onClick={() => void loadReconciliation()}>重新核对</Button></Col>
                    <Col>{reconciliation?.status === 'CONFIRMED' ? <Tag color="success">已确认日结</Tag> : <Tag color="processing">待确认</Tag>}</Col>
                  </Row>
                  <Row gutter={[16, 16]} style={{ marginBottom: 20 }}>
                    <Col xs={24} md={12}><Card size="small" loading={reconciliationLoading}><Statistic title={`系统应收现金（${reconciliation?.cashCount || 0} 笔）`} value={reconciliation?.expectedCash || 0} prefix="¥" precision={2} /></Card></Col>
                    <Col xs={24} md={12}><Card size="small" loading={reconciliationLoading}><Statistic title={`系统外支付（${reconciliation?.externalCount || 0} 笔）`} value={reconciliation?.expectedExternal || 0} prefix="¥" precision={2} /></Card></Col>
                  </Row>
                  <Form form={reconciliationForm} layout="vertical" onFinish={() => void confirmReconciliation()}>
                    <Row gutter={16}>
                      <Col xs={24} md={12}><Form.Item label="盘点实收现金" name="actualCash" rules={[{ required: true, message: '请输入盘点现金' }]}><InputNumber min={0} precision={2} prefix="¥" style={{ width: '100%' }} /></Form.Item></Col>
                      <Col xs={24} md={12}><Form.Item label="核对系统外支付" name="actualExternal" rules={[{ required: true, message: '请输入系统外支付实收' }]}><InputNumber min={0} precision={2} prefix="¥" style={{ width: '100%' }} /></Form.Item></Col>
                    </Row>
                    <Form.Item label="日结备注" name="note"><Input.TextArea rows={3} maxLength={500} showCount placeholder="记录长短款原因、交接信息等" /></Form.Item>
                    {reconciliation?.status === 'CONFIRMED' && <Typography.Paragraph type={Number(reconciliation.discrepancy || 0) === 0 ? 'success' : 'danger'}>
                      上次确认差额：{yuan(reconciliation.discrepancy || 0)} · {dateTime(reconciliation.confirmedAt)}
                    </Typography.Paragraph>}
                    <Button type="primary" htmlType="submit" loading={reconciliationLoading}>确认本日线下支付日结</Button>
                  </Form>
                </div>
              ),
            },
          ]}
        />
      </Card>

      <Modal title="订单退款" open={!!refundTarget} onCancel={() => { setRefundTarget(null); setRefundIdempotencyKey(''); }} onOk={() => void submitRefund()} confirmLoading={refundSaving} okText="确认退款" okButtonProps={{ danger: true }}>
        {refundTarget && (
          <>
            <div className="refund-summary">
              <Typography.Text type="secondary">订单号</Typography.Text><strong>{refundTarget.orderNo}</strong>
              <Typography.Text type="secondary">原支付金额</Typography.Text><strong>{yuan(refundTarget.amount)}</strong>
              <Typography.Text type="secondary">当前可退</Typography.Text><strong>{yuan(refundTarget.refundableAmount ?? refundTarget.amount)}</strong>
            </div>
            <Form form={refundForm} layout="vertical">
              <Form.Item
                label="退款金额"
                name="amount"
                rules={[
                  { required: true, message: '请输入退款金额' },
                  { type: 'number', min: 0.01, max: Number(refundTarget.refundableAmount ?? refundTarget.amount), message: `退款金额应在 0.01 至 ${refundTarget.refundableAmount ?? refundTarget.amount} 元之间` },
                ]}
              >
                <InputNumber min={0.01} max={Number(refundTarget.refundableAmount ?? refundTarget.amount)} precision={2} prefix="¥" style={{ width: '100%' }} />
              </Form.Item>
              <Form.Item label="退款原因" name="reason" rules={[{ required: true, message: '请填写退款原因' }]}><Input.TextArea rows={3} maxLength={100} showCount placeholder="原因将记录在操作审计中" /></Form.Item>
            </Form>
          </>
        )}
      </Modal>
    </div>
  );
}
