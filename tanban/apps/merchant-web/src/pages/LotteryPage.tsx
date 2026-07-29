import {
  DeleteOutlined,
  EditOutlined,
  GiftOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
  TrophyOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Col,
  DatePicker,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Progress,
  Row,
  Select,
  Space,
  Statistic,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import type { Dayjs } from 'dayjs';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { errorMessage } from '../api/client';
import { FeatureAvailabilityNotice } from '../components/FeatureAvailabilityNotice';
import { PageHeading } from '../components/PageHeading';
import { marketingApi } from '../features/marketing/api';
import type { CouponCampaign, LotteryCampaign, LotteryCampaignPayload, LotteryPrize, LotteryPrizeType } from '../features/marketing/types';
import { beijingPickerValue, dateTime, toBeijingRFC3339 } from '../utils/format';
import './marketing.css';

interface PrizeFormValue {
  name: string;
  prize_type: LotteryPrizeType;
  coupon_campaign_id?: string | number;
  weight: number;
  total_stock: number;
  sort_order: number;
  enabled: boolean;
}

interface LotteryFormValues {
  name: string;
  description: string;
  channel_scope: string;
  active_range?: [Dayjs, Dayjs];
  daily_limit: number;
  total_limit: number;
  terms: string;
  prizes: PrizeFormValue[];
}

function lotteryStatus(status: LotteryCampaign['status']) {
  if (status === 'ACTIVE') return <Tag color="success">进行中</Tag>;
  if (status === 'PAUSED') return <Tag color="warning">已暂停</Tag>;
  if (status === 'ENDED') return <Tag>已结束</Tag>;
  return <Tag>草稿</Tag>;
}

function prizeStock(prize: LotteryPrize) {
  if (prize.prizeType === 'THANKS') return '不限';
  return `${Math.max(prize.totalStock - prize.awardedCount, 0)} / ${prize.totalStock}`;
}

export function LotteryPage() {
  const [items, setItems] = useState<LotteryCampaign[]>([]);
  const [coupons, setCoupons] = useState<CouponCampaign[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<LotteryCampaign>();
  const [form] = Form.useForm<LotteryFormValues>();
  const [messageApi, holder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [campaigns, couponItems] = await Promise.all([marketingApi.listLotteries(), marketingApi.listCoupons()]);
      setItems(campaigns);
      setCoupons(couponItems);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [messageApi]);

  useEffect(() => { void load(); }, [load]);

  const activeCount = items.filter((item) => item.status === 'ACTIVE').length;
  const draws = items.reduce((sum, item) => sum + item.drawCount, 0);
  const wins = items.reduce((sum, item) => sum + item.winCount, 0);
  const couponOptions = useMemo(() => coupons.filter((item) => item.status !== 'ENDED' && item.distributionChannel === 'LOTTERY_ONLY').map((item) => ({ value: item.id, label: `${item.name} · 剩余 ${Math.max(item.totalStock - item.issuedCount, 0)}` })), [coupons]);

  const openEditor = (item?: LotteryCampaign) => {
    setEditing(item);
    form.resetFields();
    form.setFieldsValue(item ? {
      name: item.name,
      description: item.description,
      channel_scope: item.channelScope,
      active_range: item.activeFrom && item.activeTo ? [beijingPickerValue(item.activeFrom)!, beijingPickerValue(item.activeTo)!] : undefined,
      daily_limit: item.dailyLimit,
      total_limit: item.totalLimit,
      terms: item.terms,
      prizes: item.prizes.map((prize) => ({
        name: prize.name,
        prize_type: prize.prizeType,
        coupon_campaign_id: prize.couponCampaignId,
        weight: prize.weight,
        total_stock: prize.totalStock,
        sort_order: prize.sortOrder,
        enabled: prize.status !== 'DISABLED',
      })),
    } : {
      channel_scope: 'ALL',
      daily_limit: 1,
      total_limit: 30,
      terms: '每位顾客每天可参与 1 次，奖品数量有限，发完即止。',
      prizes: [
        { name: '优惠券', prize_type: 'COUPON', weight: 10, total_stock: 100, sort_order: 0, enabled: true },
        { name: '谢谢参与', prize_type: 'THANKS', weight: 90, total_stock: 0, sort_order: 1, enabled: true },
      ],
    });
    setEditorOpen(true);
  };

  const save = async () => {
    const values = await form.validateFields();
    if (Number(values.daily_limit) > Number(values.total_limit)) {
      messageApi.error('每人每日次数不能高于每人活动期总次数');
      return;
    }
    if (values.prizes.some((item) => Number(item.weight) <= 0)) {
      messageApi.error('每个奖项的权重都必须大于 0，停用奖项请关闭启用开关');
      return;
    }
    const enabledPrizes = values.prizes.filter((item) => item.enabled);
    if (!enabledPrizes.length || enabledPrizes.reduce((sum, item) => sum + Number(item.weight || 0), 0) <= 0) {
      messageApi.error('至少需要一个启用且权重大于 0 的奖项');
      return;
    }
    const missingCoupon = enabledPrizes.some((item) => item.prize_type === 'COUPON' && !item.coupon_campaign_id);
    if (missingCoupon) {
      messageApi.error('优惠券奖项必须关联一张“仅抽奖发放”的优惠券');
      return;
    }
    const payload: LotteryCampaignPayload = {
      name: values.name.trim(),
      description: values.description?.trim() || '',
      channel_scope: values.channel_scope,
      active_from: toBeijingRFC3339(values.active_range![0])!,
      active_to: toBeijingRFC3339(values.active_range![1])!,
      daily_limit: Number(values.daily_limit),
      total_limit: Number(values.total_limit),
      terms: values.terms.trim(),
      prizes: values.prizes.map((item, index) => ({
        name: item.name.trim(),
        prize_type: item.prize_type,
        coupon_campaign_id: item.prize_type === 'COUPON' ? item.coupon_campaign_id : undefined,
        weight: Number(item.weight),
        total_stock: item.prize_type === 'COUPON' ? Number(item.total_stock) : 0,
        sort_order: Number(item.sort_order ?? index),
        status: item.enabled ? 'ACTIVE' : 'DISABLED',
      })),
    };
    setSaving(true);
    try {
      if (editing) await marketingApi.updateLottery(editing.id, payload);
      else await marketingApi.createLottery(payload);
      setEditorOpen(false);
      messageApi.success(editing ? '抽奖活动已更新' : '抽奖活动草稿已创建');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const transition = async (item: LotteryCampaign, action: 'activate' | 'pause') => {
    try {
      if (action === 'activate') await marketingApi.activateLottery(item.id);
      else await marketingApi.pauseLottery(item.id);
      messageApi.success(action === 'activate' ? '抽奖活动已启用' : '抽奖活动已暂停');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  return (
    <div className="page-shell marketing-page">
      {holder}
      <PageHeading title="抽奖活动" description="配置参与周期、每日次数、奖项权重和有限奖品库存" extra={<Space><Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>刷新</Button><Button type="primary" icon={<PlusOutlined />} onClick={() => openEditor()}>新建抽奖</Button></Space>} />
      <FeatureAvailabilityNotice className="marketing-alert" type="warning" feature="LOTTERY_REWARD_REDEMPTION" />
      <Row gutter={[16, 16]} className="marketing-summary-grid">
        <Col xs={12} lg={6}><Card bordered={false}><Statistic title="抽奖活动" value={items.length} prefix={<TrophyOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card bordered={false}><Statistic title="进行中" value={activeCount} /></Card></Col>
        <Col xs={12} lg={6}><Card bordered={false}><Statistic title="累计抽奖" value={draws} /></Card></Col>
        <Col xs={12} lg={6}><Card bordered={false}><Statistic title="累计中奖" value={wins} prefix={<GiftOutlined />} /></Card></Col>
      </Row>
      <Card bordered={false} className="content-card marketing-table-card">
        <Table<LotteryCampaign>
          rowKey="id"
          loading={loading}
          dataSource={items}
          scroll={{ x: 1080 }}
          expandable={{
            expandedRowRender: (campaign) => <div className="lottery-prize-panel"><Typography.Text strong>奖项配置</Typography.Text><Table<LotteryPrize> size="small" rowKey={(prize) => String(prize.id ?? `${prize.sortOrder}-${prize.name}`)} pagination={false} dataSource={campaign.prizes} columns={[
              { title: '奖项', dataIndex: 'name' },
              { title: '类型', dataIndex: 'prizeType', render: (value) => value === 'COUPON' ? <Tag color="gold">优惠券</Tag> : <Tag>谢谢参与</Tag> },
              { title: '权重', dataIndex: 'weight' },
              { title: '剩余 / 总库存', render: (_, prize) => prizeStock(prize) },
              { title: '已发放', dataIndex: 'awardedCount' },
            ]} /></div>,
          }}
          columns={[
            { title: '活动', width: 240, render: (_, item) => <Space direction="vertical" size={0}><Typography.Text strong>{item.name}</Typography.Text><Typography.Text type="secondary">{item.activeFrom ? `${dateTime(item.activeFrom)} 至 ${item.activeTo ? dateTime(item.activeTo) : '长期'}` : '长期有效'}</Typography.Text></Space> },
            { title: '参与限制', width: 190, render: (_, item) => <Space direction="vertical" size={0}><span>每人每日 {item.dailyLimit} 次</span><Typography.Text type="secondary">每人活动期 {item.totalLimit} 次</Typography.Text></Space> },
            { title: '活动数据', width: 210, render: (_, item) => <div className="lottery-progress"><Progress percent={item.drawCount ? Math.min(item.winCount / item.drawCount * 100, 100) : 0} showInfo={false} /><Typography.Text type="secondary">{item.drawCount} 次抽奖 · {item.winCount} 次中奖</Typography.Text></div> },
            { title: '奖项', dataIndex: 'prizes', width: 100, render: (prizes: LotteryPrize[]) => `${prizes.length} 项` },
            { title: '渠道', width: 160, render: (_, item) => <Tag>{item.channelScope === 'ALL' ? '全部店内渠道' : item.channelScope === 'DINE_IN' ? '桌码堂食' : item.channelScope === 'TAKEOUT' ? '快餐自取' : '外卖暂未开放'}</Tag> },
            { title: '状态', dataIndex: 'status', width: 100, render: lotteryStatus },
            { title: '操作', fixed: 'right', width: 220, render: (_, item) => <Space size={4}><Button type="link" icon={<EditOutlined />} title={item.drawCount > 0 ? '已有抽奖记录，活动与奖项不可再编辑' : undefined} disabled={item.status === 'ACTIVE' || item.status === 'ENDED' || item.drawCount > 0} onClick={() => openEditor(item)}>编辑奖项</Button>{item.status === 'ACTIVE' ? <Popconfirm title="暂停后顾客不能继续抽奖" onConfirm={() => void transition(item, 'pause')}><Button type="link" icon={<PauseCircleOutlined />}>暂停</Button></Popconfirm> : item.status !== 'ENDED' ? <Popconfirm title="启用前将校验活动周期、奖项和库存" onConfirm={() => void transition(item, 'activate')}><Button type="link" icon={<PlayCircleOutlined />}>启用</Button></Popconfirm> : null}</Space> },
          ]}
        />
      </Card>

      <Modal title={editing ? '编辑抽奖活动与奖项' : '新建抽奖活动'} width={980} open={editorOpen} onCancel={() => setEditorOpen(false)} onOk={() => void save()} confirmLoading={saving} destroyOnHidden>
        <Form form={form} layout="vertical">
          <Row gutter={16}><Col span={12}><Form.Item label="活动名称" name="name" rules={[{ required: true, whitespace: true }, { max: 100 }]}><Input placeholder="例如：夏日咖啡幸运转盘" /></Form.Item></Col><Col span={12}><Form.Item label="活动周期（北京时间）" name="active_range" rules={[{ required: true, message: '请选择活动周期' }]}><DatePicker.RangePicker showTime format="YYYY-MM-DD HH:mm:ss" style={{ width: '100%' }} /></Form.Item></Col></Row>
          <Form.Item label="活动简介" name="description" rules={[{ max: 500 }]}><Input placeholder="展示在活动列表中的一句话简介" /></Form.Item>
          <Row gutter={16}><Col span={8}><Form.Item label="每人每日次数" name="daily_limit" rules={[{ required: true }]}><InputNumber min={1} max={100} precision={0} style={{ width: '100%' }} /></Form.Item></Col><Col span={8}><Form.Item label="每人活动期总次数" name="total_limit" rules={[{ required: true }]}><InputNumber min={1} max={10000} precision={0} style={{ width: '100%' }} /></Form.Item></Col><Col span={8}><Form.Item label="渠道范围" name="channel_scope" rules={[{ required: true }]}><Select options={[{ value: 'ALL', label: '全部店内渠道' }, { value: 'DINE_IN', label: '桌码堂食' }, { value: 'TAKEOUT', label: '快餐自取' }, { value: 'DELIVERY', label: '外卖（暂未开放）', disabled: true }]} /></Form.Item></Col></Row>
          <Form.Item label="活动说明与规则" name="terms" rules={[{ required: true, whitespace: true }, { max: 5000 }]}><Input.TextArea rows={3} maxLength={5000} showCount /></Form.Item>
          <div className="lottery-editor-heading"><div><Typography.Title level={5}>奖项设置</Typography.Title><Typography.Text type="secondary">权重只决定相对概率；优惠券奖项还受券库存与奖项库存双重限制。</Typography.Text></div></div>
          <Form.List name="prizes" rules={[{ validator: async (_, value) => { if (!value || value.length < 1) throw new Error('至少配置一个奖项'); if (value.length > 50) throw new Error('最多配置 50 个奖项'); } }]}>
            {(fields, { add, remove }, { errors }) => <>
              <div className="lottery-prize-editor">
                {fields.map((field, index) => <Card size="small" key={field.key} className="lottery-prize-card" title={`奖项 ${index + 1}`} extra={fields.length > 1 ? <Button type="text" danger icon={<DeleteOutlined />} onClick={() => remove(field.name)}>删除</Button> : null}>
                  <Row gutter={12}><Col span={7}><Form.Item label="奖项名称" name={[field.name, 'name']} rules={[{ required: true, whitespace: true }, { max: 100 }]}><Input /></Form.Item></Col><Col span={5}><Form.Item label="奖项类型" name={[field.name, 'prize_type']} rules={[{ required: true }]}><Select options={[{ value: 'COUPON', label: '优惠券' }, { value: 'THANKS', label: '谢谢参与' }]} /></Form.Item></Col><Col span={4}><Form.Item label="权重" name={[field.name, 'weight']} rules={[{ required: true }]}><InputNumber min={1} max={1_000_000_000_000} precision={0} style={{ width: '100%' }} /></Form.Item></Col><Col span={4}><Form.Item label="排序" name={[field.name, 'sort_order']}><InputNumber min={0} precision={0} style={{ width: '100%' }} /></Form.Item></Col><Col span={4}><Form.Item label="启用" name={[field.name, 'enabled']} valuePropName="checked"><Switch /></Form.Item></Col></Row>
                  <Form.Item noStyle shouldUpdate>{({ getFieldValue }) => {
                    const type = getFieldValue(['prizes', field.name, 'prize_type']) as LotteryPrizeType;
                    return type === 'COUPON' ? <Row gutter={12}><Col span={16}><Form.Item label="关联优惠券" name={[field.name, 'coupon_campaign_id']} rules={[{ required: true, message: '请选择优惠券' }]}><Select showSearch optionFilterProp="label" options={couponOptions} placeholder={couponOptions.length ? '选择仅抽奖发放的优惠券' : '请先创建“仅抽奖发放”优惠券'} /></Form.Item></Col><Col span={8}><Form.Item label="奖项总库存" name={[field.name, 'total_stock']} rules={[{ required: true }]}><InputNumber min={1} precision={0} style={{ width: '100%' }} /></Form.Item></Col></Row> : <Alert type="info" showIcon message="“谢谢参与”不占用奖品库存" />;
                  }}</Form.Item>
                </Card>)}
              </div>
              <Form.ErrorList errors={errors} />
              <Button block type="dashed" icon={<PlusOutlined />} onClick={() => add({ name: `奖项 ${fields.length + 1}`, prize_type: 'COUPON', weight: 10, total_stock: 100, sort_order: fields.length, enabled: true })}>新增奖项</Button>
            </>}
          </Form.List>
        </Form>
      </Modal>
    </div>
  );
}
