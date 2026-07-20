import {
  CloudUploadOutlined,
  EditOutlined,
  EyeOutlined,
  LinkOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Col,
  DatePicker,
  Empty,
  Form,
  Image,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Statistic,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd';
import type { Dayjs } from 'dayjs';
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import { decorationApi } from '../features/decoration/api';
import { marketingApi } from '../features/marketing/api';
import type {
  CouponCampaign,
  LotteryCampaign,
  MarketingPlacement,
  MarketingPlacementAction,
  MarketingPlacementCode,
  MarketingPlacementFrequency,
  MarketingPlacementPayload,
} from '../features/marketing/types';
import './marketing.css';

interface PlacementFormValues {
  name: string;
  title: string;
  subtitle: string;
  image_url: string;
  placement_code: MarketingPlacementCode;
  frequency: MarketingPlacementFrequency;
  action_type: MarketingPlacementAction;
  action_target_id?: string | number;
  priority: number;
  channel_scope: string;
  active_range?: [Dayjs, Dayjs];
}

const actionNames: Record<MarketingPlacementAction, string> = {
  NONE: '仅展示',
  OPEN_MENU: '进入点单',
  OPEN_COUPONS: '进入领券中心',
  CLAIM_COUPON: '领取指定优惠券',
  OPEN_LOTTERY: '进入指定抽奖',
};

const frequencyNames: Record<MarketingPlacementFrequency, string> = {
  EVERY_VISIT: '每次进入',
  DAILY: '每天一次',
  ONCE_PER_CAMPAIGN: '活动期仅一次',
};

const placementNames: Record<MarketingPlacementCode, string> = {
  HOME_POPUP: '门店首页',
  MENU_POPUP: '商品点单页',
  CHECKOUT_POPUP: '订单结算页',
  ORDER_RESULT_POPUP: '支付 / 订单结果页',
  PROFILE_POPUP: '会员中心',
};

function placementStatus(status: MarketingPlacement['status']) {
  if (status === 'ACTIVE') return <Tag color="success">投放中</Tag>;
  if (status === 'PAUSED') return <Tag color="warning">已暂停</Tag>;
  if (status === 'ENDED') return <Tag>已结束</Tag>;
  return <Tag>草稿</Tag>;
}

export function PopupAdsPage() {
  const [items, setItems] = useState<MarketingPlacement[]>([]);
  const [coupons, setCoupons] = useState<CouponCampaign[]>([]);
  const [lotteries, setLotteries] = useState<LotteryCampaign[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [imageUploading, setImageUploading] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<MarketingPlacement>();
  const [form] = Form.useForm<PlacementFormValues>();
  const [messageApi, holder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [placements, couponItems, lotteryItems] = await Promise.all([
        marketingApi.listPlacements(),
        marketingApi.listCoupons(),
        marketingApi.listLotteries(),
      ]);
      setItems(placements);
      setCoupons(couponItems);
      setLotteries(lotteryItems);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [messageApi]);

  useEffect(() => { void load(); }, [load]);

  const activeCount = items.filter((item) => item.status === 'ACTIVE').length;
  const exposures = items.reduce((sum, item) => sum + item.exposureCount, 0);
  const clicks = items.reduce((sum, item) => sum + item.clickCount, 0);
  const clickRate = exposures > 0 ? clicks / exposures * 100 : 0;

  const openEditor = (item?: MarketingPlacement) => {
    setEditing(item);
    form.resetFields();
    form.setFieldsValue(item ? {
      name: item.name,
      title: item.title,
      subtitle: item.subtitle,
      image_url: item.imageUrl,
      placement_code: item.slot,
      frequency: item.frequency,
      action_type: item.actionType,
      action_target_id: item.actionTargetId,
      priority: item.priority,
      channel_scope: item.channelScope,
      active_range: item.startsAt && item.endsAt ? [dayjs(item.startsAt), dayjs(item.endsAt)] : undefined,
    } : {
      placement_code: 'HOME_POPUP',
      frequency: 'ONCE_PER_CAMPAIGN',
      action_type: 'OPEN_MENU',
      priority: 100,
      channel_scope: 'ALL',
    });
    setEditorOpen(true);
  };

  const save = async () => {
    const values = await form.validateFields();
    const needsTarget = values.action_type === 'CLAIM_COUPON' || values.action_type === 'OPEN_LOTTERY';
    if (needsTarget && !values.action_target_id) {
      messageApi.error('请选择动作关联的活动');
      return;
    }
    const payload: MarketingPlacementPayload = {
      name: values.name.trim(),
      title: values.title.trim(),
      subtitle: values.subtitle?.trim() || '',
      image_url: values.image_url.trim(),
      placement_code: values.placement_code,
      frequency: values.frequency,
      action_type: values.action_type,
      action_target_id: needsTarget ? values.action_target_id : undefined,
      priority: Number(values.priority),
      channel_scope: values.channel_scope,
      active_from: values.active_range?.[0].toISOString(),
      active_to: values.active_range?.[1].toISOString(),
    };
    setSaving(true);
    try {
      if (editing) await marketingApi.updatePlacement(editing.id, payload);
      else await marketingApi.createPlacement(payload);
      setEditorOpen(false);
      messageApi.success(editing ? '弹窗广告已更新' : '弹窗广告草稿已创建');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const uploadImage = async (file: File) => {
    setImageUploading(true);
    try {
      const asset = await decorationApi.uploadAsset(file, `营销弹窗-${file.name}`);
      form.setFieldValue('image_url', asset.url);
      messageApi.success('图片已上传并填入弹窗素材');
    } catch (error) {
      messageApi.error(errorMessage(error));
      throw error;
    } finally {
      setImageUploading(false);
    }
  };

  const transition = async (item: MarketingPlacement, action: 'activate' | 'pause') => {
    try {
      if (action === 'activate') await marketingApi.activatePlacement(item.id);
      else await marketingApi.pausePlacement(item.id);
      messageApi.success(action === 'activate' ? '弹窗广告已开始投放' : '弹窗广告已暂停');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    }
  };

  const actionTargetOptions = useMemo(() => ({
    coupons: coupons.filter((item) => item.status !== 'ENDED' && item.distributionChannel === 'PUBLIC_CLAIM').map((item) => ({ value: item.id, label: `${item.name} · ${item.status}` })),
    lotteries: lotteries.filter((item) => item.status !== 'ENDED').map((item) => ({ value: item.id, label: `${item.name} · ${item.status}` })),
  }), [coupons, lotteries]);

  return (
    <div className="page-shell marketing-page">
      {holder}
      <PageHeading title="弹窗广告" description="管理小程序各关键位置的弹窗素材、展示频次、业务动作和渠道范围" extra={<Space><Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>刷新</Button><Button type="primary" icon={<PlusOutlined />} onClick={() => openEditor()}>新建弹窗</Button></Space>} />
      <Alert className="marketing-alert" type="info" showIcon message="曝光与点击分开记录" description="曝光以小程序真实渲染成功为准；“每天一次”按顾客设备的本地自然日控制。暂停后客户端将不再获得该投放。" />
      <Row gutter={[16, 16]} className="marketing-summary-grid">
        <Col xs={12} lg={6}><Card bordered={false}><Statistic title="投放中" value={activeCount} /></Card></Col>
        <Col xs={12} lg={6}><Card bordered={false}><Statistic title="累计曝光" value={exposures} prefix={<EyeOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card bordered={false}><Statistic title="累计点击" value={clicks} prefix={<LinkOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card bordered={false}><Statistic title="点击率" value={clickRate} precision={1} suffix="%" /></Card></Col>
      </Row>
      {items.length ? <Row gutter={[16, 16]} className="placement-grid">
        {items.map((item) => <Col xs={24} xl={12} key={item.id}>
          <Card bordered={false} className="content-card placement-card" loading={loading}>
            <div className="placement-visual"><Image src={item.imageUrl} alt={item.title} fallback="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='360' height='220'%3E%3Crect width='100%25' height='100%25' fill='%23f3eee8'/%3E%3Ctext x='50%25' y='50%25' text-anchor='middle' fill='%23968779'%3E%E5%BC%B9%E7%AA%97%E5%9B%BE%E7%89%87%3C/text%3E%3C/svg%3E" /></div>
            <div className="placement-body">
              <div className="placement-heading"><div><Typography.Title level={5}>{item.title || item.name}</Typography.Title><Typography.Text type="secondary">{item.subtitle || '未设置副标题'}</Typography.Text></div>{placementStatus(item.status)}</div>
              <Space wrap className="placement-tags"><Tag color="purple">{placementNames[item.slot]}</Tag><Tag>{frequencyNames[item.frequency]}</Tag><Tag color="blue">{actionNames[item.actionType]}</Tag><Tag>优先级 {item.priority}</Tag><Tag>{item.channelScope === 'ALL' ? '全部店内渠道' : item.channelScope === 'DINE_IN' ? '桌码堂食' : item.channelScope === 'TAKEOUT' ? '快餐自取' : '外卖预留'}</Tag></Space>
              <div className="placement-metrics"><span>曝光 <strong>{item.exposureCount}</strong></span><span>点击 <strong>{item.clickCount}</strong></span><span>点击率 <strong>{item.exposureCount ? (item.clickCount / item.exposureCount * 100).toFixed(1) : '0.0'}%</strong></span></div>
              <Space><Button icon={<EditOutlined />} disabled={item.status === 'ACTIVE' || item.status === 'ENDED'} onClick={() => openEditor(item)}>编辑</Button>{item.status === 'ACTIVE' ? <Popconfirm title="确认暂停此弹窗？" onConfirm={() => void transition(item, 'pause')}><Button icon={<PauseCircleOutlined />}>暂停</Button></Popconfirm> : item.status !== 'ENDED' ? <Popconfirm title="启用后将按优先级和频次向顾客展示" onConfirm={() => void transition(item, 'activate')}><Button type="primary" icon={<PlayCircleOutlined />}>启用</Button></Popconfirm> : null}</Space>
            </div>
          </Card>
        </Col>)}
      </Row> : <Card bordered={false} className="content-card marketing-empty-card" loading={loading}><Empty description="尚未创建弹窗广告"><Button type="primary" onClick={() => openEditor()}>创建第一个弹窗</Button></Empty></Card>}

      <Modal title={editing ? '编辑弹窗广告' : '新建弹窗广告'} width={820} open={editorOpen} onCancel={() => setEditorOpen(false)} onOk={() => void save()} confirmLoading={saving} destroyOnHidden>
        <Form form={form} layout="vertical">
          <Row gutter={16}><Col span={10}><Form.Item label="投放名称" name="name" rules={[{ required: true, whitespace: true }, { max: 100 }]}><Input placeholder="例如：七月新品弹窗" /></Form.Item></Col><Col span={7}><Form.Item label="展示位置" name="placement_code" rules={[{ required: true }]}><Select options={Object.entries(placementNames).map(([value, label]) => ({ value, label }))} /></Form.Item></Col><Col span={7}><Form.Item label="展示频次" name="frequency" rules={[{ required: true }]}><Select options={Object.entries(frequencyNames).map(([value, label]) => ({ value, label }))} /></Form.Item></Col></Row>
          <Form.Item label="主标题" name="title" rules={[{ max: 80 }]}><Input placeholder="例如：夏日咖啡新品" /></Form.Item>
          <Form.Item label="副标题" name="subtitle" rules={[{ max: 160 }]}><Input placeholder="一句话说明活动利益点" /></Form.Item>
          <Form.Item label="弹窗图片" required extra="建议使用竖版活动海报；上传成功后仍可手工替换 HTTPS 地址。">
            <Space.Compact block>
              <Form.Item name="image_url" noStyle rules={[{ required: true }, { type: 'url', message: '请输入完整图片 URL' }, { validator: (_, value: string) => !value || value.startsWith('https://') ? Promise.resolve() : Promise.reject(new Error('生产图片必须使用 HTTPS')) }]}><Input placeholder="https://..." /></Form.Item>
              <Upload
                accept="image/jpeg,image/png,image/gif"
                maxCount={1}
                showUploadList={false}
                beforeUpload={(file) => {
                  if (file.size > 8 * 1024 * 1024) {
                    messageApi.error('图片不能超过 8MB');
                    return Upload.LIST_IGNORE;
                  }
                  return true;
                }}
                customRequest={({ file, onError, onSuccess }) => {
                  void uploadImage(file as File).then((asset) => onSuccess?.(asset)).catch((error: Error) => onError?.(error));
                }}
              >
                <Button loading={imageUploading} icon={<CloudUploadOutlined />}>上传图片</Button>
              </Upload>
            </Space.Compact>
          </Form.Item>
          <Row gutter={16}><Col span={9}><Form.Item label="点击动作" name="action_type" rules={[{ required: true }]}><Select onChange={() => form.setFieldValue('action_target_id', undefined)} options={Object.entries(actionNames).map(([value, label]) => ({ value, label }))} /></Form.Item></Col><Col span={9}><Form.Item noStyle shouldUpdate={(previous, current) => previous.action_type !== current.action_type}>{({ getFieldValue }) => getFieldValue('action_type') === 'CLAIM_COUPON' ? <Form.Item label="关联优惠券" name="action_target_id" rules={[{ required: true }]}><Select options={actionTargetOptions.coupons} placeholder="选择优惠券" /></Form.Item> : getFieldValue('action_type') === 'OPEN_LOTTERY' ? <Form.Item label="关联抽奖" name="action_target_id" rules={[{ required: true }]}><Select options={actionTargetOptions.lotteries} placeholder="选择抽奖活动" /></Form.Item> : <Form.Item label="关联活动"><Input disabled placeholder="此动作无需关联 ID" /></Form.Item>}</Form.Item></Col><Col span={6}><Form.Item label="优先级" name="priority" extra="数值越大越优先"><InputNumber min={-10000} max={10000} precision={0} style={{ width: '100%' }} /></Form.Item></Col></Row>
          <Row gutter={16}><Col span={10}><Form.Item label="渠道范围" name="channel_scope" rules={[{ required: true }]}><Select options={[{ value: 'ALL', label: '全部店内渠道' }, { value: 'DINE_IN', label: '桌码堂食' }, { value: 'TAKEOUT', label: '快餐自取' }, { value: 'DELIVERY', label: '外卖（预留）', disabled: true }]} /></Form.Item></Col><Col span={14}><Form.Item label="投放时间" name="active_range"><DatePicker.RangePicker showTime style={{ width: '100%' }} /></Form.Item></Col></Row>
        </Form>
      </Modal>
    </div>
  );
}
