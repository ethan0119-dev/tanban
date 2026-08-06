import { CheckCircleOutlined, CloseCircleOutlined, ExclamationCircleOutlined, ReloadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Checkbox, Col, Descriptions, Empty, Image, Input, Modal, Row, Space, Spin, Table, Tabs, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useState } from 'react';
import { PageHeader } from '../components/PageHeader';
import { tenantService } from '../lib/services';
import type { OnboardingReviewDetail, PendingOnboardingApplication } from '../types';

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
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<OnboardingReviewDetail>();
  const [materialsConfirmed, setMaterialsConfirmed] = useState(false);
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
      await tenantService.reviewOnboarding(String(reviewing.tenantId), action, note, action === 'approve' && materialsConfirmed);
      messageApi.success(action === 'approve' ? '已通过审核' : '已驳回');
      setReviewing(undefined);
      setApproveOpen(false);
      setRejectOpen(false);
      setRejectNote('');
      setDetailOpen(false);
      setDetail(undefined);
      setMaterialsConfirmed(false);
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

  const openDetail = async (record: PendingOnboardingApplication) => {
    setReviewing(record);
    setDetailOpen(true);
    setDetail(undefined);
    setMaterialsConfirmed(false);
    setDetailLoading(true);
    try {
      setDetail(await tenantService.getOnboardingReviewDetail(String(record.tenantId)));
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '加载完整进件资料失败');
    } finally {
      setDetailLoading(false);
    }
  };

  const openReject = (record: PendingOnboardingApplication) => {
    setReviewing(record);
    setRejectNote('');
    setRejectOpen(true);
  };

  const pendingColumns: ColumnsType<PendingOnboardingApplication> = [
    {
      title: '商户', key: 'tenant', width: 200,
      render: (_, row) => <div><Button type="link" style={{ padding: 0, height: 'auto', fontWeight: 600 }} onClick={() => void openDetail(row)}>{row.tenantName}</Button>{row.tenantCode && <div><small style={{ color: '#999' }}>{row.tenantCode}</small></div>}</div>,
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
          <Button type="primary" size="small" onClick={() => void openDetail(record)}>查看资料</Button>
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
        title="进件审核"
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
                  locale={{ emptyText: <Empty description="暂无待审核的进件申请" /> }}
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
                  locale={{ emptyText: <Empty description="暂无审核记录" /> }}
                  pagination={false}
                  scroll={{ x: 1200 }}
                />
              ),
            },
          ]}
        />
      </Card>

      <Modal
        title={`进件资料 · ${detail?.tenantName || reviewing?.tenantName || ''}`}
        open={detailOpen}
        width={1080}
        onCancel={() => { setDetailOpen(false); setMaterialsConfirmed(false); }}
        footer={detail ? <Space>
          <Button danger onClick={() => { setRejectNote(''); setRejectOpen(true); }}>驳回补充</Button>
          <Button
            type="primary"
            icon={<CheckCircleOutlined />}
            disabled={!detail.reviewReady || !materialsConfirmed}
            onClick={() => reviewing && openApprove(reviewing)}
          >确认通过并递交微信</Button>
        </Space> : null}
      >
        {detailLoading ? <div style={{ padding: 72, textAlign: 'center' }}><Spin tip="正在解密并校验商户资料" /></div> : detail ? <Space direction="vertical" size={20} style={{ width: '100%' }}>
          {detail.reviewReady ? <Alert type="success" showIcon message="系统校验通过：必填字段和审核图片副本齐全" description="系统只能判断资料是否存在；证件内容、姓名一致性、照片清晰度和经营场景真实性仍需人工核对。" /> : <Alert
            type="error"
            showIcon
            message={`系统发现 ${detail.missingItems.length} 项资料缺失，当前不能通过`}
            description={<ul style={{ marginBottom: 0 }}>{detail.missingItems.map((item) => <li key={item}>{item}</li>)}</ul>}
          />}
          <Alert type="warning" showIcon message="敏感资料仅限进件审核使用" description="本次查看已记录安全审计日志。请勿截图、下载或向无关人员传播身份证、银行卡和经营材料。" />

          <div>
            <Typography.Title level={5}>主体与联系方式</Typography.Title>
            <Descriptions bordered size="small" column={2}>
              <Descriptions.Item label="租户编号">{detail.tenantCode || '—'}</Descriptions.Item>
              <Descriptions.Item label="申请状态">{statusText[detail.application.applicationStatus] || detail.application.applicationStatus}</Descriptions.Item>
              <Descriptions.Item label="主体类型">{subjectTypeText[detail.application.subjectType] || detail.application.subjectType}</Descriptions.Item>
              <Descriptions.Item label="经营场景">{detail.application.businessScene === 'MOBILE' ? '流动摊位／便民服务' : '固定门店'}</Descriptions.Item>
              <Descriptions.Item label="商户简称">{detail.application.merchantShortName || '—'}</Descriptions.Item>
              <Descriptions.Item label="客服电话">{detail.application.servicePhone || '—'}</Descriptions.Item>
              <Descriptions.Item label="实际经营地址" span={2}>{detail.application.businessAddress || '—'}</Descriptions.Item>
              <Descriptions.Item label="经营者／法定代表人">{detail.application.operatorName || '—'}</Descriptions.Item>
              <Descriptions.Item label="联系电话">{detail.application.contactPhone || '—'}</Descriptions.Item>
              <Descriptions.Item label="联系邮箱">{detail.application.contactEmail || '—'}</Descriptions.Item>
              <Descriptions.Item label="统一社会信用代码">{detail.application.licenseNumber || '不适用'}</Descriptions.Item>
              <Descriptions.Item label="资料真实性确认">{detail.application.qualificationConfirmed ? '已确认' : '未确认'}</Descriptions.Item>
              <Descriptions.Item label="身份证材料">{detail.application.identityMaterialReady ? '已确认备齐' : '未确认'}</Descriptions.Item>
              <Descriptions.Item label="结算账户材料">{detail.application.settlementAccountReady ? '已确认备齐' : '未确认'}</Descriptions.Item>
              <Descriptions.Item label="经营场景材料">{detail.application.businessMaterialReady ? '已确认备齐' : '未确认'}</Descriptions.Item>
              <Descriptions.Item label="提交时间">{detail.application.submittedAt || '—'}</Descriptions.Item>
              <Descriptions.Item label="最近更新时间">{detail.application.updatedAt || '—'}</Descriptions.Item>
              {detail.application.platformNote && <Descriptions.Item label="平台反馈" span={2}>{detail.application.platformNote}</Descriptions.Item>}
            </Descriptions>
          </div>

          <div>
            <Typography.Title level={5}>证件与结算账户</Typography.Title>
            <Descriptions bordered size="small" column={2}>
              <Descriptions.Item label="证件姓名">{detail.sensitive.idCardName || '—'}</Descriptions.Item>
              <Descriptions.Item label="身份证号码">{detail.sensitive.idCardNumber || '—'}</Descriptions.Item>
              <Descriptions.Item label="身份证住址" span={2}>{detail.sensitive.idCardAddress || '—'}</Descriptions.Item>
              <Descriptions.Item label="证件有效期">{detail.sensitive.cardPeriodBegin || '—'} 至 {detail.sensitive.cardPeriodEnd || '—'}</Descriptions.Item>
              <Descriptions.Item label="营业执照主体">{detail.sensitive.merchantName || '不适用'}</Descriptions.Item>
              <Descriptions.Item label="账户类型">{detail.sensitive.accountType === 'BANK_ACCOUNT_TYPE_CORPORATE' ? '对公账户' : '经营者个人账户'}</Descriptions.Item>
              <Descriptions.Item label="开户名称">{detail.sensitive.accountName || '—'}</Descriptions.Item>
              <Descriptions.Item label="银行账号">{detail.sensitive.accountNumber || '—'}</Descriptions.Item>
              <Descriptions.Item label="开户银行">{detail.sensitive.accountBank || '—'}</Descriptions.Item>
              <Descriptions.Item label="支行／联行号">{detail.sensitive.bankName || detail.sensitive.bankBranchId || '—'}</Descriptions.Item>
              <Descriptions.Item label="银行地区编码">{detail.sensitive.bankAddressCode || '—'}</Descriptions.Item>
            </Descriptions>
          </div>

          <div>
            <Typography.Title level={5}>经营与结算规则</Typography.Title>
            <Descriptions bordered size="small" column={2}>
              <Descriptions.Item label="门店名称">{detail.sensitive.storeName || '—'}</Descriptions.Item>
              <Descriptions.Item label="门店地区编码">{detail.sensitive.storeAddressCode || '—'}</Descriptions.Item>
              <Descriptions.Item label="结算规则 ID">{detail.sensitive.settlementId || '—'}</Descriptions.Item>
              <Descriptions.Item label="所属行业">{detail.sensitive.qualificationType || '—'}</Descriptions.Item>
            </Descriptions>
          </div>

          <div>
            <Typography.Title level={5}>证件与经营照片</Typography.Title>
            {detail.media.length === 0 ? <Empty description="暂无可供人工预审的加密图片副本，请驳回并要求商户重新上传" /> : <Image.PreviewGroup>
              <Row gutter={[16, 16]}>
                {detail.media.map((item) => <Col xs={24} sm={12} md={8} key={item.fieldName}>
                  <Card size="small" title={item.label || item.fieldName}>
                    <Image src={item.dataUrl} alt={item.label} style={{ width: '100%', height: 180, objectFit: 'contain', background: '#f5f5f5' }} />
                  </Card>
                </Col>)}
              </Row>
            </Image.PreviewGroup>}
          </div>

          <Checkbox checked={materialsConfirmed} disabled={!detail.reviewReady} onChange={(event) => setMaterialsConfirmed(event.target.checked)}>
            我已逐项核对主体名称、经营者身份、结算账户、证件有效期和全部照片，确认内容清晰、真实且相互一致
          </Checkbox>
        </Space> : <Empty description="完整资料加载失败" />}
      </Modal>

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
            <p style={{ color: '#999', fontSize: 13 }}>确认后将立即调用微信支付进件接口；只有微信成功接收申请，状态才会更新为“微信支付审核中”。</p>
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
