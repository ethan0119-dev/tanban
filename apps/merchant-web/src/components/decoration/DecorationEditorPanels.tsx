import { ArrowDownOutlined, ArrowUpOutlined, CheckCircleFilled, DeleteOutlined, FolderOpenOutlined, HomeOutlined, PictureOutlined, PlusOutlined, ShoppingOutlined, SnippetsOutlined, UserOutlined } from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Col,
  ColorPicker,
  Form,
  Input,
  InputNumber,
  Radio,
  Row,
  Segmented,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
} from 'antd';
import { useState } from 'react';
import { HOME_MODULE_LABELS, NAVIGATION_LABELS, type DecorationConfig, type DecorationTemplate, type HomeModuleType, type MediaAsset } from '../../features/decoration/model';
import { HotspotImageEditor } from './HotspotImageEditor';
import { MediaLibraryModal } from '../media/MediaLibraryModal';

interface ConfigPanelProps {
  config: DecorationConfig;
  onChange: (config: DecorationConfig) => void;
}

interface PageModulesPanelProps extends ConfigPanelProps {
  assets?: MediaAsset[];
  assetUploading?: boolean;
  onUploadAsset?: (file: File) => Promise<MediaAsset>;
}

export function PageModulesPanel({ config, onChange, assets = [], assetUploading = false, onUploadAsset }: PageModulesPanelProps) {
  const [newType, setNewType] = useState<HomeModuleType>('TEXT');
  const [libraryTarget, setLibraryTarget] = useState<number | null>(null);
  const updateModule = (index: number, patch: Partial<DecorationConfig['homeModules'][number]>) => {
    const modules = config.homeModules.map((item, itemIndex) => itemIndex === index ? { ...item, ...patch } : item);
    onChange({ ...config, homeModules: modules });
  };
  const move = (index: number, direction: -1 | 1) => {
    const destination = index + direction;
    if (destination < 0 || destination >= config.homeModules.length) return;
    const modules = [...config.homeModules];
    [modules[index], modules[destination]] = [modules[destination], modules[index]];
    onChange({ ...config, homeModules: modules });
  };
  const addModule = () => {
    const nextIndex = config.homeModules.length;
    const id = `${newType.toLowerCase().replaceAll('_', '-')}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 7)}`;
    const defaults = moduleDefaults(newType);
    onChange({ ...config, homeModules: [...config.homeModules, { id, type: newType, enabled: true, sortOrder: (nextIndex + 1) * 10, ...defaults }] });
  };
  const removeModule = (index: number) => onChange({ ...config, homeModules: config.homeModules.filter((_, itemIndex) => itemIndex !== index) });
  return (
    <div className="decoration-panel-stack">
      <PanelIntro title="页面编排" description="用安全的受控模块组合首页。调整顺序、文案和显示状态后，可立即在右侧看到效果。" />
      <Card size="small" className="decor-section-card">
        <Space.Compact block>
          <Select<HomeModuleType> value={newType} options={Object.entries(HOME_MODULE_LABELS).map(([value, label]) => ({ value: value as HomeModuleType, label }))} onChange={setNewType} />
          <Button type="primary" icon={<PlusOutlined />} onClick={addModule}>添加模块</Button>
        </Space.Compact>
      </Card>
      {config.homeModules.map((module, index) => (
        <Card
          size="small"
          className={`decor-module-card ${module.enabled ? '' : 'disabled'}`}
          key={module.id}
          title={<Space><span className="module-order">{index + 1}</span><span>{HOME_MODULE_LABELS[module.type]}</span></Space>}
          extra={<Space size={4}>
            <Button type="text" size="small" icon={<ArrowUpOutlined />} disabled={index === 0} aria-label="上移" onClick={() => move(index, -1)} />
            <Button type="text" size="small" icon={<ArrowDownOutlined />} disabled={index === config.homeModules.length - 1} aria-label="下移" onClick={() => move(index, 1)} />
            <Button type="text" danger size="small" icon={<DeleteOutlined />} aria-label="删除" onClick={() => removeModule(index)} />
            <Switch size="small" checked={module.enabled} onChange={(enabled) => updateModule(index, { enabled })} />
          </Space>}
        >
          {module.type === 'HOTSPOT_IMAGE'
            ? <HotspotImageEditor
              module={module}
              assets={assets}
              uploading={assetUploading}
              onChange={(patch) => updateModule(index, patch)}
              onUpload={onUploadAsset ?? (async () => { throw new Error('图片上传服务暂不可用'); })}
            />
            : module.type === 'STORE_HEADER'
            ? <Typography.Text type="secondary">展示门店设置中的 Logo、营业状态和经营地址。</Typography.Text>
            : module.type === 'SPACER'
              ? <Form.Item label="留白高度"><InputNumber min={4} max={160} addonAfter="px" value={Number.parseInt(module.subtitle, 10) || 24} onChange={(value) => updateModule(index, { subtitle: String(value ?? 24) })} /></Form.Item>
              : <>
                <Row gutter={12}>
                  <Col span={12}><Form.Item label={module.type === 'ANNOUNCEMENT' ? '公告前缀' : '主标题'}><Input value={module.title} maxLength={module.type === 'ANNOUNCEMENT' ? 16 : 30} onChange={(event) => updateModule(index, { title: event.target.value })} /></Form.Item></Col>
                  {module.type !== 'ANNOUNCEMENT' && <Col span={12}><Form.Item label="辅助文案"><Input value={module.subtitle} maxLength={80} onChange={(event) => updateModule(index, { subtitle: event.target.value })} /></Form.Item></Col>}
                </Row>
                {(module.type === 'HERO_BANNER' || module.type === 'IMAGE') && <Form.Item label="模块图片"><Space.Compact block><Input value={module.imageUrl} prefix={<PictureOutlined />} placeholder="从图片库选择，或填入 HTTPS 地址" onChange={(event) => updateModule(index, { imageUrl: event.target.value })} /><Button icon={<FolderOpenOutlined />} onClick={() => setLibraryTarget(index)}>图片库</Button></Space.Compact></Form.Item>}
                {module.type === 'ANNOUNCEMENT' && <Typography.Text type="secondary">公告正文沿用“门店设置”里的店铺公告，这里仅配置前缀。</Typography.Text>}
              </>}
        </Card>
      ))}
      <MediaLibraryModal
        open={libraryTarget !== null}
        title="选择装修图片"
        excludeUrls={libraryTarget === null ? [] : [config.homeModules[libraryTarget]?.imageUrl || '']}
        onCancel={() => setLibraryTarget(null)}
        onConfirm={(selected) => {
          if (libraryTarget !== null && selected[0]) updateModule(libraryTarget, { imageUrl: selected[0].url });
          setLibraryTarget(null);
        }}
      />
    </div>
  );
}

