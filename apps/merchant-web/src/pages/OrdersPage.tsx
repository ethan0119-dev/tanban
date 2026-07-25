import {
  ClockCircleOutlined,
  CloseCircleOutlined,
  CoffeeOutlined,
  DesktopOutlined,
  EyeOutlined,
  NumberOutlined,
  PrinterOutlined,
  ReloadOutlined,
  SearchOutlined,
  SyncOutlined,
  TableOutlined,
  TabletOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Col,
  DatePicker,
  Descriptions,
  Divider,
  Drawer,
  Empty,
  Form,
  Input,
  InputNumber,
  List,
  Modal,
  Pagination,
  Row,
  Segmented,
  Select,
  Space,
  Steps,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import type { TablePaginationConfig } from 'antd';
import type { Dayjs } from 'dayjs';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { api, CASHIER_TOKEN_KEY, errorMessage } from '../api/client';
import { useAuth } from '../auth/AuthContext';
import { canManageMerchant } from '../auth/permissions';
import { OrderStatusTag, orderStatusMap } from '../components/OrderStatusTag';
import { PageHeading } from '../components/PageHeading';
import { ordersForBusinessType } from '../features/orders/model';
import { merchantFeatureCopy } from '../features/availability/copy';
import { normalizeOrder } from '../features/storefront/model';
import type { ListResult, Order, OrderBusinessType, OrderItem, OrderReturnRequest, OrderStatus, OrderType, TableBoardResponse, TableBoardTable } from '../types';
import { dateTime, toBeijingRFC3339, yuan } from '../utils/format';

const { RangePicker } = DatePicker;
const statusTabs: Array<{ key: 'ALL' | OrderStatus; label: string }> = [
  { key: 'ALL', label: '全部' },
  { key: 'PENDING_PAYMENT', label: '待付款' },
  { key: 'PAID', label: '已付款' },
  { key: 'PREPARING', label: '制作中' },
  { key: 'READY', label: '待取餐' },
  { key: 'COMPLETED', label: '已完成' },
  { key: 'CLOSED', label: '已关闭' },
];

const timelineStatuses: OrderStatus[] = ['PENDING_PAYMENT', 'PAID', 'PREPARING', 'READY', 'COMPLETED'];
const nextStatus: Partial<Record<OrderStatus, { status: OrderStatus; text: string }>> = {
  PAID: { status: 'PREPARING', text: '开始制作' },
  PREPARING: { status: 'READY', text: '通知取餐' },
  READY: { status: 'COMPLETED', text: '完成订单' },
};

function orderSceneLabel(order: Order): string {
  if (order.orderType === 'DINE_IN') return order.tableAreaName || '店内桌码';
  if (order.orderType === 'TAKEOUT') return order.fastFoodPlateName
    ? `放餐位置：${[order.fastFoodPlateCode, order.fastFoodPlateName].filter(Boolean).join(' · ')}`
    : '快餐 / 到店自取 · 未指定码牌';
  return '外卖配送';
}

function OrderWorkCard({ order, onOpen }: { order: Order; onOpen: (order: Order) => void }) {
  const dineIn = order.orderType === 'DINE_IN';
  const products = order.items?.slice(0, 3).map((item) => `${item.productName} ×${item.quantity}`).join('、') || '等待加载商品明细';
  return (
    <Card
      bordered={false}
      className={`order-work-card ${dineIn ? 'dine-in' : 'takeout'}`}
      onClick={() => onOpen(order)}
      role="button"
      tabIndex={0}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') onOpen(order);
      }}
    >
      <div className="order-work-card-head">
        <Tag color={dineIn ? 'blue' : 'gold'}>{dineIn ? '桌码堂食' : '快餐自取'}</Tag>
        {order.settlementMode === 'PAY_AFTER' && order.paymentStatus === 'UNPAID' ? <Tag color="error">待结账</Tag> : <OrderStatusTag status={order.status} />}
      </div>
      <div className="order-work-card-scene">
        <span className="order-work-card-icon">{dineIn ? <TableOutlined /> : <NumberOutlined />}</span>
        <div>
          <small>{dineIn ? '当前桌台' : '取餐号'}</small>
          <strong>{dineIn ? (order.tableName || '未绑定桌台') : `#${order.pickupNo || '--'}`}</strong>
          <span>{orderSceneLabel(order)}</span>
        </div>
      </div>
      <div className="order-work-card-products"><CoffeeOutlined /> <span>{products}</span></div>
      {order.remark && <div className="order-work-card-remark">备注：{order.remark}</div>}
      <div className="order-work-card-meta">
        <span><ClockCircleOutlined /> {dateTime(order.createdAt)}</span>
        <strong>{yuan(order.paymentStatus === 'UNPAID' ? order.remainingAmount ?? order.amount : order.paidAmount ?? order.amount)}</strong>
      </div>
      <Button type="primary" ghost block icon={<EyeOutlined />} onClick={(event) => { event.stopPropagation(); onOpen(order); }}>查看并处理</Button>
    </Card>
  );
}

