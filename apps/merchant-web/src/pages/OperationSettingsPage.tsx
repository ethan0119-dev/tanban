import {
  BankOutlined,
  BellOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  EnvironmentOutlined,
  SafetyCertificateOutlined,
  SaveOutlined,
  TeamOutlined,
  UserOutlined,
  WechatOutlined,
  PrinterOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Col,
  Descriptions,
  Divider,
  Form,
  Input,
  InputNumber,
  Radio,
  Row,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { api, errorMessage } from '../api/client';
import { DeveloperOnlyNote } from '../components/DeveloperOnlyNote';
import { FeatureAvailabilityNotice } from '../components/FeatureAvailabilityNotice';
import { PageHeading } from '../components/PageHeading';
import { MediaLibraryModal } from '../components/media/MediaLibraryModal';
import { ImagePickerField } from '../components/media/ImagePickerField';
import { WechatPayOnboardingCard } from '../components/WechatPayOnboardingCard';
import { merchantFeatureCopy } from '../features/availability/copy';
import { merchantSettingsWritePayload, operationSettingsWritePayload } from '../features/settings/payloads';
import type { MerchantOperationSettings, MerchantOperationSettingsResponse, MerchantPaymentSettings, MerchantSettings } from '../types';

export type SettingsSection = 'ORDER' | 'PAYMENT' | 'NOTIFICATION' | 'PRIVACY' | 'PRINT';

const sectionMeta: Record<SettingsSection, { title: string; description: string }> = {
  ORDER: { title: '点餐设置', description: '配置堂食结算、多人点餐、距离校验和顾客下单规则' },
  PAYMENT: { title: '支付设置', description: '查看当前收款通道、商户绑定、资金流向和支付确认方式' },
  NOTIFICATION: { title: '通知设置', description: '配置商户希望接收的消息类型，并查看微信通知开通状态' },
  PRIVACY: { title: '隐私与客服', description: '维护小程序隐私政策、用户协议和私人客服联系方式' },
  PRINT: { title: '打印设置', description: '配置门店打印总开关与新模板的默认触发点' },
};

const eventOptions = [
  { label: '支付成功新订单', value: 'ORDER_PAID' },
  { label: '退款申请 / 完成', value: 'REFUND_CREATED' },
  { label: '打印失败', value: 'PRINT_FAILED' },
  { label: '门店经营异常', value: 'STORE_EXCEPTION' },
];

function SettingRow({ title, description, control, tag }: { title: string; description: string; control?: ReactNode; tag?: ReactNode }) {
  return <div className="setting-switch-row"><div><strong>{title}</strong><p>{description}</p></div>{control || tag}</div>;
}

export function OperationSettingsPage({ section }: { section: SettingsSection }) {
  const [form] = Form.useForm<Partial<MerchantOperationSettings & MerchantSettings>>();
  const [operation, setOperation] = useState<MerchantOperationSettings | null>(null);
  const [operationMeta, setOperationMeta] = useState<MerchantOperationSettingsResponse | null>(null);
  const [merchantSettings, setMerchantSettings] = useState<MerchantSettings | null>(null);
  const [payment, setPayment] = useState<MerchantPaymentSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [qrLibraryOpen, setQrLibraryOpen] = useState(false);
  const [messageApi, contextHolder] = message.useMessage();
  const distanceEnabled = Form.useWatch('distanceCheckEnabled', form);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      if (section === 'PAYMENT') {
        setPayment(await api.get<MerchantPaymentSettings>('/merchant/payment-settings'));
      } else {
        const [response, storeSettings] = await Promise.all([
          api.get<MerchantOperationSettingsResponse>('/merchant/operation-settings'),
          api.get<MerchantSettings>('/merchant/settings'),
        ]);
        setOperation(response.settings);
        setOperationMeta(response);
        setMerchantSettings(storeSettings);
        form.setFieldsValue({ ...response.settings, ...storeSettings });
      }
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [form, messageApi, section]);

  useEffect(() => { void load(); }, [load]);

  const save = async () => {
    if (!operation) return;
    await form.validateFields();
    const values = form.getFieldsValue(true);
    setSaving(true);
    try {
      if (section !== 'PRINT') {
        const operationPatch: Partial<MerchantOperationSettings> = section === 'ORDER' ? {
          settlementMode: values.settlementMode,
          orderingMode: values.orderingMode,
          distanceCheckEnabled: values.distanceCheckEnabled,
          distanceLimitM: values.distanceLimitM,
          storeLatitude: values.storeLatitude,
          storeLongitude: values.storeLongitude,
          requireCustomerPhone: values.requireCustomerPhone,
          allowOrderRemark: values.allowOrderRemark,
          allowItemRemark: values.allowItemRemark,
          orderReminderEnabled: values.orderReminderEnabled,
          orderReminderIntervalMinutes: values.orderReminderIntervalMinutes,
        } : section === 'NOTIFICATION' ? {
          officialAccountNotifyEnabled: values.officialAccountNotifyEnabled,
          officialAccountEvents: values.officialAccountEvents,
          notificationRecipientLabel: values.notificationRecipientLabel,
        } : {
          customerServicePhone: values.customerServicePhone,
          customerServiceWechat: values.customerServiceWechat,
          customerServiceQrUrl: values.customerServiceQrUrl,
          privacyPolicyText: values.privacyPolicyText,
          userAgreementText: values.userAgreementText,
        };
        await api.put('/merchant/operation-settings', operationSettingsWritePayload(operation, operationPatch));
      }
      if (section === 'ORDER' && merchantSettings) {
        await api.put('/merchant/settings', merchantSettingsWritePayload(merchantSettings, {
          allowLatePayment: values.allowLatePayment,
          paymentTimeoutMinutes: values.paymentTimeoutMinutes,
        }));
      } else if (section === 'PRINT' && merchantSettings) {
        await api.put('/merchant/settings', merchantSettingsWritePayload(merchantSettings, {
          printTrigger: values.printTrigger,
          autoPrintReceipt: values.autoPrintReceipt,
          autoPrintLabel: values.autoPrintLabel,
        }));
      }
      messageApi.success('设置已保存');
      await load();
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const heading = section === 'PAYMENT' && payment
    ? { title: '支付设置', description: `查看${payment.providerDisplayName}的商户绑定、资金流向和通道状态` }
    : sectionMeta[section];
  const extra = section === 'PAYMENT' ? <Button onClick={() => void load()}>刷新通道状态</Button> : <Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={() => void save()}>保存设置</Button>;

  if (loading) return <div className="page-shell"><PageHeading title={heading.title} description={heading.description} /><div className="settings-loading"><Spin size="large" /></div></div>;

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading title={heading.title} description={heading.description} extra={extra} />

      {section === 'PAYMENT' && payment && <PaymentSettings payment={payment} />}

      {section !== 'PAYMENT' && <Form form={form} layout="vertical">
        {section === 'ORDER' && (
          <Row gutter={[16, 16]}>
            <Col xs={24} xl={15}>
              <Card bordered={false} className="content-card settings-card" title={<Space><BankOutlined />结算与点餐模式</Space>}>
                <Form.Item label="堂食结算模式" name="settlementMode">
                  <Radio.Group>
                    <Radio.Button value="PAY_BEFORE">先结账后用餐</Radio.Button>
                    <Radio.Button value="PAY_AFTER" disabled>先用餐后结账（暂未开放）</Radio.Button>
                  </Radio.Group>
                </Form.Item>
                <FeatureAvailabilityNotice feature="PAY_AFTER_MEAL" />
                <Divider />
                <Form.Item label="堂食点餐模式" name="orderingMode">
                  <Radio.Group>
                    <Radio.Button value="MULTI_PERSON"><TeamOutlined /> 多人点餐</Radio.Button>
                    <Radio.Button value="SINGLE_PERSON"><UserOutlined /> 单人点餐</Radio.Button>
                  </Radio.Group>
                </Form.Item>
                <Typography.Paragraph type="secondary">多人点餐允许同一桌的不同顾客分别提交订单；单人点餐会拒绝其他顾客占用同一桌的活动订单。</Typography.Paragraph>
              </Card>

              <Card bordered={false} className="content-card settings-card" title={<Space><EnvironmentOutlined />距离与顾客校验</Space>}>
                <SettingRow title="判定用户距离" description="启用后，顾客下单必须授权定位并位于门店允许范围内；下单时系统会再次核对距离。" control={<Form.Item name="distanceCheckEnabled" valuePropName="checked" noStyle><Switch /></Form.Item>} />
                {distanceEnabled && <>
                  <Alert type="warning" showIcon message="启用后没有定位信息的订单将被拒绝" />
                  <Row gutter={12} style={{ marginTop: 16 }}>
                    <Col xs={24} md={8}><Form.Item label="允许距离" name="distanceLimitM" rules={[{ required: true }]}><InputNumber min={100} max={100000} addonAfter="米" style={{ width: '100%' }} /></Form.Item></Col>
                    <Col xs={24} md={8}><Form.Item label="门店纬度" name="storeLatitude" rules={[{ required: true }]}><InputNumber min={-90} max={90} precision={7} style={{ width: '100%' }} /></Form.Item></Col>
                    <Col xs={24} md={8}><Form.Item label="门店经度" name="storeLongitude" rules={[{ required: true }]}><InputNumber min={-180} max={180} precision={7} style={{ width: '100%' }} /></Form.Item></Col>
                  </Row>
                </>}
                <SettingRow title="下单必须填写手机号" description="适用于需要联系顾客或线下核验的门店。" control={<Form.Item name="requireCustomerPhone" valuePropName="checked" noStyle><Switch /></Form.Item>} />
                <SettingRow title="允许整单备注" description="允许顾客在结算页填写整单要求。" control={<Form.Item name="allowOrderRemark" valuePropName="checked" noStyle><Switch /></Form.Item>} />
                <SettingRow title="允许单品备注" description="允许顾客对某一杯饮品或餐品单独备注。" control={<Form.Item name="allowItemRemark" valuePropName="checked" noStyle><Switch /></Form.Item>} />
              </Card>

              <Card bordered={false} className="content-card settings-card" title="订单时效与提醒">
                <SettingRow title="自动接单" description={merchantFeatureCopy.AUTO_ACCEPT_ORDER.description} tag={<Tag>暂未开放</Tag>} />
                <SettingRow title="后台语音提醒" description={merchantFeatureCopy.ORDER_VOICE_NOTICE.description} tag={<Tag>暂未开放</Tag>} />
                <SettingRow title="超时后允许继续付款" description="关闭后，超过支付提示时间的未付款订单会被关单；关闭订单后始终禁止付款。" control={<Form.Item name="allowLatePayment" valuePropName="checked" noStyle><Switch /></Form.Item>} />
                <Form.Item label="未支付提示 / 关单时长" name="paymentTimeoutMinutes" rules={[{ required: true }]}><InputNumber min={1} max={1440} precision={0} addonAfter="分钟" style={{ width: 240 }} /></Form.Item>
                <SettingRow title="允许顾客催单" description={merchantFeatureCopy.ORDER_REMINDER.description} control={<Form.Item name="orderReminderEnabled" valuePropName="checked" noStyle><Switch disabled /></Form.Item>} />
                <Form.Item label="催单最短间隔" name="orderReminderIntervalMinutes"><InputNumber min={1} max={120} precision={0} addonAfter="分钟" style={{ width: 240 }} /></Form.Item>
              </Card>
            </Col>
            <Col xs={24} xl={9}>
              <Card bordered={false} className="content-card settings-card" title={<Space><SafetyCertificateOutlined />支付与库存安全策略</Space>}>
                <Alert type="success" showIcon message="关键资金保护策略由系统强制开启，不允许商户关闭" />
                <SettingRow title="已取消订单迟到支付隔离" description="支付回调晚于关单时，订单进入支付异常，不错误推进制作；退款由有权限的人员确认金额后发起。" tag={<Tag color="success">强制开启</Tag>} />
                <SettingRow title="重复付款识别与隔离" description="追加式支付尝试和唯一机构流水能识别重复实收；系统保留两笔事实并进入支付异常，不静默吞单。" tag={<Tag color="success">强制开启</Tag>} />
                <SettingRow title="库存扣减" description="下单时预留库存，支付成功后确认；关单或过期时释放。" tag={<Tag color="blue">系统保障</Tag>} />
              </Card>
              <Card bordered={false} className="content-card settings-card" title="尚未开通的服务">
                <SettingRow title="超时未接单自动退款" description={merchantFeatureCopy.AUTO_REFUND.description} tag={<Tag>暂未开放</Tag>} />
                <SettingRow title="自提核销" description={merchantFeatureCopy.PICKUP_VERIFICATION.description} tag={<Tag>暂未开放</Tag>} />
                <SettingRow title="顾客评价" description={merchantFeatureCopy.CUSTOMER_REVIEW.description} tag={<Tag>暂未开放</Tag>} />
              </Card>
            </Col>
          </Row>
        )}

        {section === 'NOTIFICATION' && (
          <Row gutter={[16, 16]}>
            <Col xs={24} xl={15}>
              <Card bordered={false} className="content-card settings-card" title={<Space><WechatOutlined />微信服务号通知</Space>}>
                {operationMeta?.officialAccount.platformConfigured
                  ? <Alert type="info" showIcon message="平台微信通知服务已准备" description="完成接收人绑定后，系统会按下方选择发送消息。" />
                  : <FeatureAvailabilityNotice type="warning" feature="OFFICIAL_ACCOUNT_NOTICE" />}
                <DeveloperOnlyNote style={{ marginTop: 12 }}>消息发送还需要平台服务号参数、消息模板和商户接收人身份绑定；未完成全部配置时不得标记为发送成功。</DeveloperOnlyNote>
                <SettingRow title="启用服务号通知偏好" description="开启只代表商户希望接收，实际发送状态以下方接入状态为准。" control={<Form.Item name="officialAccountNotifyEnabled" valuePropName="checked" noStyle><Switch /></Form.Item>} />
                <Form.Item label="通知事件" name="officialAccountEvents"><Checkbox.Group options={eventOptions} /></Form.Item>
                <Form.Item label="接收人备注" name="notificationRecipientLabel"><Input placeholder="例如：老板微信、夜班店长" maxLength={120} /></Form.Item>
              </Card>
            </Col>
            <Col xs={24} xl={9}>
              <Card bordered={false} className="content-card settings-card" title={<Space><BellOutlined />接入状态</Space>}>
                <Descriptions column={1} size="small">
                  <Descriptions.Item label="平台服务号">{operationMeta?.officialAccount.platformConfigured ? <Tag color="success">已配置参数</Tag> : <Tag>待申请 / 配置</Tag>}</Descriptions.Item>
                  <Descriptions.Item label="商户接收人"><Tag>待扫码绑定</Tag></Descriptions.Item>
                  <Descriptions.Item label="消息投递"><Tag>未启用</Tag></Descriptions.Item>
                </Descriptions>
                <Divider />
                <Typography.Paragraph type="secondary">短信、云喇叭和语音播报需要单独开通；未开通前请以站内通知和订单列表为准。</Typography.Paragraph>
              </Card>
            </Col>
          </Row>
        )}

        {section === 'PRIVACY' && (
          <Row gutter={[16, 16]}>
            <Col xs={24} xl={15}>
              <Card bordered={false} className="content-card settings-card" title={<Space><SafetyCertificateOutlined />协议与隐私</Space>}>
                <Alert type="info" showIcon message="小程序提交审核前必须补齐真实、可执行的文本" description="隐私政策应与微信《用户隐私保护指引》、实际收集字段和第三方共享清单保持一致，不能直接复制其他经营主体的信息。" />
                <Form.Item label="隐私政策" name="privacyPolicyText" rules={[{ required: true, message: '请输入隐私政策' }]}><Input.TextArea rows={10} maxLength={20000} showCount placeholder="填写当前商户适用的隐私说明，平台通用隐私政策另由平台管理端维护" /></Form.Item>
                <Form.Item label="用户协议" name="userAgreementText" rules={[{ required: true, message: '请输入用户协议' }]}><Input.TextArea rows={8} maxLength={20000} showCount placeholder="填写点餐、退款、储值等规则" /></Form.Item>
              </Card>
            </Col>
            <Col xs={24} xl={9}>
              <Card bordered={false} className="content-card settings-card" title="私人客服">
                <Form.Item label="客服电话" name="customerServicePhone"><Input placeholder="顾客可点击拨打" maxLength={32} /></Form.Item>
                <Form.Item label="客服微信" name="customerServiceWechat"><Input placeholder="客服微信号" maxLength={80} /></Form.Item>
                <Form.Item label="客服二维码" name="customerServiceQrUrl"><ImagePickerField alt="客服二维码" hint="顾客可在小程序中查看并长按识别" onOpenLibrary={() => setQrLibraryOpen(true)} /></Form.Item>
              </Card>
            </Col>
          </Row>
        )}

        {section === 'PRINT' && (
          <Row gutter={[16, 16]}>
            <Col xs={24} xl={15}>
              <Card bordered={false} className="content-card settings-card" title={<Space><PrinterOutlined />打印总策略</Space>}>
                <Alert type="warning" showIcon message="默认建议选择支付成功后打印" description="下单后打印会让未付款订单也出单；每个店内/外卖模板仍可在对应打印模板页单独设置触发点和商家联、顾客联、厨房联、标签等版式。" />
                <Form.Item label="新模板默认触发点" name="printTrigger" rules={[{ required: true }]} style={{ marginTop: 18 }}>
                  <Radio.Group>
                    <Radio.Button value="PAYMENT_SUCCESS"><BankOutlined /> 付款后打印</Radio.Button>
                    <Radio.Button value="ORDER_CREATED"><ClockCircleOutlined /> 下单后打印</Radio.Button>
                  </Radio.Group>
                </Form.Item>
                <SettingRow title="自动打印订单小票" description="命中场景模板后创建整单小票任务。" control={<Form.Item name="autoPrintReceipt" valuePropName="checked" noStyle><Switch /></Form.Item>} />
                <SettingRow title="自动打印商品标签" description="按商品数量拆分杯贴/标签任务。" control={<Form.Item name="autoPrintLabel" valuePropName="checked" noStyle><Switch /></Form.Item>} />
              </Card>
            </Col>
            <Col xs={24} xl={9}>
              <Card bordered={false} className="content-card settings-card" title="打印链路说明">
                <Typography.Paragraph>打印总开关 → 场景模板触发点 → 打印机绑定与路由 → 厂商云队列 / 虚拟打印机。</Typography.Paragraph>
                <Alert type="info" showIcon message="补打不会复用首次任务" description="补打会生成独立任务并标记“补打”，保留操作人、时间和失败原因。" />
              </Card>
            </Col>
          </Row>
        )}
      </Form>}
      <MediaLibraryModal open={qrLibraryOpen} title="选择客服二维码" onCancel={() => setQrLibraryOpen(false)} onConfirm={(selected) => { if (selected[0]) form.setFieldValue('customerServiceQrUrl', selected[0].url); setQrLibraryOpen(false); }} />
    </div>
  );
}

function PaymentSettings({ payment }: { payment: MerchantPaymentSettings }) {
  const wechat = payment.provider === 'wechat_partner';
  const mock = payment.provider === 'mock';
  const bound = payment.bindingStatus === 'BOUND' || payment.bindingStatus === 'ACTIVE';
  const transactionReady = payment.providerActive && payment.adapterImplemented && payment.onboardingReady;
  const effectiveProviderNames: Record<string, string> = {
    mock: '模拟支付',
    tianque: '会生活 · 随行付',
    wechat_partner: '微信支付（普通服务商）',
  };
  const checkoutModeLabels: Record<string, string> = {
    MOCK: '模拟支付（仅联调）',
    HALF_SCREEN_CASHIER: '会生活半屏收银台',
    WECHAT_MINI_PROGRAM: '微信小程序支付',
  };
  const bindingLabels: Record<string, string> = {
    DEVELOPMENT: '开发环境',
    PENDING_BINDING: '待平台进件绑定',
    BOUND: '商户已绑定',
    NOT_APPLIED: '未进件',
    REVIEWING: '审核中',
    PENDING_SIGNING: '待签约',
    ACTIVE: '特约商户已开通',
    REJECTED: '进件被驳回',
  };
  const authorizationLabels: Record<string, string> = {
    NOT_AUTHORIZED: '未授权',
    PENDING: '授权处理中',
    AUTHORIZED: '已授权',
    REVOKED: '授权已撤销',
  };
  return (
    <>
    <Row gutter={[16, 16]}>
      <Col xs={24} xl={15}>
        <Card bordered={false} className="content-card settings-card" title={<Space><BankOutlined />收款通道</Space>}>
          <div className="payment-provider-hero">
            <span className="provider-icon"><BankOutlined /></span>
            <div><Typography.Title level={4}>{payment.providerDisplayName}</Typography.Title><Tag color={bound ? 'success' : payment.bindingStatus === 'REJECTED' ? 'error' : payment.bindingStatus === 'DEVELOPMENT' ? 'blue' : 'warning'}>{bindingLabels[payment.bindingStatus] || payment.bindingStatus}</Tag></div>
          </div>
          <Descriptions bordered column={{ xs: 1, md: 2 }}>
            <Descriptions.Item label={mock ? '支付环境' : wechat ? '微信支付特约商户号（sub_mchid）' : '支付商户号'}>{mock ? '开发联调环境' : payment.merchantNoMasked || '未绑定'}</Descriptions.Item>
            <Descriptions.Item label={mock ? '真实资金' : wechat ? '小程序接入方式' : '小程序子 AppID'}>{mock ? '不产生真实扣款' : wechat && payment.sharedServiceProviderApp ? '共用摊伴小程序，无需商户填写 AppID' : payment.subAppIdConfigured ? '已配置独立 sub_appid' : '未配置'}</Descriptions.Item>
            {wechat && <Descriptions.Item label="小程序支付产品授权"><Tag color={payment.productAuthorizationStatus === 'AUTHORIZED' ? 'success' : 'warning'}>{authorizationLabels[payment.productAuthorizationStatus] || payment.productAuthorizationStatus}</Tag></Descriptions.Item>}
            {wechat && <Descriptions.Item label="服务商 API 退款授权"><Tag color={payment.refundAuthorized ? 'success' : 'warning'}>{payment.refundAuthorized ? '已授权' : '未授权'}</Tag></Descriptions.Item>}
            {!mock && <Descriptions.Item label="支付费率">{payment.feeRatePercent.toFixed(2)}%</Descriptions.Item>}
            {!mock && <Descriptions.Item label="结算周期">{payment.settlementCycle === 'T1' ? 'T+1' : payment.settlementCycle}</Descriptions.Item>}
            <Descriptions.Item label="收银方式">{checkoutModeLabels[payment.checkoutMode] || payment.checkoutMode}</Descriptions.Item>
            <Descriptions.Item label="部分退款">{mock ? '支持模拟退款' : payment.supportsPartialRefund ? '支持' : '未授权'}</Descriptions.Item>
          </Descriptions>
          {!payment.acceptanceEnabled && <Alert style={{ marginTop: 18 }} type="error" showIcon message="平台已暂停在线收款" description="所有商户的在线支付入口当前均不可用，请联系平台管理员恢复支付接收。" />}
          {payment.acceptanceEnabled && !payment.providerActive && <Alert style={{ marginTop: 18 }} type="error" showIcon message="商户通道与当前运行适配器不一致" description={`本商户配置为${payment.providerDisplayName}，API 当前运行的是${effectiveProviderNames[payment.effectiveProvider] || payment.effectiveProvider}；在两者一致前无法发起支付。`} />}
          {!payment.adapterImplemented && <Alert style={{ marginTop: 18 }} type="warning" showIcon message={`${payment.providerDisplayName}真实交易适配尚未启用`} description={wechat ? '这里展示的是进件与授权配置。平台完成 API v3 下单、验签解密、查单和退款联调前，不能用于真实收款。' : '当前版本只保留该通道的接口边界，完成官方接口、验签、查单和退款联调前不能用于真实收款。'} />}
          {mock
            ? <Alert style={{ marginTop: 18 }} type="info" showIcon message="当前为模拟支付" description="仅用于开发联调，不会向顾客发起真实扣款，也不会产生结算资金。" />
            : <Alert style={{ marginTop: 18 }} type="success" showIcon message="摊伴不经手顾客资金" description={wechat ? '顾客付款后，资金进入微信支付为该特约商户建立的商户账户，并按商户签约规则结算到其结算账户；摊伴只处理订单与支付状态。' : '顾客付款由支付机构直接受理，并按商户与支付机构的签约规则结算；摊伴只处理订单与支付状态。'} />}
        </Card>
      </Col>
      <Col xs={24} xl={9}>
        <Card bordered={false} className="content-card settings-card" title={<Space><SafetyCertificateOutlined />资金安全边界</Space>}>
          <SettingRow title="回调验签" description="只有验签、金额、商户号和订单身份全部一致才确认支付。" tag={mock ? <Tag color="blue">模拟环境不适用</Tag> : transactionReady ? <CheckCircleOutlined className="safe-text" /> : <Tag color="warning">待通道实现</Tag>} />
          <SettingRow title="主动查单补偿" description="回调丢失时由对账任务向支付机构查询，不凭小程序前端结果认款。" tag={mock ? <Tag color="blue">模拟环境不适用</Tag> : transactionReady ? <CheckCircleOutlined className="safe-text" /> : <Tag color="warning">待通道实现</Tag>} />
          <SettingRow title="幂等与追加支付尝试" description="支付机构流水唯一，同一订单可安全重试但不会重复推进。" tag={payment.providerActive ? <CheckCircleOutlined className="safe-text" /> : <Tag color="warning">通道未生效</Tag>} />
          <SettingRow title="敏感参数" description="商户号、密钥和结算卡由平台管理；商户端只显示脱敏状态。" tag={<Tag color="blue">平台管理</Tag>} />
        </Card>
      </Col>
    </Row>
    <div style={{ marginTop: 16 }}>
      <WechatPayOnboardingCard />
    </div>
    </>
  );
}