function moduleDefaults(type: HomeModuleType): Pick<DecorationConfig['homeModules'][number], 'title' | 'subtitle' | 'imageUrl' | 'hotspots'> {
  switch (type) {
    case 'HERO_BANNER': return { title: '首页主视觉', subtitle: '点击进入点单', imageUrl: '' };
    case 'STORE_HEADER': return { title: '营业中', subtitle: '展示门店信息', imageUrl: '' };
    case 'ANNOUNCEMENT': return { title: '公告', subtitle: '门店公告正文来自门店设置', imageUrl: '' };
    case 'QUICK_ACTIONS': return { title: '堂食 / 自提点单', subtitle: '选好口味，在线下单', imageUrl: '' };
    case 'IMAGE': return { title: '活动图片', subtitle: '', imageUrl: 'https://placehold.co/1200x600/png' };
    case 'HOTSPOT_IMAGE': return { title: '首页导航图', subtitle: '', imageUrl: '', hotspots: [] };
    case 'SPACER': return { title: '留白', subtitle: '24', imageUrl: '' };
    case 'TEXT':
    default: return { title: '品牌故事', subtitle: '在这里介绍门店或活动内容', imageUrl: '' };
  }
}

export function ThemePanel({ config, onChange }: ConfigPanelProps) {
  const updateTheme = (patch: Partial<DecorationConfig['theme']>) => onChange({ ...config, theme: { ...config.theme, ...patch } });
  return (
    <div className="decoration-panel-stack">
      <PanelIntro title="全店风格" description="统一控制品牌色、内容表面、文字和导航色，应用于首页、点单页和底部导航。" />
      <Card size="small" className="decor-section-card" title="品牌色板">
        <Row gutter={[16, 10]}>
          <Col span={12}><ColorField label="品牌主色" value={config.theme.primaryColor} onChange={(primaryColor) => updateTheme({ primaryColor })} /></Col>
          <Col span={12}><ColorField label="强调色" value={config.theme.accentColor} onChange={(accentColor) => updateTheme({ accentColor })} /></Col>
          <Col span={12}><ColorField label="页面背景" value={config.theme.backgroundColor} onChange={(backgroundColor) => updateTheme({ backgroundColor })} /></Col>
          <Col span={12}><ColorField label="卡片表面" value={config.theme.surfaceColor} onChange={(surfaceColor) => updateTheme({ surfaceColor })} /></Col>
          <Col span={12}><ColorField label="正文颜色" value={config.theme.textColor} onChange={(textColor) => updateTheme({ textColor })} /></Col>
          <Col span={12}><ColorField label="辅助文字" value={config.theme.mutedColor} onChange={(mutedColor) => updateTheme({ mutedColor })} /></Col>
        </Row>
      </Card>
      <Card size="small" className="decor-section-card" title="底部导航色板">
        <Row gutter={[16, 10]}>
          <Col span={12}><ColorField label="导航背景" value={config.theme.navBackgroundColor} onChange={(navBackgroundColor) => updateTheme({ navBackgroundColor })} /></Col>
          <Col span={12}><ColorField label="普通文字" value={config.theme.navTextColor} onChange={(navTextColor) => updateTheme({ navTextColor })} /></Col>
          <Col span={12}><ColorField label="选中颜色" value={config.theme.navSelectedColor} onChange={(navSelectedColor) => updateTheme({ navSelectedColor })} /></Col>
        </Row>
      </Card>
      <Card size="small" className="decor-section-card" title="形状">
        <Form layout="vertical"><Form.Item label="全局圆角"><Segmented block value={config.theme.radius} options={[{ label: '小', value: 'SM' }, { label: '中', value: 'MD' }, { label: '大', value: 'LG' }]} onChange={(radius) => updateTheme({ radius: radius as DecorationConfig['theme']['radius'] })} /></Form.Item></Form>
      </Card>
    </div>
  );
}

