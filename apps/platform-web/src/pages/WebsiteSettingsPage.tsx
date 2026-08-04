import {
  CloudUploadOutlined,
  CopyOutlined,
  DeleteOutlined,
  GlobalOutlined,
  PictureOutlined,
  PhoneOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Col,
  Form,
  Image,
  Input,
  Popconfirm,
  Row,
  Space,
  Spin,
  Tabs,
  Upload,
  message,
} from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { PageHeader } from '../components/PageHeader';
import { useAuth } from '../context/AuthContext';
import { canManagePlatformUsers } from '../lib/permissions';
import { websiteService } from '../lib/services';
import type { WebsiteMedia, WebsiteSettings } from '../types';
import { isValidWebsiteImageURL, websiteImagePreviewURL } from '../features/website/urls';

type ImageFieldProps = {
  label: string;
  name: keyof WebsiteSettings;
  hint: string;
  form: ReturnType<typeof Form.useForm<WebsiteSettings>>[0];
  writable: boolean;
  onUploaded: () => void;
};

function WebsiteImageField({ label, name, hint, form, writable, onUploaded }: ImageFieldProps) {
  const value = Form.useWatch(name, form) as string | undefined;
  const [uploading, setUploading] = useState(false);
  const [messageApi, holder] = message.useMessage();

  return (
    <Form.Item label={label} required>
      {holder}
      <div className="website-image-field">
        <div className="website-image-field__preview">
          {value ? <Image src={websiteImagePreviewURL(value)} alt={label} fallback="/favicon.svg" /> : <PictureOutlined />}
        </div>
        <div className="website-image-field__controls">
          <Form.Item name={name} noStyle rules={[{
            validator: (_, fieldValue: string | undefined) => isValidWebsiteImageURL(fieldValue || '')
              ? Promise.resolve()
              : Promise.reject(new Error('请输入 HTTPS 图片地址或 /website/ 开头的官网内置图片路径')),
          }]}>
            <Input placeholder="上传图片后自动填写，也可粘贴完整图片地址" disabled={!writable} />
          </Form.Item>
          <Space wrap>
            <Upload
              accept="image/jpeg,image/png,image/gif"
              showUploadList={false}
              disabled={!writable || uploading}
              beforeUpload={async (file) => {
                setUploading(true);
                try {
                  const media = await websiteService.uploadMedia(file, file.name, label);
                  form.setFieldValue(name, media.url);
                  messageApi.success(`${label}上传成功`);
                  onUploaded();
                } catch (error) {
                  messageApi.error(error instanceof Error ? error.message : '图片上传失败');
                } finally {
                  setUploading(false);
                }
                return Upload.LIST_IGNORE;
              }}
            >
              <Button icon={<CloudUploadOutlined />} loading={uploading} disabled={!writable}>上传图片</Button>
            </Upload>
            {value && <Button onClick={() => form.setFieldValue(name, '')} disabled={!writable}>清除</Button>}
          </Space>
          <small>{hint}，支持 JPG、PNG、GIF，单张不超过 8 MB。</small>
        </div>
      </div>
    </Form.Item>
  );
}

