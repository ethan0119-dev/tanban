import {
  ArrowDownOutlined,
  ArrowRightOutlined,
  ArrowUpOutlined,
  ClockCircleOutlined,
  CoffeeOutlined,
  DollarOutlined,
  ReloadOutlined,
  ShoppingOutlined,
} from '@ant-design/icons';
import { Button, Card, Col, Empty, List, Progress, Row, Skeleton, Space, Statistic, Table, Typography, message } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { api, errorMessage } from '../api/client';
import { useAuth } from '../auth/AuthContext';
import { canViewMerchantFinancials, isMerchantStaff } from '../auth/permissions';
import { OrderStatusTag } from '../components/OrderStatusTag';
import { PageHeading } from '../components/PageHeading';
import type { DashboardData, Order } from '../types';
import { beijingNowDateTime, dateTime, percentChange, yuan } from '../utils/format';

function normalizeDashboard(raw: unknown): DashboardData {
  const value = (raw ?? {}) as Record<string, unknown>;
  return {
    todayRevenue: value.today_revenue_cents !== undefined
      ? Number(value.today_revenue_cents) / 100
      : Number(value.todayRevenue ?? value.today_revenue ?? value.revenue ?? 0),
    todayOrders: Number(value.todayOrders ?? value.today_orders ?? value.orderCount ?? 0),
    pendingOrders: Number(value.pendingOrders ?? value.pending_orders ?? value.pendingCount ?? value.active_orders ?? 0),
    averageOrderValue: Number(value.averageOrderValue ?? value.average_order_value ?? value.avgOrderAmount ?? 0),
    yesterdayRevenue: Number(value.yesterdayRevenue ?? value.yesterday_revenue ?? 0),
    refundAmount: value.today_refunded_cents !== undefined ? Number(value.today_refunded_cents) / 100 : Number(value.refundAmount ?? value.refund_amount ?? 0),
    revenueTrend: (value.revenueTrend ?? value.revenue_trend ?? []) as DashboardData['revenueTrend'],
    popularProducts: (value.popularProducts ?? value.popular_products ?? []) as DashboardData['popularProducts'],
    recentOrders: (value.recentOrders ?? value.recent_orders ?? []) as Order[],
  };
}

