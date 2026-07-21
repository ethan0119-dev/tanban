import {
  CalendarOutlined,
  CopyOutlined,
  CoffeeOutlined,
  DeleteOutlined,
  PlusOutlined,
  SaveOutlined,
  ShopOutlined,
  QrcodeOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Col,
  DatePicker,
  Divider,
  Form,
  Image,
  Input,
  Radio,
  QRCode,
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
import { useCallback, useEffect, useState } from 'react';
import { api, errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import { DeveloperOnlyNote } from '../components/DeveloperOnlyNote';
import { merchantFeatureCopy } from '../features/availability/copy';
import { MediaLibraryModal } from '../components/media/MediaLibraryModal';
import { ImagePickerField } from '../components/media/ImagePickerField';
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
  const [logoLibraryOpen, setLogoLibraryOpen] = useState(false);
  const [storeCode, setStoreCode] = useState('');
  const [documents, setDocuments] = useState<Pick<MerchantSettings, 'businessLicenseUrl' | 'foodBusinessLicenseUrl'>>({});

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
      setStoreCode(settings.storeCode || '');
      setDocuments({ businessLicenseUrl: settings.businessLicenseUrl, foodBusinessLicenseUrl: settings.foodBusinessLicenseUrl });
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
      <PageHeading title="门店设置" description="配置门店资料、营业时间与紧急开关店状态" extra={<Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={() => void save()}>保存设置</Button>} />
      <Form<SettingsFormValues> form={form} layout="vertical" disabled={loading} className="settings-form">
        <Row gutter={[16, 16]}>
          <Col xs={24}>
            <Card bordered={false} className="content-card settings-card" title={<Space><ShopOutlined />门店资料</Space>}>
              <Row gutter={16}>
                <Col xs={24} md={12}><Form.Item label="门店名称" name="storeName" rules={[{ required: true, message: '请输入门店名称' }]}><Input prefix={<CoffeeOutlined />} placeholder="码农咖啡" /></Form.Item></Col>
                <Col xs={24} md={12}><Form.Item label="联系电话" name="phone"><Input placeholder="门店联系电话" /></Form.Item></Col>
              </Row>
              <Form.Item label="门店 Logo" name="logo"><ImagePickerField alt="门店 Logo" hint="将用于商户后台和顾客小程序的门店标识" onOpenLibrary={() => setLogoLibraryOpen(true)} /></Form.Item>
              <Form.Item label="经营地址" name="address"><Input placeholder="夜市、街区或摊位位置" /></Form.Item>
              <Form.Item label="店铺公告" name="announcement"><Input.TextArea rows={3} maxLength={120} showCount placeholder="将在顾客点单首页展示" /></Form.Item>
            </Card>

            <Card bordered={false} className="content-card settings-card" title={<Space><ShopOutlined />经营证照</Space>}>
              <Alert type="info" showIcon message="证照由平台统一维护" description="如需上传或更换，请联系平台管理员。商户账号只能查看，无法删除或修改。" />
              <Row gutter={[16, 16]} className="merchant-license-grid">
                {[
                  { title: '营业执照', url: documents.businessLicenseUrl },
                  { title: '食品经营许可证', url: documents.foodBusinessLicenseUrl },
                ].map((document) => <Col xs={24} md={12} key={document.title}><div className="merchant-license-card"><strong>{document.title}</strong>{document.url ? <Image src={document.url} alt={document.title} /> : <div className="merchant-license-empty">平台尚未上传</div>}</div></Col>)}
              </Row>
            </Card>

            <Card bordered={false} className="content-card settings-card" title={<Space><QrcodeOutlined />门店点单入口</Space>}>
              <Alert type="info" showIcon message="每家门店都有专属点单入口" description="顾客扫描正式点单码后，会直接进入当前门店并看到对应的商品、活动和订单。" />
              <Row gutter={[24, 16]} align="middle" style={{ marginTop: 20 }}>
                <Col xs={24} md={8}><div style={{ display: 'flex', justifyContent: 'center' }}>{import.meta.env.DEV && storeCode ? <QRCode size={190} value={`pages/home/index?scene=${encodeURIComponent(`s=${storeCode}`)}`} /> : <div className="official-code-placeholder"><QrcodeOutlined /><span>{merchantFeatureCopy.OFFICIAL_MINIAPP_CODE.title}</span></div>}</div></Col>
                <Col xs={24} md={16}>
                  <Typography.Title level={5}>门店识别码：{storeCode || '—'}</Typography.Title>
                  <Typography.Paragraph type="secondary">{merchantFeatureCopy.OFFICIAL_MINIAPP_CODE.description}</Typography.Paragraph>
                  <Button disabled={!storeCode} icon={<CopyOutlined />} onClick={() => void navigator.clipboard.writeText(storeCode).then(() => messageApi.success('门店识别码已复制'))}>复制门店识别码</Button>
                  <div style={{ marginTop: 16 }}><DeveloperOnlyNote>开发预览参数为 <Typography.Text code>{storeCode ? `s=${storeCode}` : '—'}</Typography.Text>；这里只用于开发排查，不会出现在正式构建中。</DeveloperOnlyNote></div>
                </Col>
              </Row>
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

          </Col>
        </Row>
      </Form>
      <MediaLibraryModal open={logoLibraryOpen} title="选择门店 Logo" onCancel={() => setLogoLibraryOpen(false)} onConfirm={(selected) => { if (selected[0]) form.setFieldValue('logo', selected[0].url); setLogoLibraryOpen(false); }} />
    </div>
  );
}
