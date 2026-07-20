import {
  ClockCircleOutlined,
  CloseCircleOutlined,
  CoffeeOutlined,
  EyeOutlined,
  NumberOutlined,
  PrinterOutlined,
  ReloadOutlined,
  SearchOutlined,
  SyncOutlined,
  TableOutlined,
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
  Input,
  List,
  Modal,
  Pagination,
  Row,
  Segmented,
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
import { api, errorMessage } from '../api/client';
import { OrderStatusTag, orderStatusMap } from '../components/OrderStatusTag';
import { PageHeading } from '../components/PageHeading';
import { normalizeOrder } from '../features/storefront/model';
import type { ListResult, Order, OrderBusinessType, OrderStatus } from '../types';
import { dateTime, yuan } from '../utils/format';

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
        <OrderStatusTag status={order.status} />
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
        <strong>{yuan(order.paidAmount ?? order.amount)}</strong>
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

export function OrdersPage({ businessType = 'DINE_IN', unavailable = false, sceneMode }: { businessType?: OrderBusinessType; unavailable?: boolean; sceneMode?: 'DINE_IN' | 'TAKEOUT' }) {
  const [status, setStatus] = useState<'ALL' | OrderStatus>('ALL');
  const [serviceMode, setServiceMode] = useState<'DINE_IN' | 'TAKEOUT'>(sceneMode || 'DINE_IN');
  const [keyword, setKeyword] = useState('');
  const [dates, setDates] = useState<[Dayjs | null, Dayjs | null] | null>(null);
  const [result, setResult] = useState<ListResult<Order>>({ items: [], meta: { page: 1, pageSize: 20, total: 0 } });
  const [loading, setLoading] = useState(false);
  const [selected, setSelected] = useState<Order | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
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
        startAt: dates?.[0]?.startOf('day').toISOString(),
        endAt: dates?.[1]?.endOf('day').toISOString(),
        order_type: isDelivery ? 'DELIVERY' : serviceMode,
        page,
        page_size: pageSize,
      });
      const items = normalized.items.map(normalizeOrder);
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

  const openDetail = async (order: Order) => {
    setSelected(order);
    setDrawerOpen(true);
    try {
      setSelected(normalizeOrder(await api.get<Order>(`/merchant/orders/${order.id}`)));
    } catch {
      // 列表信息已经足够展示；详情接口失败不阻断现场处理订单。
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

  const reprint = async () => {
    if (!selected) return;
    setActionLoading(true);
    try {
      await api.post(`/merchant/orders/${selected.id}/reprint`, { type: 'RECEIPT', business_type: businessType, markAsReprint: true });
      messageApi.success('补打任务已提交，小票将标记“补打”');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setActionLoading(false);
    }
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

  const counts = useMemo(() => result.items.reduce<Record<string, number>>((acc, item) => {
    acc[item.status] = (acc[item.status] ?? 0) + 1;
    return acc;
  }, {}), [result.items]);

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
        description={isDelivery ? '独立承载配送订单、收货信息和外卖打印链路' : '查看桌码堂食和门店自取订单，统一处理支付、制作与出单'}
        extra={<Button icon={<ReloadOutlined />} loading={loading} disabled={unavailable} onClick={() => void load()}>刷新</Button>}
      />
      {unavailable && <Alert className="order-domain-alert" type="warning" showIcon message="外卖订单一期未开放" description="系统已经将外卖识别为独立经营域，但尚未接入配送地址、配送范围、运费、骑手或第三方外卖平台。当前不会创建外卖订单，也不会把店内订单误显示到这里。" />}
      <Card bordered={false} className="content-card order-filter-card">
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
          <Col xs={24} sm={16} lg={9}><RangePicker disabled={unavailable} value={dates} onChange={(value) => setDates(value)} style={{ width: '100%' }} /></Col>
          <Col xs={24} sm={8} lg={6}><Button type="primary" block disabled={unavailable} icon={<SearchOutlined />} onClick={() => void load(1)}>查询订单</Button></Col>
        </Row>
      </Card>
      <Card bordered={false} className="content-card table-card">
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
      </Card>

      <Drawer
        title={selected ? <Space><span>订单详情</span><OrderStatusTag status={selected.status} /></Space> : '订单详情'}
        width={680}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        extra={<Button icon={<PrinterOutlined />} loading={actionLoading} onClick={() => void reprint()}>补打小票</Button>}
        footer={selected && (
          <div className="drawer-footer-actions">
            {selected.status === 'PENDING_PAYMENT' && <Button danger icon={<CloseCircleOutlined />} onClick={closeOrder}>关闭订单</Button>}
            {nextStatus[selected.status] && (
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
                <List.Item extra={<strong>{yuan(item.amount ?? item.unitPrice * item.quantity)}</strong>}>
                  <List.Item.Meta
                    title={<Space>{item.productName}<Typography.Text type="secondary">× {item.quantity}</Typography.Text></Space>}
                    description={[item.skuName, ...itemConfigurationSummary(item), item.remark].filter(Boolean).join(' · ') || `单价 ${yuan(item.unitPrice)}`}
                  />
                </List.Item>
              )}
            />
            <div className="order-total-row"><span>订单金额</span><strong>{yuan(selected.amount)}</strong></div>
            {(selected.refundAmount ?? 0) > 0 && <div className="order-total-row refund"><span>已退款</span><strong>-{yuan(selected.refundAmount)}</strong></div>}
            <Divider />
            <Descriptions title="订单信息" column={1} size="small">
              <Descriptions.Item label="下单时间">{dateTime(selected.createdAt)}</Descriptions.Item>
              <Descriptions.Item label="支付时间">{dateTime(selected.paidAt)}</Descriptions.Item>
              <Descriptions.Item label="支付方式">{selected.paymentMethod || '--'}</Descriptions.Item>
              <Descriptions.Item label="订单类型">{selected.businessType === 'DELIVERY' ? '外卖订单' : '店内订单'}</Descriptions.Item>
              <Descriptions.Item label="取餐方式">{selected.orderType === 'DELIVERY' ? '外卖配送' : selected.orderType === 'DINE_IN' ? '桌码堂食' : '快餐 / 到店自取'}</Descriptions.Item>
              {selected.tableName && <Descriptions.Item label="桌台">{[selected.tableAreaName, selected.tableName].filter(Boolean).join(' · ')}</Descriptions.Item>}
              {selected.fastFoodPlateName && <Descriptions.Item label="快餐码牌">{[selected.fastFoodPlateCode, selected.fastFoodPlateName].filter(Boolean).join(' · ')}</Descriptions.Item>}
              <Descriptions.Item label="顾客">{selected.customerName || '微信顾客'} {selected.customerPhone}</Descriptions.Item>
              <Descriptions.Item label="订单备注">{selected.remark || '无'}</Descriptions.Item>
              <Descriptions.Item label="打印次数">{selected.printCount ?? 0} 次</Descriptions.Item>
            </Descriptions>
          </div>
        ) : <Empty />}
      </Drawer>
    </div>
  );
}
