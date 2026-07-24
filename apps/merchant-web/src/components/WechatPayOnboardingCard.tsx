import {
  BankOutlined,
  CheckCircleOutlined,
  FileProtectOutlined,
  IdcardOutlined,
  SafetyCertificateOutlined,
  ShopOutlined,
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
  subjectType: 'MICRO',
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
};

export function WechatPayOnboardingCard() {
  const [form] = Form.useForm<WechatPayOnboardingApplication>();
  const [application, setApplication] = useState<WechatPayOnboardingApplication>(emptyApplication);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [messageApi, contextHolder] = message.useMessage();
  const subjectType = Form.useWatch('subjectType', form);
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

  const persist = async (submit: boolean) => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const result = submit
        ? await api.post<WechatPayOnboardingApplication>('/merchant/wechat-pay-onboarding/submit', values)
        : await api.put<WechatPayOnboardingApplication>('/merchant/wechat-pay-onboarding', values);
      setApplication(result);
      form.setFieldsValue(result);
      messageApi.success(submit ? '申请已提交给摊伴审核' : '进件资料草稿已保存');
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card
      bordered={false}
      className="content-card settings-card"
      title={<Space><SafetyCertificateOutlined />申请微信支付特约商户</Space>}
      extra={<Tag color={status.color}>{status.label}</Tag>}
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
        message="当前先提交进件预审资料"
        description="身份证照片、完整身份证号和银行卡号属于高度敏感信息，当前页面不会收集或写入普通图库。平台开通小微进件权限并完成专用加密存储后，再通过安全步骤补充并提交微信支付。"
      />}
      {application.platformNote && <Alert style={{ marginTop: 12 }} type="warning" showIcon message="平台反馈" description={application.platformNote} />}

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
          type="warning"
          showIcon
          message="无营业执照不等于一定符合小微商户"
          description="仅限依法免办理工商登记的实体经营者；固定咖啡店或依法应登记的经营者应选择个体工商户或企业。"
          style={{ marginBottom: 16 }}
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
          <Col xs={24} md={12}><Form.Item name="contactEmail" label="联系邮箱" rules={[{ type: 'email', message: '邮箱格式不正确' }, { max: 160 }]}><Input /></Form.Item></Col>
          {subjectType !== 'MICRO' && <Col xs={24} md={12}><Form.Item name="licenseNumber" label="统一社会信用代码" rules={[{ required: true, message: '请输入营业执照统一社会信用代码' }, { pattern: /^[0-9A-Z]{18}$/, message: '请输入18位统一社会信用代码' }]}><Input /></Form.Item></Col>}
        </Row>

        <Typography.Title level={5}><FileProtectOutlined /> 资料准备确认</Typography.Title>
        <Space direction="vertical" size={12}>
          {subjectType === 'MICRO' && <Form.Item name="qualificationConfirmed" valuePropName="checked" noStyle><Checkbox>我确认经营者属于依法免办理工商登记的实体经营者，并愿意接受平台与微信支付审核</Checkbox></Form.Item>}
          <Form.Item name="identityMaterialReady" valuePropName="checked" noStyle><Checkbox>经营者身份证原件正反面及有效期已准备</Checkbox></Form.Item>
          <Form.Item name="settlementAccountReady" valuePropName="checked" noStyle><Checkbox><BankOutlined /> 经营者本人银行卡、开户行及支行信息已准备</Checkbox></Form.Item>
          <Form.Item name="businessMaterialReady" valuePropName="checked" noStyle><Checkbox>门头／摊位、经营环境、商品及租赁或摊位证明等材料已准备</Checkbox></Form.Item>
        </Space>

        {!locked && <Space style={{ marginTop: 24 }}>
          <Button loading={saving} onClick={() => void persist(false)}>保存草稿</Button>
          <Button type="primary" icon={<CheckCircleOutlined />} loading={saving} onClick={() => void persist(true)}>提交平台预审</Button>
        </Space>}
      </Form>
    </Card>
  );
}