function itemConfigurationSummary(item: Order['items'][number]): string[] {
  const options = (item.configuration?.options || [])
    .map((option) => [option.groupName, option.valueName].filter(Boolean).join('：'))
    .filter(Boolean);
  const modifiers = (item.configuration?.modifiers || [])
    .map((modifier) => modifier.name ? `${modifier.name}${Number(modifier.quantity || 1) > 1 ? `×${modifier.quantity}` : ''}` : '')
    .filter(Boolean);
  return [
    ...options,
    ...(modifiers.length ? [`加料：${modifiers.join('、')}`] : []),
    ...(item.itemRemark ? [`单品备注：${item.itemRemark}`] : []),
  ];
}

function normalizeReturnRequest(value: Record<string, unknown>): OrderReturnRequest {
  return {
    id: value.id as string | number,
    orderItemId: (value.order_item_id ?? value.orderItemId) as string | number,
    skuId: (value.sku_id ?? value.skuId) as string | number,
    productName: String(value.product_name ?? value.productName ?? ''),
    quantity: Number(value.quantity ?? 0),
    amount: Number(value.amount_cents ?? value.amount ?? 0) / (value.amount_cents === undefined ? 1 : 100),
    reason: String(value.reason ?? ''),
    status: String(value.status ?? 'PENDING') as OrderReturnRequest['status'],
    requestedBy: (value.requested_by ?? value.requestedBy) as string | number,
    reviewedBy: (value.reviewed_by ?? value.reviewedBy) as string | number,
    reviewedAt: String(value.reviewed_at ?? value.reviewedAt ?? ''),
    reviewRemark: String(value.review_remark ?? value.reviewRemark ?? ''),
    createdAt: String(value.created_at ?? value.createdAt ?? ''),
  };
}

type OrderOperation =
  | { type: 'TRANSFER' }
  | { type: 'MERGE' }
  | { type: 'SPLIT' }
  | { type: 'RETURN'; item: OrderItem };

