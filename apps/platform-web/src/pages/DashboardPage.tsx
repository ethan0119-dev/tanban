import {
  ArrowRightOutlined,
  DollarOutlined,
  ReloadOutlined,
  ShopOutlined,
  ShoppingCartOutlined,
  TeamOutlined,
} from '@ant-design/icons';
import { Button, Card, Col, Row, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { LoadError, PageSkeleton } from '../components/AsyncState';
import { MetricCard } from '../components/MetricCard';
import { PageHeader } from '../components/PageHeader';
import { StatusTag } from '../components/StatusTag';
import { TrendChart } from '../components/TrendChart';
import { dashboardService } from '../lib/services';
import type { DashboardData, Tenant, TrendPoint } from '../types';
import { formatBeijingDateTime } from '../utils/datetime';

const defaultTrend: TrendPoint[] = Array.from({ length: 7 }, (_, index) => ({
  date: `--${String(index + 1).padStart(2, '0')}`,
  orders: 0,
}));

export function DashboardPage() {
  const [data, setData] = useState<DashboardData>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const navigate = useNavigate();

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setData(await dashboardService.get());
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '无法获取经营数据');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  if (loading && !data) return <PageSkeleton />;

  const columns: ColumnsType<Tenant> = [
    { title: '商户', dataIndex: 'name', key: 'name', render: (value, row) => <div className="entity-name"><span>{String(value).slice(0, 1)}</span><div><strong>{value}</strong><small>{row.code || '—'}</small></div></div> },
    { title: '联系人', dataIndex: 'contactName', key: 'contactName', render: (value, row) => <div>{value || '—'}<small className="table-subtext">{row.contactPhone || ''}</small></div> },
    { title: '点单码', dataIndex: 'storeCode', key: 'storeCode', render: (value) => value || '—' },
    { title: '状态', dataIndex: 'status', key: 'status', render: (value) => <StatusTag status={value} /> },
    { title: '创建时间', dataIndex: 'createdAt', key: 'createdAt', render: formatBeijingDateTime },
  ];

  return (
    <div>
      <PageHeader
        title="经营总览"
        description="掌握平台核心经营指标和最新商户动态。"
        extra={<Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>刷新数据</Button>}
      />
      {error && <div className="section-gap"><LoadError message={error} onRetry={() => void load()} /></div>}
      <Row gutter={[16, 16]} className="metric-grid">
        <Col xs={24} sm={12} xl={6}><MetricCard title="平台商户" value={data?.tenantCount || 0} suffix="家" prefix={<TeamOutlined />} trend={data?.metrics?.[0]?.trend} /></Col>
        <Col xs={24} sm={12} xl={6}><MetricCard title="正常商户" value={data?.activeTenantCount || 0} suffix="家" prefix={<ShopOutlined />} accent="blue" trend={data?.metrics?.[1]?.trend} /></Col>
        <Col xs={24} sm={12} xl={6}><MetricCard title="今日订单" value={data?.todayOrderCount || 0} suffix="单" prefix={<ShoppingCartOutlined />} accent="green" trend={data?.metrics?.[2]?.trend} /></Col>
        <Col xs={24} sm={12} xl={6}><MetricCard title="今日交易额" value={data?.todayTransactionAmount || 0} suffix="元" prefix={<DollarOutlined />} accent="purple" trend={data?.metrics?.[3]?.trend} /></Col>
      </Row>

      <Row gutter={[16, 16]} className="section-gap">
        <Col xs={24} xl={17}>
          <Card bordered={false} title="近 7 日订单趋势" extra={<Typography.Text type="secondary">平台全部商户</Typography.Text>}>
            <TrendChart data={data?.trend?.length ? data.trend : defaultTrend} />
          </Card>
        </Col>
        <Col xs={24} xl={7}>
          <Card bordered={false} title="运营状态" className="operation-card">
            <div className="operation-stat"><span>正常商户</span><strong>{data?.activeTenantCount || 0}</strong><Tag color="success">运营中</Tag></div>
            <div className="operation-stat"><span>本月交易额</span><strong>¥ {(data?.monthTransactionAmount || 0).toLocaleString('zh-CN', { minimumFractionDigits: 2 })}</strong></div>
            <div className="operation-stat"><span>平台可用性</span><strong>正常</strong><Tag color="success">在线</Tag></div>
            <Button type="link" onClick={() => navigate('/tenants')}>查看全部商户 <ArrowRightOutlined /></Button>
          </Card>
        </Col>
      </Row>

      <Card
        bordered={false}
        className="section-gap"
        title="最近入驻商户"
        extra={<Button type="link" onClick={() => navigate('/tenants')}>商户管理 <ArrowRightOutlined /></Button>}
      >
        <Table<Tenant>
          rowKey="id"
          columns={columns}
          dataSource={data?.recentTenants || []}
          pagination={false}
          scroll={{ x: 760 }}
          locale={{ emptyText: '暂无新入驻商户' }}
        />
      </Card>
    </div>
  );
}
