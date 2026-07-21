import { CopyOutlined, DeleteOutlined, EditOutlined, LinkOutlined, PlusOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Empty, Form, Image, Input, Modal, Popconfirm, Select, Space, Tag, Typography } from 'antd';
import { useEffect } from 'react';
import type { MediaAsset } from '../../features/decoration/model';

interface AssetLibraryPanelProps {
  assets: MediaAsset[];
  loading: boolean;
  saving: boolean;
  onCreate: (asset: Pick<MediaAsset, 'name' | 'url' | 'type'>) => Promise<void>;
  onUpdate: (id: string | number, asset: Pick<MediaAsset, 'name' | 'url' | 'type'>) => Promise<void>;
  onDelete: (asset: MediaAsset) => Promise<void>;
  onUse: (asset: MediaAsset, target: 'BANNER' | 'SPLASH') => void;
  editing: MediaAsset | null;
  modalOpen: boolean;
  onOpen: (asset?: MediaAsset) => void;
  onClose: () => void;
}

interface AssetFormValues {
  name: string;
  url: string;
  type: 'IMAGE' | 'VIDEO';
}

export function AssetLibraryPanel(props: AssetLibraryPanelProps) {
  const [form] = Form.useForm<AssetFormValues>();

  useEffect(() => {
    if (!props.modalOpen) return;
    form.setFieldsValue(props.editing ? {
      name: props.editing.name,
      url: props.editing.url,
      type: props.editing.type,
    } : { name: '', url: '', type: 'IMAGE' });
  }, [form, props.editing, props.modalOpen]);

  const submit = async () => {
    const values = await form.validateFields();
    if (props.editing) await props.onUpdate(props.editing.id, values);
    else await props.onCreate(values);
  };

  return (
    <div className="decoration-panel-stack">
      <div className="decor-panel-intro with-action">
        <div><Typography.Title level={4}>素材管理</Typography.Title><Typography.Paragraph>统一维护门店常用图片，可用于首页、启动页和活动页面。</Typography.Paragraph></div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => props.onOpen()}>录入素材</Button>
      </div>
      <Alert showIcon type="info" message="请使用长期有效的 HTTPS 图片地址" description="若图片无法显示，请联系平台管理员检查图片访问设置。" />
      {!props.loading && !props.assets.length
        ? <Card className="decor-section-card"><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有素材，先录入 Logo 或 Banner 图片地址" /></Card>
        : <div className="asset-grid">
          {props.assets.map((asset) => (
            <Card key={asset.id} size="small" loading={props.loading} className="asset-card" cover={asset.type === 'IMAGE' ? <Image height={130} preview src={asset.url} alt={asset.name} fallback={transparentPixel} /> : <div className="asset-video-placeholder">VIDEO</div>}>
              <div className="asset-title"><strong>{asset.name}</strong><Tag>{asset.type === 'IMAGE' ? '图片' : '视频'}</Tag></div>
              <Typography.Text ellipsis copyable={{ text: asset.url }} className="asset-url">{asset.url}</Typography.Text>
              <Space wrap className="asset-actions">
                <Select<'BANNER' | 'SPLASH'>
                  size="small"
                  placeholder="应用到…"
                  style={{ width: 105 }}
                  options={[{ label: '首页 Banner', value: 'BANNER' }, { label: '启动页', value: 'SPLASH' }]}
                  onChange={(target) => props.onUse(asset, target)}
                />
                <Button size="small" icon={<CopyOutlined />} onClick={() => void navigator.clipboard.writeText(asset.url)}>复制</Button>
                <Button size="small" icon={<EditOutlined />} onClick={() => props.onOpen(asset)} />
                <Popconfirm title="删除这个素材？" description="正在被商品、装修、营销或门店使用的图片不可删除，请先解除引用。" onConfirm={() => void props.onDelete(asset)}>
                  <Button size="small" danger icon={<DeleteOutlined />} />
                </Popconfirm>
              </Space>
            </Card>
          ))}
        </div>}

      <Modal
        title={props.editing ? '编辑素材' : '录入 URL 素材'}
        open={props.modalOpen}
        confirmLoading={props.saving}
        okText={props.editing ? '保存修改' : '保存素材'}
        onOk={() => void submit()}
        onCancel={props.onClose}
        destroyOnClose
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item name="name" label="素材名称" rules={[{ required: true, message: '请输入素材名称' }]}><Input maxLength={40} showCount placeholder="例如：夏日活动 Banner" /></Form.Item>
          <Form.Item name="type" hidden><Input /></Form.Item>
          <Form.Item name="url" label="HTTPS 地址" rules={[{ required: true, message: '请输入素材地址' }, { type: 'url', message: '请输入完整 URL' }, { pattern: /^https:\/\//i, message: '小程序素材必须使用 HTTPS' }]}>
            <Input prefix={<LinkOutlined />} placeholder="https://..." />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}

const transparentPixel = 'data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=';