export function OrderingPanel({ config, onChange }: ConfigPanelProps) {
  const update = (patch: Partial<DecorationConfig['ordering']>) => onChange({ ...config, ordering: { ...config.ordering, ...patch } });
  return (
    <div className="decoration-panel-stack">
      <PanelIntro title="点单风格" description="改变分类、商品和加购方式，不会改变商品、规格、加料或价格数据。" />
      <Card size="small" className="decor-section-card">
        <Form layout="vertical">
          <Form.Item label="分类导航布局"><Radio.Group value={config.ordering.layout} onChange={(event) => update({ layout: event.target.value })}><Radio.Button value="CATEGORY_LEFT">左侧分类</Radio.Button><Radio.Button value="CATEGORY_TOP">顶部分类</Radio.Button></Radio.Group></Form.Item>
          <Form.Item label="商品布局"><Segmented block value={config.ordering.productLayout} options={[{ label: '列表', value: 'LIST' }, { label: '宫格', value: 'GRID' }]} onChange={(productLayout) => update({ productLayout: productLayout as DecorationConfig['ordering']['productLayout'] })} /></Form.Item>
          <Form.Item label="信息密度"><Segmented block value={config.ordering.density} options={[{ label: '舒适', value: 'COMFORTABLE' }, { label: '紧凑', value: 'COMPACT' }]} onChange={(density) => update({ density: density as DecorationConfig['ordering']['density'] })} /></Form.Item>
          <Form.Item label="商品加载"><Segmented block value={config.ordering.loadMode} options={[{ label: '按分类加载', value: 'BY_CATEGORY' }, { label: '一次加载全部', value: 'ALL' }]} onChange={(loadMode) => update({ loadMode: loadMode as DecorationConfig['ordering']['loadMode'] })} /></Form.Item>
          <Form.Item label="加购方式"><Segmented block value={config.ordering.productActionMode} options={[{ label: '打开规格面板', value: 'SKU_SHEET' }, { label: '直接加入', value: 'DIRECT_ADD' }]} onChange={(productActionMode) => update({ productActionMode: productActionMode as DecorationConfig['ordering']['productActionMode'] })} /></Form.Item>
        </Form>
      </Card>
      <Card size="small" className="decor-section-card" title="商品信息">
        <SwitchRow label="显示商品描述" description="在名称下展示商品简介" checked={config.ordering.showDescription} onChange={(showDescription) => update({ showDescription })} />
        <SwitchRow label="展示售罄状态" description="库存为零时显示售罄标识" checked={config.ordering.showSoldOut} onChange={(showSoldOut) => update({ showSoldOut })} />
        <SwitchRow label="展示剩余库存" description="在商品卡片中展示当前可售库存" checked={config.ordering.showStock} onChange={(showStock) => update({ showStock })} />
        <SwitchRow label="展示月销量" description="在商品卡片中展示近期销量" checked={config.ordering.showSales} onChange={(showSales) => update({ showSales })} />
      </Card>
      <Alert showIcon type="info" message="规格、属性和加料选项仍由商品中心维护" description="装修只决定呈现与加购方式；杯型、温度、甜度和加料来自商品配置。" />
    </div>
  );
}

