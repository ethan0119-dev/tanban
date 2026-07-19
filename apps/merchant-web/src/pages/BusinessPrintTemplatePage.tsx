import {
  FileTextOutlined,
  ReloadOutlined,
  SaveOutlined,
  TagsOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Col,
  Divider,
  Form,
  Input,
  InputNumber,
  Row,
  Segmented,
  Select,
  Space,
  Switch,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import { useCallback, useEffect, useRef, useState } from 'react';
import { api, errorMessage } from '../api/client';
import { PageHeading } from '../components/PageHeading';
import {
  PRINT_TEMPLATES_ENDPOINT,
  defaultPrintTemplate,
  normalizePrintTemplates,
  printTemplatePayload,
} from '../features/storefront/model';
import type { BusinessPrintTemplate, OrderBusinessType, PrintBusinessType, PrintTemplateRecord } from '../types';
import { dateTime } from '../utils/format';

const commonVariables = [
  '{{store_name}}', '{{order_no}}', '{{order_type}}', '{{pickup_no}}', '{{items}}',
  '{{total_cents}}', '{{paid_amount}}', '{{remark}}',
];
const dineInVariables = ['{{table_area}}', '{{table_name}}', '{{table_code}}'];
const labelItemVariables = [
  '{{product_name}}', '{{sku_name}}', '{{quantity}}', '{{ordered_quantity}}',
  '{{options}}', '{{modifiers}}', '{{item_remark}}',
];

const previewValues: Record<string, string> = {
  store_name: '码农咖啡', order_no: 'TB202607200001', order_type: 'DINE_IN', pickup_no: '0038',
  table_area: '露台', table_name: 'B02 桌', table_code: 'B02',
  items: '冰美式 中杯 x1\n拿铁 大杯 x1\n  温度：少冰\n  加料：燕麦奶', total_cents: '3700', paid_amount: '37.00', remark: '拿铁少冰',
  product_name: '拿铁', sku_name: '大杯', quantity: '1', ordered_quantity: '1',
  options: '温度：少冰', modifiers: '加料：燕麦奶', item_remark: '杯身写 Ethan',
};

function renderPreview(template: string): string {
  return template.replace(/{{\s*([a-z_]+)\s*}}/gi, (token, name: string) => previewValues[name] ?? token);
}

function TemplateEditor({ section }: { section: 'receipt' | 'label' }) {
  const title = section === 'receipt' ? '订单小票' : '商品标签';
  const text = Form.useWatch([section, 'templateText']) ?? '';
  return (
    <Row gutter={[18, 18]}>
      <Col xs={24} xl={14}>
        <Card size="small" className="print-template-editor" title={<Space><FileTextOutlined />{title}内容</Space>}>
          <div className="print-template-switch-row">
            <div><strong>启用{title}</strong><Typography.Paragraph type="secondary">{section === 'receipt' ? '每笔订单按模板打印整单信息' : '按商品数量拆分杯贴或餐品标签'}</Typography.Paragraph></div>
            <Form.Item name={[section, 'enabled']} valuePropName="checked" noStyle><Switch /></Form.Item>
          </div>
          <Form.Item label="模板名称" name={[section, 'name']} rules={[{ required: true, message: '请输入模板名称' }]}><Input maxLength={100} /></Form.Item>
          <Row gutter={12}>
            <Col span={15}><Form.Item label="打印触发点" name={[section, 'triggerEvent']} rules={[{ required: true }]}><Select options={[{ value: 'PAYMENT_SUCCESS', label: '付款成功后打印' }, { value: 'ORDER_CREATED', label: '下单后打印（含待付款）' }]} /></Form.Item></Col>
            <Col span={9}><Form.Item label="打印份数" name={[section, 'copies']} rules={[{ required: true }]}><InputNumber min={1} max={5} precision={0} addonAfter="份" style={{ width: '100%' }} /></Form.Item></Col>
          </Row>
          <Form.Item label="模板内容" name={[section, 'templateText']} rules={[{ required: true, message: '请输入模板内容' }]}>
            <Input.TextArea rows={section === 'receipt' ? 14 : 9} className="template-textarea" spellCheck={false} />
          </Form.Item>
          <Typography.Paragraph type="secondary">变量必须保留双大括号，例如 <Typography.Text code>{'{{order_no}}'}</Typography.Text>。服务端只替换白名单变量，未知变量按原文打印。</Typography.Paragraph>
        </Card>
      </Col>
      <Col xs={24} xl={10}>
        <Card size="small" className="print-paper-preview" title={`${title}预览`}>
          <pre>{renderPreview(text)}</pre>
          <Divider dashed />
          <Typography.Text type="secondary">预览使用示例数据；真正打印时由订单快照填充。</Typography.Text>
        </Card>
      </Col>
    </Row>
  );
}

export function BusinessPrintTemplatePage({ businessType }: { businessType: OrderBusinessType }) {
  const [form] = Form.useForm<BusinessPrintTemplate>();
  const [activeType, setActiveType] = useState<PrintBusinessType>(businessType);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [loadWarning, setLoadWarning] = useState('');
  const loadRevision = useRef(0);
  const [messageApi, contextHolder] = message.useMessage();
  const domainName = businessType === 'DINE_IN' ? '店内' : '外卖';
  const sceneName = activeType === 'DINE_IN' ? '桌码堂食' : activeType === 'TAKEOUT' ? '快餐 / 到店自取' : '外卖配送';
  const orderVariables = [...commonVariables, ...(activeType === 'DINE_IN' ? dineInVariables : [])];

  useEffect(() => {
    setActiveType(businessType);
    form.resetFields();
  }, [businessType, form]);

  const load = useCallback(async () => {
    const revision = ++loadRevision.current;
    setLoading(true);
    setLoadWarning('');
    try {
      const result = await api.getList<PrintTemplateRecord>(PRINT_TEMPLATES_ENDPOINT, { business_type: activeType, page_size: 20 });
      if (revision !== loadRevision.current) return;
      form.setFieldsValue(normalizePrintTemplates(result.items, activeType));
    } catch (error) {
      if (revision !== loadRevision.current) return;
      form.setFieldsValue(defaultPrintTemplate(activeType));
      setLoadWarning(`尚未读取到已保存模板，当前展示安全默认值：${errorMessage(error)}`);
    } finally {
      if (revision === loadRevision.current) setLoading(false);
    }
  }, [activeType, form]);

  useEffect(() => { void load(); }, [load]);

  const save = async () => {
    const values = await form.validateFields();
    const normalized: BusinessPrintTemplate = { ...values, businessType: activeType };
    setSaving(true);
    try {
      const saved = await Promise.all((['RECEIPT', 'LABEL'] as const).map((templateType) => {
        const section = templateType === 'RECEIPT' ? normalized.receipt : normalized.label;
        return section.id
          ? api.put<PrintTemplateRecord>(`${PRINT_TEMPLATES_ENDPOINT}/${section.id}`, printTemplatePayload(normalized, templateType))
          : api.post<PrintTemplateRecord>(PRINT_TEMPLATES_ENDPOINT, printTemplatePayload(normalized, templateType));
      }));
      form.setFieldsValue(normalizePrintTemplates(saved, activeType));
      setLoadWarning('');
      messageApi.success(`${sceneName}打印模板已保存`);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const reset = () => {
    form.setFieldsValue(defaultPrintTemplate(activeType));
    messageApi.info('已恢复默认模板，点击保存后才会生效');
  };

  return (
    <div className="page-shell">
      {contextHolder}
      <PageHeading
        title={`${domainName}打印模板`}
        description={`${domainName}订单使用独立的小票和商品标签模板，不与另一经营域混用`}
        extra={<Space><Button icon={<ReloadOutlined />} loading={loading} onClick={() => void load()}>重新加载</Button><Button onClick={reset}>恢复默认</Button><Button type="primary" icon={<SaveOutlined />} loading={saving} disabled={loading} onClick={() => void save()}>保存模板</Button></Space>}
      />
      {businessType === 'DELIVERY' && <Alert className="printer-tip" type="info" showIcon message="可提前配置外卖打印模板" description="外卖订单和实际打印一期尚未启用，但模板可以正常保存；以后开启配送能力时直接使用 DELIVERY 独立模板，不会影响店内出单。" />}
      {businessType === 'DINE_IN' && <Card bordered={false} className="content-card print-scene-switch"><Typography.Text strong>店内订单场景</Typography.Text><Segmented value={activeType} onChange={(value) => setActiveType(value as PrintBusinessType)} options={[{ label: '桌码堂食', value: 'DINE_IN' }, { label: '快餐 / 到店自取', value: 'TAKEOUT' }]} /></Card>}
      {loadWarning && <Alert className="printer-tip" type="warning" showIcon message="使用默认打印模板" description={loadWarning} />}
      <Form<BusinessPrintTemplate> form={form} layout="vertical" disabled={loading}>
        <Card bordered={false} className="content-card print-policy-card">
          <Row gutter={[20, 16]} align="middle">
            <Col xs={24} xl={14}>
              <Alert type="info" showIcon message={`${sceneName}使用独立模板`} description="订单小票与商品标签分别保存触发点、份数和内容。生成任务时服务端按订单 order_type 选择对应模板，避免店内堂食、自提和外卖串用内容。" />
            </Col>
            <Col xs={24} xl={10}>
              <div className="template-variable-box">
                <strong><TagsOutlined /> 订单级变量</strong>
                <div>{orderVariables.map((value) => <Tag key={value}>{value}</Tag>)}</div>
                <strong>商品标签额外变量</strong>
                <div>{labelItemVariables.map((value) => <Tag key={value} color="blue">{value}</Tag>)}</div>
              </div>
            </Col>
          </Row>
        </Card>
        <Card bordered={false} className="content-card print-template-tabs">
          <Tabs items={[
            { key: 'receipt', label: '订单小票', children: <TemplateEditor section="receipt" /> },
            { key: 'label', label: '商品标签', children: <TemplateEditor section="label" /> },
          ]} />
        </Card>
        <Typography.Paragraph type="secondary" className="template-meta">小票更新：{dateTime(Form.useWatch(['receipt', 'updatedAt'], form))} · 标签更新：{dateTime(Form.useWatch(['label', 'updatedAt'], form))}</Typography.Paragraph>
      </Form>
    </div>
  );
}
