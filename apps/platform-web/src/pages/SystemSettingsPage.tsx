import { ClockCircleOutlined, CustomerServiceOutlined, SafetyOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Col, Form, Input, InputNumber, Row, Space, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { LoadError, PageSkeleton } from '../components/AsyncState';
import { PageHeader } from '../components/PageHeader';
import { settingsService } from '../lib/services';
import type { SystemSettings } from '../types';

export function SystemSettingsPage() {
  const [form] = Form.useForm<SystemSettings>();
  const [settings, setSettings] = useState<SystemSettings>();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const result = await settingsService.getSystem();
      const values = { platformName: '摊伴餐饮系统', marketingTitle: '让每一家小店，都能轻松拥有自己的数字化点餐系统', marketingSubtitle: '点餐、营销、会员与门店经营，一套系统顺畅连接。', marketingPageUrl: '', orderExpireMinutes: 15, loginFailureLimit: 5, sessionExpireMinutes: 720, ...result };
      setSettings(values);
      form.setFieldsValue(values);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '系统配置加载失败');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => { void load(); }, [load]);

  const save = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const result = await settingsService.updateSystem(values);
      setSettings({ ...values, ...result });
      messageApi.success('系统设置已保存');
    } catch (saveError) {
      messageApi.error(saveError instanceof Error ? saveError.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  if (loading && !settings) return <PageSkeleton />;

  return (
    <div>
      {contextHolder}
      <PageHeader title="系统设置" description="维护平台基础信息和通用安全策略。" />
      {error && <div className="section-gap"><LoadError message={error} onRetry={() => void load()} /></div>}
      <Form form={form} layout="vertical" requiredMark={false} onFinish={() => void save()}>
        <Row gutter={[16, 16]}>
          <Col xs={24} xl={12}>
            <Card bordered={false} title={<span><CustomerServiceOutlined /> 基础信息</span>}>
              <Form.Item label="平台名称" name="platformName" rules={[{ required: true, message: '请输入平台名称' }]}><Input /></Form.Item>
              <Row gutter={12}>
                <Col xs={24} md={12}><Form.Item label="客服电话" name="supportPhone"><Input placeholder="商户服务电话" /></Form.Item></Col>
                <Col xs={24} md={12}><Form.Item label="客服邮箱" name="supportEmail" rules={[{ type: 'email', message: '邮箱格式不正确' }]}><Input placeholder="support@example.com" /></Form.Item></Col>
              </Row>
              <Alert type="info" showIcon message="这些信息会用于商户后台的帮助与支持入口。" />
              <div style={{ marginTop: 20 }}>
                <Form.Item label="版权说明页主标题" name="marketingTitle" rules={[{ required: true, message: '请输入宣传标题' }]}><Input maxLength={80} /></Form.Item>
                <Form.Item label="版权说明页简介" name="marketingSubtitle"><Input.TextArea rows={3} maxLength={200} showCount /></Form.Item>
                <Form.Item label="版权说明 HTML 页面地址" name="marketingPageUrl" rules={[{ type: 'url', message: '请输入完整的 HTTPS 页面地址' }]}><Input placeholder="https://tanban-saas.liuxiaoyicn.chatgpt.site/copyright" /></Form.Item>
                <Row gutter={12}>
                  <Col xs={24} md={12}><Form.Item label="联系微信号" name="contactWechat"><Input placeholder="用于顾客咨询" /></Form.Item></Col>
                  <Col xs={24} md={12}><Form.Item label="微信二维码图片地址" name="contactQrUrl" rules={[{ type: 'url', message: '请输入完整的 HTTPS 图片地址' }]}><Input placeholder="https://..." /></Form.Item></Col>
                </Row>
                <Alert type="info" showIcon message="以上内容会展示在顾客小程序的“版权说明”页面，二维码由平台统一配置，不属于商户配置。" />
              </div>
            </Card>
          </Col>
          <Col xs={24} xl={12}>
            <Card bordered={false} title={<span><SafetyOutlined /> 安全与业务策略</span>}>
              <Row gutter={12}>
                <Col xs={24} md={8}><Form.Item label="订单支付时限" name="orderExpireMinutes" rules={[{ required: true }]}><InputNumber min={1} max={1440} addonAfter="分钟" style={{ width: '100%' }} /></Form.Item></Col>
                <Col xs={24} md={8}><Form.Item label="登录失败限制" name="loginFailureLimit" rules={[{ required: true }]}><InputNumber min={3} max={20} addonAfter="次" style={{ width: '100%' }} /></Form.Item></Col>
                <Col xs={24} md={8}><Form.Item label="会话有效期" name="sessionExpireMinutes" rules={[{ required: true }]}><InputNumber min={30} max={10080} addonAfter="分钟" style={{ width: '100%' }} /></Form.Item></Col>
              </Row>
              <Alert icon={<ClockCircleOutlined />} type="warning" showIcon message="订单支付时限变更只影响新创建订单，已存在订单仍沿用创建时的策略。" />
            </Card>
          </Col>
        </Row>
        <Space className="settings-actions"><Button type="primary" htmlType="submit" loading={saving}>保存设置</Button><Button onClick={() => form.setFieldsValue(settings || {})}>撤销修改</Button></Space>
      </Form>
    </div>
  );
}
