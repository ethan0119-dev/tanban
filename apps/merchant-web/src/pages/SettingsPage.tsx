import {
  CalendarOutlined,
  CheckCircleFilled,
  ClockCircleOutlined,
  CoffeeOutlined,
  CopyOutlined,
  DeleteOutlined,
  EnvironmentOutlined,
  EyeInvisibleOutlined,
  EyeOutlined,
  FolderOpenOutlined,
  GlobalOutlined,
  InfoCircleOutlined,
  PictureOutlined,
  PlusOutlined,
  QrcodeOutlined,
  SafetyCertificateOutlined,
  SaveOutlined,
  ShopOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Col,
  DatePicker,
  Divider,
  Form,
  Image,
  Input,
  InputNumber,
  Radio,
  QRCode,
  Row,
  Select,
  Space,
  Switch,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import type { Dayjs } from 'dayjs';
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { api, errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import { DeveloperOnlyNote } from '../components/DeveloperOnlyNote';
import { merchantFeatureCopy } from '../features/availability/copy';
import { MediaLibraryModal } from '../components/media/MediaLibraryModal';
import { ImagePickerField } from '../components/media/ImagePickerField';
import type { MerchantSettings, MerchantStoreProfile, StoreBusinessDay, StoreBusinessHours } from '../types';
import { beijingNowDateTime, beijingPickerValue, toBeijingRFC3339 } from '../utils/format';

type ServiceChannel = MerchantStoreProfile['serviceChannels'][number];
type GalleryTarget = 'environment' | 'foodSafety' | null;

interface SettingsFormValues {
  storeName: string;
  logo?: string;
  phone?: string;
  address?: string;
  announcement?: string;
  visibleInMiniapp: boolean;
  contactName?: string;
  region?: string;
  mainProducts?: string;
  averageSpendYuan?: number;
  serviceChannels: ServiceChannel[];
  storeLatitude?: number;
  storeLongitude?: number;
}

const weekdayNames = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];
const timeOptions = Array.from({ length: 97 }, (_, index) => {
  const minutes = index * 15;
  const value = minutes === 1440 ? '24:00' : `${String(Math.floor(minutes / 60)).padStart(2, '0')}:${String(minutes % 60).padStart(2, '0')}`;
  return { value, label: value };
});

const channelOptions: Array<{ label: string; value: ServiceChannel; disabled?: boolean }> = [
  { label: '堂食', value: 'DINE_IN' },
  { label: '到店自取', value: 'TAKEOUT' },
  { label: '外卖配送（待开通）', value: 'DELIVERY', disabled: true },
];

function emptyWeek(): StoreBusinessDay[] {
  return weekdayNames.map((_, index) => ({ weekday: index + 1, periods: [] }));
}

function fullDayWeek(): StoreBusinessDay[] {
  return weekdayNames.map((_, index) => ({ weekday: index + 1, periods: [{ start: '00:00', end: '00:00' }] }));
}

function isFullDayWeek(schedule: StoreBusinessDay[]) {
  return schedule.length === 7 && schedule.every((day) => day.periods.length === 1 && day.periods[0].start === '00:00' && day.periods[0].end === '00:00');
}

function ImageGalleryField({ title, hint, value, onChange, onOpen }: {
  title: string;
  hint: string;
  value: string[];
  onChange: (value: string[]) => void;
  onOpen: () => void;
}) {
  return (
    <div className="store-gallery-field">
      <div className="store-gallery-heading">
        <div><strong>{title}</strong><Typography.Text type="secondary">{hint}</Typography.Text></div>
        <Tag>{value.length}/6</Tag>
      </div>
      <div className="store-gallery-grid">
        {value.map((url, index) => (
          <div className="store-gallery-image" key={`${url}-${index}`}>
            <Image src={url} alt={`${title}${index + 1}`} />
            <Button type="text" danger aria-label={`移除${title}${index + 1}`} icon={<DeleteOutlined />} onClick={() => onChange(value.filter((_, itemIndex) => itemIndex !== index))} />
          </div>
        ))}
        {value.length < 6 && <button className="store-gallery-add" type="button" onClick={onOpen}><PlusOutlined /><span>从图片库选择</span></button>}
      </div>
    </div>
  );
}

export function SettingsPage() {
  const [form] = Form.useForm<SettingsFormValues>();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [activeTab, setActiveTab] = useState('basic');
  const [businessHours, setBusinessHours] = useState<StoreBusinessHours | null>(null);
  const [weeklySchedule, setWeeklySchedule] = useState<StoreBusinessDay[]>(emptyWeek);
  const [scheduleMode, setScheduleMode] = useState<'FULL_DAY' | 'CUSTOM'>('CUSTOM');
  const [overrideStatus, setOverrideStatus] = useState<'NONE' | 'OPEN' | 'CLOSED'>('NONE');
  const [overrideUntil, setOverrideUntil] = useState<Dayjs | null>(null);
  const [overrideReason, setOverrideReason] = useState('');
  const [messageApi, contextHolder] = message.useMessage();
  const [logoLibraryOpen, setLogoLibraryOpen] = useState(false);
  const [galleryTarget, setGalleryTarget] = useState<GalleryTarget>(null);
  const [storeCode, setStoreCode] = useState('');
  const [documents, setDocuments] = useState<Pick<MerchantSettings, 'businessLicenseUrl' | 'foodBusinessLicenseUrl'>>({});
  const [environmentImages, setEnvironmentImages] = useState<string[]>([]);
  const [foodSafetyImages, setFoodSafetyImages] = useState<string[]>([]);
  const [locatingStore, setLocatingStore] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [settings, profile, hours] = await Promise.all([
        api.get<MerchantSettings>('/merchant/settings'),
        api.get<MerchantStoreProfile>('/merchant/store-profile'),
        api.get<StoreBusinessHours>('/merchant/business-hours'),
      ]);
      form.setFieldsValue({
        storeName: settings.storeName,
        logo: settings.logo,
        phone: settings.phone,
        address: settings.address,
        announcement: settings.announcement,
        visibleInMiniapp: profile.visibleInMiniapp ?? true,
        contactName: profile.contactName,
        region: profile.region,
        mainProducts: profile.mainProducts,
        averageSpendYuan: profile.averageSpendCents ? profile.averageSpendCents / 100 : undefined,
        serviceChannels: profile.serviceChannels?.length ? profile.serviceChannels : ['DINE_IN', 'TAKEOUT'],
        storeLatitude: profile.storeLatitude,
        storeLongitude: profile.storeLongitude,
      });
      setStoreCode(settings.storeCode || '');
      setDocuments({ businessLicenseUrl: settings.businessLicenseUrl, foodBusinessLicenseUrl: settings.foodBusinessLicenseUrl });
      setEnvironmentImages(profile.environmentImageUrls || []);
      setFoodSafetyImages(profile.foodSafetyImageUrls || []);
      setBusinessHours(hours);
      const schedule = hours.weeklySchedule?.length ? hours.weeklySchedule : emptyWeek();
      setWeeklySchedule(schedule);
      setScheduleMode(isFullDayWeek(schedule) ? 'FULL_DAY' : 'CUSTOM');
      setOverrideStatus(hours.temporaryOverride?.status || 'NONE');
      setOverrideUntil(beijingPickerValue(hours.temporaryOverride?.endsAt));
      setOverrideReason(hours.temporaryOverride?.reason || '');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [form, messageApi]);

  useEffect(() => { void load(); }, [load]);

  const save = async () => {
    let values: SettingsFormValues;
    try {
      values = await form.validateFields();
    } catch {
      messageApi.warning('请先补全必填信息');
      return;
    }
    if (overrideStatus !== 'NONE' && (!overrideUntil || !overrideUntil.isAfter(dayjs(beijingNowDateTime())))) {
      setActiveTab('operation');
      messageApi.error('临时营业状态的截止时间必须晚于当前时间');
      return;
    }
    setSaving(true);
    try {
      await Promise.all([
        api.put('/merchant/settings', {
          storeName: values.storeName,
          logo: values.logo || '',
          phone: values.phone || '',
          address: values.address || '',
          announcement: values.announcement || '',
        }),
        api.put('/merchant/store-profile', {
          visibleInMiniapp: values.visibleInMiniapp,
          contactName: values.contactName || '',
          region: values.region || '',
          mainProducts: values.mainProducts || '',
          averageSpendCents: Math.round((values.averageSpendYuan || 0) * 100),
          serviceChannels: values.serviceChannels,
          environmentImageUrls: environmentImages,
          foodSafetyImageUrls: foodSafetyImages,
          storeLatitude: values.storeLatitude,
          storeLongitude: values.storeLongitude,
        }),
        api.put('/merchant/business-hours', { timezone: 'Asia/Shanghai', weeklySchedule }),
        api.put('/merchant/business-status', {
          status: overrideStatus,
          endsAt: overrideStatus === 'NONE' ? undefined : toBeijingRFC3339(overrideUntil),
          reason: overrideReason,
        }),
      ]);
      messageApi.success('店铺设置已保存并同步到顾客端');
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

  const location = Form.useWatch(['storeLongitude'], form);
  const latitude = Form.useWatch(['storeLatitude'], form);
  const address = Form.useWatch(['address'], form);
  const visible = Form.useWatch(['visibleInMiniapp'], form);
  const mapURL = useMemo(() => location != null && latitude != null
    ? `https://uri.amap.com/marker?position=${location},${latitude}&name=${encodeURIComponent(address || form.getFieldValue('storeName') || '门店位置')}`
    : '', [address, form, latitude, location]);
  const locateStore = () => {
    if (!navigator.geolocation) {
      messageApi.error('当前浏览器不支持定位，请手动填写经纬度');
      return;
    }
    setLocatingStore(true);
    navigator.geolocation.getCurrentPosition(
      ({ coords }) => {
        form.setFieldsValue({
          storeLatitude: Number(coords.latitude.toFixed(7)),
          storeLongitude: Number(coords.longitude.toFixed(7)),
        });
        setLocatingStore(false);
        messageApi.success('已选择当前位置，请核对地图后保存');
      },
      (error) => {
        setLocatingStore(false);
        const denied = error.code === error.PERMISSION_DENIED;
        messageApi.error(denied ? '定位权限未开启，请在浏览器地址栏允许位置权限后重试' : '暂时无法获取当前位置，请手动填写经纬度');
      },
      { enableHighAccuracy: true, timeout: 12000, maximumAge: 0 },
    );
  };

  const basicTab = (
    <div className="store-settings-stack">
      <Card bordered={false} className="content-card settings-card store-status-card">
        <div className="store-status-summary">
          <div className={`store-status-icon ${visible ? 'active' : ''}`}>{visible ? <EyeOutlined /> : <EyeInvisibleOutlined />}</div>
          <div><Typography.Text type="secondary">顾客端展示</Typography.Text><strong>{visible ? '门店已展示' : '门店已隐藏'}</strong></div>
        </div>
        <div className="store-status-summary">
          <div className={`store-status-icon ${businessHours?.businessStatus === 'OPEN' ? 'active' : 'warning'}`}><ClockCircleOutlined /></div>
          <div><Typography.Text type="secondary">实时营业状态</Typography.Text><strong>{businessHours?.businessStatus === 'OPEN' ? '营业中' : '休息中'}</strong></div>
        </div>
        <div className="store-status-summary store-code-summary">
          <div className="store-status-icon"><ShopOutlined /></div>
          <div><Typography.Text type="secondary">门店识别码</Typography.Text><strong>{storeCode || '—'}</strong></div>
          <Button type="text" icon={<CopyOutlined />} disabled={!storeCode} onClick={() => void navigator.clipboard.writeText(storeCode).then(() => messageApi.success('门店识别码已复制'))}>复制</Button>
        </div>
      </Card>

      <Card bordered={false} className="content-card settings-card" title={<Space><InfoCircleOutlined />基础资料</Space>}>
        <Row gutter={[18, 4]}>
          <Col xs={24} lg={12}><Form.Item label="门店名称" name="storeName" rules={[{ required: true, whitespace: true, message: '请输入门店名称' }, { max: 120 }]}><Input prefix={<CoffeeOutlined />} placeholder="例如：码农咖啡鼓楼店" /></Form.Item></Col>
          <Col xs={24} lg={12}><Form.Item label="门店显示" name="visibleInMiniapp" valuePropName="checked"><Switch checkedChildren="显示" unCheckedChildren="隐藏" /></Form.Item><Typography.Text className="store-field-help" type="secondary">隐藏后，顾客将无法通过门店码进入点单页面。</Typography.Text></Col>
        </Row>
        <Form.Item label="门店 Logo" name="logo"><ImagePickerField alt="门店 Logo" hint="用于商户后台和顾客小程序，建议使用 1:1 图片" onOpenLibrary={() => setLogoLibraryOpen(true)} /></Form.Item>
        <Divider />
        <Row gutter={[18, 4]}>
          <Col xs={24} lg={12}><Form.Item label="门店联系人" name="contactName" rules={[{ max: 80 }]}><Input placeholder="负责门店经营事务的联系人" /></Form.Item></Col>
          <Col xs={24} lg={12}><Form.Item label="联系电话" name="phone" rules={[{ max: 32 }]}><Input placeholder="顾客和平台联系门店时使用" /></Form.Item></Col>
          <Col xs={24} lg={12}><Form.Item label="所属区域" name="region" rules={[{ max: 120 }]}><Input placeholder="例如：天津市 / 红桥区" /></Form.Item></Col>
          <Col xs={24} lg={12}><Form.Item label="详细地址" name="address" rules={[{ max: 255 }]}><Input prefix={<EnvironmentOutlined />} placeholder="街道、商场、楼层或摊位位置" /></Form.Item></Col>
        </Row>
        <div className="store-location-box">
          <div className="store-location-copy"><GlobalOutlined /><div><strong>门店地图定位</strong><Typography.Text type="secondary">在门店现场可直接选择当前位置，也可以手动填写经纬度；保存后顾客可在小程序中导航到店。</Typography.Text></div></div>
          <Row gutter={12} className="store-coordinate-row">
            <Col xs={24} sm={9}><Form.Item name="storeLongitude" label="经度" dependencies={['storeLatitude']} rules={[({ getFieldValue }) => ({ validator(_, value) { const other = getFieldValue('storeLatitude'); return (value == null) === (other == null) ? Promise.resolve() : Promise.reject(new Error('经纬度需同时填写')); } })]}><InputNumber min={-180} max={180} precision={7} placeholder="117.1714700" style={{ width: '100%' }} /></Form.Item></Col>
            <Col xs={24} sm={9}><Form.Item name="storeLatitude" label="纬度" dependencies={['storeLongitude']} rules={[({ getFieldValue }) => ({ validator(_, value) { const other = getFieldValue('storeLongitude'); return (value == null) === (other == null) ? Promise.resolve() : Promise.reject(new Error('经纬度需同时填写')); } })]}><InputNumber min={-90} max={90} precision={7} placeholder="39.1435280" style={{ width: '100%' }} /></Form.Item></Col>
            <Col xs={24} sm={6} className="store-map-action">
              <Space wrap>
                <Button icon={<EnvironmentOutlined />} loading={locatingStore} onClick={locateStore}>选择当前位置</Button>
                <Button href={mapURL || undefined} target="_blank" disabled={!mapURL}>地图核对</Button>
              </Space>
            </Col>
          </Row>
        </div>
        <Form.Item label="店铺公告" name="announcement" rules={[{ max: 500 }]}><Input.TextArea rows={3} maxLength={120} showCount placeholder="营业提醒、取餐说明等内容，将展示在顾客端" /></Form.Item>
      </Card>
    </div>
  );

  const operationTab = (
    <div className="store-settings-stack">
      <Card bordered={false} className="content-card settings-card" title={<Space><CoffeeOutlined />经营信息</Space>}>
        <Row gutter={[18, 4]}>
          <Col xs={24} lg={12}><Form.Item label="经营渠道" name="serviceChannels" rules={[{ required: true, message: '至少选择一个经营渠道' }]}><Checkbox.Group options={channelOptions} /></Form.Item><Typography.Text className="store-field-help" type="secondary">用于门店档案和渠道联动；商品是否售卖仍以商品渠道设置为准。</Typography.Text></Col>
          <Col xs={24} lg={12}><Form.Item label="人均消费" name="averageSpendYuan"><InputNumber min={0} max={1000000} precision={2} prefix="¥" placeholder="例如：25" style={{ width: '100%' }} /></Form.Item></Col>
          <Col xs={24}><Form.Item label="主营产品" name="mainProducts" rules={[{ max: 255 }]}><Input placeholder="例如：美式咖啡、拿铁、气泡水、甜品" /></Form.Item></Col>
        </Row>
      </Card>

      <Card bordered={false} className="content-card settings-card" title={<Space><CalendarOutlined />营业时间与临时状态</Space>}>
        <Alert
          type={businessHours?.businessStatus === 'OPEN' ? 'success' : 'warning'}
          showIcon
          message={<Space>当前{businessHours?.businessStatus === 'OPEN' ? '营业中' : '休息中'}<Tag>{businessHours?.businessStatusReason || 'WEEKLY_SCHEDULE'}</Tag></Space>}
          description={businessHours?.businessStatusMessage || '保存后，小程序会按门店时区实时判断是否允许创建新订单。'}
        />
        <div className="store-schedule-mode">
          <div><strong>营业时间模式</strong><Typography.Text type="secondary">可选择全天营业，或按每周日期配置多个营业时段。</Typography.Text></div>
          <Radio.Group value={scheduleMode} optionType="button" buttonStyle="solid" onChange={(event) => {
            const mode = event.target.value as 'FULL_DAY' | 'CUSTOM';
            setScheduleMode(mode);
            setWeeklySchedule(mode === 'FULL_DAY' ? fullDayWeek() : (isFullDayWeek(weeklySchedule) ? weekdayNames.map((_, index) => ({ weekday: index + 1, periods: [{ start: '09:00', end: '22:00' }] })) : weeklySchedule));
          }}>
            <Radio.Button value="FULL_DAY">全天24小时</Radio.Button>
            <Radio.Button value="CUSTOM">自定义时段</Radio.Button>
          </Radio.Group>
        </div>
        <Form.Item label="营业时区" className="store-timezone-field"><Input value="北京时间（UTC+8）" disabled addonBefore="统一时区" /></Form.Item>
        {scheduleMode === 'CUSTOM' && <div className="business-week-editor">
          {weeklySchedule.map((day) => (
            <div className="business-day-row" key={day.weekday}>
              <div className="business-day-name">
                <Switch size="small" checked={day.periods.length > 0} onChange={(checked) => updateDay(day.weekday, (current) => ({ ...current, periods: checked ? (current.periods.length ? current.periods : [{ start: '09:00', end: '22:00' }]) : [] }))} />
                <strong>{weekdayNames[day.weekday - 1]}</strong>
              </div>
              <div className="business-period-list">
                {day.periods.length === 0 ? <Typography.Text type="secondary">休息</Typography.Text> : day.periods.map((period, index) => (
                  <Space key={`${day.weekday}-${index}`} wrap>
                    <Select showSearch value={period.start} options={timeOptions.slice(0, -1)} onChange={(value) => updateDay(day.weekday, (current) => ({ ...current, periods: current.periods.map((item, itemIndex) => itemIndex === index ? { ...item, start: value } : item) }))} style={{ width: 104 }} />
                    <span>至</span>
                    <Select showSearch value={period.end} options={timeOptions} onChange={(value) => updateDay(day.weekday, (current) => ({ ...current, periods: current.periods.map((item, itemIndex) => itemIndex === index ? { ...item, end: value } : item) }))} style={{ width: 104 }} />
                    <Button type="text" danger aria-label={`删除${weekdayNames[day.weekday - 1]}第${index + 1}个时段`} icon={<DeleteOutlined />} onClick={() => updateDay(day.weekday, (current) => ({ ...current, periods: current.periods.filter((_, itemIndex) => itemIndex !== index) }))} />
                  </Space>
                ))}
                {day.periods.length > 0 && day.periods.length < 6 && <Button type="link" icon={<PlusOutlined />} onClick={() => updateDay(day.weekday, (current) => ({ ...current, periods: [...current.periods, { start: '09:00', end: '22:00' }] }))}>增加时段</Button>}
              </div>
            </div>
          ))}
        </div>}
        <Divider />
        <div className="store-override-heading"><div><Typography.Title level={5}>临时营业覆盖</Typography.Title><Typography.Text type="secondary">适合节假日加开、设备维护或临时闭店，到期后自动恢复周计划。</Typography.Text></div></div>
        <Radio.Group value={overrideStatus} optionType="button" onChange={(event) => setOverrideStatus(event.target.value)}>
          <Radio.Button value="NONE">按周计划</Radio.Button>
          <Radio.Button value="OPEN">临时营业</Radio.Button>
          <Radio.Button value="CLOSED">临时闭店</Radio.Button>
        </Radio.Group>
        {overrideStatus !== 'NONE' && <Row gutter={12} style={{ marginTop: 16 }}>
          <Col xs={24} md={10}><DatePicker showTime format="YYYY-MM-DD HH:mm:ss" value={overrideUntil} onChange={setOverrideUntil} placeholder="选择北京时间截止时间" style={{ width: '100%' }} /></Col>
          <Col xs={24} md={14}><Input value={overrideReason} maxLength={255} onChange={(event) => setOverrideReason(event.target.value)} placeholder={overrideStatus === 'CLOSED' ? '例如：设备维护，今晚暂停营业' : '例如：节日临时加开'} /></Col>
        </Row>}
      </Card>

      <Card bordered={false} className="content-card settings-card" title={<Space><PictureOutlined />门店展示与食品安全</Space>}>
        <ImageGalleryField title="商家环境" hint="展示门头、用餐区、制作区等真实环境，最多 6 张。" value={environmentImages} onChange={setEnvironmentImages} onOpen={() => setGalleryTarget('environment')} />
        <Divider />
        <ImageGalleryField title="食品安全档案" hint="可上传后厨公示、消毒记录、员工健康证明等，最多 6 张。" value={foodSafetyImages} onChange={setFoodSafetyImages} onOpen={() => setGalleryTarget('foodSafety')} />
      </Card>

      <Card bordered={false} className="content-card settings-card" title={<Space><SafetyCertificateOutlined />经营证照</Space>}>
        <Alert type="info" showIcon message="证照由平台统一审核维护" description="如需上传或更换，请联系平台管理员。商户账号可查看审核后的最新版本。" />
        <Row gutter={[16, 16]} className="merchant-license-grid">
          {[
            { title: '营业执照', url: documents.businessLicenseUrl },
            { title: '食品经营许可证', url: documents.foodBusinessLicenseUrl },
          ].map((document) => <Col xs={24} md={12} key={document.title}><div className="merchant-license-card"><div className="merchant-license-title"><CheckCircleFilled /><strong>{document.title}</strong></div>{document.url ? <Image src={document.url} alt={document.title} /> : <div className="merchant-license-empty">平台尚未上传</div>}</div></Col>)}
        </Row>
      </Card>

      <Card bordered={false} className="content-card settings-card" title={<Space><QrcodeOutlined />门店点单入口</Space>}>
        <Row gutter={[24, 16]} align="middle">
          <Col xs={24} md={8}><div className="store-code-preview">{import.meta.env.DEV && storeCode ? <QRCode size={176} value={`pages/home/index?scene=${encodeURIComponent(`s=${storeCode}`)}`} /> : <div className="official-code-placeholder"><QrcodeOutlined /><span>{merchantFeatureCopy.OFFICIAL_MINIAPP_CODE.title}</span></div>}</div></Col>
          <Col xs={24} md={16}>
            <Typography.Title level={5}>门店识别码：{storeCode || '—'}</Typography.Title>
            <Typography.Paragraph type="secondary">{merchantFeatureCopy.OFFICIAL_MINIAPP_CODE.description}</Typography.Paragraph>
            <Button disabled={!storeCode} icon={<CopyOutlined />} onClick={() => void navigator.clipboard.writeText(storeCode).then(() => messageApi.success('门店识别码已复制'))}>复制门店识别码</Button>
            <div style={{ marginTop: 16 }}><DeveloperOnlyNote>开发预览参数为 <Typography.Text code>{storeCode ? `s=${storeCode}` : '—'}</Typography.Text>；这里只用于开发排查，不会出现在正式构建中。</DeveloperOnlyNote></div>
          </Col>
        </Row>
      </Card>
    </div>
  );

  return (
    <div className="page-shell store-settings-page">
      {contextHolder}
      <PageHeading title="店铺设置" description="维护门店对外资料、经营信息、营业计划与安全档案" extra={<Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={() => void save()}>保存设置</Button>} />
      <Form<SettingsFormValues> form={form} layout="vertical" disabled={loading} className="settings-form">
        <Card bordered={false} className="content-card store-settings-tabs" styles={{ body: { padding: 0 } }}>
          <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
            { key: 'basic', label: <Space><ShopOutlined />基础信息</Space>, children: basicTab },
            { key: 'operation', label: <Space><CoffeeOutlined />门店信息</Space>, children: operationTab },
          ]} />
        </Card>
      </Form>
      <MediaLibraryModal open={logoLibraryOpen} title="选择门店 Logo" onCancel={() => setLogoLibraryOpen(false)} onConfirm={(selected) => { if (selected[0]) form.setFieldValue('logo', selected[0].url); setLogoLibraryOpen(false); }} />
      <MediaLibraryModal
        open={galleryTarget !== null}
        title={galleryTarget === 'environment' ? '选择商家环境图片' : '选择食品安全档案图片'}
        multiple
        maxSelection={6 - (galleryTarget === 'environment' ? environmentImages.length : foodSafetyImages.length)}
        excludeUrls={galleryTarget === 'environment' ? environmentImages : foodSafetyImages}
        onCancel={() => setGalleryTarget(null)}
        onConfirm={(selected) => {
          const urls = selected.map((item) => item.url);
          if (galleryTarget === 'environment') setEnvironmentImages((current) => [...current, ...urls].slice(0, 6));
          if (galleryTarget === 'foodSafety') setFoodSafetyImages((current) => [...current, ...urls].slice(0, 6));
          setGalleryTarget(null);
        }}
      />
    </div>
  );
}
