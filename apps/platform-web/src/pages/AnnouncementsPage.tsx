import { EditOutlined, EyeOutlined, PlusOutlined, ReloadOutlined, SearchOutlined, SendOutlined, StopOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Col, Descriptions, Drawer, Form, Input, Modal, Popconfirm, Radio, Row, Select, Space, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { PageHeader } from '../components/PageHeader';
import { useAuth } from '../context/AuthContext';
import { canManagePlatformUsers } from '../lib/permissions';
import { announcementService, tenantService } from '../lib/services';
import type { AnnouncementValues, PageMeta, PlatformAnnouncement, Tenant } from '../types';

const categoryOptions = [
  { value: 'SYSTEM_UPDATE', label: '系统迭代' },
  { value: 'BUG_FIX', label: '问题修复' },
  { value: 'NEW_FEATURE', label: '新功能说明' },
  { value: 'NOTICE', label: '注意事项' },
  { value: 'ACTION_REQUIRED', label: '待办提醒' },
];
const categoryText = Object.fromEntries(categoryOptions.map((item) => [item.value, item.label]));
const severityOptions = [{ value: 'INFO', label: '普通' }, { value: 'IMPORTANT', label: '重要' }, { value: 'URGENT', label: '紧急' }];
const severityText: Record<string, string> = { INFO: '普通', IMPORTANT: '重要', URGENT: '紧急' };
const severityColor: Record<string, string> = { INFO: 'blue', IMPORTANT: 'orange', URGENT: 'red' };
const statusText: Record<string, string> = { DRAFT: '草稿', PUBLISHED: '已发布', WITHDRAWN: '已撤回' };
const statusColor: Record<string, string> = { DRAFT: 'default', PUBLISHED: 'success', WITHDRAWN: 'warning' };

function displayTime(value?: string) {
  return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—';
}

export function AnnouncementsPage() {
  const { user } = useAuth();
  const writable = canManagePlatformUsers(user);
  const [rows, setRows] = useState<PlatformAnnouncement[]>([]);
  const [meta, setMeta] = useState<PageMeta>({ page: 1, pageSize: 20, total: 0 });
  const [keyword, setKeyword] = useState('');
  const [status, setStatus] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<PlatformAnnouncement>();
  const [selected, setSelected] = useState<PlatformAnnouncement>();
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [form] = Form.useForm<AnnouncementValues>();
  const [messageApi, holder] = message.useMessage();
  const audienceType = Form.useWatch('audienceType', form);

  const load = useCallback(async (page = meta.page, pageSize = meta.pageSize) => {
    setLoading(true);
    try {
      const result = await announcementService.list({ page, pageSize, keyword, status });
      setRows(result.items);
      setMeta(result.meta);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '通知列表加载失败');
    } finally {
      setLoading(false);
    }
  }, [keyword, messageApi, meta.page, meta.pageSize, status]);

  useEffect(() => { void load(1); }, []); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    tenantService.list({ page: 1, pageSize: 100 }).then((result) => setTenants(result.items)).catch(() => undefined);
  }, []);

  const openEditor = (record?: PlatformAnnouncement) => {
    setEditing(record);
    form.setFieldsValue(record ? {
      title: record.title,
      summary: record.summary,
      content: record.content,
      category: record.category,
      severity: record.severity,
      audienceType: record.audienceType,
      tenantIds: record.tenantIds,
    } : { category: 'SYSTEM_UPDATE', severity: 'INFO', audienceType: 'ALL', tenantIds: [] });
    setEditorOpen(true);
  };

  const save = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      if (editing) await announcementService.update(editing.id, values);
      else await announcementService.create(values);
      messageApi.success(editing ? '通知草稿已更新' : '通知草稿已创建');
      setEditorOpen(false);
      form.resetFields();
      await load(editing ? meta.page : 1);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const publish = async (record: PlatformAnnouncement) => {
    try {
      await announcementService.publish(record.id);
      messageApi.success(`通知已发送给 ${record.targetCount} 个商户`);
      void load();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '发布失败');
    }
  };

  const withdraw = async (record: PlatformAnnouncement) => {
    try {
      await announcementService.withdraw(record.id);
      messageApi.success('通知已撤回，商户收件箱将不再展示');
      void load();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '撤回失败');
    }
  };

  const tenantOptions = useMemo(() => tenants.map((tenant) => ({ value: tenant.id, label: `${tenant.name}（${tenant.code || tenant.id}）` })), [tenants]);
  const columns: ColumnsType<PlatformAnnouncement> = [
    { title: '通知', key: 'title', fixed: 'left', width: 310, render: (_, item) => <div className="announcement-title-cell"><Space><strong>{item.title}</strong>{item.severity !== 'INFO' && <Tag color={severityColor[item.severity]}>{severityText[item.severity]}</Tag>}</Space><small>{item.summary || '未填写摘要'}</small></div> },
    { title: '类型', dataIndex: 'category', width: 120, render: (value) => <Tag>{categoryText[value] || value}</Tag> },
    { title: '状态', dataIndex: 'status', width: 100, render: (value) => <Tag color={statusColor[value]}>{statusText[value] || value}</Tag> },
    { title: '接收范围', key: 'audience', width: 150, render: (_, item) => `${item.audienceType === 'ALL' ? '全部商户' : '指定商户'} · ${item.targetCount} 家` },
    { title: '已读商户', key: 'read', width: 110, align: 'right', render: (_, item) => item.status === 'PUBLISHED' ? `${item.readCount}/${item.targetCount}` : '—' },
    { title: '发布时间', dataIndex: 'publishedAt', width: 180, render: displayTime },
    { title: '操作', key: 'actions', fixed: 'right', width: 230, render: (_, item) => <Space size={2}>
      <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => setSelected(item)}>查看</Button>
      {writable && item.status === 'DRAFT' && <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEditor(item)}>编辑</Button>}
      {writable && item.status === 'DRAFT' && <Popconfirm title="确认发布这条通知？" description={`发布后将发送给 ${item.targetCount} 个商户，正文不可再修改。`} onConfirm={() => void publish(item)}><Button type="link" size="small" icon={<SendOutlined />}>发布</Button></Popconfirm>}
      {writable && item.status === 'PUBLISHED' && <Popconfirm title="确认撤回这条通知？" description="撤回后商户收件箱将立即隐藏该通知。" onConfirm={() => void withdraw(item)}><Button danger type="link" size="small" icon={<StopOutlined />}>撤回</Button></Popconfirm>}
    </Space> },
  ];

  return <div>
    {holder}
    <PageHeader title="通知中心" description="统一向商户发布系统迭代、问题修复、新功能说明和重要注意事项。" extra={<Space><Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>刷新</Button>{writable && <Button type="primary" icon={<PlusOutlined />} onClick={() => openEditor()}>新建通知</Button>}</Space>} />
    {!writable && <Alert type="info" showIcon message="当前为只读运营账号；通知创建、发布和撤回仅限平台管理员。" style={{ marginBottom: 16 }} />}
    <Card bordered={false}>
      <Row gutter={[12, 12]} className="table-toolbar">
        <Col xs={24} md={10} lg={8}><Input allowClear prefix={<SearchOutlined />} placeholder="搜索标题或摘要" value={keyword} onChange={(event) => setKeyword(event.target.value)} onPressEnter={() => void load(1)} /></Col>
        <Col xs={12} md={6} lg={4}><Select allowClear placeholder="全部状态" value={status} onChange={setStatus} style={{ width: '100%' }} options={[{ value: 'DRAFT', label: '草稿' }, { value: 'PUBLISHED', label: '已发布' }, { value: 'WITHDRAWN', label: '已撤回' }]} /></Col>
        <Col xs={12} md={8} lg={12}><Button type="primary" icon={<SearchOutlined />} onClick={() => void load(1)}>查询</Button></Col>
      </Row>
      <Table rowKey="id" columns={columns} dataSource={rows} loading={loading} scroll={{ x: 1200 }} pagination={{ current: meta.page, pageSize: meta.pageSize, total: meta.total, showSizeChanger: true, showTotal: (total) => `共 ${total} 条`, onChange: (page, pageSize) => void load(page, pageSize) }} />
    </Card>
    <Modal title={editing ? '编辑通知草稿' : '新建通知'} width={720} open={editorOpen} onCancel={() => setEditorOpen(false)} onOk={() => void save()} confirmLoading={saving} okText="保存草稿" destroyOnClose>
      <Form form={form} layout="vertical" requiredMark={false} className="modal-form">
        <Form.Item name="title" label="通知标题" rules={[{ required: true, message: '请输入通知标题' }, { max: 160 }]}><Input showCount maxLength={160} /></Form.Item>
        <Form.Item name="summary" label="列表摘要" rules={[{ max: 300 }]}><Input.TextArea rows={2} showCount maxLength={300} placeholder="用一句话说明本次通知重点" /></Form.Item>
        <Row gutter={12}><Col span={12}><Form.Item name="category" label="通知类型" rules={[{ required: true }]}><Select options={categoryOptions} /></Form.Item></Col><Col span={12}><Form.Item name="severity" label="重要程度" rules={[{ required: true }]}><Select options={severityOptions} /></Form.Item></Col></Row>
        <Form.Item name="content" label="通知正文" rules={[{ required: true, message: '请输入通知正文' }, { max: 20000 }]}><Input.TextArea rows={9} showCount maxLength={20000} placeholder="支持换行，建议说明变更内容、影响范围、商户需要执行的动作和生效时间。" /></Form.Item>
        <Form.Item name="audienceType" label="接收范围" rules={[{ required: true }]}><Radio.Group><Radio value="ALL">全部商户</Radio><Radio value="SELECTED">指定商户</Radio></Radio.Group></Form.Item>
        {audienceType === 'SELECTED' && <Form.Item name="tenantIds" label="接收商户" rules={[{ required: true, type: 'array', min: 1, message: '至少选择一个商户' }]}><Select mode="multiple" showSearch optionFilterProp="label" maxTagCount="responsive" options={tenantOptions} placeholder="选择要接收通知的商户" /></Form.Item>}
        <Alert type="warning" showIcon message="发布后正文和接收范围不可修改；如内容有误，可撤回并重新创建通知。" />
      </Form>
    </Modal>
    <Drawer title="通知详情" width={620} open={Boolean(selected)} onClose={() => setSelected(undefined)}>
      {selected && <><Descriptions bordered size="small" column={1}><Descriptions.Item label="标题">{selected.title}</Descriptions.Item><Descriptions.Item label="类型">{categoryText[selected.category]}</Descriptions.Item><Descriptions.Item label="状态"><Tag color={statusColor[selected.status]}>{statusText[selected.status]}</Tag></Descriptions.Item><Descriptions.Item label="重要程度"><Tag color={severityColor[selected.severity]}>{severityText[selected.severity]}</Tag></Descriptions.Item><Descriptions.Item label="接收范围">{selected.audienceType === 'ALL' ? '全部商户' : `指定 ${selected.targetCount} 个商户`}</Descriptions.Item><Descriptions.Item label="发布时间">{displayTime(selected.publishedAt)}</Descriptions.Item></Descriptions><Typography.Title level={5} style={{ marginTop: 24 }}>通知正文</Typography.Title><Typography.Paragraph className="announcement-content">{selected.content}</Typography.Paragraph></>}
    </Drawer>
  </div>;
}
