import { CheckCircleOutlined, CloseCircleOutlined, EditOutlined, ExclamationCircleOutlined, ReloadOutlined, SaveOutlined, UploadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Checkbox, Col, Descriptions, Divider, Empty, Form, Image, Input, Modal, Radio, Row, Select, Space, Spin, Table, Tabs, Tag, Typography, Upload, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useCallback, useEffect, useState } from 'react';
import { PageHeader } from '../components/PageHeader';
import { tenantService } from '../lib/services';
import type { OnboardingFieldMapping, OnboardingReviewDetail, PendingOnboardingApplication } from '../types';

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
	const [editForm] = Form.useForm<Pick<OnboardingReviewDetail, 'application' | 'sensitive'>>();
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
  const [editing, setEditing] = useState(false);
  const [editSaving, setEditSaving] = useState(false);
  const [uploadingField, setUploadingField] = useState('');
  const [messageApi, contextHolder] = message.useMessage();
  const editedSubjectType = Form.useWatch(['application', 'subjectType'], editForm);
  const editedSettlementId = Form.useWatch(['sensitive', 'settlementId'], editForm);
  const editedIndustry = Form.useWatch(['sensitive', 'qualificationType'], editForm);

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
      setEditing(false);
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
    setEditing(false);
    setDetailLoading(true);
    try {
      const loaded = await tenantService.getOnboardingReviewDetail(String(record.tenantId));
      setDetail(loaded);
      editForm.setFieldsValue({ application: loaded.application, sensitive: loaded.sensitive });
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '加载完整进件资料失败');
    } finally {
      setDetailLoading(false);
    }
  };

  const saveDetail = async () => {
    if (!reviewing) return;
    const values = await editForm.validateFields();
    setEditSaving(true);
    try {
      const updated = await tenantService.updateOnboardingReviewDetail(String(reviewing.tenantId), values);
      setDetail(updated);
      editForm.setFieldsValue({ application: updated.application, sensitive: updated.sensitive });
      setEditing(false);
      setMaterialsConfirmed(false);
      messageApi.success('进件资料已加密保存并重新校验');
      void load();
    } catch (error) {
      messageApi.error(error instanceof Error ? error.message : '保存进件资料失败');
    } finally {
      setEditSaving(false);
    }
  };

  const replaceMedia = (field: string) => async (options: { file: unknown; onSuccess?: (body?: unknown) => void; onError?: (error: Error) => void }) => {
    if (!reviewing) return;
    setUploadingField(field);
    try {
      const updated = await tenantService.uploadOnboardingReviewMedia(String(reviewing.tenantId), field, options.file as File);
      setDetail(updated);
      editForm.setFieldsValue({ application: updated.application, sensitive: updated.sensitive });
      setMaterialsConfirmed(false);
      options.onSuccess?.(updated);
      messageApi.success('图片已重新上传微信并替换审核副本');
    } catch (error) {
      options.onError?.(error instanceof Error ? error : new Error('上传失败'));
      messageApi.error(error instanceof Error ? error.message : '替换图片失败');
    } finally {
      setUploadingField('');
    }
  };

  const mappingExtra = (mappingByField: Record<string, OnboardingFieldMapping>, localField: string) => {
    const mapping = mappingByField[localField];
    if (!mapping) return undefined;
    return <span>微信字段：<Typography.Text code>{mapping.wechatPath}</Typography.Text>{mapping.sensitive && <Tag color="warning" style={{ marginLeft: 6 }}>加密传输</Tag>}</span>;
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

  const fieldMappingByLocalField = Object.fromEntries((detail?.fieldMappings || []).map((item) => [item.localField, item]));
  const availableIndustries = (detail?.industryOptions || []).filter((item) => item.subjectType === (editedSubjectType || detail?.application.subjectType));
  const selectedIndustryRequirement = availableIndustries.find((item) => item.settlementId === editedSettlementId && item.industry === editedIndustry) || (!editing ? detail?.selectedIndustryRequirement : undefined);
  const mediaByField = Object.fromEntries((detail?.media || []).map((item) => [item.fieldName, item]));

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
        width={1180}
        onCancel={() => { setDetailOpen(false); setMaterialsConfirmed(false); setEditing(false); }}
        footer={detail ? <Space>
          <Button danger onClick={() => { setRejectNote(''); setRejectOpen(true); }}>驳回补充</Button>
          {editing ? <>
            <Button onClick={() => { setEditing(false); editForm.setFieldsValue({ application: detail.application, sensitive: detail.sensitive }); }}>取消编辑</Button>
            <Button type="primary" icon={<SaveOutlined />} loading={editSaving} onClick={() => void saveDetail()}>保存并重新校验</Button>
          </> : <Button icon={<EditOutlined />} onClick={() => setEditing(true)}>编辑全部资料</Button>}
          <Button
            type="primary"
            icon={<CheckCircleOutlined />}
            disabled={editing || !detail.reviewReady || !materialsConfirmed}
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
          {detail.submissionBlockers.length > 0 && <Alert
            type="error"
            showIcon
            message="当前存在平台权限阻断，不能递交微信"
            description={<ul style={{ marginBottom: 0 }}>{detail.submissionBlockers.map((item) => <li key={item}>{item}</li>)}</ul>}
          />}
          {detail.warnings.map((item) => <Alert key={item} type="warning" showIcon message={item} />)}
          <Alert type="warning" showIcon message="敏感资料仅限进件审核使用" description="本次查看已记录安全审计日志。请勿截图、下载或向无关人员传播身份证、银行卡和经营材料。" />

          <Descriptions bordered size="small" column={2}>
            <Descriptions.Item label="租户编号">{detail.tenantCode || '—'}</Descriptions.Item>
            <Descriptions.Item label="申请状态">{statusText[detail.application.applicationStatus] || detail.application.applicationStatus}</Descriptions.Item>
            <Descriptions.Item label="提交时间">{detail.application.submittedAt || '—'}</Descriptions.Item>
            <Descriptions.Item label="最近更新时间">{detail.application.updatedAt || '—'}</Descriptions.Item>
          </Descriptions>

          <Form form={editForm} layout="vertical" disabled={!editing} style={{ width: '100%' }}>
            <Typography.Title level={5}>主体与联系方式</Typography.Title>
            <Row gutter={16}>
              <Col xs={24} md={12}><Form.Item name={['application', 'subjectType']} label="主体类型" extra={mappingExtra(fieldMappingByLocalField, 'application.subjectType')} rules={[{ required: true }]}><Select onChange={(value) => {
                editForm.setFieldValue(['sensitive', 'settlementId'], value === 'MICRO' ? '703' : value === 'ENTERPRISE' ? '716' : '719');
                editForm.setFieldValue(['sensitive', 'qualificationType'], undefined);
              }} options={[{ value: 'MICRO', label: '小微商户' }, { value: 'INDIVIDUAL', label: '个体工商户' }, { value: 'ENTERPRISE', label: '企业' }]} /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['application', 'businessScene']} label="经营场景" extra={mappingExtra(fieldMappingByLocalField, 'application.businessScene')} rules={[{ required: true }]}><Radio.Group><Radio value="STORE">固定门店</Radio><Radio value="MOBILE">流动摊位／便民服务</Radio></Radio.Group></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['application', 'merchantShortName']} label="商户简称" extra={mappingExtra(fieldMappingByLocalField, 'application.merchantShortName')} rules={[{ required: true }]}><Input /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['application', 'servicePhone']} label="客服电话" extra={mappingExtra(fieldMappingByLocalField, 'application.servicePhone')} rules={[{ required: true }]}><Input /></Form.Item></Col>
              <Col span={24}><Form.Item name={['application', 'businessAddress']} label="实际经营地址" extra={mappingExtra(fieldMappingByLocalField, 'application.businessAddress')} rules={[{ required: true }]}><Input.TextArea rows={2} /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['application', 'operatorName']} label="经营者／法定代表人" extra={mappingExtra(fieldMappingByLocalField, 'application.operatorName')} rules={[{ required: true }]}><Input /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['application', 'contactPhone']} label="联系电话" extra={mappingExtra(fieldMappingByLocalField, 'application.contactPhone')} rules={[{ required: true }]}><Input /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['application', 'contactEmail']} label="联系邮箱" extra={mappingExtra(fieldMappingByLocalField, 'application.contactEmail')} rules={[{ required: true }, { type: 'email' }]}><Input /></Form.Item></Col>
              {editedSubjectType !== 'MICRO' && <Col xs={24} md={12}><Form.Item name={['application', 'licenseNumber']} label="统一社会信用代码" extra={mappingExtra(fieldMappingByLocalField, 'application.licenseNumber')} rules={[{ required: true }]}><Input /></Form.Item></Col>}
            </Row>
            <Space direction="vertical" size={8} style={{ marginBottom: 20 }}>
              <Form.Item name={['application', 'qualificationConfirmed']} valuePropName="checked" noStyle><Checkbox>资料真实有效已确认</Checkbox></Form.Item>
              <Form.Item name={['application', 'identityMaterialReady']} valuePropName="checked" noStyle><Checkbox>身份证材料已备齐</Checkbox></Form.Item>
              <Form.Item name={['application', 'settlementAccountReady']} valuePropName="checked" noStyle><Checkbox>结算账户材料已备齐</Checkbox></Form.Item>
              <Form.Item name={['application', 'businessMaterialReady']} valuePropName="checked" noStyle><Checkbox>经营场景材料已备齐</Checkbox></Form.Item>
            </Space>

            <Divider />
            <Typography.Title level={5}>证件与结算账户</Typography.Title>
            <Row gutter={16}>
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'idCardName']} label="身份证姓名" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.idCardName')} rules={[{ required: true }]}><Input /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'idCardNumber']} label="身份证号码" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.idCardNumber')} rules={[{ required: true }]}><Input.Password /></Form.Item></Col>
              <Col span={24}><Form.Item name={['sensitive', 'idCardAddress']} label="身份证住址" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.idCardAddress')} rules={[{ required: true }]}><Input /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'cardPeriodBegin']} label="证件有效期开始" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.cardPeriodBegin')} rules={[{ required: true }]}><Input placeholder="YYYY-MM-DD" /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'cardPeriodEnd']} label="证件有效期结束" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.cardPeriodEnd')} rules={[{ required: true }]}><Input placeholder="YYYY-MM-DD 或 长期" /></Form.Item></Col>
              {editedSubjectType !== 'MICRO' && <><Col xs={24} md={12}><Form.Item name={['sensitive', 'merchantName']} label="营业执照主体" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.merchantName')} rules={[{ required: true }]}><Input /></Form.Item></Col><Col xs={24} md={12}><Form.Item name={['sensitive', 'legalPerson']} label="营业执照经营者／法定代表人" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.legalPerson')} rules={[{ required: true }]}><Input /></Form.Item></Col></>}
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'accountType']} label="账户类型" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.accountType')} rules={[{ required: true }]}><Select options={editedSubjectType === 'ENTERPRISE' ? [{ value: 'BANK_ACCOUNT_TYPE_CORPORATE', label: '对公账户' }] : [{ value: 'BANK_ACCOUNT_TYPE_PERSONAL', label: '经营者个人账户' }, ...(editedSubjectType === 'MICRO' ? [] : [{ value: 'BANK_ACCOUNT_TYPE_CORPORATE', label: '对公账户' }])]} /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'accountName']} label="开户名称" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.accountName')} rules={[{ required: true }]}><Input.Password /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'accountNumber']} label="银行账号" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.accountNumber')} rules={[{ required: true }]}><Input.Password /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'accountBank']} label="开户银行" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.accountBank')} rules={[{ required: true }]}><Input /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'bankAddressCode']} label="银行地区编码" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.bankAddressCode')} rules={[{ pattern: /^\d{6}$/, message: '请输入6位数字' }]}><Input /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'bankBranchId']} label="联行号" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.bankBranchId')} rules={[{ pattern: /^\d{12}$/, message: '请输入12位数字' }]}><Input /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'bankName']} label="开户支行全称" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.bankName')}><Input /></Form.Item></Col>
            </Row>

            <Divider />
            <Typography.Title level={5}>经营与结算规则</Typography.Title>
            <Row gutter={16}>
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'storeName']} label="门店／经营名称" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.storeName')} rules={[{ required: true }]}><Input /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'storeAddressCode']} label="门店省市编码" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.storeAddressCode')} rules={[{ required: true }, { pattern: /^\d+$/ }]}><Input /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'settlementId']} label="结算规则 ID" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.settlementId')} rules={[{ required: true }]}><Select options={[{ value: editedSubjectType === 'MICRO' ? '703' : editedSubjectType === 'ENTERPRISE' ? '716' : '719', label: editedSubjectType === 'MICRO' ? '703 · 小微' : editedSubjectType === 'ENTERPRISE' ? '716 · 企业' : '719 · 个体工商户' }]} /></Form.Item></Col>
              <Col xs={24} md={12}><Form.Item name={['sensitive', 'qualificationType']} label="所属行业" extra={mappingExtra(fieldMappingByLocalField, 'sensitive.qualificationType')} rules={[{ required: true }]}><Select showSearch options={availableIndustries.map((item) => ({ value: item.industry, label: item.industry }))} /></Form.Item></Col>
            </Row>
            {selectedIndustryRequirement && <Alert
              type={selectedIndustryRequirement.qualificationMode === 'NONE' ? 'info' : selectedIndustryRequirement.qualificationMode === 'REQUIRED' ? 'warning' : 'success'}
              showIcon
              message={`${selectedIndustryRequirement.industry} · ${selectedIndustryRequirement.settlementId} · ${selectedIndustryRequirement.qualificationMode === 'NONE' ? '无需特殊资质' : selectedIndustryRequirement.qualificationMode === 'REQUIRED' ? '必须提供特殊资质' : selectedIndustryRequirement.qualificationMode === 'ALTERNATIVE' ? '许可证或经营照片二选一' : '按实际经营内容判断'}`}
              description={<>{selectedIndustryRequirement.requirement} <a href={selectedIndustryRequirement.sourceUrl} target="_blank" rel="noreferrer">查看微信官方对照表</a></>}
            />}
          </Form>

          <div>
            <Typography.Title level={5}>证件与经营照片及微信字段映射</Typography.Title>
            <Image.PreviewGroup>
              <Row gutter={[16, 16]}>
                {detail.mediaRequirements.map((requirement) => {
                  const item = mediaByField[requirement.fieldName];
                  return <Col xs={24} sm={12} md={8} key={requirement.fieldName}>
                  <Card size="small" title={<Space>{requirement.label}<Tag color={requirement.required ? 'error' : 'default'}>{requirement.required ? '必需' : '按规则'}</Tag></Space>}>
                    {item ? <Image src={item.dataUrl} alt={requirement.label} style={{ width: '100%', height: 180, objectFit: 'contain', background: '#f5f5f5' }} /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="尚未上传" style={{ height: 180, paddingTop: 35 }} />}
                    <Typography.Paragraph style={{ marginTop: 10, marginBottom: 4 }}><Typography.Text code>{requirement.wechatPath}</Typography.Text></Typography.Paragraph>
                    <Typography.Paragraph type="secondary" style={{ minHeight: 44 }}>{requirement.requirement}</Typography.Paragraph>
                    {editing && <Upload accept="image/jpeg,image/png,image/bmp" maxCount={1} showUploadList={false} customRequest={replaceMedia(requirement.fieldName)}>
                      <Button block icon={<UploadOutlined />} loading={uploadingField === requirement.fieldName}>{item ? '重新上传并替换' : '上传材料'}</Button>
                    </Upload>}
                  </Card>
                </Col>})}
              </Row>
            </Image.PreviewGroup>
          </div>

          <div>
            <Typography.Title level={5}>全部字段提交映射</Typography.Title>
            <Table
              size="small"
              pagination={false}
              rowKey="localField"
              dataSource={detail.fieldMappings}
              columns={[
                { title: '审核内容', dataIndex: 'label', key: 'label', width: 180 },
                { title: '本地字段', dataIndex: 'localField', key: 'localField', width: 240, render: (value: string) => <Typography.Text code>{value}</Typography.Text> },
                { title: '微信接口字段', dataIndex: 'wechatField', key: 'wechatField', width: 220, render: (value: string) => <Typography.Text code>{value}</Typography.Text> },
                { title: '微信请求路径', dataIndex: 'wechatPath', key: 'wechatPath', render: (value: string) => <Typography.Text code>{value}</Typography.Text> },
              ]}
              scroll={{ x: 980 }}
            />
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
