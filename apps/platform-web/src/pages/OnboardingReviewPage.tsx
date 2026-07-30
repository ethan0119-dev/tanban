import { CheckCircleOutlined, CloseCircleOutlined, ReloadOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { Button, Card, Empty, Input, Modal, Space, Table, Tag, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useState } from 'react';
import { PageHeader } from '../components/PageHeader';
import { tenantService } from '../lib/services';
import type { PendingOnboardingApplication } from '../types';

const subjectTypeText: Record<string, string> = {
  MICRO: '小微商户',
  INDIVIDUAL: '个体工商户',
  ENTERPRISE: '企业',
};

export function OnboardingReviewPage() {
  const [items, setItems] = useState<PendingOnboardingApplication[]>([]);
  const [loading, setLoading] = useState(false);
  const [reviewing, setReviewing] = useState<PendingOnboardingApplication>();
  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectNote, setRejectNote] = useState('');
  const [saving, setSaving] = useState(false);
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setItems(await tenantService.listPendingOnboarding());
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, [messageApi]);

  useEffect(() => { void load(); }, [load]);

  const review = async (action: 'approve' | 'reject', note?: string) => {
    if (!reviewing) return;
    setSaving(true);
    try {
      await tenantService.reviewOnboarding(String(reviewing.tenantId), action, note);
      messageApi.success(action === 'approve' ? '已通过审核' : '已驳回');
      setReviewing(undefined);
      setRejectOpen(false);
      setRejectNote('');
      void load();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '操作失败');
    } finally {
      setSaving(false);
    }
  };

  const openReject = (record: PendingOnboardingApplication) => {
    setReviewing(record);
    setRejectNote('');
    setRejectOpen(true);
  };

  const columns: ColumnsType<PendingOnboardingApplication> = [
    {
      title: '商户', key: 'tenant', width: 200,
      render: (_, row) => <div><strong>{row.tenantName}</strong>{row.tenantCode && <div><small style={{ color: '#999' }}>{row.tenantCode}</small></div>}</div>,
    },
    { title: '商户简称', dataIndex: 'merchantShortName', key: 'merchantShortName', width: 150 },
    {
      title: '主体类型', dataIndex: 'subjectType', key: 'subjectType', width: 120,
      render: (value) => <Tag>{subjectTypeText[value] || value}</Tag>,
    },
    { title: '经营者', dataIndex: 'operatorName', key: 'operatorName', width: 110 },
    { title: '联系电话', dataIndex: 'contactPhone', key: 'contactPhone', width: 140 },
    { title: '提交时间', dataIndex: 'submittedAt', key: 'submittedAt', width: 170 },
    {
      title: '操作', key: 'actions', fixed: 'right', width: 180,
      render: (_, record) => (
        <Space>
          <Button
            type="primary"
            size="small"
            icon={<CheckCircleOutlined />}
            loading={saving && reviewing?.tenantId === record.tenantId}
            onClick={() => {
              setReviewing(record);
              void review('approve');
            }}
          >通过</Button>
          <Button
            danger
            size="small"
            icon={<CloseCircleOutlined />}
            onClick={() => openReject(record)}
          >驳回</Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      {contextHolder}
      <PageHeader
        title={<Space><SafetyCertificateOutlined />进件审核</Space>}
        description="审核商户提交的微信支付特约商户进件申请。"
        extra={<Button icon={<ReloadOutlined />} onClick={() => void load()}>刷新</Button>}
      />
      <Card bordered={false}>
        <Table<PendingOnboardingApplication>
          rowKey="tenantId"
          columns={columns}
          dataSource={items}
          loading={loading}
          locale={{ empty: <Empty description="暂无待审核的进件申请" /> }}
          pagination={false}
          scroll={{ x: 1100 }}
        />
      </Card>

      <Modal
        title={`驳回申请 · ${reviewing?.merchantShortName || ''}`}
        open={rejectOpen}
        okText="确认驳回"
        okButtonProps={{ danger: true, disabled: !rejectNote.trim() }}
        onOk={() => void review('reject', rejectNote.trim())}
        onCancel={() => { setRejectOpen(false); setRejectNote(''); }}
        confirmLoading={saving}
      >
        <p>请填写驳回原因，商户将看到此内容并修改后重新提交：</p>
        <Input.TextArea
          rows={4}
          placeholder="例如：经营者姓名与身份证不一致，请核对后重新提交"
          value={rejectNote}
          onChange={(event) => setRejectNote(event.target.value)}
          maxLength={500}
          showCount
        />
      </Modal>
    </div>
  );
}
