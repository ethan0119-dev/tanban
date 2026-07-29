import { EyeOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import { Button, Card, Col, Descriptions, Drawer, Input, Row, Select, Space, Table, Tag, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useState } from 'react';
import { PageHeader } from '../components/PageHeader';
import { auditService } from '../lib/services';
import type { AuditLog, PageMeta } from '../types';
import { formatBeijingDateTime } from '../utils/datetime';

const moduleOptions = [
  { value: 'auth', label: '登录认证' },
  { value: 'user', label: '管理员' },
  { value: 'tenant', label: '商户' },
  { value: 'store', label: '门店' },
  { value: 'payment', label: '支付' },
  { value: 'settings', label: '系统设置' },
];

export function AuditLogsPage() {
  const [rows, setRows] = useState<AuditLog[]>([]);
  const [meta, setMeta] = useState<PageMeta>({ page: 1, pageSize: 20, total: 0 });
  const [keyword, setKeyword] = useState('');
  const [module, setModule] = useState<string>();
  const [loading, setLoading] = useState(false);
  const [selected, setSelected] = useState<AuditLog>();
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async (page = meta.page, pageSize = meta.pageSize) => {
    setLoading(true);
    try {
      const result = await auditService.list({ page, pageSize, keyword, module });
      setRows(result.items);
      setMeta(result.meta);
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '审计日志加载失败');
    } finally {
      setLoading(false);
    }
  }, [keyword, messageApi, meta.page, meta.pageSize, module]);

  useEffect(() => { void load(1); }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: ColumnsType<AuditLog> = [
    { title: '时间', dataIndex: 'createdAt', key: 'createdAt', width: 180, fixed: 'left', render: formatBeijingDateTime },
    { title: '操作人', dataIndex: 'operatorName', key: 'operatorName', width: 130, render: (value) => value || '系统任务' },
    { title: '模块', dataIndex: 'module', key: 'module', width: 110, render: (value) => <Tag>{moduleOptions.find((item) => item.value === value)?.label || value || '其他'}</Tag> },
    { title: '操作', dataIndex: 'action', key: 'action', width: 180 },
    { title: '操作对象', dataIndex: 'target', key: 'target', width: 210, ellipsis: true, render: (value) => value || '—' },
    { title: '来源 IP', dataIndex: 'ip', key: 'ip', width: 150, render: (value) => value || '—' },
    { title: '详情', key: 'detail', fixed: 'right', width: 90, render: (_, record) => <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => setSelected(record)}>查看</Button> },
  ];

  return (
    <div>
      {contextHolder}
      <PageHeader title="审计日志" description="记录平台关键操作，为安全复核和问题追踪提供依据。" extra={<Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>刷新</Button>} />
      <Card bordered={false}>
        <Row gutter={[12, 12]} className="table-toolbar">
          <Col xs={24} md={10} lg={8}><Input allowClear prefix={<SearchOutlined />} placeholder="搜索操作人、操作对象或内容" value={keyword} onChange={(event) => setKeyword(event.target.value)} onPressEnter={() => void load(1)} /></Col>
          <Col xs={12} md={6} lg={4}><Select allowClear placeholder="全部模块" value={module} onChange={setModule} options={moduleOptions} style={{ width: '100%' }} /></Col>
          <Col xs={12} md={8} lg={12}><Space><Button type="primary" icon={<SearchOutlined />} onClick={() => void load(1)}>查询</Button><Button onClick={() => { setKeyword(''); setModule(undefined); setTimeout(() => void load(1), 0); }}>重置</Button></Space></Col>
        </Row>
        <Table<AuditLog>
          rowKey="id"
          columns={columns}
          dataSource={rows}
          loading={loading}
          scroll={{ x: 1060 }}
          pagination={{ current: meta.page, pageSize: meta.pageSize, total: meta.total, showSizeChanger: true, showTotal: (total) => `共 ${total} 条记录`, onChange: (page, pageSize) => void load(page, pageSize) }}
        />
      </Card>
      <Drawer title="审计详情" width={520} open={Boolean(selected)} onClose={() => setSelected(undefined)}>
        {selected && <Descriptions bordered column={1} size="small">
          <Descriptions.Item label="发生时间">{formatBeijingDateTime(selected.createdAt)}</Descriptions.Item>
          <Descriptions.Item label="操作人">{selected.operatorName || '系统任务'} {selected.operatorId ? `（${selected.operatorId}）` : ''}</Descriptions.Item>
          <Descriptions.Item label="业务模块">{moduleOptions.find((item) => item.value === selected.module)?.label || selected.module || '其他'}</Descriptions.Item>
          <Descriptions.Item label="操作名称">{selected.action}</Descriptions.Item>
          <Descriptions.Item label="操作对象">{selected.target || '—'}</Descriptions.Item>
          <Descriptions.Item label="来源 IP">{selected.ip || '—'}</Descriptions.Item>
          <Descriptions.Item label="详细内容"><pre className="audit-detail">{selected.detail || '无附加信息'}</pre></Descriptions.Item>
        </Descriptions>}
      </Drawer>
    </div>
  );
}
