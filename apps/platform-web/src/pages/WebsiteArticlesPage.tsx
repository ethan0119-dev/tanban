import {
  DeleteOutlined,
  EditOutlined,
  EyeOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  SendOutlined,
  StopOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Drawer,
  Form,
  Image,
  Input,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { pinyin } from 'pinyin-pro';
import { useCallback, useEffect, useState } from 'react';
import { PageHeader } from '../components/PageHeader';
import { useAuth } from '../context/AuthContext';
import { canManagePlatformUsers } from '../lib/permissions';
import { websiteService } from '../lib/services';
import type { PageMeta, WebsiteArticle, WebsiteArticleValues } from '../types';
import { formatBeijingDateTime } from '../utils/datetime';

const statusText: Record<string, string> = { DRAFT: '草稿', PUBLISHED: '已发布', WITHDRAWN: '已撤回' };
const statusColor: Record<string, string> = { DRAFT: 'default', PUBLISHED: 'success', WITHDRAWN: 'warning' };

function slugFromTitle(title: string) {
  return pinyin(title, { toneType: 'none', type: 'array' })
    .join('-')
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
    .slice(0, 180);
}
export function WebsiteArticlesPage() {
  const { user } = useAuth();
  const writable = canManagePlatformUsers(user);
  const [rows, setRows] = useState<WebsiteArticle[]>([]);
  const [meta, setMeta] = useState<PageMeta>({ page: 1, pageSize: 20, total: 0 });
  const [keyword, setKeyword] = useState('');
  const [status, setStatus] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<WebsiteArticle>();
  const [selected, setSelected] = useState<WebsiteArticle>();
  const [form] = Form.useForm<WebsiteArticleValues>();
  const [messageApi, holder] = message.useMessage();

  const load = useCallback(async (page = meta.page, pageSize = meta.pageSize) => {
    setLoading(true);
    try {
      const result = await websiteService.listArticles({ page, pageSize, keyword, status });
      setRows(result.items);
      setMeta(result.meta);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '官网动态加载失败');
    } finally {
      setLoading(false);
    }
  }, [keyword, messageApi, meta.page, meta.pageSize, status]);

  useEffect(() => { void load(1); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const openEditor = (record?: WebsiteArticle) => {
    setEditing(record);
    form.setFieldsValue(record ? {
      slug: record.slug,
      title: record.title,
      summary: record.summary,
      coverUrl: record.coverUrl,
      content: record.content,
      isFeatured: record.isFeatured,
    } : { slug: '', title: '', summary: '', coverUrl: '', content: '', isFeatured: false });
    setEditorOpen(true);
  };

  const save = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      if (editing) await websiteService.updateArticle(editing.id, values);
      else await websiteService.createArticle(values);
      messageApi.success(editing ? '动态草稿已更新' : '动态草稿已创建');
      setEditorOpen(false);
      form.resetFields();
      await load(editing ? meta.page : 1);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const changeStatus = async (record: WebsiteArticle, action: 'publish' | 'withdraw') => {
    try {
      if (action === 'publish') await websiteService.publishArticle(record.id);
      else await websiteService.withdrawArticle(record.id);
      messageApi.success(action === 'publish' ? '动态已发布到官网' : '动态已从官网撤回');
      void load();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '状态修改失败');
    }
  };

  const remove = async (record: WebsiteArticle) => {
    try {
      await websiteService.deleteArticle(record.id);
      messageApi.success('动态已删除');
      void load(1);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '删除失败');
    }
  };

  const columns: ColumnsType<WebsiteArticle> = [
    { title: '动态', key: 'article', fixed: 'left', width: 390, render: (_, item) => <div className="announcement-title-cell"><Space><strong>{item.title}</strong>{item.isFeatured && <Tag color="gold">首页推荐</Tag>}</Space><small>{item.summary || '未填写摘要'}</small></div> },
    { title: '访问路径', dataIndex: 'slug', width: 220, render: (value) => <Typography.Text code>/news/{value}</Typography.Text> },
    { title: '状态', dataIndex: 'status', width: 100, render: (value) => <Tag color={statusColor[value]}>{statusText[value] || value}</Tag> },
    { title: '发布时间', dataIndex: 'publishedAt', width: 170, render: (value) => formatBeijingDateTime(value) },
    { title: '更新时间', dataIndex: 'updatedAt', width: 170, render: (value) => formatBeijingDateTime(value) },
    { title: '操作', key: 'actions', fixed: 'right', width: 270, render: (_, item) => <Space size={2}>
      <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => setSelected(item)}>查看</Button>
      {writable && item.status !== 'PUBLISHED' && <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEditor(item)}>编辑</Button>}
      {writable && item.status !== 'PUBLISHED' && <Popconfirm title="确认发布这条动态？" description="发布后会立即显示在官网。" onConfirm={() => void changeStatus(item, 'publish')}><Button type="link" size="small" icon={<SendOutlined />}>发布</Button></Popconfirm>}
      {writable && item.status === 'PUBLISHED' && <Popconfirm title="确认撤回这条动态？" onConfirm={() => void changeStatus(item, 'withdraw')}><Button danger type="link" size="small" icon={<StopOutlined />}>撤回</Button></Popconfirm>}
      {writable && item.status !== 'PUBLISHED' && <Popconfirm title="确认删除这条动态？" onConfirm={() => void remove(item)}><Button danger type="link" size="small" icon={<DeleteOutlined />} /></Popconfirm>}
    </Space> },
  ];

  return <div>
    {holder}
    <PageHeader title="官网动态" description="创建和发布面向官网访客的品牌动态，与商户通知中心相互独立。" extra={<Space><Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>刷新</Button>{writable && <Button type="primary" icon={<PlusOutlined />} onClick={() => openEditor()}>新建动态</Button>}</Space>} />
    {!writable && <Alert type="info" showIcon message="当前为只读运营账号；动态编辑、发布和撤回仅限平台管理员。" style={{ marginBottom: 16 }} />}
    <Card bordered={false}>
      <Row gutter={[12, 12]} className="table-toolbar">
        <Col xs={24} md={10} lg={8}><Input allowClear prefix={<SearchOutlined />} placeholder="搜索标题或摘要" value={keyword} onChange={(event) => setKeyword(event.target.value)} onPressEnter={() => void load(1)} /></Col>
        <Col xs={12} md={6} lg={4}><Select allowClear placeholder="全部状态" value={status} onChange={setStatus} style={{ width: '100%' }} options={[{ value: 'DRAFT', label: '草稿' }, { value: 'PUBLISHED', label: '已发布' }, { value: 'WITHDRAWN', label: '已撤回' }]} /></Col>
        <Col xs={12} md={8} lg={12}><Button type="primary" icon={<SearchOutlined />} onClick={() => void load(1)}>查询</Button></Col>
      </Row>
      <Table rowKey="id" columns={columns} dataSource={rows} loading={loading} scroll={{ x: 1320 }} pagination={{ current: meta.page, pageSize: meta.pageSize, total: meta.total, showSizeChanger: true, showTotal: (total) => `共 ${total} 条`, onChange: (page, pageSize) => void load(page, pageSize) }} />
    </Card>
    <Modal title={editing ? '编辑官网动态' : '新建官网动态'} width={760} open={editorOpen} onCancel={() => setEditorOpen(false)} onOk={() => void save()} confirmLoading={saving} okText="保存草稿" destroyOnClose>
      <Form form={form} layout="vertical" requiredMark={false} className="modal-form">
        <Form.Item name="title" label="动态标题" rules={[{ required: true, message: '请输入动态标题' }, { max: 180 }]}>
          <Input showCount maxLength={180} onBlur={(event) => { if (!form.getFieldValue('slug')) form.setFieldValue('slug', slugFromTitle(event.target.value)); }} />
        </Form.Item>
        <Form.Item name="slug" label="访问路径" extra="仅支持小写字母、数字和单个短横线，发布后建议不再修改。" rules={[{ required: true }, { pattern: /^[a-z0-9]+(?:-[a-z0-9]+)*$/, message: '请输入有效的英文访问路径' }, { max: 180 }]}><Input addonBefore="/news/" /></Form.Item>
        <Form.Item name="summary" label="列表摘要" rules={[{ max: 400 }]}><Input.TextArea rows={2} showCount maxLength={400} /></Form.Item>
        <Form.Item name="coverUrl" label="封面图片地址" rules={[{ type: 'url', warningOnly: true }]}><Input placeholder="可从官网图片素材库复制图片地址" /></Form.Item>
        <Form.Item name="content" label="动态正文" rules={[{ required: true, message: '请输入动态正文' }, { max: 50000 }]}><Input.TextArea rows={12} showCount maxLength={50000} placeholder="支持换行；当前版本按自然段安全展示，不执行 HTML。" /></Form.Item>
        <Form.Item name="isFeatured" label="首页推荐" valuePropName="checked"><Switch checkedChildren="推荐" unCheckedChildren="普通" /></Form.Item>
        <Alert type="info" showIcon message="保存后仍为草稿；点击列表中的“发布”才会显示到官网。" />
      </Form>
    </Modal>
    <Drawer title="官网动态预览" width={680} open={Boolean(selected)} onClose={() => setSelected(undefined)}>
      {selected && <div className="website-article-preview">
        <Descriptions bordered size="small" column={1}>
          <Descriptions.Item label="状态"><Tag color={statusColor[selected.status]}>{statusText[selected.status]}</Tag></Descriptions.Item>
          <Descriptions.Item label="访问路径">/news/{selected.slug}</Descriptions.Item>
          <Descriptions.Item label="发布时间">{formatBeijingDateTime(selected.publishedAt)}</Descriptions.Item>
        </Descriptions>
        {selected.coverUrl && <Image src={selected.coverUrl} alt="" />}
        <Typography.Title level={2}>{selected.title}</Typography.Title>
        <Typography.Paragraph type="secondary">{selected.summary}</Typography.Paragraph>
        <div className="website-article-preview__body">{selected.content.split(/\n{2,}/).map((paragraph) => <Typography.Paragraph key={paragraph}>{paragraph}</Typography.Paragraph>)}</div>
      </div>}
    </Drawer>
  </div>;
}