export function OrdersPage({ businessType = 'DINE_IN', unavailable = false, sceneMode }: { businessType?: OrderBusinessType; unavailable?: boolean; sceneMode?: 'DINE_IN' | 'TAKEOUT' }) {
  const { user } = useAuth();
  const canReviewReturns = canManageMerchant(user);
  const [viewMode, setViewMode] = useState<'ORDERS' | 'TABLES'>('ORDERS');
  const [status, setStatus] = useState<'ALL' | OrderStatus>('ALL');
  const [serviceMode, setServiceMode] = useState<'DINE_IN' | 'TAKEOUT'>(sceneMode || 'DINE_IN');
  const [keyword, setKeyword] = useState('');
  const [dates, setDates] = useState<[Dayjs | null, Dayjs | null] | null>(null);
  const [result, setResult] = useState<ListResult<Order>>({ items: [], meta: { page: 1, pageSize: 20, total: 0 } });
  const [loading, setLoading] = useState(false);
  const [selected, setSelected] = useState<Order | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [tableBoard, setTableBoard] = useState<TableBoardResponse | null>(null);
  const [tableBoardLoading, setTableBoardLoading] = useState(false);
  const [operation, setOperation] = useState<OrderOperation | null>(null);
  const [returnRequests, setReturnRequests] = useState<OrderReturnRequest[]>([]);
  const [operationForm] = Form.useForm<{ targetTableId?: string | number; sourceOrderId?: string | number; amount?: number; method?: 'CASH' | 'EXTERNAL'; quantity?: number; reason?: string; remark?: string }>();
  const [messageApi, contextHolder] = message.useMessage();
  const isDelivery = businessType === 'DELIVERY';
  const domainName = isDelivery ? '外卖' : sceneMode === 'TAKEOUT' ? '快餐' : sceneMode === 'DINE_IN' ? '堂食' : '店内';

  const load = useCallback(async (page = 1, pageSize = result.meta.pageSize ?? 20) => {
    if (unavailable) {
      setResult({ items: [], meta: { page: 1, pageSize, total: 0 } });
      return;
    }
    setLoading(true);
    try {
      const normalized = await api.getList<Order>('/merchant/orders', {
        status: status === 'ALL' ? undefined : status,
        keyword: keyword || undefined,
        startAt: toBeijingRFC3339(dates?.[0]?.startOf('day')),
        endAt: toBeijingRFC3339(dates?.[1]?.endOf('day')),
        order_type: isDelivery ? 'DELIVERY' : serviceMode,
        page,
        page_size: pageSize,
      });
      const expectedType: OrderType = isDelivery ? 'DELIVERY' : serviceMode;
      const items = ordersForBusinessType(normalized.items.map(normalizeOrder), expectedType);
      setResult({
        ...normalized,
        items,
        meta: { page, pageSize, ...normalized.meta },
      });
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [dates, isDelivery, keyword, messageApi, result.meta.pageSize, serviceMode, status, unavailable]);

  useEffect(() => { void load(1); }, [serviceMode, status]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (sceneMode) setServiceMode(sceneMode);
  }, [sceneMode]);

  const loadTableBoard = useCallback(async () => {
    setTableBoardLoading(true);
    try {
      setTableBoard(await api.get<TableBoardResponse>('/merchant/table-board'));
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setTableBoardLoading(false);
    }
  }, [messageApi]);

  useEffect(() => {
    if (viewMode === 'TABLES' && sceneMode === 'DINE_IN') void loadTableBoard();
  }, [loadTableBoard, sceneMode, viewMode]);

  const openDetail = async (order: Order) => {
    setSelected(order);
    setDrawerOpen(true);
    try {
      const [detail, requests] = await Promise.all([
        api.get<Order>(`/merchant/orders/${order.id}`),
        api.get<Record<string, unknown>[]>(`/merchant/orders/${order.id}/return-requests`),
      ]);
      setSelected(normalizeOrder(detail));
      setReturnRequests(requests.map(normalizeReturnRequest));
    } catch {
      // 列表信息已经足够展示；详情接口失败不阻断现场处理订单。
    }
  };

  const openDetailByID = async (orderID: string | number) => {
    setSelected(null);
    setDrawerOpen(true);
    try {
      const [detail, requests] = await Promise.all([
        api.get<Order>(`/merchant/orders/${orderID}`),
        api.get<Record<string, unknown>[]>(`/merchant/orders/${orderID}/return-requests`),
      ]);
      setSelected(normalizeOrder(detail));
      setReturnRequests(requests.map(normalizeReturnRequest));
    } catch (error) {
      setDrawerOpen(false);
      messageApi.error(errorMessage(error));
    }
  };

  const updateStatus = async (target: OrderStatus) => {
    if (!selected) return;
    setActionLoading(true);
    try {
      const updated = await api.post<Order>(`/merchant/orders/${selected.id}/status`, { status: target });
      const next = Object.keys(updated ?? {}).length ? normalizeOrder(updated) : { ...selected, status: target };
      setSelected(next);
      setResult((current) => ({ ...current, items: current.items.map((order) => order.id === selected.id ? next : order) }));
      messageApi.success(`订单已更新为${orderStatusMap[target].text}`);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setActionLoading(false);
    }
  };

  const reprint = async (copyRole?: 'MERCHANT' | 'CUSTOMER') => {
    if (!selected) return;
    setActionLoading(true);
    try {
      await api.post(`/merchant/orders/${selected.id}/reprint`, { output_type: 'RECEIPT', ...(copyRole ? { copy_role: copyRole } : {}) });
      messageApi.success(copyRole === 'CUSTOMER' ? '客户核对联打印任务已提交' : '补打任务已提交，小票将标记“补打”');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setActionLoading(false);
    }
  };

  const refreshSelected = async (orderID = selected?.id) => {
    if (!orderID) return;
    const [detail, requests] = await Promise.all([
      api.get<Order>(`/merchant/orders/${orderID}`),
      api.get<Record<string, unknown>[]>(`/merchant/orders/${orderID}/return-requests`),
    ]);
    const normalized = normalizeOrder(detail);
    setSelected(normalized);
    setReturnRequests(requests.map(normalizeReturnRequest));
    setResult((current) => ({ ...current, items: current.items.map((order) => order.id === normalized.id ? normalized : order) }));
  };

  const openOperation = async (next: OrderOperation) => {
    if (!selected) return;
    operationForm.resetFields();
    if (next.type === 'SPLIT') operationForm.setFieldsValue({ amount: selected.remainingAmount ?? selected.amount, method: 'CASH' });
    if (next.type === 'RETURN') operationForm.setFieldsValue({ quantity: 1 });
    if (next.type === 'TRANSFER' || next.type === 'MERGE') await loadTableBoard();
    setOperation(next);
  };

  const submitOperation = async () => {
    if (!selected || !operation) return;
    const values = await operationForm.validateFields();
    setActionLoading(true);
    try {
      if (operation.type === 'TRANSFER') {
        await api.post(`/merchant/orders/${selected.id}/transfer-table`, { target_table_id: Number(values.targetTableId), remark: values.remark || '' });
        messageApi.success('转台完成，桌台账单已同步迁移');
      } else if (operation.type === 'MERGE') {
        await api.post(`/merchant/orders/${selected.id}/merge`, { source_order_id: Number(values.sourceOrderId), remark: values.remark || '' });
        messageApi.success('并台完成，商品批次和账单已合并');
      } else if (operation.type === 'SPLIT') {
        await api.postIdempotent(`/merchant/orders/${selected.id}/split-settle`, {
          amount_cents: Math.round(Number(values.amount) * 100),
          method: values.method,
          remark: values.remark || '',
        }, globalThis.crypto?.randomUUID?.() || `split-${Date.now()}-${Math.random().toString(16).slice(2)}`);
        messageApi.success('本次收款已登记');
      } else {
        await api.post(`/merchant/orders/${selected.id}/return-requests`, {
          order_item_id: Number(operation.item.id),
          quantity: Number(values.quantity),
          reason: values.reason,
        });
        messageApi.success('退菜申请已提交，等待店长审批');
      }
      setOperation(null);
      operationForm.resetFields();
      await refreshSelected(selected.id);
      await load(1);
      if (sceneMode === 'DINE_IN') await loadTableBoard();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setActionLoading(false);
    }
  };

  const reviewReturn = async (request: OrderReturnRequest, action: 'APPROVE' | 'REJECT') => {
    if (!selected) return;
    setActionLoading(true);
    try {
      await api.post(`/merchant/order-return-requests/${request.id}/review`, { action, remark: action === 'APPROVE' ? '店长确认退菜' : '店长驳回退菜' });
      messageApi.success(action === 'APPROVE' ? '退菜已批准，库存与订单金额已更新' : '退菜申请已驳回');
      await refreshSelected(selected.id);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setActionLoading(false);
    }
  };

  const settleOffline = (method: 'CASH' | 'EXTERNAL') => {
    if (!selected) return;
    const label = method === 'CASH' ? '现金收款' : '系统外支付';
    Modal.confirm({
      title: `确认已完成${label}？`,
      content: `确认后将把剩余 ${yuan(selected.remainingAmount ?? selected.amount)} 记为已收款，并结束该桌本次账单。请先核对顾客付款结果，此操作会保留审计记录。`,
      okText: '确认结账完毕',
      cancelText: '暂不结账',
      onOk: async () => {
        setActionLoading(true);
        try {
          const updated = normalizeOrder(await api.post<Order>(`/merchant/orders/${selected.id}/settle`, { method, remark: `店员确认${label}` }));
          setSelected(updated);
          setResult((current) => ({ ...current, items: current.items.map((order) => order.id === selected.id ? updated : order) }));
          messageApi.success('已登记收款并完成结账');
          if (viewMode === 'TABLES') await loadTableBoard();
        } catch (error) {
          messageApi.error(errorMessage(error));
          throw error;
        } finally {
          setActionLoading(false);
        }
      },
    });
  };

  const closeOrder = () => {
    if (!selected) return;
    Modal.confirm({
      title: '确认关闭订单？',
      content: '关闭后顾客不能继续付款，此操作会保留审计记录。',
      okText: '确认关闭',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: () => updateStatus('CLOSED'),
    });
  };

  const openPickupDisplay = (layout: 'landscape' | 'portrait') => {
    const url = `/pickup-display?layout=${layout}`;
    const popup = window.open(url, `tanban-pickup-display-${layout}`, 'popup=yes');
    if (!popup) {
      messageApi.error('浏览器阻止了新窗口，请允许本站打开弹窗后重试');
      return;
    }
    popup.opener = null;
    popup.focus();
    void api.post<{ accessToken: string }>('/merchant/cashier/session')
      .then((session) => {
        if (session.accessToken) localStorage.setItem(CASHIER_TOKEN_KEY, session.accessToken);
      })
      .catch(() => {
        messageApi.warning('取餐大屏已打开，将暂时沿用当前后台登录会话');
      });
  };

  const counts = useMemo(() => result.items.reduce<Record<string, number>>((acc, item) => {
    acc[item.status] = (acc[item.status] ?? 0) + 1;
    return acc;
  }, {}), [result.items]);

  const boardTables = useMemo(() => tableBoard?.areas.flatMap((area) => area.tables.map((table) => ({ ...table, areaName: area.name }))) || [], [tableBoard]);
  const transferableTables = boardTables.filter((table) => !table.orderId && String(table.id) !== String(selected?.tableCodeId));
  const mergeableOrders = boardTables.filter((table) => table.orderId && String(table.orderId) !== String(selected?.id));

  const columns = [
    {
      title: isDelivery ? '配送单' : '桌台 / 取餐号', key: 'serviceContext', width: 160,
      render: (_: unknown, order: Order) => order.tableName
        ? <div className="table-context-cell"><strong>{order.tableName}</strong><small>{order.tableAreaName || '店内桌码'}</small></div>
        : <strong className="pickup-no large">#{order.pickupNo || '--'}</strong>,
    },
    {
      title: '场景', key: 'fulfillmentType', width: 100,
      render: (_: unknown, order: Order) => order.orderType === 'DELIVERY'
        ? <Tag color="purple">外卖配送</Tag>
        : order.orderType === 'DINE_IN' ? <Tag color="blue">桌码堂食</Tag> : <Tag color="gold">快餐自取</Tag>,
    },
    { title: '订单号', dataIndex: 'orderNo', width: 190, ellipsis: true },
    {
      title: '商品', key: 'items', minWidth: 240,
      render: (_: unknown, order: Order) => (
        <div className="order-product-cell">
          <Typography.Text>{order.items?.slice(0, 2).map((item) => `${item.productName} ×${item.quantity}`).join('、') || '--'}</Typography.Text>
          {(order.items?.length ?? 0) > 2 && <Typography.Text type="secondary"> 等 {order.items.length} 件</Typography.Text>}
          {order.remark && <div><Tag color="orange">备注：{order.remark}</Tag></div>}
        </div>
      ),
    },
    { title: '实付金额', dataIndex: 'paidAmount', width: 125, render: (value: number, order: Order) => <strong>{yuan(value ?? order.amount)}</strong> },
    { title: '状态', dataIndex: 'status', width: 110, render: (value: OrderStatus) => <OrderStatusTag status={value} /> },
    { title: '下单时间', dataIndex: 'createdAt', width: 180, render: dateTime },
    {
      title: '操作', key: 'action', width: 105, fixed: 'right' as const,
      render: (_: unknown, order: Order) => <Button type="link" icon={<EyeOutlined />} onClick={() => void openDetail(order)}>详情</Button>,
    },
  ];

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading
        title={`${domainName}订单`}
        description={isDelivery
          ? '集中查看配送订单、收货信息和外卖打印记录'
          : sceneMode === 'DINE_IN'
            ? '集中处理桌码堂食订单的支付、制作和桌台状态'
            : sceneMode === 'TAKEOUT'
              ? '集中处理快餐与到店自取订单，按取餐号和码牌安排放餐'
              : '查看店内订单，统一处理支付、制作与出单'}
        extra={sceneMode === 'TAKEOUT' ? (
          <Space wrap>
            <Button type="primary" icon={<DesktopOutlined />} onClick={() => openPickupDisplay('landscape')}>横屏取餐大屏</Button>
            <Button icon={<TabletOutlined />} onClick={() => openPickupDisplay('portrait')}>竖屏取餐大屏</Button>
            <Button icon={<ReloadOutlined />} loading={loading} disabled={unavailable} onClick={() => void load()}>刷新</Button>
          </Space>
        ) : <Button icon={<ReloadOutlined />} loading={loading} disabled={unavailable} onClick={() => void load()}>刷新</Button>}
      />
      {unavailable && <Alert className="order-domain-alert" type="warning" showIcon message={merchantFeatureCopy.DELIVERY.title} description={merchantFeatureCopy.DELIVERY.description} />}
      {sceneMode === 'DINE_IN' && <Card bordered={false} className="content-card order-view-tabs-card">
        <Tabs activeKey={viewMode} onChange={(key) => setViewMode(key as typeof viewMode)} items={[
          { key: 'ORDERS', label: '订单管理' },
          { key: 'TABLES', label: <Space><TableOutlined />桌台状态</Space> },
        ]} />
      </Card>}
      {viewMode === 'ORDERS' && <Card bordered={false} className="content-card order-filter-card">
        <Tabs
          activeKey={status}
          onChange={(key) => setStatus(key as typeof status)}
          items={statusTabs.map((tab) => ({
            key: tab.key,
            disabled: unavailable,
            label: <span>{tab.label}{tab.key !== 'ALL' && counts[tab.key] ? <em className="tab-count">{counts[tab.key]}</em> : null}</span>,
          }))}
        />
        {!isDelivery && !sceneMode && <div className="order-service-mode"><Typography.Text strong>店内场景</Typography.Text><Segmented disabled={unavailable} value={serviceMode} onChange={(value) => setServiceMode(value as typeof serviceMode)} options={[{ label: '桌码堂食', value: 'DINE_IN' }, { label: '快餐 / 到店自取', value: 'TAKEOUT' }]} /></div>}
        <Row gutter={[12, 12]}>
          <Col xs={24} lg={9}>
            <Input
              allowClear
              disabled={unavailable}
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              onPressEnter={() => void load(1)}
              prefix={<SearchOutlined />}
              placeholder={isDelivery ? '搜索订单号、收货人或手机号' : '搜索订单号、桌号、取餐号或顾客手机号'}
            />
          </Col>
          <Col xs={24} sm={16} lg={9}><RangePicker format="YYYY-MM-DD" disabled={unavailable} value={dates} onChange={(value) => setDates(value)} style={{ width: '100%' }} /></Col>
          <Col xs={24} sm={8} lg={6}><Button type="primary" block disabled={unavailable} icon={<SearchOutlined />} onClick={() => void load(1)}>查询订单</Button></Col>
        </Row>
      </Card>}
      {viewMode === 'ORDERS' && <Card bordered={false} className="content-card table-card">
        {isDelivery ? (
          <Table<Order>
            rowKey="id"
            loading={loading}
            dataSource={result.items}
            columns={columns}
            scroll={{ x: 1050 }}
            locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={unavailable ? '外卖能力开放后，配送订单将在此展示' : `没有符合条件的${domainName}订单`} /> }}
            pagination={{
              current: result.meta.page,
              pageSize: result.meta.pageSize,
              total: result.meta.total,
              showSizeChanger: true,
              showTotal: (total) => `共 ${total} 笔订单`,
            }}
            onChange={(pagination: TablePaginationConfig) => void load(pagination.current, pagination.pageSize)}
          />
        ) : (
          <div className="order-workboard" aria-busy={loading}>
            {loading && !result.items.length ? <div className="order-workboard-empty"><SyncOutlined spin /> 正在刷新现场订单</div> : result.items.length ? (
              <>
                <div className="order-work-grid">
                  {result.items.map((order) => <OrderWorkCard key={order.id} order={order} onOpen={(item) => void openDetail(item)} />)}
                </div>
                <div className="order-work-pagination">
                  <Pagination
                    current={result.meta.page}
                    pageSize={result.meta.pageSize}
                    total={result.meta.total}
                    showSizeChanger
                    showTotal={(total) => `共 ${total} 笔订单`}
                    onChange={(page, pageSize) => void load(page, pageSize)}
                  />
                </div>
              </>
            ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={`没有符合条件的${serviceMode === 'DINE_IN' ? '堂食' : '快餐'}订单`} />}
          </div>
        )}
      </Card>}
      {viewMode === 'TABLES' && sceneMode === 'DINE_IN' && <TableBoard board={tableBoard} loading={tableBoardLoading} onRefresh={() => void loadTableBoard()} onOpenOrder={(orderId) => void openDetailByID(orderId)} />}

      <Drawer
        title={selected ? <Space><span>订单详情</span><OrderStatusTag status={selected.status} /></Space> : '订单详情'}
        width={680}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        extra={<Space><Button icon={<PrinterOutlined />} loading={actionLoading} onClick={() => void reprint('CUSTOMER')}>打印客户核对联</Button><Button loading={actionLoading} onClick={() => void reprint()}>补打全部</Button></Space>}
        footer={selected && (
          <div className="drawer-footer-actions">
            {selected.status === 'PENDING_PAYMENT' && <Button danger icon={<CloseCircleOutlined />} onClick={closeOrder}>关闭订单</Button>}
            {selected.settlementMode === 'PAY_AFTER' && selected.paymentStatus === 'UNPAID' && <>
              {selected.orderType === 'DINE_IN' && Number(selected.paidAmount || 0) === 0 && <Button loading={actionLoading} onClick={() => void openOperation({ type: 'TRANSFER' })}>转台</Button>}
              {selected.orderType === 'DINE_IN' && Number(selected.paidAmount || 0) === 0 && <Button loading={actionLoading} onClick={() => void openOperation({ type: 'MERGE' })}>并台</Button>}
              {selected.orderType === 'DINE_IN' && <Button loading={actionLoading} onClick={() => void openOperation({ type: 'SPLIT' })}>拆分收款</Button>}
              <Button size="large" loading={actionLoading} onClick={() => settleOffline('EXTERNAL')}>确认系统外支付</Button>
              <Button type="primary" size="large" loading={actionLoading} onClick={() => settleOffline('CASH')}>现金结账完毕</Button>
            </>}
            {nextStatus[selected.status] && !(nextStatus[selected.status]?.status === 'COMPLETED' && selected.settlementMode === 'PAY_AFTER' && selected.paymentStatus === 'UNPAID') && (
              <Button type="primary" size="large" loading={actionLoading} icon={<SyncOutlined />} onClick={() => void updateStatus(nextStatus[selected.status]!.status)}>
                {nextStatus[selected.status]!.text}
              </Button>
            )}
          </div>
        )}
      >
        {selected ? (
          <div className="order-detail">
            <div className="detail-hero">
              <span>{selected.tableName ? '当前桌台' : isDelivery ? '配送单' : '取餐号'}</span><strong>{selected.tableName || `#${selected.pickupNo || '--'}`}</strong>
              <small>{selected.orderNo}</small>
            </div>
            {selected.status !== 'CLOSED' ? (
              <Steps
                size="small"
                current={Math.max(timelineStatuses.indexOf(selected.status), 0)}
                items={timelineStatuses.map((item) => ({ title: orderStatusMap[item].text }))}
              />
            ) : <Tag color="error">该订单已关闭</Tag>}
            <Divider />
            <Typography.Title level={5}>商品明细</Typography.Title>
            <List
              dataSource={selected.items ?? []}
              locale={{ emptyText: '暂无商品明细' }}
              renderItem={(item) => (
                <List.Item extra={<Space direction="vertical" align="end"><strong>{yuan(item.amount ?? item.unitPrice * item.quantity)}</strong>{selected.settlementMode === 'PAY_AFTER' && selected.paymentStatus === 'UNPAID' && Number(selected.paidAmount || 0) === 0 && item.id && item.quantity > 0 ? <Button size="small" danger onClick={() => void openOperation({ type: 'RETURN', item })}>申请退菜</Button> : null}</Space>}>
                  <List.Item.Meta
                    title={<Space>{item.productName}<Typography.Text type="secondary">× {item.quantity}</Typography.Text>{Number(item.additionSequence) > 1 && <Tag color="orange">加菜 #{Number(item.additionSequence) - 1}</Tag>}</Space>}
                    description={[item.skuName, ...itemConfigurationSummary(item), item.remark, item.memberDiscount ? `${item.memberLevelName || '会员'}优惠 ${yuan(item.memberDiscount)}` : ''].filter(Boolean).join(' · ') || `单价 ${yuan(item.unitPrice)}`}
                  />
                </List.Item>
              )}
            />
            <div className="order-total-row"><span>订单金额</span><strong>{yuan(selected.amount)}</strong></div>
            {(selected.paidAmount ?? 0) > 0 && selected.paymentStatus === 'UNPAID' && <div className="order-total-row"><span>已收金额</span><strong>{yuan(selected.paidAmount)}</strong></div>}
            {selected.paymentStatus === 'UNPAID' && <div className="order-total-row"><span>待收金额</span><strong>{yuan(selected.remainingAmount ?? selected.amount)}</strong></div>}
            {(selected.memberDiscount ?? 0) > 0 && <div className="order-total-row refund"><span>{selected.memberLevelName || '会员'}优惠</span><strong>-{yuan(selected.memberDiscount)}</strong></div>}
            {(selected.refundAmount ?? 0) > 0 && <div className="order-total-row refund"><span>已退款</span><strong>-{yuan(selected.refundAmount)}</strong></div>}
            {returnRequests.length > 0 && <>
              <Divider />
              <Typography.Title level={5}>退菜记录</Typography.Title>
              <List
                dataSource={returnRequests}
                renderItem={(request) => (
                  <List.Item actions={request.status === 'PENDING' && canReviewReturns ? [
                    <Button key="reject" size="small" onClick={() => void reviewReturn(request, 'REJECT')}>驳回</Button>,
                    <Button key="approve" size="small" type="primary" danger onClick={() => void reviewReturn(request, 'APPROVE')}>批准退菜</Button>,
                  ] : undefined}>
                    <List.Item.Meta
                      title={<Space>{request.productName} × {request.quantity}<Tag color={request.status === 'APPROVED' ? 'success' : request.status === 'REJECTED' ? 'default' : 'processing'}>{request.status === 'APPROVED' ? '已批准' : request.status === 'REJECTED' ? '已驳回' : '待审批'}</Tag></Space>}
                      description={`${request.reason} · ${dateTime(request.createdAt)}${request.reviewRemark ? ` · ${request.reviewRemark}` : ''}`}
                    />
                    <strong>{yuan(request.amount)}</strong>
                  </List.Item>
                )}
              />
            </>}
            <Divider />
            <Descriptions title="订单信息" column={1} size="small">
              <Descriptions.Item label="下单时间">{dateTime(selected.createdAt)}</Descriptions.Item>
              <Descriptions.Item label="支付时间">{dateTime(selected.paidAt)}</Descriptions.Item>
              <Descriptions.Item label="支付方式">{selected.paymentMethod || '--'}</Descriptions.Item>
              <Descriptions.Item label="结算模式">{selected.settlementMode === 'PAY_AFTER' ? '先用餐后结账' : '先结账后用餐'}</Descriptions.Item>
              {selected.settlementMode === 'PAY_AFTER' && <Descriptions.Item label="结账状态">{selected.paymentStatus === 'UNPAID' ? <Tag color="error">待结账</Tag> : <Tag color="success">已结账</Tag>}</Descriptions.Item>}
              <Descriptions.Item label="订单类型">{selected.businessType === 'DELIVERY' ? '外卖订单' : '店内订单'}</Descriptions.Item>
              <Descriptions.Item label="取餐方式">{selected.orderType === 'DELIVERY' ? '外卖配送' : selected.orderType === 'DINE_IN' ? '桌码堂食' : '快餐 / 到店自取'}</Descriptions.Item>
              {selected.tableName && <Descriptions.Item label="桌台">{[selected.tableAreaName, selected.tableName].filter(Boolean).join(' · ')}</Descriptions.Item>}
              {selected.dinerCount ? <Descriptions.Item label="就餐人数">{selected.dinerCount} 人</Descriptions.Item> : null}
              {Number(selected.additionCount) > 1 ? <Descriptions.Item label="加菜次数">{Number(selected.additionCount) - 1} 次</Descriptions.Item> : null}
              {selected.fastFoodPlateName && <Descriptions.Item label="快餐码牌">{[selected.fastFoodPlateCode, selected.fastFoodPlateName].filter(Boolean).join(' · ')}</Descriptions.Item>}
              <Descriptions.Item label="顾客">{selected.customerName || '微信顾客'} {selected.customerPhone}</Descriptions.Item>
              <Descriptions.Item label="订单备注">{selected.remark || '无'}</Descriptions.Item>
              <Descriptions.Item label="打印次数">{selected.printCount ?? 0} 次</Descriptions.Item>
            </Descriptions>
          </div>
        ) : <Empty />}
      </Drawer>
      <Modal
        title={operation?.type === 'TRANSFER' ? '转移桌台' : operation?.type === 'MERGE' ? '合并桌台账单' : operation?.type === 'SPLIT' ? '拆分收款' : operation?.type === 'RETURN' ? `申请退菜：${operation.item.productName}` : ''}
        open={!!operation}
        onCancel={() => { setOperation(null); operationForm.resetFields(); }}
        onOk={() => void submitOperation()}
        confirmLoading={actionLoading}
        okText={operation?.type === 'RETURN' ? '提交审批' : '确认'}
      >
        <Form form={operationForm} layout="vertical">
          {operation?.type === 'TRANSFER' && <Form.Item label="目标空闲桌台" name="targetTableId" rules={[{ required: true, message: '请选择目标桌台' }]}>
            <Select
              showSearch
              optionFilterProp="label"
              placeholder={tableBoardLoading ? '正在加载桌台' : '选择空闲桌台'}
              options={transferableTables.map((table) => ({ value: table.id, label: `${table.areaName} · ${table.name}（${table.tableCode}）` }))}
            />
          </Form.Item>}
          {operation?.type === 'MERGE' && <Form.Item label="并入当前账单的桌台" name="sourceOrderId" rules={[{ required: true, message: '请选择要合并的桌台账单' }]}>
            <Select
              showSearch
              optionFilterProp="label"
              placeholder={tableBoardLoading ? '正在加载桌台' : '选择已有账单'}
              options={mergeableOrders.map((table) => ({ value: table.orderId, label: `${table.areaName} · ${table.name}（${table.orderNo || '进行中'}）` }))}
            />
          </Form.Item>}
          {operation?.type === 'SPLIT' && <>
            <Alert type="info" showIcon message={`当前待收 ${yuan(selected?.remainingAmount ?? selected?.amount ?? 0)}`} description="可分多次登记现金或系统外支付；收足后订单会自动结账。" style={{ marginBottom: 16 }} />
            <Form.Item label="本次收款金额" name="amount" rules={[{ required: true, message: '请输入收款金额' }, { type: 'number', min: 0.01, max: selected?.remainingAmount ?? selected?.amount, message: '金额应大于 0 且不超过待收金额' }]}>
              <InputNumber min={0.01} max={selected?.remainingAmount ?? selected?.amount} precision={2} prefix="¥" style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item label="收款方式" name="method" rules={[{ required: true, message: '请选择收款方式' }]}>
              <Select options={[{ value: 'CASH', label: '现金' }, { value: 'EXTERNAL', label: '系统外支付' }]} />
            </Form.Item>
          </>}
          {operation?.type === 'RETURN' && <>
            <Form.Item label="退菜数量" name="quantity" rules={[{ required: true, message: '请输入退菜数量' }, { type: 'number', min: 1, max: operation.item.quantity, message: `最多可退 ${operation.item.quantity} 份` }]}>
              <InputNumber min={1} max={operation.item.quantity} precision={0} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item label="退菜原因" name="reason" rules={[{ required: true, message: '请填写退菜原因' }]}>
              <Input.TextArea rows={3} maxLength={255} showCount placeholder="例如：顾客点错、菜品未制作" />
            </Form.Item>
          </>}
          {operation && operation.type !== 'RETURN' && <Form.Item label="操作备注（可选）" name="remark">
            <Input.TextArea rows={2} maxLength={255} showCount />
          </Form.Item>}
        </Form>
      </Modal>
    </div>
  );
}

const tableStateMeta: Record<TableBoardTable['state'], { label: string; color: string; hint: string }> = {
  UNOPENED: { label: '未开台', color: '#a5a5a5', hint: '当前没有活动订单' },
  PENDING_PAYMENT: { label: '待付款', color: '#fa8c16', hint: '先付订单尚未完成收款' },
  SETTLED: { label: '已结账', color: '#52a378', hint: '已收款，等待制作或接单' },
  DINING: { label: '就餐中', color: '#faad14', hint: '商户已开始制作或待出餐' },
  UNSETTLED: { label: '待结账', color: '#ff4d4f', hint: '用餐账单尚未完成收款' },
};

function TableBoard({ board, loading, onRefresh, onOpenOrder }: { board: TableBoardResponse | null; loading: boolean; onRefresh: () => void; onOpenOrder: (orderId: string | number) => void }) {
  return (
    <Card bordered={false} className="content-card dine-in-table-board" loading={loading}>
      <div className="table-board-intro">
        <div>
          <Typography.Title level={5}>桌台现场</Typography.Title>
          <Typography.Paragraph type="secondary">未开台表示空闲；待付款与已结账分别展示先付订单的收款前后状态；就餐中表示正在制作或出餐；待结账表示后付账订单尚未完成收款。</Typography.Paragraph>
        </div>
        <Button icon={<ReloadOutlined />} loading={loading} onClick={onRefresh}>刷新桌台</Button>
      </div>
      {board?.areas?.length ? board.areas.map((area) => (
        <section className="table-board-area" key={String(area.id)}>
          <div className="table-board-area-title"><strong>{area.name}</strong><span>{area.tables.length} 桌</span></div>
          <div className="table-board-grid">
            {area.tables.map((table) => {
              const meta = tableStateMeta[table.state];
              return (
                <button className={`table-status-card state-${table.state.toLowerCase()}`} type="button" key={String(table.id)} onClick={() => table.orderId && onOpenOrder(table.orderId)} disabled={!table.orderId}>
                  <div className="table-status-card-head"><strong>{table.name}</strong><span>{table.tableCode}</span></div>
                  <div className="table-status-main" style={{ color: meta.color }}><TableOutlined /><b>{meta.label}</b></div>
                  {table.orderNo ? <div className="table-status-order"><span>{table.orderNo}</span><strong>{yuan((table.totalCents || 0) / 100)}</strong></div> : <small>{meta.hint}</small>}
                  <div className="table-status-capacity">{table.dinerCount ? `${table.dinerCount} 人就餐` : `${table.capacity} 座`}{Number(table.additionCount) > 1 ? ` · 加菜 ${Number(table.additionCount) - 1} 次` : ''}</div>
                </button>
              );
            })}
          </div>
        </section>
      )) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="尚未配置可用桌台，请先到“店内 → 桌码管理”创建区域和桌码" />}
      <div className="table-board-footer">
        <div className="table-board-legend">{Object.entries(tableStateMeta).map(([key, value]) => <span key={key}><i style={{ background: value.color }} />{value.label}</span>)}</div>
        <div className="table-board-modes"><span>结算模式：<b>{board?.settlementMode === 'PAY_AFTER' ? '先用餐后结账' : '先结账后用餐'}</b></span><span>点餐模式：<b>{board?.orderingMode === 'MULTI_PERSON' ? '多人点餐' : '单人点餐'}</b></span></div>
      </div>
    </Card>
  );
}
