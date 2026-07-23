import {
  AppstoreOutlined,
  FileTextOutlined,
  FireOutlined,
  FontSizeOutlined,
  QrcodeOutlined,
  ReloadOutlined,
  SaveOutlined,
  SettingOutlined,
  ShopOutlined,
  TagsOutlined,
  UserOutlined,
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Col,
  Collapse,
  Divider,
  Input,
  InputNumber,
  Modal,
  QRCode,
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
import type { CSSProperties, ReactNode } from 'react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { api, errorMessage } from '../api/client';
import { FeatureAvailabilityNotice } from '../components/FeatureAvailabilityNotice';
import { PageHeading } from '../components/PageHeading';
import {
  PRINT_TEMPLATES_ENDPOINT,
  defaultPrintTemplate,
  normalizePrintTemplates,
  printTemplatePayload,
} from '../features/storefront/model';
import type {
  BusinessPrintTemplate,
  OrderBusinessType,
  PrintBusinessType,
  PrintCopyRole,
  PrintLayoutPreset,
  PrintTemplateLayout,
  PrintTemplateRecord,
  PrintTemplateSection,
} from '../types';
import { dateTime } from '../utils/format';

const roleMeta: Record<PrintCopyRole, { label: string; description: string; icon: ReactNode; color: string }> = {
  MERCHANT: { label: '商家联', description: '留给门店核对整单、支付和桌台信息', icon: <ShopOutlined />, color: 'brown' },
  CUSTOMER: { label: '顾客联', description: '交给顾客，可展示金额、联系方式和取单二维码', icon: <UserOutlined />, color: 'blue' },
  KITCHEN: { label: '厨房联', description: '突出商品、规格、加料和备注，默认隐藏价格', icon: <FireOutlined />, color: 'orange' },
  ITEM: { label: '商品标签', description: '按商品数量拆分杯贴或餐品标签', icon: <TagsOutlined />, color: 'green' },
};

const commonVariables = [
  '{{store_name}}', '{{order_no}}', '{{order_type}}', '{{pickup_no}}', '{{items}}',
  '{{total_cents}}', '{{paid_amount}}', '{{remark}}',
];
const dineInVariables = ['{{table_area}}', '{{table_name}}', '{{table_code}}'];
const labelItemVariables = [
  '{{product_name}}', '{{sku_name}}', '{{quantity}}', '{{ordered_quantity}}',
  '{{item_index}}', '{{item_total}}', '{{item_sequence}}',
  '{{options}}', '{{modifiers}}', '{{item_remark}}',
];

const visibilityOptions: Array<{ key: keyof PrintTemplateLayout; label: string; roles?: PrintCopyRole[] }> = [
  { key: 'showStoreName', label: '店铺名称' },
  { key: 'showOrderType', label: '订单类型' },
  { key: 'showOrderNo', label: '订单编号' },
  { key: 'showOrderTime', label: '下单时间' },
  { key: 'showPickupNo', label: '取单号' },
  { key: 'showTable', label: '桌台信息' },
  { key: 'showItems', label: '商品明细' },
  { key: 'showItemSequence', label: '标签序号 1/2', roles: ['ITEM'] },
  { key: 'showItemHeader', label: '商品表头', roles: ['MERCHANT', 'CUSTOMER', 'KITCHEN'] },
  { key: 'showItemOptions', label: '规格与加料' },
  { key: 'showOptionGroupNames', label: '显示属性名称' },
  { key: 'showPrices', label: '单价与金额', roles: ['MERCHANT', 'CUSTOMER'] },
  { key: 'showPayment', label: '支付与合计', roles: ['MERCHANT', 'CUSTOMER'] },
  { key: 'emphasizePaid', label: '放大实付金额', roles: ['MERCHANT', 'CUSTOMER'] },
  { key: 'showRemark', label: '订单备注' },
  { key: 'showCustomer', label: '顾客信息', roles: ['MERCHANT', 'CUSTOMER'] },
  { key: 'showAddress', label: '配送地址', roles: ['MERCHANT', 'CUSTOMER'] },
  { key: 'showQrCode', label: '取单二维码', roles: ['CUSTOMER'] },
];

const presetMeta: Record<PrintLayoutPreset, { name: string; description: string; icon: ReactNode }> = {
  LARGE: { name: '大字优先', description: '取餐码和商品名更醒目，适合后厨快速识别', icon: <FontSizeOutlined /> },
  DETAILED: { name: '信息完整', description: '保留订单、规格、金额和备注等完整信息', icon: <FileTextOutlined /> },
  COMPACT: { name: '简洁省纸', description: '只保留制作与取餐所需的核心字段', icon: <AppstoreOutlined /> },
  CUSTOM: { name: '自定义', description: '继续使用字段开关和自定义文案精细配置', icon: <SettingOutlined /> },
};

const labelPaperSizes = [
  { label: '40 × 30', value: '40x30', width: 40, height: 30 },
  { label: '50 × 60', value: '50x60', width: 50, height: 60 },
  { label: '50 × 70', value: '50x70', width: 50, height: 70 },
  { label: '40 × 60', value: '40x60', width: 40, height: 60 },
];

function cents(value: number): string {
  return (value / 100).toFixed(2);
}

function applyLayoutPreset(section: PrintTemplateSection, preset: PrintLayoutPreset): PrintTemplateLayout {
  const current = section.layout;
  if (preset === 'CUSTOM') return { ...current, preset };
  if (section.copyRole === 'ITEM') {
    if (preset === 'COMPACT') {
      return {
        ...current, preset, fontSize: 'NORMAL', showStoreName: false, showOrderType: true,
        showOrderNo: false, showOrderTime: true, showPickupNo: true, showTable: true,
        showItems: true, showItemSequence: true, showItemHeader: false, showItemOptions: false,
        showOptionGroupNames: false, showPrices: false, showPayment: false, emphasizePaid: false,
        showRemark: false, showCustomer: false, showAddress: false, showQrCode: false,
      };
    }
    if (preset === 'LARGE') {
      return {
        ...current, preset, fontSize: 'LARGE', showStoreName: false, showOrderType: true,
        showOrderNo: false, showOrderTime: true, showPickupNo: true, showTable: true,
        showItems: true, showItemSequence: true, showItemHeader: false, showItemOptions: true,
        showOptionGroupNames: false, showPrices: false, showPayment: false, emphasizePaid: false,
        showRemark: true, showCustomer: false, showAddress: false, showQrCode: false,
      };
    }
    return {
      ...current, preset, fontSize: 'NORMAL', showStoreName: false, showOrderType: true,
      showOrderNo: true, showOrderTime: true, showPickupNo: true, showTable: true,
      showItems: true, showItemSequence: true, showItemHeader: false, showItemOptions: true,
      showOptionGroupNames: false, showPrices: false, showPayment: false, emphasizePaid: false,
      showRemark: true, showCustomer: false, showAddress: false, showQrCode: false,
    };
  }

  const customer = section.copyRole === 'CUSTOMER';
  const kitchen = section.copyRole === 'KITCHEN';
  if (preset === 'COMPACT') {
    return {
      ...current, preset, fontSize: 'NORMAL', headerStyle: 'SIMPLE', showStoreName: false,
      showOrderType: true, showOrderNo: false, showOrderTime: false, showPickupNo: true,
      showTable: true, showItems: true, showItemSequence: false, showItemHeader: true,
      showItemOptions: false, showOptionGroupNames: false, showPrices: !kitchen,
      showPayment: !kitchen, emphasizePaid: !kitchen, showRemark: true,
      showCustomer: false, showAddress: false, showQrCode: false, showEndMarker: true,
    };
  }
  if (preset === 'LARGE') {
    return {
      ...current, preset, fontSize: 'LARGE', headerStyle: 'SIMPLE', showStoreName: false,
      showOrderType: true, showOrderNo: false, showOrderTime: false, showPickupNo: true,
      showTable: true, showItems: true, showItemSequence: false, showItemHeader: true,
      showItemOptions: true, showOptionGroupNames: false, showPrices: false,
      showPayment: false, emphasizePaid: false, showRemark: true,
      showCustomer: false, showAddress: false, showQrCode: false, showEndMarker: true,
    };
  }
  return {
    ...current, preset, fontSize: kitchen ? 'LARGE' : 'NORMAL', headerStyle: 'PROMINENT',
    showStoreName: true, showOrderType: true, showOrderNo: true, showOrderTime: true,
    showPickupNo: true, showTable: true, showItems: true, showItemSequence: false,
    showItemHeader: true, showItemOptions: true, showOptionGroupNames: false,
    showPrices: !kitchen, showPayment: !kitchen, emphasizePaid: !kitchen,
    showRemark: true, showCustomer: customer, showAddress: customer,
    showQrCode: customer, showEndMarker: true,
  };
}

function PaperPreview({ section, businessType }: { section: PrintTemplateSection; businessType: PrintBusinessType }) {
  const { layout } = section;
  const label = section.copyRole === 'ITEM';
  const scene = businessType === 'DINE_IN' ? '桌码堂食' : businessType === 'TAKEOUT' ? '到店自取' : '外卖配送';
  const sceneShort = businessType === 'DINE_IN' ? '堂食' : businessType === 'TAKEOUT' ? '自提' : '外卖';
  const products = [
    { name: '冰美式', sku: '中杯', quantity: 1, price: 1600, options: '少冰，不另外加糖', namedOptions: '温度：少冰，甜度：不另外加糖' },
    { name: '燕麦拿铁', sku: '大杯', quantity: 1, price: 2100, options: '热，加燕麦奶', namedOptions: '温度：热，加料：燕麦奶' },
  ];

  if (label) {
    const sizeStyle = { '--label-ratio': `${layout.labelWidthMM} / ${layout.labelHeightMM}` } as CSSProperties;
    return (
      <div className={`thermal-paper thermal-label font-${layout.fontSize.toLowerCase()}`} style={sizeStyle}>
        {layout.customHeader && <div className="thermal-custom">{layout.customHeader}</div>}
        <div className="thermal-label-meta">
          <span>{layout.showTable && businessType === 'DINE_IN' ? '桌号：VIP-001' : layout.showPickupNo ? '取餐：0038' : sceneShort}</span>
          {layout.showItemSequence && <strong>数量：1/2</strong>}
        </div>
        {layout.showItems && (
          <>
            <div className="thermal-label-product"><strong>燕麦拿铁</strong></div>
            <div className="thermal-label-detail">规格：大杯、燕麦奶</div>
          </>
        )}
        {layout.showItemOptions && <div className="thermal-label-detail">属性：{layout.showOptionGroupNames ? '温度：热、甜度：少糖' : '热、少糖'}</div>}
        {layout.showRemark && <div className="thermal-label-detail">备注：杯身写 Ethan</div>}
        {layout.showOrderNo && <div className="thermal-muted">订单：TB202607200001</div>}
        {layout.customFooter && <div className="thermal-footer">{layout.customFooter}</div>}
        <div className="thermal-label-bottom">
          {layout.showOrderTime && <span>07-23 12:11</span>}
          {layout.showOrderType && <b>{sceneShort}</b>}
        </div>
      </div>
    );
  }

  const pickupCode = 'A002';
  const endText = layout.endMarkerText || `--#${pickupCode}完--`;
  return (
    <div className={`thermal-paper paper-${section.paperWidth} font-${layout.fontSize.toLowerCase()}`}>
      {layout.customHeader && <div className="thermal-custom">{layout.customHeader}</div>}
      {layout.showStoreName && <div className={`thermal-store header-${layout.headerStyle.toLowerCase()}`}>码农咖啡</div>}
      {layout.showPickupNo
        ? <div className="thermal-pickup">({layout.copyTitle})取餐码:{pickupCode}</div>
        : <div className="thermal-pickup">({layout.copyTitle}){roleMeta[section.copyRole].label}</div>}
      <div className="thermal-rule" />
      {layout.showOrderType && <div className="thermal-pair"><span>类型</span><b>{scene}</b></div>}
      {layout.showOrderNo && <div className="thermal-pair"><span>订单编号</span><b>TB202607200001</b></div>}
      {layout.showTable && businessType === 'DINE_IN' && <div className="thermal-pair"><span>桌台</span><b>露台 · B02 桌</b></div>}
      {layout.showOrderTime && <div className="thermal-pair"><span>下单时间</span><b>2026-07-23 12:11</b></div>}
      {layout.showCustomer && <div className="thermal-pair"><span>顾客</span><b>赵先生 186****6557</b></div>}
      {layout.showAddress && businessType === 'DELIVERY' && <div className="thermal-address">天津市和平区南京路 88 号 A 座 1206</div>}
      {layout.showItems && (
        <>
          <div className="thermal-rule" />
          {layout.showItemHeader && <div className="thermal-items-head"><span>商品</span><span>数量</span>{layout.showPrices && <><span>单价</span><span>金额</span></>}</div>}
          {products.map((product) => (
            <div className="thermal-item" key={product.name}>
              <div className="thermal-item-name">
                {product.name} {product.sku}
                {layout.showItemOptions && `（${layout.showOptionGroupNames ? product.namedOptions : product.options}）`}
              </div>
              <div className="thermal-item-values">
                <span aria-hidden="true" />
                <strong>×{product.quantity}</strong>
                {layout.showPrices && <><strong>{cents(product.price)}</strong><strong>{cents(product.price * product.quantity)}</strong></>}
              </div>
            </div>
          ))}
        </>
      )}
      {layout.showPayment && (
        <>
          <div className="thermal-rule" />
          <div className="thermal-pair"><span>商品金额</span><b>37.00</b></div>
          <div className={layout.emphasizePaid ? 'thermal-total is-emphasized' : 'thermal-total'}><span>实付</span><strong>37.00</strong></div>
          <div className="thermal-pair"><span>支付方式</span><b>会生活聚合支付</b></div>
        </>
      )}
      {layout.showRemark && <div className="thermal-note"><b>备注</b> 燕麦拿铁少冰，杯身写 Ethan</div>}
      {layout.showQrCode && <div className="thermal-qr"><QRCode value="https://miniapp.example/order/TB202607200001" size={112} bordered={false} /><span>扫码查看订单</span></div>}
      {layout.customFooter && <div className="thermal-footer">{layout.customFooter}</div>}
      {layout.showEndMarker && <div className="thermal-end">{endText}</div>}
      {Array.from({ length: layout.feedLines }).map((_, index) => <div className="thermal-feed-line" key={index} />)}
    </div>
  );
}

export function BusinessPrintTemplatePage({
  businessType,
  previewMode = false,
}: {
  businessType: OrderBusinessType;
  previewMode?: boolean;
}) {
  const [template, setTemplate] = useState<BusinessPrintTemplate>(() => defaultPrintTemplate(businessType));
  const [activeType, setActiveType] = useState<PrintBusinessType>(businessType);
  const [activeRole, setActiveRole] = useState<PrintCopyRole>('MERCHANT');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [loadWarning, setLoadWarning] = useState('');
  const [dirtyRoles, setDirtyRoles] = useState<Set<PrintCopyRole>>(() => new Set());
  const loadRevision = useRef(0);
  const [messageApi, contextHolder] = message.useMessage();
  const [modal, modalContextHolder] = Modal.useModal();
  const domainName = businessType === 'DINE_IN' ? '店内' : '外卖';
  const sceneName = activeType === 'DINE_IN' ? '桌码堂食' : activeType === 'TAKEOUT' ? '快餐 / 到店自取' : '外卖配送';
  const section = template.sections[activeRole];
  const orderVariables = [...commonVariables, ...(activeType === 'DINE_IN' ? dineInVariables : [])];

  useEffect(() => {
    setActiveType(businessType);
    setActiveRole('MERCHANT');
    setTemplate(defaultPrintTemplate(businessType));
    setDirtyRoles(new Set());
  }, [businessType]);

  const load = useCallback(async () => {
    if (previewMode) {
      setTemplate(defaultPrintTemplate(activeType));
      setDirtyRoles(new Set());
      setLoadWarning('');
      setLoading(false);
      return;
    }
    const revision = ++loadRevision.current;
    setLoading(true);
    setLoadWarning('');
    try {
      const result = await api.getList<PrintTemplateRecord>(PRINT_TEMPLATES_ENDPOINT, { business_type: activeType, page_size: 20 });
      if (revision !== loadRevision.current) return;
      setTemplate(normalizePrintTemplates(result.items, activeType));
      setDirtyRoles(new Set());
    } catch (error) {
      if (revision !== loadRevision.current) return;
      setTemplate(defaultPrintTemplate(activeType));
      setDirtyRoles(new Set());
      setLoadWarning(`尚未读取到已保存模板，当前展示安全默认值：${errorMessage(error)}`);
    } finally {
      if (revision === loadRevision.current) setLoading(false);
    }
  }, [activeType, previewMode]);

  useEffect(() => { void load(); }, [load]);

  const updateSection = (patch: Partial<PrintTemplateSection>) => {
    setTemplate((current) => ({
      ...current,
      sections: { ...current.sections, [activeRole]: { ...current.sections[activeRole], ...patch } },
    }));
    setDirtyRoles((current) => new Set(current).add(activeRole));
  };

  const customizeLayout = <K extends keyof PrintTemplateLayout>(key: K, value: PrintTemplateLayout[K]) => {
    updateSection({ layout: { ...section.layout, preset: 'CUSTOM', [key]: value } });
  };

  const selectPreset = (preset: PrintLayoutPreset) => {
    updateSection({ layout: applyLayoutPreset(section, preset) });
  };

  const selectLabelPaper = (value: string) => {
    const paper = labelPaperSizes.find((item) => item.value === value);
    if (!paper) return;
    updateSection({ layout: { ...section.layout, labelWidthMM: paper.width, labelHeightMM: paper.height } });
  };

  const save = async () => {
    if (!section.name.trim()) {
      messageApi.warning('请填写模板名称');
      return;
    }
    setSaving(true);
    try {
      const saved = section.id
        ? await api.put<PrintTemplateRecord>(`${PRINT_TEMPLATES_ENDPOINT}/${section.id}`, printTemplatePayload(template, activeRole))
        : await api.post<PrintTemplateRecord>(PRINT_TEMPLATES_ENDPOINT, printTemplatePayload(template, activeRole));
      const normalized = normalizePrintTemplates([saved], activeType).sections[activeRole];
      setTemplate((current) => ({
        ...current,
        sections: { ...current.sections, [activeRole]: normalized },
      }));
      setDirtyRoles((current) => {
        const next = new Set(current);
        next.delete(activeRole);
        return next;
      });
      setLoadWarning('');
      messageApi.success(`${sceneName}${roleMeta[activeRole].label}模板已保存`);
    } catch (error) {
      messageApi.error(errorMessage(error));
    } finally {
      setSaving(false);
    }
  };

  const reset = () => {
    updateSection(defaultPrintTemplate(activeType).sections[activeRole]);
    messageApi.info(`已恢复${roleMeta[activeRole].label}默认格式，点击保存后才会生效`);
  };

  const dirtyRoleNames = (Object.keys(roleMeta) as PrintCopyRole[])
    .filter((role) => dirtyRoles.has(role))
    .map((role) => roleMeta[role].label);

  const confirmDiscard = (target: string, onConfirm: () => void) => {
    if (dirtyRoles.size === 0) {
      onConfirm();
      return;
    }
    modal.confirm({
      title: `放弃未保存的模板修改并${target}？`,
      content: `${dirtyRoleNames.join('、')}存在未保存修改。继续后这些修改将无法恢复。`,
      okText: `放弃并${target}`,
      cancelText: '继续编辑',
      okButtonProps: { danger: true },
      onOk: onConfirm,
    });
  };

  const reloadWithGuard = () => confirmDiscard('重新加载', () => { void load(); });

  const changeSceneWithGuard = (nextType: PrintBusinessType) => {
    if (nextType === activeType) return;
    confirmDiscard('切换场景', () => {
      setActiveRole('MERCHANT');
      setActiveType(nextType);
    });
  };

  const tabItems = (Object.keys(roleMeta) as PrintCopyRole[]).map((role) => ({
    key: role,
    label: <Space>{roleMeta[role].icon}<span>{roleMeta[role].label}</span>{dirtyRoles.has(role) ? <Tag color="warning">未保存</Tag> : template.sections[role].enabled && <Tag color="success">启用</Tag>}</Space>,
  }));

  const availableVisibility = visibilityOptions.filter((item) => !item.roles || item.roles.includes(activeRole));

  return (
    <div className="page-shell">
      {contextHolder}
      {modalContextHolder}
      <PageHeading
        title={`${domainName}打印模板`}
        description="选择常用版式或继续自定义；页面预览与芯烨打印指令使用同一套配置"
        extra={<Space><Button icon={<ReloadOutlined />} loading={loading} disabled={saving} onClick={reloadWithGuard}>重新加载</Button><Button disabled={loading || saving} onClick={reset}>恢复当前默认</Button><Button type="primary" icon={<SaveOutlined />} loading={saving} disabled={loading || !dirtyRoles.has(activeRole)} onClick={() => void save()}>保存当前联次</Button></Space>}
      />
      {businessType === 'DELIVERY' && <FeatureAvailabilityNotice className="printer-tip" feature="DELIVERY" />}
      {businessType === 'DINE_IN' && (
        <Card bordered={false} className="content-card print-scene-switch">
          <Typography.Text strong>店内订单场景</Typography.Text>
          <Segmented value={activeType} disabled={loading || saving} onChange={(value) => changeSceneWithGuard(value as PrintBusinessType)} options={[{ label: '桌码堂食', value: 'DINE_IN' }, { label: '快餐 / 到店自取', value: 'TAKEOUT' }]} />
        </Card>
      )}
      {loadWarning && <Alert className="printer-tip" type="warning" showIcon message="使用默认打印模板" description={loadWarning} />}

      <Card bordered={false} className="content-card print-copy-tabs" loading={loading}>
        <Tabs activeKey={activeRole} items={tabItems} onChange={(value) => setActiveRole(value as PrintCopyRole)} />
        <Alert type="info" showIcon message={`${sceneName} · ${roleMeta[activeRole].label}`} description={roleMeta[activeRole].description} />
      </Card>

      <Row gutter={[18, 18]} className={`print-builder-grid ${loading ? 'is-loading' : ''}`} aria-busy={loading}>
        <Col xs={24} xl={9}>
          <Card bordered={false} className="content-card print-preview-stage" title={<Space>{roleMeta[activeRole].icon}<span>打印效果预览</span><Tag>{activeRole === 'ITEM' ? `${section.layout.labelWidthMM} × ${section.layout.labelHeightMM}mm` : `${section.paperWidth}mm`}</Tag></Space>}>
            <PaperPreview section={section} businessType={activeType} />
            <Typography.Paragraph type="secondary" className="print-preview-note">示例订单仅用于预览；字号、字段、结束标识和标签序号会同步用于实际打印。</Typography.Paragraph>
          </Card>
        </Col>
        <Col xs={24} xl={15}>
          <Card bordered={false} className="content-card print-layout-editor" title="选择打印版式">
            <div className="print-template-switch-row">
              <div><strong>启用{roleMeta[activeRole].label}</strong><Typography.Paragraph type="secondary">关闭后该联次不会自动生成打印任务，手工补打仍受操作权限控制。</Typography.Paragraph></div>
              <Switch checked={section.enabled} onChange={(checked) => updateSection({ enabled: checked })} />
            </div>
            <div className="print-preset-grid" role="radiogroup" aria-label="打印版式">
              {(Object.keys(presetMeta) as PrintLayoutPreset[]).map((preset) => {
                const meta = presetMeta[preset];
                const selected = section.layout.preset === preset;
                return (
                  <button type="button" role="radio" aria-checked={selected} className={`print-preset-option ${selected ? 'is-selected' : ''}`} key={preset} onClick={() => selectPreset(preset)}>
                    <span className="print-preset-icon">{meta.icon}</span>
                    <span><strong>{meta.name}</strong><small>{meta.description}</small></span>
                    <span className="print-preset-check" aria-hidden="true">{selected ? '✓' : ''}</span>
                  </button>
                );
              })}
            </div>
          </Card>

          <Card bordered={false} className="content-card print-layout-editor" title={activeRole === 'ITEM' ? '标签与打印设置' : '纸张与打印设置'}>
            <Row gutter={14}>
              <Col xs={24} md={12}><label className="field-label">模板名称</label><Input value={section.name} maxLength={100} onChange={(event) => updateSection({ name: event.target.value })} /></Col>
              <Col xs={24} md={12}><label className="field-label">打印触发点</label><Select value={section.triggerEvent} style={{ width: '100%' }} onChange={(value) => updateSection({ triggerEvent: value })} options={[{ value: 'PAYMENT_SUCCESS', label: '付款成功后打印' }, { value: 'ORDER_CREATED', label: '下单后打印（含待付款）' }]} /></Col>
              {activeRole === 'ITEM' ? (
                <Col xs={24}>
                  <label className="field-label">标签纸尺寸（宽 × 高）</label>
                  <Segmented block value={`${section.layout.labelWidthMM}x${section.layout.labelHeightMM}`} onChange={(value) => selectLabelPaper(String(value))} options={labelPaperSizes.map((item) => ({ value: item.value, label: item.label }))} />
                  <Typography.Paragraph type="secondary" className="field-help">XP-T271U 使用 40 × 30mm；这里的尺寸会直接写入每一张标签的打印指令。</Typography.Paragraph>
                </Col>
              ) : (
                <Col xs={24} md={8}><label className="field-label">纸张宽度</label><Segmented block value={section.paperWidth} onChange={(value) => updateSection({ paperWidth: value as 58 | 80 })} options={[{ value: 58, label: '58mm' }, { value: 80, label: '80mm' }]} /></Col>
              )}
              <Col xs={24} md={activeRole === 'ITEM' ? 12 : 8}><label className="field-label">打印份数</label><InputNumber min={1} max={5} precision={0} value={section.copies} addonAfter="份" style={{ width: '100%' }} onChange={(value) => updateSection({ copies: Number(value || 1) })} /></Col>
              <Col xs={24} md={activeRole === 'ITEM' ? 12 : 8}><label className="field-label">正文字号</label><Segmented block value={section.layout.fontSize} onChange={(value) => customizeLayout('fontSize', value as PrintTemplateLayout['fontSize'])} options={[{ value: 'NORMAL', label: '普通' }, { value: 'LARGE', label: '大字' }]} /></Col>
            </Row>
          </Card>

          <Card bordered={false} className="content-card print-layout-editor" title={activeRole === 'ITEM' ? '标签显示内容' : '单据显示内容'}>
            <div className="print-visibility-grid">
              {availableVisibility.map((item) => (
                <Checkbox key={item.key} checked={Boolean(section.layout[item.key])} onChange={(event) => customizeLayout(item.key, event.target.checked as never)}>{item.label}</Checkbox>
              ))}
            </div>
            <Divider />
            <Row gutter={14}>
              {activeRole !== 'ITEM' && <Col xs={24} md={8}><label className="field-label">联次简称</label><Input value={section.layout.copyTitle} maxLength={4} placeholder="商 / 客 / 厨" onChange={(event) => customizeLayout('copyTitle', event.target.value)} /></Col>}
              <Col xs={24} md={activeRole === 'ITEM' ? 12 : 8}><label className="field-label">自定义抬头</label><Input value={section.layout.customHeader} maxLength={100} placeholder="例如：预订单 / 请优先制作" onChange={(event) => customizeLayout('customHeader', event.target.value)} /></Col>
              <Col xs={24} md={activeRole === 'ITEM' ? 12 : 8}><label className="field-label">自定义底部</label><Input value={section.layout.customFooter} maxLength={200} placeholder="例如：感谢光临" onChange={(event) => customizeLayout('customFooter', event.target.value)} /></Col>
              {activeRole !== 'ITEM' && (
                <>
                  <Col xs={24} md={8}><label className="field-label">显示结束标识</label><Switch checked={section.layout.showEndMarker} checkedChildren="显示" unCheckedChildren="隐藏" onChange={(checked) => customizeLayout('showEndMarker', checked)} /></Col>
                  <Col xs={24} md={8}><label className="field-label">结束文案</label><Input value={section.layout.endMarkerText} maxLength={40} placeholder="默认：--#取餐码完--" onChange={(event) => customizeLayout('endMarkerText', event.target.value)} /></Col>
                  <Col xs={24} md={8}><label className="field-label">打印后走纸</label><InputNumber min={0} max={8} precision={0} value={section.layout.feedLines} addonAfter="行" style={{ width: '100%' }} onChange={(value) => customizeLayout('feedLines', Number(value ?? 0))} /></Col>
                </>
              )}
            </Row>
          </Card>

          <Card bordered={false} className="content-card print-layout-editor print-advanced-card">
            <Collapse ghost items={[{
              key: 'advanced',
              label: '高级兼容：查看纯文本回退模板',
              children: <><Input.TextArea rows={8} className="template-textarea" value={section.templateText} spellCheck={false} onChange={(event) => updateSection({ templateText: event.target.value })} /><Typography.Paragraph type="secondary">结构化布局优先用于新打印任务；此文本用于旧设备或布局解析失败时安全回退。</Typography.Paragraph></>,
            }, {
              key: 'variables',
              label: '可用打印变量',
              children: <div className="template-variable-box"><strong><TagsOutlined /> 订单级变量</strong><div>{orderVariables.map((value) => <Tag key={value}>{value}</Tag>)}</div>{activeRole === 'ITEM' && <><strong>商品标签额外变量</strong><div>{labelItemVariables.map((value) => <Tag key={value} color="blue">{value}</Tag>)}</div></>}</div>,
            }]} />
          </Card>
          <Typography.Paragraph type="secondary" className="template-meta">当前联次更新：{dateTime(section.updatedAt)}</Typography.Paragraph>
        </Col>
      </Row>
      <Alert className="printer-compatibility-note" icon={<QrcodeOutlined />} type="success" showIcon message="预览和实际打印已使用同一套结构" description="商品标签会按整单商品数量显示 1/2、2/2；单据会突出联次简称和取餐码，并在结束标识后按设置继续走纸。" />
    </div>
  );
}
