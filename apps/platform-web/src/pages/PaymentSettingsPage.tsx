import {
  ApiOutlined,
  CheckCircleFilled,
  ExperimentOutlined,
  LockOutlined,
  SafetyCertificateOutlined,
  WechatOutlined,
} from '@ant-design/icons';
import { Alert, Button, Card, Col, Form, Input, Radio, Row, Select, Space, Switch, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { LoadError, PageSkeleton } from '../components/AsyncState';
import { PageHeader } from '../components/PageHeader';
import { settingsService } from '../lib/services';
import type { PaymentSettings } from '../types';

const DEFAULT_NOTIFY_URL = 'https://tbapi.666qwe.cn/api/v1/payments/tianque/callback';
const WECHAT_API_BASE_URL = 'https://api.mch.weixin.qq.com';
const WECHAT_NOTIFY_URL = 'https://tbapi.666qwe.cn/api/v1/payments/wechat-partner/callback';
const WECHAT_REFUND_NOTIFY_URL = 'https://tbapi.666qwe.cn/api/v1/payments/wechat-partner/refund-callback';
const providerNames: Record<string, string> = {
  mock: 'Mock 模拟支付',
  tianque: '会生活 · 随行付',
  wechat_partner: '微信支付（普通服务商）',
};

export function PaymentSettingsPage() {
  const [form] = Form.useForm<PaymentSettings>();
  const [settings, setSettings] = useState<PaymentSettings>();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [messageApi, contextHolder] = message.useMessage();
  const provider = Form.useWatch('provider', form);

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const result = await settingsService.getPayment();
      const value: PaymentSettings = {
        ...result,
        provider: result.provider || 'mock',
        enabled: result.enabled ?? true,
        environment: result.environment || 'sandbox',
        notifyUrl: result.notifyUrl || (result.provider === 'wechat_partner' ? WECHAT_NOTIFY_URL : DEFAULT_NOTIFY_URL),
        refundNotifyUrl: result.refundNotifyUrl || WECHAT_REFUND_NOTIFY_URL,
      };
      setSettings(value);
      form.setFieldsValue(value);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '支付配置加载失败');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => { void load(); }, [load]);

  const selectProvider = (nextProvider: PaymentSettings['provider']) => {
    if (nextProvider === 'wechat_partner') {
      form.setFieldsValue({
        apiBaseUrl: WECHAT_API_BASE_URL,
        notifyUrl: WECHAT_NOTIFY_URL,
        refundNotifyUrl: WECHAT_REFUND_NOTIFY_URL,
        environment: 'production',
      });
    } else if (nextProvider === 'tianque') {
      form.setFieldsValue({
        apiBaseUrl: settings?.provider === 'tianque' ? settings.apiBaseUrl : '',
        notifyUrl: settings?.provider === 'tianque' ? settings.notifyUrl : DEFAULT_NOTIFY_URL,
        refundNotifyUrl: undefined,
      });
    }
  };

  const save = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const result = await settingsService.updatePayment(values);
      setSettings({ ...settings, ...values, ...result });
      messageApi.success('支付服务商配置已保存');
    } catch (saveError) {
      messageApi.error(saveError instanceof Error ? saveError.message : '配置保存失败');
    } finally {
      setSaving(false);
    }
  };

  if (loading && !settings) return <PageSkeleton />;
  const flow = provider === 'wechat_partner'
    ? ['创建待支付订单', '调用服务商小程序下单', '验签并解密支付通知', '主动查单补偿', '更新订单；授权后支持原路退款']
    : provider === 'tianque'
      ? ['创建待支付订单', '调用天阙统一下单', '验签接收支付通知', '主动查询补偿', '更新订单并触发打印']
      : ['创建待支付订单', '生成模拟支付参数', '测试确认支付结果', '更新订单状态', '触发后续业务'];

  return (
    <div>
      {contextHolder}
      <PageHeader title="支付服务商" description="配置平台支付适配器；资金始终由支付机构直接结算给商户。" />
      {error && <div className="section-gap"><LoadError message={error} onRetry={() => void load()} /></div>}
      <Alert
        showIcon
        type="info"
        message="平台不沉淀商户资金"
        description="摊伴只发起支付、验证回调并推动订单状态。微信支付普通服务商模式下，资金归属特约商户，并按商户与微信支付的签约规则结算；平台账户不收取顾客货款。"
        className="settings-alert"
      />
	  {settings?.restartRequired && <Alert
		showIcon
		type="warning"
			message={`配置已保存，但当前运行中的适配器仍为 ${providerNames[settings.effectiveProvider || 'mock'] || settings.effectiveProvider}`}
		description="支付适配器和密钥由服务器环境变量注入；完成部署并重启 API 后才会生效。"
		className="settings-alert"
	  />}
		  {provider === 'tianque' && settings?.tianqueAdapterImplemented === false && <Alert
		showIcon
		type="error"
		message="真实会生活/天阙适配器尚未启用"
		description="当前版本只保留接口边界。取得官方联调文档、机构号、密钥和沙箱权限并完成验签测试前，不可用于真实收款。"
		className="settings-alert"
	  />}
		  {provider === 'wechat_partner' && settings?.wechatPartnerAdapterImplemented === false && <Alert
		showIcon
		type="warning"
		message="微信支付（普通服务商）配置入口已就绪，真实交易适配尚未启用"
		description="可以先维护服务商参数和商户进件状态；在完成 API v3 下单、回调验签解密、查单、退款及沙箱/生产联调前，系统不会把它标记为可真实收款。"
		className="settings-alert"
	  />}

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={16}>
          <Card bordered={false} title="服务商配置">
            <Form form={form} layout="vertical" requiredMark={false} onFinish={() => void save()}>
              <Form.Item label="当前支付适配器" name="provider" rules={[{ required: true }]}>
	                <Radio.Group className="provider-selector" onChange={(event) => selectProvider(event.target.value)}>
                  <Radio.Button value="mock">
                    <span className="provider-option"><ExperimentOutlined /><span><strong>Mock 模拟支付</strong><small>开发调试，无真实资金</small></span></span>
                  </Radio.Button>
                  <Radio.Button value="tianque">
                    <span className="provider-option"><ApiOutlined /><span><strong>天阙开放平台</strong><small>随行付真实支付通道</small></span></span>
                  </Radio.Button>
                  <Radio.Button value="wechat_partner">
                    <span className="provider-option"><WechatOutlined /><span><strong>微信支付（普通服务商）</strong><small>API v3 · 服务商小程序支付</small></span></span>
                  </Radio.Button>
                </Radio.Group>
              </Form.Item>

              <Form.Item label="启用状态" name="enabled" valuePropName="checked">
                <Switch checkedChildren="已启用" unCheckedChildren="已停用" />
              </Form.Item>

              {provider === 'tianque' && <div className="settings-fields">
                <Row gutter={14}>
                  <Col xs={24} md={12}><Form.Item label="运行环境" name="environment" rules={[{ required: true }]}><Select options={[{ value: 'sandbox', label: '测试环境' }, { value: 'production', label: '生产环境' }]} /></Form.Item></Col>
                  <Col xs={24} md={12}><Form.Item label="合作方机构号（orgId）" name="orgId" rules={[{ required: true, message: '请输入天阙 orgId' }]}><Input placeholder="由天阙开放平台分配" /></Form.Item></Col>
                </Row>
                <Form.Item label="接口网关地址" name="apiBaseUrl" rules={[{ required: true, message: '请输入接口网关地址' }, { type: 'url', message: '请输入有效 URL' }]}><Input placeholder="测试与生产地址由天阙对接方提供" /></Form.Item>
                <Form.Item label="支付结果通知地址" name="notifyUrl" rules={[{ required: true, message: '请输入回调地址' }, { type: 'url', message: '请输入有效 URL' }]} extra="该地址必须可被公网 HTTPS 访问；接口需验签、幂等并按协议返回成功响应。"><Input /></Form.Item>
              </div>}
              {provider === 'wechat_partner' && <div className="settings-fields">
                <Alert type="info" showIcon message="平台参数与商户参数分开管理" description="这里填写服务商号与服务商小程序 AppID；每个商户的 sub_mchid、进件和授权状态在“商户管理 → 支付配置”维护。" style={{ marginBottom: 16 }} />
                <Row gutter={14}>
                  <Col xs={24} md={12}><Form.Item label="服务商商户号（sp_mchid）" name="spMchId" rules={[{ required: true, message: '请输入服务商商户号' }, { pattern: /^\d{8,32}$/, message: '请输入 8 至 32 位数字' }]}><Input placeholder="微信支付分配的服务商商户号" /></Form.Item></Col>
                  <Col xs={24} md={12}><Form.Item label="服务商小程序 AppID（sp_appid）" name="spAppId" rules={[{ required: true, message: '请输入服务商小程序 AppID' }, { pattern: /^wx[a-zA-Z0-9]{16}$/, message: 'AppID 格式不正确' }]}><Input placeholder="须已绑定至服务商商户号" /></Form.Item></Col>
                </Row>
                <Form.Item label="微信支付 API 地址" name="apiBaseUrl"><Input disabled placeholder="https://api.mch.weixin.qq.com" /></Form.Item>
                <Form.Item label="支付结果通知地址" name="notifyUrl" rules={[{ required: true, message: '请输入支付通知地址' }, { type: 'url', message: '请输入有效 URL' }]}><Input /></Form.Item>
                <Form.Item label="退款结果通知地址" name="refundNotifyUrl" rules={[{ required: true, message: '请输入退款通知地址' }, { type: 'url', message: '请输入有效 URL' }]}><Input /></Form.Item>
              </div>}

              <Space><Button type="primary" htmlType="submit" loading={saving}>保存配置</Button><Button onClick={() => form.setFieldsValue(settings || {})}>撤销修改</Button></Space>
            </Form>
          </Card>
        </Col>

        <Col xs={24} xl={8}>
          <Card bordered={false} title="密钥与接入状态" className="credential-card">
            {provider === 'wechat_partner' ? <>
              <div className="credential-row"><span><SafetyCertificateOutlined /> 商户 API 证书序列号</span>{settings?.apiCertSerialConfigured ? <Tag icon={<CheckCircleFilled />} color="success">已配置</Tag> : <Tag>未配置</Tag>}</div>
              <div className="credential-row"><span><LockOutlined /> 商户 API 私钥</span>{settings?.privateKeyConfigured ? <Tag icon={<CheckCircleFilled />} color="success">已配置</Tag> : <Tag>未配置</Tag>}</div>
              <div className="credential-row"><span><LockOutlined /> APIv3 密钥</span>{settings?.apiV3KeyConfigured ? <Tag icon={<CheckCircleFilled />} color="success">已配置</Tag> : <Tag>未配置</Tag>}</div>
              <div className="credential-row"><span><SafetyCertificateOutlined /> 微信支付公钥</span>{settings?.wechatPayPublicKeyConfigured ? <Tag icon={<CheckCircleFilled />} color="success">已配置</Tag> : <Tag>未配置</Tag>}</div>
            </> : <>
              <div className="credential-row"><span><SafetyCertificateOutlined /> 平台公钥</span>{settings?.publicKeyConfigured ? <Tag icon={<CheckCircleFilled />} color="success">已配置</Tag> : <Tag>未配置</Tag>}</div>
              <div className="credential-row"><span><LockOutlined /> 平台私钥</span>{settings?.privateKeyConfigured ? <Tag icon={<CheckCircleFilled />} color="success">已配置</Tag> : <Tag>未配置</Tag>}</div>
            </>}
            <Alert type="warning" showIcon message="管理端不展示密钥原文" description="私钥只应通过服务器环境变量或密钥管理服务注入，API 也不得返回原始内容。" />
          </Card>
          <Card bordered={false} title="处理链路" className="section-gap payment-flow-card">
            {flow.map((item, index) => <div className="payment-step" key={item}><span>{index + 1}</span><Typography.Text>{item}</Typography.Text></div>)}
          </Card>
        </Col>
      </Row>
    </div>
  );
}
