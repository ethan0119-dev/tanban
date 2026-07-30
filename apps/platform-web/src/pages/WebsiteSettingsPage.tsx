import { PictureOutlined, SaveOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Col, Form, Input, Row, Space, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { PageHeader } from '../components/PageHeader';
import { http } from '../lib/api';

interface WebsiteSettings {
  contact_phone: string;
  contact_wechat: string;
  contact_email: string;
  wechat_qr_url: string;
  hero_image_url: string;
  seo_title: string;
  seo_description: string;
  [key: string]: string;
}

export function WebsiteSettingsPage() {
  const [form] = Form.useForm<WebsiteSettings>();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const result = await http.get<WebsiteSettings>('/platform/settings/website');
      form.setFieldsValue(result.data);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, [form, messageApi]);

  useEffect(() => { void load(); }, [load]);

  const save = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      await http.put('/platform/settings/website', values);
      messageApi.success('网站设置已保存');
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      {contextHolder}
      <PageHeader
        title={<Space><PictureOutlined />网站维护</Space>}
        description="管理营销官网的展示内容和联系方式。"
        extra={<Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={() => void save()}>保存设置</Button>}
      />
      <Card bordered={false} loading={loading}>
        <Alert
          type="info"
          showIcon
          message="修改后即时生效"
          description="营销官网（www 站点）会实时读取这些配置，无需重新部署。"
          style={{ marginBottom: 24 }}
        />
        <Form form={form} layout="vertical">
          <Row gutter={24}>
            <Col xs={24} md={12}>
              <Form.Item label="客服电话" name="contact_phone" rules={[{ required: true, message: '请输入客服电话' }]}>
                <Input placeholder="例如：400-888-1234" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label="客服微信说明" name="contact_wechat">
                <Input placeholder="例如：扫码添加客服微信" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={24}>
            <Col xs={24} md={12}>
              <Form.Item label="客服邮箱" name="contact_email" rules={[{ type: 'email', message: '邮箱格式不正确' }]}>
                <Input placeholder="例如：hello@tanban.cn" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label="客服微信二维码图片 URL" name="wechat_qr_url" extra="留空则显示默认占位图。可上传图片后填入链接。">
                <Input placeholder="https://..." />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item label="首页大图 URL" name="hero_image_url" extra="替换首页 Hero 区域的产品截图。建议尺寸 1200x600，留空则使用默认 CSS 模拟图。">
            <Input placeholder="https://..." />
          </Form.Item>
          <Row gutter={24}>
            <Col xs={24} md={12}>
              <Form.Item label="SEO 标题" name="seo_title">
                <Input placeholder="网页标题" />
              </Form.Item>
            </Col>
            <Col xs={24} md={12}>
              <Form.Item label="SEO 描述" name="seo_description">
                <Input placeholder="搜索引擎展示的描述文字" />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Card>
    </div>
  );
}
