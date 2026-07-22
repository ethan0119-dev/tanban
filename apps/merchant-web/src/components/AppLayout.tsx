/* eslint-disable @next/next/no-img-element -- this Vite app imports a fingerprinted local brand asset */
import {
  AppstoreOutlined,
  BgColorsOutlined,
  BellOutlined,
  CarOutlined,
  CheckOutlined,
  DashboardOutlined,
  LogoutOutlined,
  GiftOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  PrinterOutlined,
  QrcodeOutlined,
  SettingOutlined,
  ShopOutlined,
  ShoppingCartOutlined,
  TeamOutlined,
  TransactionOutlined,
  UsergroupAddOutlined,
} from '@ant-design/icons';
import { App as AntApp, Avatar, Badge, Button, Dropdown, Layout, Menu, Tooltip, Typography, type MenuProps } from 'antd';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import tanbanIcon from '../assets/brand/tanban-icon.png';
import { useAuth } from '../auth/AuthContext';
import { isMerchantStaff } from '../auth/permissions';
import { initials } from '../utils/format';
import { notificationApi, NOTIFICATIONS_CHANGED_EVENT } from '../features/notifications/api';

const { Header, Sider, Content } = Layout;

const managementNavigationItems: MenuProps['items'] = [
  { key: '/dashboard', label: '经营总览', icon: <DashboardOutlined /> },
  {
    key: 'dine-in-domain', label: '店内', icon: <ShopOutlined />,
    children: [
      { key: '/dine-in/orders', label: '堂食订单' },
      { key: '/dine-in/fast-food-orders', label: '快餐订单' },
      { key: '/dine-in/table-codes', label: '桌码管理', icon: <QrcodeOutlined /> },
      { key: '/dine-in/fast-food-plates', label: '快餐码牌', icon: <QrcodeOutlined /> },
    ],
  },
  {
    key: 'delivery-domain', label: '外卖', icon: <CarOutlined />,
    children: [
      { key: '/delivery/orders', label: <span className="menu-label-with-status"><span>外卖订单</span><em>未开放</em></span> },
    ],
  },
  {
    key: 'catalog-domain', label: '商品管理', icon: <AppstoreOutlined />,
    children: [
      { key: '/products', label: '商品管理' },
      { key: '/catalog', label: '商品配置中心' },
      { key: '/media-library', label: '图片库' },
    ],
  },
  {
    key: 'customer-domain', label: '用户管理', icon: <UsergroupAddOutlined />,
    children: [
      { key: '/customers', label: '用户管理' },
      { key: '/membership', label: '会员管理' },
      { key: '/stored-value', label: '储值管理' },
    ],
  },
  { key: '/decoration', label: '店铺装修', icon: <BgColorsOutlined /> },
  {
    key: 'marketing-domain', label: '营销应用', icon: <GiftOutlined />,
    children: [
      { key: '/marketing', label: '应用中心' },
      { key: '/marketing/coupons', label: '优惠券' },
      { key: '/marketing/popup-ads', label: '弹窗广告' },
      { key: '/marketing/lottery', label: '抽奖活动' },
    ],
  },
  { key: '/payments', label: '支付与退款', icon: <TransactionOutlined /> },
  {
    key: 'settings-domain', label: '设置', icon: <SettingOutlined />,
    children: [
      { key: '/settings/store', label: '门店设置', icon: <ShopOutlined /> },
      { key: '/settings/order', label: '点餐设置', icon: <ShoppingCartOutlined /> },
      { key: '/settings/payment', label: '支付设置', icon: <TransactionOutlined /> },
      { key: '/settings/notifications', label: '通知设置', icon: <BellOutlined /> },
      { key: '/settings/privacy', label: '隐私与客服' },
      { key: '/settings/print', label: '打印设置', icon: <PrinterOutlined /> },
      { key: '/settings/printers', label: '打印机管理', icon: <PrinterOutlined /> },
      { key: '/settings/dine-in-print-template', label: '店内打印模板' },
      { key: '/settings/delivery-print-template', label: <span className="menu-label-with-status"><span>外卖打印模板</span><em>预配置</em></span> },
      { key: '/settings/staff', label: '员工与角色', icon: <TeamOutlined /> },
    ],
  },
];

function navigationRouteKeys(items: MenuProps['items']): string[] {
  const keys: string[] = [];
  for (const item of items || []) {
    if (!item || 'type' in item) continue;
    if (typeof item.key === 'string' && item.key.startsWith('/')) keys.push(item.key);
    if ('children' in item && item.children) keys.push(...navigationRouteKeys(item.children));
  }
  return keys;
}

const staffNavigationItems: MenuProps['items'] = [
  { key: '/dashboard', label: '经营总览', icon: <DashboardOutlined /> },
  { key: '/dine-in/orders', label: '店内订单', icon: <ShoppingCartOutlined /> },
  { key: '/print-jobs', label: '打印任务', icon: <PrinterOutlined /> },
];