export function TemplatePanel({ config, templates, loading, onApply }: { config: DecorationConfig; templates: DecorationTemplate[]; loading: boolean; onApply: (template: DecorationTemplate) => void }) {
  return (
    <div className="decoration-panel-stack">
      <PanelIntro title="装修模板" description="快速套用一组经过验证的颜色、圆角、首页模块和点单风格。套用后仍可逐项调整。" />
      <Alert showIcon type="warning" message="套用模板会覆盖当前视觉和页面编排" description="已配置的首页 Banner 会保留；点击顶部“保存草稿”前不会写入服务器。" />
      <Row gutter={[14, 14]} className="template-grid">
        {templates.map((template) => <Col xs={24} md={12} key={template.key}>
          <Card className={`template-card ${config.templateKey === template.key ? 'selected' : ''}`} loading={loading} hoverable onClick={() => onApply(template)}>
            <div className="template-tone" style={{ background: template.tone }}><span>{template.name.slice(0, 1)}</span></div>
            <div className="template-copy"><Space><strong>{template.name}</strong>{config.templateKey === template.key && <CheckCircleFilled />}</Space><p>{template.description}</p><Button size="small" type={config.templateKey === template.key ? 'primary' : 'default'}>{config.templateKey === template.key ? '当前模板' : '应用模板'}</Button></div>
          </Card>
        </Col>)}
      </Row>
    </div>
  );
}

export function NavigationPanel({ config, onChange }: ConfigPanelProps) {
  const update = (index: number, patch: Partial<DecorationConfig['navigation'][number]>) => onChange({ ...config, navigation: config.navigation.map((item, itemIndex) => itemIndex === index ? { ...item, ...patch } : item) });
  const templates = [
    { key: 'classic' as const, name: '经典线框', description: '白底、线框图标，适合多数门店。', background: '#FFFFFF', text: '#7B807A', selected: config.theme.primaryColor },
    { key: 'soft' as const, name: '轻柔圆润', description: '柔和底色与强调色，视觉更轻盈。', background: config.theme.surfaceColor, text: config.theme.mutedColor, selected: config.theme.primaryColor },
    { key: 'warm' as const, name: '暖调门店', description: '奶油底与暖棕图标，适合烘焙和咖啡。', background: '#FFF7EA', text: '#806F65', selected: '#9A5F3D' },
    { key: 'dark' as const, name: '深色强调', description: '深色底栏与高对比图标，适合夜市。', background: config.theme.textColor, text: config.theme.mutedColor, selected: config.theme.accentColor },
  ];
  const applyTemplate = (template: typeof templates[number]) => onChange({
    ...config,
    navigationTemplate: template.key,
    theme: { ...config.theme, navBackgroundColor: template.background, navTextColor: template.text, navSelectedColor: template.selected },
  });
  return (
    <div className="decoration-panel-stack">
      <PanelIntro title="导航设置" description="从预置导航模板中选择图标和底栏配色，并设置四个固定入口名称。" />
      <Row gutter={[12, 12]} className="navigation-template-grid">
        {templates.map((template) => <Col xs={24} md={12} key={template.key}>
          <Card hoverable size="small" className={`navigation-template-card ${config.navigationTemplate === template.key ? 'selected' : ''}`} onClick={() => applyTemplate(template)}>
            <div className="navigation-template-preview" style={{ background: template.background, color: template.text }}>
              {[<HomeOutlined key="home" />, <ShoppingOutlined key="menu" />, <SnippetsOutlined key="orders" />, <UserOutlined key="profile" />].map((icon, index) => <span key={index} style={{ color: index === 0 ? template.selected : template.text }}>{icon}<i>{['首页', '点单', '订单', '我的'][index]}</i></span>)}
            </div>
            <div><Space><strong>{template.name}</strong>{config.navigationTemplate === template.key && <CheckCircleFilled />}</Space><Typography.Text type="secondary">{template.description}</Typography.Text></div>
          </Card>
        </Col>)}
      </Row>
      <Alert showIcon type="info" message="图标随模板统一提供" description="页面路径和顺序固定，商户可修改入口文案；发布后真机导航会同步模板图标与配色。" />
      {config.navigation.map((item, index) => <Card size="small" className="navigation-card" key={item.id}><div><Tag>{NAVIGATION_LABELS[item.key]}</Tag><Input value={item.label} maxLength={8} onChange={(event) => update(index, { label: event.target.value })} /></div><Tag color="default">固定入口</Tag></Card>)}
    </div>
  );
}

