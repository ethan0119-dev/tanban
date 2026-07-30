import { CheckCircleOutlined, CloseCircleOutlined, ExclamationCircleOutlined, ReloadOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { Button, Card, Empty, Input, Modal, Space, Table, Tabs, Tag, message } from 'antd';
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

const statusText: Record<string, string> = {
  PENDING_PLATFORM_REVIEW: '待审核',
  SUBMITTED_TO_WECHAT: '已通过',
  NEEDS_INFO: '已驳回',
  FINISHED: '已开通',
};

const statusColor: Record<string, string> = {
  PENDING_PLATFORM_REVIEW: 'processing',
  SUBMITTED_TO_WECHAT: 'success',
  NEEDS_INFO: 'error',
  FINISHED: 'success',
};

export function OnboardingReviewPage() {
  const [tab, setTab] = useState<'pending' | 'history'>('pending');
  const [pendingItems, setPendingItems] = useState<PendingOnboardingApplication[]>([]);
  const [historyItems, setHistoryItems] = useState<PendingOnboardingApplication[]>([]);
  const [loading, setLoading] = useState(false);
  const [reviewing, setReviewing] = useState<PendingOnboardingApplication>();
  const [approveOpen, setApproveOpen] = useState(false);
  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectNote, setRejectNote] = useState('');
  const [saving, setSaving] = useState(false);
  const [messageApi, contextHolder] = message.useMessage();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [pending, history] = await Promise.all([
        tenantService.listPendingOnboarding('PENDING_PLATFORM_REVIEW'),
        tenantService.listPendingOnboarding(),
      ]);
      setPendingItems(pending);
      setHistoryItems(history.filter((item) => item.applicationStatus !== 'PENDING_PLATFORM_REVIEW'));
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
      setApproveOpen(false);
      setRejectOpen(false);
      setRejectNote('');
      void load();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '操作失败');
    } finally {
      setSaving(false);
    }
  };

  const openApprove = (record: PendingOnboardingApplication) => {
    setReviewing(record);
    setApproveOpen(true);
  };

  const openReject = (record: PendingOnboardingApplication) => {
    setReviewing(record);
    setRejectNote('');
    setRejectOpen(true);
  };

  const pendingColumns: ColumnsType<PendingOnboardingApplication> = [
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
            onClick={() => openApprove(record)}
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

  const historyColumns: ColumnsType<PendingOnboardingApplication> = [
    {
      title: '商户', key: 'tenant', width: 200,
      render: (_, row) => <div><strong>{row.tenantName}</strong>{row.tenantCode && <div><small style={{ color: '#999' }}>{row.tenantCode}</small></div>}</div>,
    },
    { title: '商户简称', dataIndex: 'merchantShortName', key: 'merchantShortName', width: 140 },
    {
      title: '主体类型', dataIndex: 'subjectType', key: 'subjectType', width: 110,
      render: (value) => <Tag>{subjectTypeText[value] || value}</Tag>,
    },
    { title: '经营者', dataIndex: 'operatorName', key: 'operatorName', width: 100 },
    {
      title: '审核结果', dataIndex: 'applicationStatus', key: 'applicationStatus', width: 100,
      render: (value) => <Tag color={statusColor[value] || 'default'}>{statusText[value] || value}</Tag>,
    },
    {
      title: '驳回原因', dataIndex: 'platformNote', key: 'platformNote', width: 200, ellipsis: true,
      render: (value) => value || '—',
    },
    { title: '提交时间', dataIndex: 'submittedAt', key: 'submittedAt', width: 160 },
    { title: '审核时间', dataIndex: 'updatedAt', key: 'updatedAt', width: 160 },
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
        <Tabs
          activeKey={tab}
          onChange={(key) => setTab(key as 'pending' | 'history')}
          items={[
            {
              key: 'pending',
              label: <>待审核{pendingItems.length > 0 && <Tag color="processing" style={{ marginLeft: 6 }}>{pendingItems.length}</Tag>}</>,
              children: (
                <Table<PendingOnboardingApplication>
                  rowKey="tenantId"
                  columns={pendingColumns}
                  dataSource={pendingItems}
                  loading={loading}
                  locale={{ empty: <Empty description="暂无待审核的进件申请" /> }}
                  pagination={false}
                  scroll={{ x: 1100 }}
                />
              ),
            },
            {
              key: 'history',
              label: '审核记录',
              children: (
                <Table<PendingOnboardingApplication>
                  rowKey="tenantId"
                  columns={historyColumns}
                  dataSource={historyItems}
                  loading={loading}
                  locale={{ empty: <Empty description="暂无审核记录" /> }}
                  pagination={false}
                  scroll={{ x: 1200 }}
                />
              ),
            },
          ]}
        />
      </Card>

      <Modal
        title={<Space><ExclamationCircleOutlined style={{ color: '#faad14' }} />确认通过审核</Space>}
        open={approveOpen}
        okText="确认通过"
        onOk={() => void review('approve')}
        onCancel={() => setApproveOpen(false)}
        confirmLoading={saving}
      >
        {reviewing && (
          <div>
            <p>确认通过以下商户的微信支付进件申请？</p>
            <p><strong>商户：</strong>{reviewing.tenantName}</p>
            <p><strong>商户简称：</strong>{reviewing.merchantShortName}</p>
            <p><strong>主体类型：</strong>{subjectTypeText[reviewing.subjectType] || reviewing.subjectType}</p>
            <p><strong>经营者：</strong>{reviewing.operatorName}</p>
            <p style={{ color: '#999', fontSize: 13 }}>通过后申请状态将更新为"微信支付审核中"，商户端将看到最新状态。</p>
          </div>
        )}
      </Modal>

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
