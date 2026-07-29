import {
  AppstoreOutlined,
  BankOutlined,
  CalendarOutlined,
  CarOutlined,
  CoffeeOutlined,
  CrownOutlined,
  GiftOutlined,
  HeartOutlined,
  NotificationOutlined,
  PercentageOutlined,
  QrcodeOutlined,
  ReloadOutlined,
  RocketOutlined,
  ShoppingCartOutlined,
  StarOutlined,
  TagsOutlined,
  TrophyOutlined,
  WalletOutlined,
} from '@ant-design/icons';
import { Alert, Button, Card, Col, Row, Space, Tag, Typography, message } from 'antd';
import type { ReactNode } from 'react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import { marketingApi } from '../features/marketing/api';
import type { MarketingAppRecord } from '../features/marketing/types';
import './marketing.css';

interface AppDefinition {
  key: string;
  name: string;
  description: string;
  icon: ReactNode;
  tone: string;
  route?: string;
  reserved?: string;
}

const MARKETING_APPS: AppDefinition[] = [
  { key: 'COUPON', name: '优惠券', description: '创建代金券和满减券，统一管理领取、库存与使用规则', icon: <TagsOutlined />, tone: 'amber', route: '/marketing/coupons' },
  { key: 'FULL_REDUCTION', name: '满额立减', description: '满足订单门槛后自动减免，可与一张优惠券叠加', icon: <PercentageOutlined />, tone: 'red', route: '/marketing/full-reductions' },
  { key: 'DELIVERY_REDUCTION', name: '配送费满减', description: '按外卖配送金额减免配送费', icon: <CarOutlined />, tone: 'blue', reserved: '随外卖服务开放' },
  { key: 'ORDER_RED_PACKET', name: '下单返红包', description: '订单完成后向顾客返还下一单权益', icon: <GiftOutlined />, tone: 'pink' },
  { key: 'POPUP_COUPON', name: '弹窗优惠券', description: '进入门店时展示领券入口', icon: <NotificationOutlined />, tone: 'purple' },
  { key: 'POPUP_AD', name: '弹窗广告', description: '配置首页弹窗图片、频次、跳转动作与投放优先级', icon: <RocketOutlined />, tone: 'blue', route: '/marketing/popup-ads' },
  { key: 'PAYMENT_GIFT', name: '支付有礼', description: '支付成功后按规则发放优惠权益', icon: <WalletOutlined />, tone: 'green' },
  { key: 'ORDER_GIFT', name: '店铺满赠', description: '满足订单门槛后赠送指定权益或商品', icon: <GiftOutlined />, tone: 'amber' },
  { key: 'EXTRA_PURCHASE', name: '超级换购', description: '满足条件后以换购价追加商品', icon: <ShoppingCartOutlined />, tone: 'red' },
  { key: 'NEW_PRODUCT', name: '新商品活动', description: '为新品设置专属活动和展示入口', icon: <StarOutlined />, tone: 'green' },
  { key: 'EXCHANGE_CODE', name: '兑换码', description: '生成或导入兑换码并记录核销结果', icon: <QrcodeOutlined />, tone: 'purple' },
  { key: 'COUPON_PACKAGE', name: '券包', description: '组合多张优惠券形成付费或赠送券包', icon: <CrownOutlined />, tone: 'amber' },
  { key: 'CASH_REGISTER', name: '收银台', description: '面向店员的活动收银和权益核销入口', icon: <BankOutlined />, tone: 'blue' },
  { key: 'TABLE_BOOKING', name: '餐桌预定', description: '顾客预约日期、桌型与到店时间', icon: <CalendarOutlined />, tone: 'green' },
  { key: 'QUEUE', name: '排队取号', description: '维护桌位类型和线上排队号码', icon: <CoffeeOutlined />, tone: 'purple' },
  { key: 'BEVERAGE_STORAGE', name: '酒水寄存', description: '记录存取酒、有效期与顾客凭证', icon: <HeartOutlined />, tone: 'red' },
  { key: 'POINT_RED_PACKET', name: '集点返红包', description: '按消费或指定商品累计集点并发放权益', icon: <TrophyOutlined />, tone: 'pink' },
  { key: 'LOTTERY', name: '抽奖活动', description: '配置抽奖周期、次数限制、奖项权重与库存', icon: <TrophyOutlined />, tone: 'amber', route: '/marketing/lottery' },
  { key: 'NEW_CUSTOMER', name: '门店新客立减', description: '按门店首个有效订单识别新客并发放优惠', icon: <AppstoreOutlined />, tone: 'green' },
];

export function MarketingAppsPage() {
  const navigate = useNavigate();
  const [records, setRecords] = useState<MarketingAppRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState('');
  const [messageApi, holder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      setRecords(await marketingApi.listApps());
    } catch (error) {
      const detail = errorMessage(error);
      setLoadError(detail);
      messageApi.error(detail);
    } finally {
      setLoading(false);
    }
  }, [messageApi]);

  useEffect(() => { void load(); }, [load]);

  const remoteByKey = useMemo(() => new Map(records.flatMap((item) => [
    [item.key.toUpperCase(), item],
    [item.name, item],
  ])), [records]);

  return (
    <div className="page-shell marketing-page">
      {holder}
      <PageHeading
        title="营销应用"
        description="集中管理门店营销活动；已开放的应用可直接进入，其余服务将陆续开放"
        extra={<Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>刷新状态</Button>}
      />
      {loadError ? <Alert className="marketing-alert" type="warning" showIcon message="应用状态暂未同步" description={`${loadError}。能力目录仍可查看，已开放页面不受影响。`} /> : null}
      <Card bordered={false} className="content-card marketing-intro-card">
        <div>
          <Typography.Title level={4}>营销玩法</Typography.Title>
          <Typography.Paragraph type="secondary">当前可配置店铺满减、优惠券、弹窗广告和抽奖活动；金额、库存和参与结果以系统记录为准。</Typography.Paragraph>
        </div>
        <Space wrap><Tag color="success">4 个已开放入口</Tag><Tag>更多玩法陆续开放</Tag><Tag color="blue">外卖活动随外卖服务开放</Tag></Space>
      </Card>
      <Row gutter={[16, 16]} className="marketing-app-grid">
        {MARKETING_APPS.map((item) => {
          const remote = remoteByKey.get(item.key) ?? remoteByKey.get(item.name);
          const available = Boolean(item.route);
          return <Col key={item.key} xs={24} sm={12} xl={8} xxl={6}>
            <Card
              bordered={false}
              hoverable={available}
              className={`marketing-app-card ${available ? 'is-available' : ''}`}
              onClick={available ? () => navigate(item.route!) : undefined}
            >
              <div className={`marketing-app-icon tone-${item.tone}`}>{item.icon}</div>
              <div className="marketing-app-copy">
                <div className="marketing-app-title"><Typography.Text strong>{item.name}</Typography.Text>{item.reserved ? <Tag color="blue">{item.reserved}</Tag> : available ? <Tag color="success">已开放</Tag> : <Tag>规划中</Tag>}</div>
                <Typography.Paragraph type="secondary" ellipsis={{ rows: 2 }}>{remote?.description || item.description}</Typography.Paragraph>
                <div className="marketing-app-footer"><Typography.Text type="secondary">{available ? '进入配置' : '敬请期待'}</Typography.Text>{available ? <span>→</span> : null}</div>
              </div>
            </Card>
          </Col>;
        })}
      </Row>
    </div>
  );
}
