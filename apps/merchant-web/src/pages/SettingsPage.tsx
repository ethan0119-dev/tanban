import {
  BankOutlined,
  CalendarOutlined,
  ClockCircleOutlined,
  CoffeeOutlined,
  DeleteOutlined,
  PlusOutlined,
  PrinterOutlined,
  SaveOutlined,
  SettingOutlined,
  ShopOutlined,
  SoundOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Col,
  DatePicker,
  Divider,
  Form,
  Input,
  InputNumber,
  Radio,
  Row,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd';
import type { Dayjs } from 'dayjs';
import dayjs from 'dayjs';
import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { api, errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import type { MerchantSettings, StoreBusinessDay, StoreBusinessHours } from '../types';

type SettingsFormValues = Omit<MerchantSettings, 'businessHours'>;

const weekdayNames = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];
const timeOptions = Array.from({ length: 97 }, (_, index) => {
  const minutes = index * 15;
  const value = minutes === 1440 ? '24:00' : `${String(Math.floor(minutes / 60)).padStart(2, '0')}:${String(minutes % 60).padStart(2, '0')}`;
  return { value, label: value };
});

function emptyWeek(): StoreBusinessDay[] {
  return weekdayNames.map((_, index) => ({ weekday: index + 1, periods: [] }));
}

export function SettingsPage() {
  const [form] = Form.useForm<SettingsFormValues>();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [businessHours, setBusinessHours] = useState<StoreBusinessHours | null>(null);
  const [timezone, setTimezone] = useState('Asia/Shanghai');
  const [weeklySchedule, setWeeklySchedule] = useState<StoreBusinessDay[]>(emptyWeek);
  const [overrideStatus, setOverrideStatus] = useState<'NONE' | 'OPEN' | 'CLOSED'>('NONE');
  const [overrideUntil, setOverrideUntil] = useState<Dayjs | null>(null);
  const [overrideReason, setOverrideReason] = useState('');
  const [messageApi, contextHolder] = message.useMessage();
  const printTrigger = Form.useWatch('printTrigger', form);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [settings, hours] = await Promise.all([
        api.get<MerchantSettings>('/merchant/settings'),
        api.get<StoreBusinessHours>('/merchant/business-hours'),
      ]);
      form.setFieldsValue({
        storeName: settings.storeName,
        logo: settings.logo,
        phone: settings.phone,
        address: settings.address,
        announcement: settings.announcement,
        autoAcceptOrder: settings.autoAcceptOrder ?? true,
        orderVoiceReminder: settings.orderVoiceReminder ?? true,
        printTrigger: settings.printTrigger ?? 'PAYMENT_SUCCESS',
        autoPrintReceipt: settings.autoPrintReceipt ?? true,
        autoPrintLabel: settings.autoPrintLabel ?? true,
        pickupMode: settings.pickupMode ?? true,
        allowLatePayment: settings.allowLatePayment ?? true,
        paymentTimeoutMinutes: settings.paymentTimeoutMinutes ?? 15,
      });
      setBusinessHours(hours);
      setTimezone(hours.timezone || 'Asia/Shanghai');
      setWeeklySchedule(hours.weeklySchedule?.length ? hours.weeklySchedule : emptyWeek());
      setOverrideStatus(hours.temporaryOverride?.status || 'NONE');
      setOverrideUntil(hours.temporaryOverride?.endsAt ? dayjs(hours.temporaryOverride.endsAt) : null);
      setOverrideReason(hours.temporaryOverride?.reason || '');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [form, messageApi]);

  useEffect(() => { void load(); }, [load]);

  const save = async () => {
    const values = await form.validateFields();
    if (overrideStatus !== 'NONE' && (!overrideUntil || !overrideUntil.isAfter(dayjs()))) {
      messageApi.error('临时营业状态的截止时间必须晚于当前时间');
      return;
    }
    setSaving(true);
    try {
      await api.put('/merchant/settings', values);
      await api.put('/merchant/business-hours', { timezone, weeklySchedule });
      await api.put('/merchant/business-status', {
        status: overrideStatus,
        endsAt: overrideStatus === 'NONE' ? undefined : overrideUntil?.toISOString(),
        reason: overrideReason,
      });
      messageApi.success('门店资料和经营策略已保存');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const updateDay = (weekday: number, updater: (day: StoreBusinessDay) => StoreBusinessDay) => {
    setWeeklySchedule((current) => current.map((day) => day.weekday === weekday ? updater(day) : day));
  };

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading title="门店设置" description="配置营业信息、接单规则与打印触发策略" extra={<Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={() => void save()}>保存设置</Button>} />
      <Form<SettingsFormValues> form={form} layout="vertical" disabled={loading} className="settings-form">
        <Row gutter={[16, 16]}>
          <Col xs={24} xl={15}>
            <Card bordered={false} className="content-card settings-card" title={<Space><ShopOutlined />门店资料</Space>}>
              <Row gutter={16}>
                <Col xs={24} md={12}><Form.Item label="门店名称" name="storeName" rules={[{ required: true, message: '请输入门店名称' }]}><Input prefix={<CoffeeOutlined />} placeholder="码农咖啡" /></Form.Item></Col>
                <Col xs={24} md={12}><Form.Item label="联系电话" name="phone"><Input placeholder="门店联系电话" /></Form.Item></Col>
              </Row>
              <Form.Item label="门店 Logo URL" name="logo"><Input placeholder="https://..." /></Form.Item>
              <Form.Item label="经营地址" name="address"><Input placeholder="夜市、街区或摊位位置" /></Form.Item>
              <Form.Item label="店铺公告" name="announcement"><Input.TextArea rows={3} maxLength={120} showCount placeholder="将在顾客点单首页展示" /></Form.Item>
            </Card>

            <Card bordered={false} className="content-card settings-card" title={<Space><CalendarOutlined />营业时间与临时状态</Space>}>
              <Alert
                type={businessHours?.businessStatus === 'OPEN' ? 'success' : 'warning'}
                showIcon
                message={<Space>当前{businessHours?.businessStatus === 'OPEN' ? '营业中' : '休息中'}<Tag>{businessHours?.businessStatusReason || 'WEEKLY_SCHEDULE'}</Tag></Space>}
                description={businessHours?.businessStatusMessage || '保存后，小程序会按门店时区实时判断是否允许创建新订单。'}
              />
              <Form.Item label="门店时区" style={{ marginTop: 18 }}>
                <Input value={timezone} onChange={(event) => setTimezone(event.target.value)} placeholder="Asia/Shanghai" />
              </Form.Item>
              <Typography.Paragraph type="secondary">每周可设置多个时段；结束时间早于开始时间表示跨夜，例如 18:00–02:00。</Typography.Paragraph>
              <div className="business-week-editor">
                {weeklySchedule.map((day) => (
                  <div className="business-day-row" key={day.weekday}>
                    <div className="business-day-name">
                      <Switch
                        size="small"
                        checked={day.periods.length > 0}
                        onChange={(checked) => updateDay(day.weekday, (current) => ({ ...current, periods: checked ? (current.periods.length ? current.periods : [{ start: '09:00', end: '22:00' }]) : [] }))}
                      />
                      <strong>{weekdayNames[day.weekday - 1]}</strong>
                    </div>
                    <div className="business-period-list">
                      {day.periods.length === 0 ? <Typography.Text type="secondary">休息</Typography.Text> : day.periods.map((period, index) => (
                        <Space key={`${day.weekday}-${index}`} wrap>
                          <Select
                            showSearch
                            value={period.start}
                            options={timeOptions.slice(0, -1)}
                            onChange={(value) => updateDay(day.weekday, (current) => ({ ...current, periods: current.periods.map((item, itemIndex) => itemIndex === index ? { ...item, start: value } : item) }))}
                            style={{ width: 104 }}
                          />
                          <span>至</span>
                          <Select
                            showSearch
                            value={period.end}
                            options={timeOptions}
                            onChange={(value) => updateDay(day.weekday, (current) => ({ ...current, periods: current.periods.map((item, itemIndex) => itemIndex === index ? { ...item, end: value } : item) }))}
                            style={{ width: 104 }}
                          />
                          <Button
                            type="text"
                            danger
                            aria-label={`删除${weekdayNames[day.weekday - 1]}第${index + 1}个时段`}
                            icon={<DeleteOutlined />}
                            onClick={() => updateDay(day.weekday, (current) => ({ ...current, periods: current.periods.filter((_, itemIndex) => itemIndex !== index) }))}
                          />
                        </Space>
                      ))}
                      {day.periods.length > 0 && day.periods.length < 6 && <Button type="link" icon={<PlusOutlined />} onClick={() => updateDay(day.weekday, (current) => ({ ...current, periods: [...current.periods, { start: '09:00', end: '22:00' }] }))}>增加时段</Button>}
                    </div>
                  </div>
                ))}
              </div>
              <Divider />
              <Typography.Title level={5}>临时营业覆盖</Typography.Title>
              <Radio.Group value={overrideStatus} onChange={(event) => setOverrideStatus(event.target.value)}>
                <Radio.Button value="NONE">按周计划</Radio.Button>
                <Radio.Button value="OPEN">临时营业</Radio.Button>
                <Radio.Button value="CLOSED">临时闭店</Radio.Button>
              </Radio.Group>
              {overrideStatus !== 'NONE' && (
                <Row gutter={12} style={{ marginTop: 16 }}>
                  <Col xs={24} md={10}><DatePicker showTime value={overrideUntil} onChange={setOverrideUntil} placeholder="选择覆盖截止时间" style={{ width: '100%' }} /></Col>
                  <Col xs={24} md={14}><Input value={overrideReason} maxLength={255} onChange={(event) => setOverrideReason(event.target.value)} placeholder={overrideStatus === 'CLOSED' ? '例如：设备维护，今晚暂停营业' : '例如：节日临时加开'} /></Col>
                </Row>
              )}
            </Card>

            <Card bordered={false} className="content-card settings-card" title={<Space><SettingOutlined />订单规则</Space>}>
              <div className="setting-switch-row"><div><strong>自动接单</strong><p>顾客完成下单后自动进入订单列表</p></div><Form.Item name="autoAcceptOrder" valuePropName="checked" noStyle><Switch /></Form.Item></div>
              <div className="setting-switch-row"><div><strong>新订单语音提醒</strong><p>商户后台打开时播报新订单</p></div><Form.Item name="orderVoiceReminder" valuePropName="checked" noStyle><Switch /></Form.Item></div>
              <div className="setting-switch-row"><div><strong>快餐取餐号</strong><p>快餐 / 到店自取订单创建时生成门店营业日内稳定取餐号</p></div><Tag color="success">已启用</Tag></div>
              <div className="setting-switch-row"><div><strong>超时后允许继续付款</strong><p>未主动关闭的订单，即使超过提示时间仍可付款</p></div><Form.Item name="allowLatePayment" valuePropName="checked" noStyle><Switch /></Form.Item></div>
              <Form.Item label="支付提示有效时间" name="paymentTimeoutMinutes" rules={[{ required: true }]}><InputNumber min={1} max={1440} precision={0} addonAfter="分钟" style={{ width: 240 }} /></Form.Item>
            </Card>

            <Card bordered={false} className="content-card settings-card" title={<Space><PrinterOutlined />打印总策略</Space>}>
              <Alert type="warning" showIcon message="建议默认选择“支付成功后打印”" description="这里的触发点用于首次生成场景模板；已存在的桌码堂食、自提和外卖模板可在各自打印模板页单独调整。下面两个自动打印开关是门店总开关，关闭后不会自动创建对应任务。" />
              <Form.Item label="新模板默认触发点" name="printTrigger" rules={[{ required: true }]} className="trigger-choice">
                <Radio.Group>
                  <Radio.Button value="PAYMENT_SUCCESS"><Space><BankOutlined />付款后打印</Space></Radio.Button>
                  <Radio.Button value="ORDER_CREATED"><Space><ClockCircleOutlined />下单后打印</Space></Radio.Button>
                </Radio.Group>
              </Form.Item>
              <Typography.Paragraph type="secondary">
                新模板默认：{printTrigger === 'ORDER_CREATED' ? '订单创建成功即生成打印任务，未付款订单也可能出单。' : '只有支付机构确认成功后生成打印任务。'}
              </Typography.Paragraph>
              <div className="setting-switch-row"><div><strong>自动打印订单小票</strong><p>打印整单信息、金额和订单备注</p></div><Form.Item name="autoPrintReceipt" valuePropName="checked" noStyle><Switch /></Form.Item></div>
              <div className="setting-switch-row"><div><strong>自动打印商品标签</strong><p>按商品数量拆分标签，适合饮品杯贴</p></div><Form.Item name="autoPrintLabel" valuePropName="checked" noStyle><Switch /></Form.Item></div>
            </Card>
          </Col>

          <Col xs={24} xl={9}>
            <Card bordered={false} className="content-card provider-card">
              <span className="provider-icon"><BankOutlined /></span>
              <Typography.Title level={4}>支付服务</Typography.Title>
              <TagLike>会生活 · 随行付</TagLike>
              <div className="provider-line"><span>资金流向</span><strong>支付机构 → 商户银行卡</strong></div>
              <div className="provider-line"><span>平台是否过款</span><strong className="safe-text">否</strong></div>
              <div className="provider-line"><span>结算信息</span><strong>请在会生活商家端查看</strong></div>
              <Alert type="info" showIcon message="商户号及结算卡由平台管理员完成进件绑定，商户后台不展示敏感银行卡信息。" />
            </Card>
            <Card bordered={false} className="content-card help-card">
              <SoundOutlined />
              <div><strong>订单提醒没有声音？</strong><p>请允许浏览器播放声音，并保持商户后台页面处于打开状态。</p></div>
            </Card>
          </Col>
        </Row>
      </Form>
    </div>
  );
}

function TagLike({ children }: { children: ReactNode }) {
  return <span className="provider-tag">{children}</span>;
}