export function DashboardPage() {
  const { user } = useAuth();
  const staffView = isMerchantStaff(user);
  const financialView = canViewMerchantFinancials(user);
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [messageApi, contextHolder] = message.useMessage();
  const load = useCallback(async () => {
    setLoading(true);
    try {
      setData(normalizeDashboard(await api.get<DashboardData>('/merchant/dashboard')));
    } catch (error) {
      messageApi.error(errorMessage(error));
      setData(normalizeDashboard({}));
    } finally {
      setLoading(false);
    }
  }, [messageApi]);

  useEffect(() => { void load(); }, [load]);
  const change = percentChange(data?.todayRevenue, data?.yesterdayRevenue);
  const maxTrend = Math.max(...(data?.revenueTrend?.map((item) => item.value) ?? [1]), 1);
  const maxPopular = Math.max(...(data?.popularProducts?.map((item) => item.count) ?? [1]), 1);
  const greeting = useMemo(() => {
    const hour = Number(beijingNowDateTime().slice(11, 13));
    if (hour < 11) return '早上好，准备开始今天的营业吧';
    if (hour < 18) return '下午好，门店正在稳定运转';
    return '晚上好，夜市的高峰要来啦';
  }, []);

  if (loading && !data) return <div className="page-shell"><Skeleton active paragraph={{ rows: 12 }} /></div>;

  const allMetrics = [
    { title: '今日营业额', value: data?.todayRevenue ?? 0, prefix: '¥', precision: 2, icon: <DollarOutlined />, tone: 'brown' },
    { title: '今日订单', value: data?.todayOrders ?? 0, suffix: '单', icon: <ShoppingOutlined />, tone: 'orange' },
    { title: '待处理订单', value: data?.pendingOrders ?? 0, suffix: '单', icon: <ClockCircleOutlined />, tone: 'blue' },
    { title: '平均客单价', value: data?.averageOrderValue ?? 0, prefix: '¥', precision: 2, icon: <CoffeeOutlined />, tone: 'green' },
  ];
  const metrics = financialView
    ? allMetrics
    : allMetrics.filter((item) => item.title === '今日订单' || item.title === '待处理订单');

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading
        title={greeting}
        description={`今日数据实时更新 · 最近刷新 ${beijingNowDateTime()}`}
        extra={<Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>刷新数据</Button>}
      />
      <Row gutter={[16, 16]}>
        {metrics.map((item) => (
          <Col xs={24} sm={12} xl={financialView ? 6 : 12} key={item.title}>
            <Card className="metric-card" bordered={false}>
              <div className={`metric-icon ${item.tone}`}>{item.icon}</div>
              <Statistic title={item.title} value={item.value} prefix={item.prefix} suffix={item.suffix} precision={item.precision} />
              <div className="metric-foot">
                {item.title === '今日营业额' && change !== null ? (
                  <span className={change >= 0 ? 'trend-up' : 'trend-down'}>
                    {change >= 0 ? <ArrowUpOutlined /> : <ArrowDownOutlined />} {Math.abs(change)}% 较昨日
                  </span>
                ) : item.title === '待处理订单' ? <span>及时处理，减少顾客等待</span> : <span>数据来自当前门店</span>}
              </div>
            </Card>
          </Col>
        ))}
      </Row>

      <Row gutter={[16, 16]} className="dashboard-grid">
        {financialView && (
          <Col xs={24} xl={15}>
            <Card title="近 7 日营业趋势" bordered={false} className="content-card">
              {data?.revenueTrend?.length ? (
                <div className="trend-chart" aria-label="近 7 日营业趋势">
                  {data.revenueTrend.map((item) => (
                    <div className="trend-column" key={item.label}>
                      <Typography.Text className="trend-value">{yuan(item.value)}</Typography.Text>
                      <div className="trend-track"><span style={{ height: `${Math.max(item.value / maxTrend * 100, 5)}%` }} /></div>
                      <Typography.Text type="secondary">{item.label}</Typography.Text>
                    </div>
                  ))}
                </div>
              ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="营业后将在这里看到趋势" />}
            </Card>
          </Col>
        )}
        <Col xs={24} xl={financialView ? 9 : 24}>
          <Card title="热销商品" bordered={false} className="content-card" extra={!staffView ? <Link to="/products">商品管理 <ArrowRightOutlined /></Link> : undefined}>
            {data?.popularProducts?.length ? (
              <List
                dataSource={data.popularProducts}
                renderItem={(item, index) => (
                  <List.Item className="popular-item">
                    <span className={`popular-rank rank-${index + 1}`}>{index + 1}</span>
                    <div className="popular-main">
                      <Space><Typography.Text strong>{item.name}</Typography.Text><Typography.Text type="secondary">{item.count} 份</Typography.Text></Space>
                      <Progress percent={Math.round(item.count / maxPopular * 100)} showInfo={false} strokeColor="#c98348" trailColor="#f2ebe5" size="small" />
                    </div>
                  </List.Item>
                )}
              />
            ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无热销商品数据" />}
          </Card>
        </Col>
      </Row>

      <Card
        title="最近订单"
        bordered={false}
        className="content-card recent-orders"
        extra={<Link to="/orders">查看全部 <ArrowRightOutlined /></Link>}
      >
        <Table<Order>
          rowKey="id"
          size="middle"
          pagination={false}
          locale={{ emptyText: '暂无订单' }}
          scroll={{ x: 720 }}
          dataSource={data?.recentOrders ?? []}
          columns={[
            { title: '取餐号', dataIndex: 'pickupNo', width: 110, render: (value) => <strong className="pickup-no">#{value || '--'}</strong> },
            { title: '订单号', dataIndex: 'orderNo', ellipsis: true },
            { title: '商品', key: 'items', render: (_, order) => order.items?.map((item) => `${item.productName} ×${item.quantity}`).join('、') || '--', ellipsis: true },
            ...(financialView ? [{ title: '金额', dataIndex: 'amount', width: 120, render: yuan }] : []),
            { title: '状态', dataIndex: 'status', width: 110, render: (status) => <OrderStatusTag status={status} /> },
            { title: '下单时间', dataIndex: 'createdAt', width: 180, render: dateTime },
          ]}
        />
      </Card>
    </div>
  );
}
