import { ApiOutlined, CloudServerOutlined, LockOutlined, SyncOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Col, Form, Input, Row, Space, Switch, Tag, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { LoadError, PageSkeleton } from '../components/AsyncState';
import { PageHeader } from '../components/PageHeader';
import { settingsService } from '../lib/services';
import type { PrinterProviderSettings } from '../types';

interface XPYunForm {
  enabled: boolean;
  developerId: string;
  secret?: string;
  baseUrl: string;
}

export function PrinterProvidersPage() {
  const [form] = Form.useForm<XPYunForm>();
  const [settings, setSettings] = useState<PrinterProviderSettings>();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [error, setError] = useState('');
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const providers = await settingsService.getPrinterProviders();
      const xpyun = providers.find((item) => item.provider === 'xpyun');
      if (!xpyun) throw new Error('芯烨云服务商配置不存在');
      setSettings(xpyun);
      form.setFieldsValue({ enabled: xpyun.enabled, developerId: xpyun.developerId, secret: '', baseUrl: xpyun.baseUrl });
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : '打印服务商配置加载失败');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => { void load(); }, [load]);

  const save = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const result = await settingsService.updateXPYun(values);
      setSettings(result);
      form.setFieldValue('secret', '');
      messageApi.success(`芯烨云配置已保存；设备同步成功 ${result.synced ?? 0} 台，失败 ${result.syncFailed ?? 0} 台`);
    } catch (saveError) {
      messageApi.error(saveError instanceof Error ? saveError.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const test = async () => {
    setTesting(true);
    try {
      const result = await settingsService.testXPYun();
      const text = `${result.deviceName}：${result.message}`;
      if (result.status === 'UNREACHABLE') messageApi.error(text); else messageApi.success(text);
    } catch (testError) {
      messageApi.error(testError instanceof Error ? testError.message : '连接测试失败');
    } finally {
      setTesting(false);
    }
  };

  if (loading && !settings) return <PageSkeleton />;

  return <div>
    {contextHolder}
    <PageHeader title="打印服务商" description="统一管理云打印开发者凭据；商户只需录入品牌、型号和 SN，系统会自动注册设备。" />
    {error && <LoadError message={error} onRetry={() => void load()} />}
    <Alert className="section-gap" type="info" showIcon message="扩展设计" description="每个厂商拥有独立适配器、凭据和设备同步逻辑。后续接入飞鹅等品牌时无需修改商户打印机模型。" />
    <Row gutter={[16, 16]} className="section-gap">
      <Col xs={24} xl={16}>
        <Card bordered={false} title={<Space><CloudServerOutlined />芯烨云 <Tag color={settings?.configured ? 'success' : 'default'}>{settings?.configured ? '已配置' : '未配置'}</Tag></Space>}>
          <Form form={form} layout="vertical" requiredMark={false} onFinish={() => void save()}>
            <Form.Item label="启用芯烨云" name="enabled" valuePropName="checked"><Switch /></Form.Item>
            <Form.Item label="开发者 ID（user）" name="developerId" rules={[{ required: true, message: '请输入芯烨云开发者 ID' }]}><Input prefix={<ApiOutlined />} autoComplete="off" /></Form.Item>
            <Form.Item label="开发者密钥（UserKEY）" name="secret" extra={settings?.secretSet ? '密钥已安全保存；留空表示不修改。系统永不回显密钥明文。' : '首次启用时必须输入。'}>
              <Input.Password prefix={<LockOutlined />} placeholder={settings?.secretSet ? '已设置，留空不修改' : '请输入 UserKEY'} autoComplete="new-password" />
            </Form.Item>
            <Form.Item label="API 地址" name="baseUrl" rules={[{ required: true }]}><Input /></Form.Item>
            <Space><Button type="primary" htmlType="submit" loading={saving}>保存并同步设备</Button><Button icon={<SyncOutlined />} loading={testing} disabled={!settings?.configured} onClick={() => void test()}>测试连接</Button></Space>
          </Form>
        </Card>
      </Col>
      <Col xs={24} xl={8}>
        <Card bordered={false} title="设备录入规则">
          <Typography.Paragraph>商户保存芯烨打印机时，系统会先查询 SN：</Typography.Paragraph>
          <Typography.Paragraph>1. 已在当前开发者账号下：直接读取在线状态。</Typography.Paragraph>
          <Typography.Paragraph>2. 尚未注册：自动调用芯烨云接口注册。</Typography.Paragraph>
          <Typography.Paragraph>3. 属于其他开发者账号：提示解绑或转移，不会重复抢占。</Typography.Paragraph>
        </Card>
      </Col>
    </Row>
  </div>;
}