export function AppLayout() {
  const { user, workspaces, switchWorkspace, logout } = useAuth();
  const { message } = AntApp.useApp();
  const location = useLocation();
  const navigate = useNavigate();
  const [collapsed, setCollapsed] = useState(false);
  const [mobile, setMobile] = useState(false);
  const [unreadNotifications, setUnreadNotifications] = useState(0);
  const navigationItems = useMemo(
    () => isMerchantStaff(user) ? staffNavigationItems : managementNavigationItems,
    [user],
  );

  useEffect(() => {
    if (mobile) setCollapsed(true);
  }, [location.pathname, mobile]);

  const loadUnreadNotifications = useCallback(async () => {
    if (!user) return;
    try {
      setUnreadNotifications(await notificationApi.unreadCount());
    } catch {
      // Header polling must not interrupt order operations when the notification service is unavailable.
    }
  }, [user]);

  useEffect(() => {
    void loadUnreadNotifications();
    const timer = window.setInterval(() => void loadUnreadNotifications(), 60_000);
    const refresh = () => void loadUnreadNotifications();
    window.addEventListener('focus', refresh);
    window.addEventListener(NOTIFICATIONS_CHANGED_EVENT, refresh);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener('focus', refresh);
      window.removeEventListener(NOTIFICATIONS_CHANGED_EVENT, refresh);
    };
  }, [loadUnreadNotifications]);

  const selectedKey = useMemo(() => navigationRouteKeys(navigationItems)
    .sort((left, right) => right.length - left.length)
    .find((key) => location.pathname.startsWith(key)) ?? '/dashboard', [location.pathname, navigationItems]);

  const workspaceMenuItems: MenuProps['items'] = workspaces.map((workspace) => ({
    key: `workspace:${workspace.tenantId}`,
    icon: String(workspace.tenantId) === String(user?.tenantId) ? <CheckOutlined /> : <ShopOutlined />,
    label: workspace.storeName || workspace.tenantName,
    disabled: String(workspace.tenantId) === String(user?.tenantId),
  }));
  const accountMenu: MenuProps['items'] = [
    { key: 'store-heading', icon: <ShopOutlined />, label: workspaces.length > 1 ? '切换店铺' : (user?.storeName || user?.merchantName || '当前门店'), disabled: true },
    ...(workspaces.length > 1 ? workspaceMenuItems : []),
    { type: 'divider' },
    { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', danger: true },
  ];

  const accountMenuClick: NonNullable<MenuProps['onClick']> = ({ key }) => {
    if (key === 'logout') {
      logout();
      navigate('/login', { replace: true });
      return;
    }
    if (key.startsWith('workspace:')) {
      const tenantId = key.slice('workspace:'.length);
      void switchWorkspace(tenantId)
        .then(() => window.location.assign('/dashboard'))
        .catch((reason: unknown) => void message.error(reason instanceof Error ? reason.message : '切换店铺失败'));
    }
  };

  return (
    <Layout className="merchant-layout">
      <Sider
        className="merchant-sider"
        width={232}
        collapsedWidth={mobile ? 0 : 72}
        collapsed={collapsed}
        trigger={null}
        breakpoint="lg"
        onBreakpoint={(broken) => {
          setMobile(broken);
          setCollapsed(broken);
        }}
      >
        <button className="sider-brand" type="button" onClick={() => navigate('/dashboard')} aria-label="返回经营总览">
          <span className="tanban-brand-mark"><img className="tanban-brand-image" src={tanbanIcon} alt="" /></span>
          {!collapsed && <span className="brand-word"><strong>摊伴</strong><small>TANBAN</small></span>}
        </button>
        <Menu
          className="merchant-menu"
          mode="inline"
          theme="dark"
          items={navigationItems}
          selectedKeys={[selectedKey]}
          defaultOpenKeys={['dine-in-domain', 'catalog-domain', 'customer-domain', 'marketing-domain', 'settings-domain']}
          onClick={({ key }) => navigate(key)}
        />
        {!collapsed && (workspaces.length > 1 ? (
          <Dropdown menu={{ items: workspaceMenuItems, onClick: accountMenuClick }} trigger={['click']} placement="topLeft">
            <button type="button" className="sider-store-card is-switchable"><ShopOutlined /><div><small>当前门店 · 点击切换</small><strong>{user?.storeName || user?.merchantName || '我的门店'}</strong></div></button>
          </Dropdown>
        ) : <div className="sider-store-card"><ShopOutlined /><div><small>当前门店</small><strong>{user?.storeName || user?.merchantName || '我的门店'}</strong></div></div>)}
      </Sider>
      {mobile && !collapsed && <button className="sider-mask" type="button" aria-label="关闭菜单" onClick={() => setCollapsed(true)} />}
      <Layout className="merchant-main-layout">
        <Header className="merchant-header">
          <Button
            type="text"
            className="collapse-button"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={() => setCollapsed((value) => !value)}
          />
          <div className="header-spacer" />
          <Tooltip title="系统通知">
            <Badge count={unreadNotifications} overflowCount={99} size="small" offset={[-3, 3]}><Button aria-label="打开系统通知" type="text" className="header-action" icon={<BellOutlined />} onClick={() => navigate('/notifications')} /></Badge>
          </Tooltip>
          <span className="header-divider" />
          <Dropdown
            menu={{
              items: accountMenu,
              onClick: accountMenuClick,
            }}
          >
            <button type="button" className="account-trigger">
              <Avatar src={user?.avatar} className="account-avatar">{!user?.avatar && initials(user?.name)}</Avatar>
              <span className="account-copy"><Typography.Text strong>{user?.name || '商户管理员'}</Typography.Text><small>{user?.roles?.[0] === 'MERCHANT_OWNER' ? '老板' : user?.roles?.[0] === 'MERCHANT_MANAGER' ? '店长' : '店员'}</small></span>
            </button>
          </Dropdown>
        </Header>
        <Content className="merchant-content"><div className="merchant-content-inner"><Outlet /></div></Content>
      </Layout>
    </Layout>
  );
}
