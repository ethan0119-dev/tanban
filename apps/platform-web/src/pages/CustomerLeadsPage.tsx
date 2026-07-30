import { ReloadOutlined, SearchOutlined, UserAddOutlined } from '@ant-design/icons';
import { Button, Card, Col, Input, Modal, Row, Select, Space, Table, Tag, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useState } from 'react';
import { PageHeader } from '../components/PageHeader';
import { http, normalizePage } from '../lib/api';
import type { PageMeta } from '../types';

interface CustomerLead {
  id: number;
  name: string;
  phone: string;
  source: string;
  status: string;
  note: string;
  ipAddress: string;
  createdAt: string;
  updatedAt: string;
}

const statusText: Record<string, string> = {
  NEW: '新线索',
  CONTACTED: '已联系',
  CONVERTED: '已转化',
  CLOSED: '已关闭',
};

const statusColor: Record<string, string> = {
  NEW: 'processing',
  CONTACTED: 'warning',
  CONVERTED: 'success',
  CLOSED: 'default',
};

export function CustomerLeadsPage() {
  const [items, setItems] = useState<CustomerLead[]>([]);
  const [meta, setMeta] = useState<PageMeta>({ page: 1, pageSize: 20, total: 0 });
  const [keyword, setKeyword] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<CustomerLead>();
  const [editStatus, setEditStatus] = useState('');
  const [editNote, setEditNote] = useState('');
  const [saving, setSaving] = useState(false);
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async (page = meta.page, pageSize = meta.pageSize) => {
    setLoading(true);
    try {
      const params: Record<string, string | number | undefined> = { page, page_size: pageSize };
      if (keyword) params.q = keyword;
      if (statusFilter) params.status = statusFilter;
      const result = await http.get<CustomerLead[]>('/platform/leads', params);
      const paged = normalizePage<CustomerLead>(result.data, result.meta, page, pageSize);
      setItems(paged.items);
      setMeta(paged.meta);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, [keyword, statusFilter, messageApi, meta.page, meta.pageSize]);

  useEffect(() => { void load(1); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const openEdit = (record: CustomerLead) => {
    setEditing(record);
    setEditStatus(record.status);
    setEditNote(record.note || '');
  };

  const saveEdit = async () => {
    if (!editing) return;
    setSaving(true);
    try {
      await http.put(`/platform/leads/${editing.id}`, { status: editStatus, note: editNote });
      messageApi.success('已更新');
      setEditing(undefined);
      void load();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '更新失败');
    } finally {
      setSaving(false);
    }
  };

  const columns: ColumnsType<CustomerLead> = [
    { title: '姓名', dataIndex: 'name', key: 'name', width: 120 },
    { title: '手机号', dataIndex: 'phone', key: 'phone', width: 140 },
    { title: '来源', dataIndex: 'source', key: 'source', width: 100 },
    {
      title: '状态', dataIndex: 'status', key: 'status', width: 100,
      render: (value) => <Tag color={statusColor[value] || 'default'}>{statusText[value] || value}</Tag>,
    },
    {
      title: '备注', dataIndex: 'note', key: 'note', width: 200, ellipsis: true,
      render: (value) => value || '—',
    },
    { title: 'IP', dataIndex: 'ipAddress', key: 'ipAddress', width: 130, render: (v) => v || '—' },
    { title: '提交时间', dataIndex: 'createdAt', key: 'createdAt', width: 170 },
    {
      title: '操作', key: 'actions', width: 100,
      render: (_, record) => (
        <Button type="link" size="small" onClick={() => openEdit(record)}>处理</Button>
      ),
    },
  ];

  return (
    <div>
      {contextHolder}
      <PageHeader
        title={<Space><UserAddOutlined />客户线索</Space>}
        description="营销官网提交的客户登记信息。"
        extra={<Button icon={<ReloadOutlined />} onClick={() => void load()}>刷新</Button>}
      />
      <Card bordered={false}>
        <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
          <Col xs={24} md={8}>
            <Input
              allowClear
              prefix={<SearchOutlined />}
              placeholder="搜索姓名或手机号"
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
              onPressEnter={() => void load(1)}
            />
          </Col>
          <Col xs={12} md={4}>
            <Select
              allowClear
              placeholder="全部状态"
              value={statusFilter}
              onChange={setStatusFilter}
              options={Object.entries(statusText).map(([value, label]) => ({ value, label }))}
              style={{ width: '100%' }}
            />
          </Col>
          <Col xs={12} md={4}>
            <Button type="primary" icon={<SearchOutlined />} onClick={() => void load(1)}>查询</Button>
          </Col>
        </Row>
        <Table<CustomerLead>
          rowKey="id"
          columns={columns}
          dataSource={items}
          loading={loading}
          scroll={{ x: 1100 }}
          pagination={{
            current: meta.page,
            pageSize: meta.pageSize,
            total: meta.total,
            showSizeChanger: true,
            showTotal: (total) => `共 ${total} 条线索`,
            onChange: (page, pageSize) => void load(page, pageSize),
          }}
        />
      </Card>

      <Modal
        title={`处理线索 · ${editing?.name || ''}`}
        open={Boolean(editing)}
        okText="保存"
        onOk={() => void saveEdit()}
        onCancel={() => setEditing(undefined)}
        confirmLoading={saving}
      >
        {editing && (
          <div>
            <p><strong>手机号：</strong>{editing.phone}</p>
            <p><strong>提交时间：</strong>{editing.createdAt}</p>
            <div style={{ marginTop: 16 }}>
              <label style={{ display: 'block', marginBottom: 8, fontWeight: 500 }}>处理状态</label>
              <Select
                value={editStatus}
                onChange={setEditStatus}
                options={Object.entries(statusText).map(([value, label]) => ({ value, label }))}
                style={{ width: '100%' }}
              />
            </div>
            <div style={{ marginTop: 16 }}>
              <label style={{ display: 'block', marginBottom: 8, fontWeight: 500 }}>备注</label>
              <Input.TextArea
                rows={3}
                placeholder="记录跟进情况..."
                value={editNote}
                onChange={(e) => setEditNote(e.target.value)}
                maxLength={500}
              />
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
