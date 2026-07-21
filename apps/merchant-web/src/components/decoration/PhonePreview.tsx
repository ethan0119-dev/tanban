import {
  HomeOutlined,
  PictureOutlined,
  ShoppingCartOutlined,
  SnippetsOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { Image, Segmented } from 'antd';
import type { CSSProperties, ReactNode } from 'react';
import { HOME_MODULE_LABELS, type DecorationConfig, type NavigationKey, type PreviewPage } from '../../features/decoration/model';

const navigationIcons: Record<NavigationKey, ReactNode> = {
  HOME: <HomeOutlined />,
  MENU: <ShoppingCartOutlined />,
  ORDERS: <SnippetsOutlined />,
  PROFILE: <UserOutlined />,
};

interface PhonePreviewProps {
  config: DecorationConfig;
  storeName?: string;
  page: PreviewPage;
  onPageChange: (page: PreviewPage) => void;
  showSplash?: boolean;
}

export function PhonePreview({ config, storeName = '我的门店', page, onPageChange, showSplash = false }: PhonePreviewProps) {
  const radius = config.theme.radius === 'SM' ? 8 : config.theme.radius === 'MD' ? 14 : 20;
  const buttonRadius = config.theme.buttonShape === 'SQUARE' ? 4 : config.theme.buttonShape === 'PILL' ? 999 : Math.max(6, radius - 3);
  const fontScale = config.theme.fontScale === 'COMPACT' ? .9 : config.theme.fontScale === 'LARGE' ? 1.1 : 1;
  const cardBorder = config.theme.surfaceStyle === 'BORDERED' ? `1px solid ${withAlpha(config.theme.textColor, .12)}` : '1px solid transparent';
  const cardShadow = config.theme.surfaceStyle === 'ELEVATED' ? `0 7px 18px ${withAlpha(config.theme.textColor, .11)}` : 'none';
  const style = {
    '--decor-primary': config.theme.primaryColor,
    '--decor-accent': config.theme.accentColor,
    '--decor-background': config.theme.backgroundColor,
    '--decor-surface': config.theme.surfaceColor,
    '--decor-text': config.theme.textColor,
    '--decor-muted': config.theme.mutedColor,
    '--decor-nav-background': config.theme.navBackgroundColor,
    '--decor-nav-text': config.theme.navTextColor,
    '--decor-nav-selected': config.theme.navSelectedColor,
    '--decor-card-radius': `${radius}px`,
    '--decor-button-radius': `${buttonRadius}px`,
    '--decor-on-primary': onColor(config.theme.primaryColor),
    '--decor-on-accent': onColor(config.theme.accentColor),
    '--decor-font-scale': fontScale,
    '--decor-card-border': cardBorder,
    '--decor-card-shadow': cardShadow,
  } as CSSProperties;

  return (
    <aside className="decoration-preview-panel">
      <div className="decoration-preview-heading">
        <div><strong>实时预览</strong><span>草稿变化不会影响线上</span></div>
        <Segmented<PreviewPage>
          size="small"
          value={page}
          options={[{ label: '首页', value: 'HOME' }, { label: '点单', value: 'MENU' }]}
          onChange={onPageChange}
        />
      </div>
      <div className="phone-shell">
        <div className="phone-speaker" />
        <div className="phone-screen" style={style}>
          <div className="mini-status"><span>09:41</span><span>● ● ▰</span></div>
          <div className="mini-nav-title">{storeName}<span>···</span></div>
          {showSplash && config.splash.enabled
            ? <SplashPreview config={config} />
            : page === 'HOME'
              ? <HomePreview config={config} storeName={storeName} />
              : <MenuPreview config={config} />}
          <nav className={`mini-tabbar nav-${config.navigationTemplate}`}>
            {config.navigation.filter((item) => item.enabled).map((item) => (
              <button
                type="button"
                key={item.id}
                className={(page === 'HOME' && item.key === 'HOME') || (page === 'MENU' && item.key === 'MENU') ? 'active' : ''}
                onClick={() => {
                  if (item.key === 'HOME' || item.key === 'MENU') onPageChange(item.key);
                }}
              >
                {navigationIcons[item.key]}<span>{item.label}</span>
              </button>
            ))}
          </nav>
        </div>
      </div>
      <p className="preview-note">预览用于确认布局和视觉层级；真机字体、状态栏及图片裁切以微信小程序为准。</p>
    </aside>
  );
}

function HomePreview({ config, storeName }: { config: DecorationConfig; storeName: string }) {
  return (
    <div className="mini-content mini-home">
      {config.homeModules.filter((item) => item.enabled).map((module) => {
        switch (module.type) {
          case 'HERO_BANNER':
            return (
              <section className="mini-hero" key={module.id} style={backgroundImage(module.imageUrl)}>
                {!module.imageUrl && <><small>{module.subtitle}</small><strong>{module.title || '上传 Banner 后展示主视觉'}</strong></>}
              </section>
            );
          case 'STORE_HEADER':
            return (
              <section className="mini-store" key={module.id}>
                <AssetCircle url="">{storeName.slice(0, 1) || '伴'}</AssetCircle>
                <div><strong>{storeName}</strong><span>{module.title} · 欢迎扫码点单</span></div>
              </section>
            );
          case 'ANNOUNCEMENT':
            return <section className="mini-notice" key={module.id}><b>{module.title || '公告'}</b> · 下单后请留意取餐码</section>;
          case 'QUICK_ACTIONS':
            return (
              <section className="mini-quick" key={module.id}>
                <button type="button"><strong>{module.title}</strong><span>{module.subtitle}</span></button>
                <button type="button"><strong>查看订单</strong><span>支付与制作进度</span></button>
              </section>
            );
          case 'TEXT':
            return <section className="mini-feature" key={module.id}><small>FOR SMALL BUSINESS</small><strong>{module.title}</strong><span>{module.subtitle}</span></section>;
          case 'IMAGE':
            return <section className="mini-hero" key={module.id} style={backgroundImage(module.imageUrl)}><strong>{module.title}</strong></section>;
          case 'HOTSPOT_IMAGE':
            return (
              <section className="mini-hotspot-image" key={module.id}>
                {module.imageUrl
                  ? <>
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img src={module.imageUrl} alt={module.title || '热区图片'} />
                  </>
                  : <div className="mini-hotspot-placeholder"><PicturePlaceholder />上传首页热区图片</div>}
                {(module.hotspots ?? []).map((hotspot, index) => <span key={hotspot.id} title={`${hotspot.label} · ${hotspot.action.type}`} style={hotspotStyle(hotspot)}>{index + 1}</span>)}
              </section>
            );
          case 'SPACER':
            return <div key={module.id} style={{ height: Math.min(80, Math.max(4, Number.parseInt(module.subtitle, 10) || 24)) }} />;
          default:
            return <section key={module.id}>{HOME_MODULE_LABELS[module.type]}</section>;
        }
      })}
    </div>
  );
}

function MenuPreview({ config }: { config: DecorationConfig }) {
  const products = [
    { name: '经典拿铁', description: '浓缩咖啡与鲜奶', price: '18', soldOut: false },
    { name: '冰美式', description: '清爽明亮，现点现做', price: '12', soldOut: false },
    { name: '桂花特调', description: '桂花、燕麦奶与咖啡', price: '22', soldOut: true },
  ];
  return (
    <div className={`mini-content mini-menu ${config.ordering.layout === 'CATEGORY_TOP' ? 'category-top' : ''} ${config.ordering.productLayout === 'GRID' ? 'product-grid' : ''} ${config.ordering.density === 'COMPACT' ? 'compact' : ''}`}>
      <div className="mini-categories"><b>咖啡</b><span>拿铁</span><span>特调</span><span>轻食</span></div>
      <div className="mini-products">
        <header><strong>今晚菜单</strong><span>现点现做</span></header>
        {products.map((product, index) => (
          <article key={product.name}>
            <div className="mini-product-image">{index === 0 ? '拿' : index === 1 ? '美' : '桂'}</div>
            <div>
              <strong>{product.name}</strong>
              {config.ordering.showDescription && <span>{product.description}</span>}
              {config.ordering.showSales && <small>月售 {28 - index * 7}</small>}
              {config.ordering.showStock && <small>库存 {16 - index * 3}</small>}
              <b>¥{product.price}</b>
            </div>
            {product.soldOut && config.ordering.showSoldOut ? <em>售罄</em> : <button type="button">＋</button>}
          </article>
        ))}
      </div>
      <div className="mini-cart"><b>2</b><strong>¥30</strong><span>已选 2 件</span><button type="button">去结算</button></div>
    </div>
  );
}

function SplashPreview({ config }: { config: DecorationConfig }) {
  return (
    <div className="mini-splash" style={backgroundImage(config.splash.imageUrl)}>
      <button type="button">{config.splash.frequency === 'EVERY_VISIT' ? '每次展示' : config.splash.frequency === 'DAILY' ? '每日一次' : '本版本一次'} · {config.splash.autoCloseSeconds}s</button>
      <div><strong>{config.splash.title}</strong><span>{config.splash.subtitle}</span></div>
    </div>
  );
}

function AssetCircle({ url, children }: { url: string; children: ReactNode }) {
  return url ? <Image className="mini-logo" src={url} alt="门店 Logo" preview={false} /> : <span className="mini-logo placeholder">{children}</span>;
}

function backgroundImage(url?: string): CSSProperties | undefined {
  return url ? { backgroundImage: `linear-gradient(180deg, transparent, rgba(17, 24, 20, .45)), url("${url.replaceAll('"', '%22')}")` } : undefined;
}

function hotspotStyle(value: { x: number; y: number; width: number; height: number }): CSSProperties {
  return { left: `${value.x}%`, top: `${value.y}%`, width: `${value.width}%`, height: `${value.height}%` };
}

function PicturePlaceholder() {
  return <PictureOutlined aria-hidden="true" />;
}

function onColor(value: string): string {
  const channels = [value.slice(1, 3), value.slice(3, 5), value.slice(5, 7)].map((part) => Number.parseInt(part, 16));
  const luminance = channels.map((channel) => {
    const normalized = channel / 255;
    return normalized <= .03928 ? normalized / 12.92 : ((normalized + .055) / 1.055) ** 2.4;
  });
  return .2126 * luminance[0] + .7152 * luminance[1] + .0722 * luminance[2] > .42 ? '#111111' : '#FFFFFF';
}

function withAlpha(value: string, alpha: number): string {
  const channels = [value.slice(1, 3), value.slice(3, 5), value.slice(5, 7)].map((part) => Number.parseInt(part, 16));
  return `rgba(${channels[0]},${channels[1]},${channels[2]},${alpha})`;
}
