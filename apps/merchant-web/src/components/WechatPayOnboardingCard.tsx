import {
  BankOutlined,
  CheckCircleOutlined,
  FileProtectOutlined,
  IdcardOutlined,
  SafetyCertificateOutlined,
  ShopOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Col,
  Form,
  Input,
  Radio,
  Row,
  Space,
  Steps,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { api, errorMessage } from '../api/client';
import type { WechatPayOnboardingApplication } from '../types';

const statusMeta: Record<string, { label: string; color: string; step: number }> = {
  DRAFT: { label: '资料草稿', color: 'default', step: 0 },
  NEEDS_INFO: { label: '需要补充资料', color: 'warning', step: 0 },
  PENDING_PLATFORM_REVIEW: { label: '等待摊伴审核', color: 'processing', step: 1 },
  SUBMITTED_TO_WECHAT: { label: '微信支付审核中', color: 'processing', step: 2 },
  FINISHED: { label: '微信支付已开通', color: 'success', step: 3 },
};

const emptyApplication: WechatPayOnboardingApplication = {
  subjectType: 'INDIVIDUAL',
  businessScene: 'STORE',
  merchantShortName: '',
  servicePhone: '',
  businessAddress: '',
  operatorName: '',
  contactPhone: '',
  contactEmail: '',
  licenseNumber: '',
  qualificationConfirmed: false,
  identityMaterialReady: false,
  settlementAccountReady: false,
  businessMaterialReady: false,
  applicationStatus: 'DRAFT',
  platformNote: '',
  submittedAt: '',
  updatedAt: '',
  sensitiveCollectionEnabled: false,
  providerSubmissionEnabled: false,
  sensitiveConfigured: false,
  wechatApplymentId: '',
  wechatApplymentState: '',
  wechatStateMessage: '',
  signUrl: '',
};

interface SensitiveOnboardingFields {
  idCardName: string; idCardNumber: string; idCardAddress: string; cardPeriodBegin: string; cardPeriodEnd: string;
  idCardCopy: string; idCardNational: string; businessLicenseCopy: string;
  merchantName: string; legalPerson: string; accountType: string; accountName: string;
  accountNumber: string; accountBank: string; bankAddressCode: string; bankBranchId: string;
  bankName: string; storeName: string; storeAddressCode: string; storeEntrancePic: string;
  indoorPic: string; cashierPic: string; miniProgramPic: string; qualificationPic: string;
  settlementId: string; qualificationType: string;
}

export function WechatPayOnboardingCard() {
  const [form] = Form.useForm<WechatPayOnboardingApplication>();
  const [sensitiveForm] = Form.useForm<SensitiveOnboardingFields>();
  const [application, setApplication] = useState<WechatPayOnboardingApplication>(emptyApplication);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [savingSensitive, setSavingSensitive] = useState(false);
  const [editingSensitive, setEditingSensitive] = useState(false);
  const [messageApi, contextHolder] = message.useMessage();
  const subjectType = Form.useWatch('subjectType', form);
  const businessScene = Form.useWatch('businessScene', form);
  const expectedCateringSettlementId = subjectType === 'MICRO' ? '703' : subjectType === 'ENTERPRISE' ? '716' : '719';
  const status = statusMeta[application.applicationStatus] || statusMeta.DRAFT;
  const locked = useMemo(() => ['PENDING_PLATFORM_REVIEW', 'SUBMITTED_TO_WECHAT', 'FINISHED'].includes(application.applicationStatus), [application.applicationStatus]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const result = await api.get<WechatPayOnboardingApplication>('/merchant/wechat-pay-onboarding');
      const next = { ...emptyApplication, ...result };
      setApplication(next);
      form.setFieldsValue(next);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [form, messageApi]);

  useEffect(() => { void load(); }, [load]);

  useEffect(() => {
    if (subjectType === 'MICRO') sensitiveForm.setFieldValue('accountType', 'BANK_ACCOUNT_TYPE_PERSONAL');
  }, [sensitiveForm, subjectType]);

  const persist = async (submit: boolean) => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const result = submit
        ? await api.post<WechatPayOnboardingApplication>('/merchant/wechat-pay-onboarding/submit', values)
        : await api.put<WechatPayOnboardingApplication>('/merchant/wechat-pay-onboarding', values);
      // Preserve exactly what the merchant submitted. Server-owned fields such
      // as status still come from the response, but a partial/default response
      // must never blank the visible draft after a successful save.
      setApplication({ ...result, ...values });
      form.setFieldsValue(values);
      messageApi.success(submit ? '申请已提交给摊伴审核' : '进件资料草稿已保存');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const syncStatus = async () => {
    setSaving(true);
    try {
      const result = await api.post<WechatPayOnboardingApplication>('/merchant/wechat-pay-onboarding/sync');
      setApplication(result);
      form.setFieldsValue(result);
      messageApi.success('已同步微信支付审核状态');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const saveSensitive = async () => {
    const applicationValues = await form.validateFields();
    const values = await sensitiveForm.validateFields();
    const { miniProgramPic, qualificationPic, ...sensitiveValues } = values;
    setSavingSensitive(true);
    try {
      // The secure endpoint requires an onboarding draft. Persist the visible
      // basic fields first so merchants do not have to remember a separate
      // "save draft" action before saving the secure material.
      await api.put<WechatPayOnboardingApplication>('/merchant/wechat-pay-onboarding', applicationValues);
      const result = await api.put<WechatPayOnboardingApplication>('/merchant/wechat-pay-onboarding/sensitive', {
        ...sensitiveValues,
        miniProgramPics: miniProgramPic ? [miniProgramPic] : [],
        qualificationPics: qualificationPic ? [qualificationPic] : [],
      });
      setApplication(result);
      messageApi.success('敏感资料已使用租户专用上下文加密保存');
      sensitiveForm.resetFields();
      setEditingSensitive(false);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSavingSensitive(false);
    }
  };

  const mediaUpload = (field: keyof SensitiveOnboardingFields) => async (options: { file: unknown; onSuccess?: (body?: unknown) => void; onError?: (error: Error) => void }) => {
    try {
      const data = new FormData();
      data.append('file', options.file as Blob);
      data.append('field', field);
      const result = await api.postForm<{ mediaId: string }>('/merchant/wechat-pay-onboarding/media', data);
      sensitiveForm.setFieldValue(field, result.mediaId);
      options.onSuccess?.(result);
      messageApi.success('图片已安全上传至微信支付');
    } catch (error) {
      options.onError?.(error instanceof Error ? error : new Error('上传失败'));
      messageApi.error(errorMessage(error));
    }
  };

  const mediaField = (name: keyof SensitiveOnboardingFields, label: string, required = true) => (
    <Form.Item name={name} label={label} rules={required ? [{ required: true, message: `请上传${label}` }] : []}>
      <Input addonAfter={<Upload accept="image/jpeg,image/png,image/bmp" maxCount={1} showUploadList={false} customRequest={mediaUpload(name)}><Button type="link" size="small" icon={<UploadOutlined />}>上传</Button></Upload>} readOnly placeholder="上传后自动写入微信 media_id" />
    </Form.Item>
  );

  return (
    <Card
      bordered={false}
      className="content-card settings-card"
      title={<Space><SafetyCertificateOutlined />申请微信支付特约商户</Space>}
      extra={<Space><Tag color={status.color}>{status.label}</Tag>{application.wechatApplymentId && application.applicationStatus !== 'FINISHED' && <Button size="small" loading={saving} onClick={() => void syncStatus()}>同步微信状态</Button>}</Space>}
      loading={loading}
    >
      {contextHolder}
      <Steps
        current={status.step}
        size="small"
        items={[
          { title: '准备资料' },
          { title: '平台审核' },
          { title: '微信审核与签约' },
          { title: '自动绑定商户号' },
        ]}
      />

      <Alert
        style={{ marginTop: 20 }}
        type="info"
        showIcon
        message="商户号不需要商户手工配置"
        description="审核签约完成后，摊伴会保存微信返回的特约商户号（sub_mchid）并自动绑定到本店。服务商商户号、API证书和密钥由平台统一管理。"
      />
      {!application.sensitiveCollectionEnabled && <Alert
        style={{ marginTop: 12 }}
        type="warning"
        showIcon
        message="安全资料能力尚未启用"
        description="平台尚未配置独立数据加密主密钥，因此不会接收身份证号、银行卡号和进件图片。"
      />}
      {application.sensitiveConfigured && <Alert style={{ marginTop: 12 }} type="success" showIcon message="安全资料已保存" description="敏感字段使用 AES-256-GCM 加密保存；图片上传微信支付后，会额外加密保留仅供平台预审查看的审核副本。出于安全原因页面不会回显明文。" />}
      {application.wechatStateMessage && <Alert style={{ marginTop: 12 }} type="info" showIcon message={`微信审核状态：${application.wechatApplymentState || '处理中'}`} description={application.wechatStateMessage} />}
      {application.signUrl && <Alert style={{ marginTop: 12 }} type="warning" showIcon message="请完成微信支付签约" description={<a href={application.signUrl} target="_blank" rel="noreferrer">打开微信支付签约链接</a>} />}
      {application.applicationStatus === 'NEEDS_INFO' && application.platformNote && (
        <Alert
          style={{ marginTop: 12 }}
          type="error"
          showIcon
          message="申请被驳回"
          description={<>驳回原因：{application.platformNote}<br />请修改上述资料后重新提交审核。</>}
        />
      )}
      {application.applicationStatus !== 'NEEDS_INFO' && application.platformNote && (
        <Alert style={{ marginTop: 12 }} type="warning" showIcon message="平台反馈" description={application.platformNote} />
      )}

      <Form form={form} layout="vertical" disabled={locked} style={{ marginTop: 20 }}>
        <Typography.Title level={5}><ShopOutlined /> 主体与经营场景</Typography.Title>
        <Form.Item name="subjectType" label="申请主体" rules={[{ required: true }]}>
          <Radio.Group optionType="button" buttonStyle="solid">
            <Radio.Button value="MICRO">小微商户（无营业执照）</Radio.Button>
            <Radio.Button value="INDIVIDUAL">个体工商户</Radio.Button>
            <Radio.Button value="ENTERPRISE">企业</Radio.Button>
          </Radio.Group>
        </Form.Item>
        {subjectType === 'MICRO' && <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="小微商户适用于依法免办理工商登记的实体经营者"
          description="需提供经营者身份证、本人银行卡和固定门店或流动经营现场证明；能否进件取决于摊伴服务商的小微进件权限及微信支付审核。"
        />}
        <Form.Item name="businessScene" label="经营场景" rules={[{ required: true }]}>
          <Radio.Group>
            <Radio value="STORE">固定门店</Radio>
            <Radio value="MOBILE">流动摊位／便民服务</Radio>
          </Radio.Group>
        </Form.Item>
        <Row gutter={16}>
          <Col xs={24} md={12}><Form.Item name="merchantShortName" label="商户简称" rules={[{ required: true, message: '请输入商户简称' }, { max: 64 }]}><Input placeholder="例如：码农咖啡" /></Form.Item></Col>
          <Col xs={24} md={12}><Form.Item name="servicePhone" label="客服电话" rules={[{ required: true, message: '请输入客服电话' }, { max: 32 }]}><Input /></Form.Item></Col>
        </Row>
        <Form.Item name="businessAddress" label="实际经营地址" rules={[{ required: true, message: '请输入实际经营地址' }, { max: 500 }]}><Input.TextArea rows={2} /></Form.Item>

        <Typography.Title level={5}><IdcardOutlined /> 经营者与联系人</Typography.Title>
        <Row gutter={16}>
          <Col xs={24} md={12}><Form.Item name="operatorName" label={subjectType === 'ENTERPRISE' ? '法定代表人姓名' : '经营者姓名'} rules={[{ required: true, message: '请输入经营者姓名' }, { max: 80 }]}><Input /></Form.Item></Col>
          <Col xs={24} md={12}><Form.Item name="contactPhone" label="联系手机号" rules={[{ required: true, message: '请输入联系手机号' }, { pattern: /^1\d{10}$/, message: '请输入正确的手机号码' }]}><Input /></Form.Item></Col>
        </Row>
        <Row gutter={16}>
          <Col xs={24} md={12}><Form.Item name="contactEmail" label="联系邮箱" rules={[{ required: true, message: '请输入用于接收微信支付通知的邮箱' }, { type: 'email', message: '邮箱格式不正确' }, { max: 160 }]}><Input /></Form.Item></Col>
          {subjectType !== 'MICRO' && <Col xs={24} md={12}><Form.Item name="licenseNumber" label="注册号／统一社会信用代码" rules={[{ required: true, message: '请输入营业执照注册号或统一社会信用代码' }, { pattern: /^(?:\d{15}|[0-9A-HJ-NPQRTUWXY]{18})$/, message: '个体户支持15位注册号或18位统一社会信用代码，企业须18位' }]}><Input /></Form.Item></Col>}
        </Row>

        <Typography.Title level={5}><FileProtectOutlined /> 资料准备确认</Typography.Title>
        <Space direction="vertical" size={12}>
          <Form.Item name="qualificationConfirmed" valuePropName="checked" noStyle><Checkbox>我确认以上资料及后续上传的证件、照片真实、清晰且在有效期内</Checkbox></Form.Item>
          <Form.Item name="identityMaterialReady" valuePropName="checked" noStyle><Checkbox>经营者身份证原件正反面及有效期已准备</Checkbox></Form.Item>
          <Form.Item name="settlementAccountReady" valuePropName="checked" noStyle><Checkbox><BankOutlined /> 经营者本人银行卡、开户行及支行信息已准备</Checkbox></Form.Item>
          <Form.Item name="businessMaterialReady" valuePropName="checked" noStyle><Checkbox>门头／摊位、经营环境、商品及租赁或摊位证明等材料已准备</Checkbox></Form.Item>
        </Space>

        {!locked && <Space style={{ marginTop: 24 }}>
          <Button loading={saving} onClick={() => void persist(false)}>保存草稿</Button>
          <Button type="primary" icon={<CheckCircleOutlined />} loading={saving} onClick={() => void persist(true)}>提交平台预审</Button>
        </Space>}
      </Form>

      {application.sensitiveCollectionEnabled && !locked && <Card type="inner" title={<Space><FileProtectOutlined />安全进件资料</Space>} style={{ marginTop: 20 }}>
        {application.sensitiveConfigured && !editingSensitive ? <Alert
          type="success"
          showIcon
          message="安全资料已加密保存，可以直接提交平台预审"
          description="身份证、银行卡和图片不会回显，这是安全保护，不代表草稿丢失。只有资料需要变更时才重新填写并整体替换。"
          action={<Button onClick={() => setEditingSensitive(true)}>重新填写并替换</Button>}
        /> : <>
          <Alert type="info" showIcon message="先保存上方基础资料，再填写本区域" description="证件号和结算账户只以密文落库；证件与门店图片上传微信支付后，会加密保留仅供平台人工预审的副本。重新保存会整体替换上一版密文。" style={{ marginBottom: 16 }} />
          <Form form={sensitiveForm} layout="vertical" initialValues={{ accountType: 'BANK_ACCOUNT_TYPE_PERSONAL' }}>
          <Typography.Title level={5}>营业执照与身份证</Typography.Title>
          <Row gutter={16}>
            {subjectType !== 'MICRO' && <Col xs={24} md={12}><Form.Item name="merchantName" label="营业执照主体全称" rules={[{ required: true }]}><Input /></Form.Item></Col>}
            {subjectType !== 'MICRO' && <Col xs={24} md={12}><Form.Item name="legalPerson" label={subjectType === 'ENTERPRISE' ? '法定代表人' : '经营者'} rules={[{ required: true }]}><Input /></Form.Item></Col>}
            <Col xs={24} md={12}><Form.Item name="idCardName" label="身份证姓名" rules={[{ required: true }]}><Input /></Form.Item></Col>
            <Col xs={24} md={12}><Form.Item name="idCardNumber" label="身份证号码" rules={[{ required: true }]}><Input.Password /></Form.Item></Col>
            <Col xs={24} md={12}><Form.Item name="idCardAddress" label="身份证住址" rules={[{ required: true }]}><Input /></Form.Item></Col>
            <Col xs={24} md={12}><Form.Item name="cardPeriodBegin" label="身份证有效期开始" rules={[{ required: true }]}><Input placeholder="YYYY-MM-DD" /></Form.Item></Col>
            <Col xs={24} md={12}><Form.Item name="cardPeriodEnd" label="身份证有效期结束" rules={[{ required: true }]}><Input placeholder="YYYY-MM-DD 或 长期" /></Form.Item></Col>
            {subjectType !== 'MICRO' && <Col xs={24} md={12}>{mediaField('businessLicenseCopy', '营业执照照片')}</Col>}
            <Col xs={24} md={12}>{mediaField('idCardCopy', '身份证人像面')}</Col>
            <Col xs={24} md={12}>{mediaField('idCardNational', '身份证国徽面')}</Col>
          </Row>
          <Typography.Title level={5}>结算账户</Typography.Title>
          <Row gutter={16}>
            <Col xs={24} md={12}><Form.Item name="accountType" label="账户类型" rules={[{ required: true }]}><Radio.Group><Radio value="BANK_ACCOUNT_TYPE_PERSONAL">经营者个人账户</Radio>{subjectType !== 'MICRO' && <Radio value="BANK_ACCOUNT_TYPE_CORPORATE">对公账户</Radio>}</Radio.Group></Form.Item></Col>
            <Col xs={24} md={12}><Form.Item name="accountName" label="账户名称" rules={[{ required: true }]}><Input.Password /></Form.Item></Col>
            <Col xs={24} md={12}><Form.Item name="accountNumber" label="银行账号" rules={[{ required: true }]}><Input.Password /></Form.Item></Col>
            <Col xs={24} md={12}><Form.Item name="accountBank" label="开户银行" extra="须使用微信银行列表的标准值；天津银行等地方银行通常填写“其他银行”" rules={[{ required: true }]}><Input placeholder="例：工商银行；地方银行填：其他银行" /></Form.Item></Col>
            <Col xs={24} md={12}><Form.Item name="bankAddressCode" label="开户银行省市编码" extra="填写支行所在地的6位微信地区编码；不确定时可留空" rules={[{ pattern: /^\d{6}$/, message: '请输入6位微信地区编码' }]}><Input placeholder="例：天津市静海区 120118" /></Form.Item></Col>
            <Col xs={24} md={12}><Form.Item name="bankBranchId" label="开户银行联行号" extra="其他银行：与开户支行全称至少填写一项" rules={[{ pattern: /^\d{12}$/, message: '联行号应为12位数字' }]}><Input /></Form.Item></Col>
            <Col xs={24} md={12}><Form.Item name="bankName" label="开户支行全称" extra="其他银行：与联行号至少填写一项" dependencies={['accountBank', 'bankBranchId']} rules={[{ validator: async (_, value) => {
              if (sensitiveForm.getFieldValue('accountBank') === '其他银行' && !String(value || '').trim() && !String(sensitiveForm.getFieldValue('bankBranchId') || '').trim()) {
                throw new Error('开户银行为其他银行时，开户支行全称和联行号至少填写一项');
              }
            } }]}><Input placeholder="填写银行查询结果中的完整支行名称" /></Form.Item></Col>
          </Row>
          <Typography.Title level={5}>经营场景与结算规则</Typography.Title>
          <Row gutter={16}>
            <Col xs={24} md={12}><Form.Item name="storeName" label={subjectType === 'MICRO' && businessScene === 'MOBILE' ? '经营／服务名称' : '门店名称'} rules={[{ required: true }]}><Input /></Form.Item></Col>
            <Col xs={24} md={12}><Form.Item name="storeAddressCode" label={subjectType === 'MICRO' && businessScene === 'MOBILE' ? '经营所在地省市编码' : '门店省市编码'} rules={[{ required: true }, { pattern: /^\d+$/, message: '省市编码只能包含数字' }]}><Input /></Form.Item></Col>
            <Col xs={24} md={12}>{mediaField('storeEntrancePic', subjectType === 'MICRO' && businessScene === 'MOBILE' ? '经营现场全景照片' : '门店门头照片')}</Col>
            <Col xs={24} md={12}>{mediaField('indoorPic', subjectType === 'MICRO' && businessScene === 'MOBILE' ? '商品／服务现场照片' : '店内环境照片')}</Col>
            <Col xs={24} md={12}>{mediaField('cashierPic', '收银台照片')}</Col>
            {subjectType !== 'MICRO' && <Col xs={24} md={12}>{mediaField('miniProgramPic', '小程序经营页面截图')}</Col>}
            <Col xs={24} md={12}>{mediaField('qualificationPic', '食品经营许可证／行业特殊资质（如有）', false)}</Col>
            <Col xs={24} md={12}><Form.Item
              name="settlementId"
              label="微信结算规则 ID"
              extra="个体餐饮填写 719；这里不是餐饮的行业 ID 1"
              rules={[
                { required: true },
                { pattern: /^\d+$/, message: '结算规则 ID 只能填写数字' },
                { validator: async (_, value) => {
                  const industry = String(sensitiveForm.getFieldValue('qualificationType') || '').trim();
                  if (industry === '餐饮' && String(value || '').trim() !== expectedCateringSettlementId) {
                    throw new Error(`${subjectType === 'MICRO' ? '小微' : subjectType === 'ENTERPRISE' ? '企业' : '个体工商户'}餐饮的结算规则 ID 应为 ${expectedCateringSettlementId}`);
                  }
                } },
              ]}
            ><Input placeholder={`餐饮场景填写 ${expectedCateringSettlementId}`} /></Form.Item></Col>
            <Col xs={24} md={12}><Form.Item name="qualificationType" label="所属行业名称" extra="餐饮商户填写“餐饮”，不要填写行业 ID 1" rules={[{ required: true }]}><Input placeholder="必须与微信结算规则表的行业名称完全一致" /></Form.Item></Col>
          </Row>
            <Space>
              <Button type="primary" loading={savingSensitive} onClick={() => void saveSensitive()}>加密保存安全资料</Button>
              {application.sensitiveConfigured && <Button onClick={() => { sensitiveForm.resetFields(); setEditingSensitive(false); }}>取消替换</Button>}
            </Space>
          </Form>
        </>}
      </Card>}
    </Card>
  );
}