export function SplashPanel({ config, onChange }: ConfigPanelProps) {
  const [libraryOpen, setLibraryOpen] = useState(false);
  const update = (patch: Partial<DecorationConfig['splash']>) => onChange({ ...config, splash: { ...config.splash, ...patch } });
  return (
    <div className="decoration-panel-stack">
      <PanelIntro title="启动页设置" description="顾客进入门店时短暂展示品牌画面。建议只用于活动或品牌内容，避免延误点单。" />
      <Card size="small" className="decor-section-card"><SwitchRow label="启用启动页" description="发布后按所选频率向进入当前门店的顾客展示" checked={config.splash.enabled} onChange={(enabled) => update({ enabled })} /></Card>
      <Card size="small" className="decor-section-card">
        <Form layout="vertical" disabled={!config.splash.enabled}>
          <Form.Item label="背景图片"><Space.Compact block><Input value={config.splash.imageUrl} placeholder="必须为 HTTPS，建议竖图 750 × 1334" onChange={(event) => update({ imageUrl: event.target.value })} /><Button icon={<FolderOpenOutlined />} onClick={() => setLibraryOpen(true)}>图片库</Button></Space.Compact></Form.Item>
          <Form.Item label="主标题"><Input value={config.splash.title} maxLength={60} onChange={(event) => update({ title: event.target.value })} /></Form.Item>
          <Form.Item label="副标题"><Input value={config.splash.subtitle} maxLength={160} onChange={(event) => update({ subtitle: event.target.value })} /></Form.Item>
          <Form.Item label="展示方式"><Segmented block value={config.splash.displayMode} options={[{ label: '弹窗', value: 'POPUP' }, { label: '全屏', value: 'FULLSCREEN' }]} onChange={(displayMode) => update({ displayMode: displayMode as DecorationConfig['splash']['displayMode'] })} /></Form.Item>
          <Form.Item label="自动关闭"><InputNumber min={0} max={30} precision={0} addonAfter="秒" value={config.splash.autoCloseSeconds} onChange={(autoCloseSeconds) => update({ autoCloseSeconds: autoCloseSeconds ?? 5 })} /></Form.Item>
          <Form.Item label="展示频率"><Select value={config.splash.frequency} options={[{ label: '每次进入', value: 'EVERY_VISIT' }, { label: '每天一次', value: 'DAILY' }, { label: '每个发布版本一次', value: 'ONCE_PER_VERSION' }]} onChange={(frequency) => update({ frequency })} /></Form.Item>
        </Form>
      </Card>
      <MediaLibraryModal open={libraryOpen} title="选择启动页图片" excludeUrls={[config.splash.imageUrl]} onCancel={() => setLibraryOpen(false)} onConfirm={(selected) => { if (selected[0]) update({ imageUrl: selected[0].url }); setLibraryOpen(false); }} />
    </div>
  );
}

function PanelIntro({ title, description }: { title: string; description: string }) {
  return <div className="decor-panel-intro"><Typography.Title level={4}>{title}</Typography.Title><Typography.Paragraph>{description}</Typography.Paragraph></div>;
}

function ColorField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return <Form.Item label={label}><Space.Compact block><ColorPicker value={value} onChangeComplete={(color) => onChange(color.toHexString().toUpperCase())} /><Input value={value} maxLength={7} onChange={(event) => onChange(event.target.value)} /></Space.Compact></Form.Item>;
}

function SwitchRow({ label, description, checked, disabled, onChange }: { label: string; description: string; checked: boolean; disabled?: boolean; onChange: (value: boolean) => void }) {
  return <div className="decor-switch-row"><div><strong>{label}</strong><p>{description}</p></div><Switch checked={checked} disabled={disabled} onChange={onChange} /></div>;
}