export function WebsiteSettingsPage() {
  const { user } = useAuth();
  const writable = canManagePlatformUsers(user);
  const [form] = Form.useForm<WebsiteSettings>();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [mediaLoading, setMediaLoading] = useState(false);
  const [media, setMedia] = useState<WebsiteMedia[]>([]);
  const [messageApi, holder] = message.useMessage();

  const loadSettings = useCallback(async () => {
    setLoading(true);
    try {
      form.setFieldsValue(await websiteService.getSettings());
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '官网设置加载失败');
    } finally {
      setLoading(false);
    }
  }, [form, messageApi]);

  const loadMedia = useCallback(async () => {
    setMediaLoading(true);
    try {
      const result = await websiteService.listMedia({ page: 1, pageSize: 100 });
      setMedia(result.items);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '图片素材加载失败');
    } finally {
      setMediaLoading(false);
    }
  }, [messageApi]);

  useEffect(() => {
    void Promise.all([loadSettings(), loadMedia()]);
  }, [loadMedia, loadSettings]);

  const save = async (values: WebsiteSettings) => {
    setSaving(true);
    try {
      const result = await websiteService.updateSettings(values);
      form.setFieldsValue(result);
      messageApi.success('官网内容已保存，前台刷新后生效');
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const deleteMedia = async (item: WebsiteMedia) => {
    try {
      await websiteService.deleteMedia(item.id);
      messageApi.success('图片已从素材库移除');
      void loadMedia();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '图片删除失败');
    }
  };

  const contentTab = (
    <Spin spinning={loading}>
      <Form form={form} layout="vertical" onFinish={save} disabled={!writable} requiredMark={false}>
        <Row gutter={[16, 16]}>
          <Col xs={24} xl={14}>
            <Card bordered={false} title={<span><GlobalOutlined /> 首页首屏</span>}>
              <Row gutter={12}>
                <Col xs={24} md={12}><Form.Item name="brandName" label="品牌中文名" rules={[{ required: true }, { max: 80 }]}><Input /></Form.Item></Col>
                <Col xs={24} md={12}><Form.Item name="brandEnglishName" label="品牌英文名" rules={[{ required: true }, { max: 40 }]}><Input /></Form.Item></Col>
              </Row>
              <Form.Item name="heroEyebrow" label="首屏眉题" rules={[{ required: true }, { max: 100 }]}><Input /></Form.Item>
              <Row gutter={12}>
                <Col xs={24} md={12}><Form.Item name="heroTitle" label="首屏主标题" rules={[{ required: true }, { max: 120 }]}><Input /></Form.Item></Col>
                <Col xs={24} md={12}><Form.Item name="heroHighlight" label="高亮标题" rules={[{ required: true }, { max: 120 }]}><Input /></Form.Item></Col>
              </Row>
              <Form.Item name="heroSubtitle" label="首屏说明" rules={[{ max: 500 }]}><Input.TextArea rows={4} showCount maxLength={500} /></Form.Item>
              <WebsiteImageField label="首屏主图" name="heroImageUrl" hint="建议比例 16:9、宽度不低于 1600px" form={form} writable={writable} onUploaded={loadMedia} />
            </Card>
          </Col>
          <Col xs={24} xl={10}>
            <Card bordered={false} title={<span><PhoneOutlined /> 客服与联系信息</span>}>
              <Form.Item name="supportPhone" label="客服电话" rules={[{ max: 40 }]}><Input placeholder="400-..." /></Form.Item>
              <Form.Item name="supportEmail" label="客服邮箱" rules={[{ type: 'email', warningOnly: true }, { max: 120 }]}><Input /></Form.Item>
              <Form.Item name="contactWechat" label="客服微信号" rules={[{ max: 80 }]}><Input /></Form.Item>
              <WebsiteImageField label="客服小程序二维码" name="contactQrUrl" hint="建议上传正方形二维码原图" form={form} writable={writable} onUploaded={loadMedia} />
              <Alert type="info" showIcon message="客服电话和二维码会展示在官网底部的咨询区域；保存前只会保留在当前表单。" />
            </Card>
          </Col>
          <Col xs={24} xl={12}>
            <Card bordered={false} title="产品能力图片">
              <WebsiteImageField label="顾客扫码点单" name="scanOrderImageUrl" hint="建议比例 2:1、宽度不低于 1200px" form={form} writable={writable} onUploaded={loadMedia} />
              <WebsiteImageField label="店员平板收银" name="cashierImageUrl" hint="建议比例 2:1、宽度不低于 1200px" form={form} writable={writable} onUploaded={loadMedia} />
              <WebsiteImageField label="后厨打印与取餐" name="kitchenImageUrl" hint="建议比例 2:1、宽度不低于 1200px" form={form} writable={writable} onUploaded={loadMedia} />
            </Card>
          </Col>
          <Col xs={24} xl={12}>
            <Card bordered={false} title="经营场景图片">
              <WebsiteImageField label="早餐摊" name="sceneBreakfastImageUrl" hint="建议比例 4:3" form={form} writable={writable} onUploaded={loadMedia} />
              <WebsiteImageField label="咖啡车" name="sceneCoffeeTruckImageUrl" hint="建议比例 4:3" form={form} writable={writable} onUploaded={loadMedia} />
              <WebsiteImageField label="小蛋糕店" name="sceneBakeryImageUrl" hint="建议比例 4:3" form={form} writable={writable} onUploaded={loadMedia} />
              <WebsiteImageField label="夜市小吃" name="sceneNightMarketImageUrl" hint="建议比例 4:3" form={form} writable={writable} onUploaded={loadMedia} />
              <WebsiteImageField label="小餐馆" name="sceneCafeImageUrl" hint="建议比例 4:3" form={form} writable={writable} onUploaded={loadMedia} />
            </Card>
          </Col>
          <Col xs={24} xl={12}>
            <Card bordered={false} title="公司与页脚">
              <Row gutter={12}>
                <Col span={12}><Form.Item name="companyName" label="公司名称" rules={[{ max: 120 }]}><Input /></Form.Item></Col>
                <Col span={12}><Form.Item name="companyAddress" label="公司地址" rules={[{ max: 200 }]}><Input /></Form.Item></Col>
              </Row>
              <Form.Item name="footerText" label="页脚品牌文案" rules={[{ max: 200 }]}><Input /></Form.Item>
              <Form.Item name="icpNumber" label="ICP备案号" rules={[{ max: 80 }]}><Input placeholder="取得备案后填写" /></Form.Item>
              <Form.Item name="merchantLoginUrl" label="商户登录地址" rules={[{ type: 'url', warningOnly: true }]}><Input /></Form.Item>
            </Card>
          </Col>
          <Col xs={24} xl={12}>
            <Card bordered={false} title="搜索与分享">
              <Form.Item name="metaTitle" label="网页标题" rules={[{ max: 160 }]}><Input showCount maxLength={160} /></Form.Item>
              <Form.Item name="metaDescription" label="网页描述" rules={[{ max: 300 }]}><Input.TextArea rows={4} showCount maxLength={300} /></Form.Item>
              <Alert type="warning" showIcon message="搜索引擎标题和描述需要在官网发布流程中同步生成，当前字段已作为统一内容源保存。" />
            </Card>
          </Col>
        </Row>
        {writable && <Space className="settings-actions">
          <Button type="primary" htmlType="submit" loading={saving}>保存官网设置</Button>
          <Button onClick={() => void loadSettings()}>撤销修改</Button>
        </Space>}
      </Form>
    </Spin>
  );

  const mediaTab = (
    <Card bordered={false}>
      <div className="website-media-toolbar">
        <Alert type="info" showIcon message="在“页面内容”中上传的图片会自动进入素材库；删除正在使用的图片会被系统阻止。" />
        <Button icon={<ReloadOutlined />} loading={mediaLoading} onClick={() => void loadMedia()}>刷新</Button>
      </div>
      <Spin spinning={mediaLoading}>
        <div className="website-media-grid">
          {media.map((item) => (
            <article key={item.id}>
              <Image src={item.url} alt={item.altText || item.name} />
              <div><strong>{item.name}</strong><small>{item.width} × {item.height} · {(item.sizeBytes / 1024 / 1024).toFixed(2)} MB</small></div>
              <Space>
                <Button size="small" icon={<CopyOutlined />} onClick={() => navigator.clipboard.writeText(item.url).then(() => messageApi.success('图片地址已复制'))}>复制地址</Button>
                {writable && <Popconfirm title="确认删除这张图片？" description="正在被官网使用的图片不会被删除。" onConfirm={() => void deleteMedia(item)}><Button danger size="small" icon={<DeleteOutlined />} /></Popconfirm>}
              </Space>
            </article>
          ))}
          {!mediaLoading && media.length === 0 && <div className="website-media-empty"><PictureOutlined /><span>还没有官网图片，请先在页面内容中上传。</span></div>}
        </div>
      </Spin>
    </Card>
  );

  return <div>
    {holder}
    <PageHeader title="官网内容" description="维护首页品牌文案、官网图片、客服电话和客服小程序二维码。" extra={<Button icon={<ReloadOutlined />} onClick={() => void Promise.all([loadSettings(), loadMedia()])}>刷新</Button>} />
    {!writable && <Alert type="info" showIcon message="当前为只读运营账号；官网内容修改仅限平台管理员。" style={{ marginBottom: 16 }} />}
    <Tabs items={[{ key: 'content', label: '页面内容', children: contentTab }, { key: 'media', label: `图片素材（${media.length}）`, children: mediaTab }]} />
  </div>;
}
