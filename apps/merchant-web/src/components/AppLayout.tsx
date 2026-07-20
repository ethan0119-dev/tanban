/* eslint-disable @next/next/no-img-element -- this Vite app imports a fingerprinted local brand asset */
import {
  AppstoreOutlined,
  BgColorsOutlined,
  BellOutlined,
  CarOutlined,
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
import { Avatar, Badge, Button, Dropdown, Layout, Menu, Tooltip, Typography, type MenuProps } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import tanbanLogo from '../assets/brand/tanban-logo-web.png';
import { useAuth } from '../auth/AuthContext';
import { isMerchantStaff } from '../auth/permissions';
import { initials } from '../utils/format';

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
      { key: '/dine-in/print-template', label: '店内打印模板' },
    ],
  },
  {
    key: 'delivery-domain', label: '外卖', icon: <CarOutlined />,
    children: [
      { key: '/delivery/orders', label: <span className="menu-label-with-status"><span>外卖订单</span><em>未开放</em></span> },
      { key: '/delivery/print-template', label: <span className="menu-label-with-status"><span>外卖打印模板</span><em>预配置</em></span> },
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
  { key: '/printers', label: '打印中心', icon: <PrinterOutlined /> },
  { key: '/staff', label: '员工与角色', icon: <TeamOutlined /> },
  { key: '/settings', label: '门店设置', icon: <SettingOutlined /> },
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
  const { user, logout } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const [collapsed, setCollapsed] = useState(false);
  const [mobile, setMobile] = useState(false);
  const navigationItems = useMemo(
    () => isMerchantStaff(user) ? staffNavigationItems : managementNavigationItems,
    [user],
  );

  useEffect(() => {
    if (mobile) setCollapsed(true);
  }, [location.pathname, mobile]);

  const selectedKey = useMemo(() => navigationRouteKeys(navigationItems)
    .sort((left, right) => right.length - left.length)
    .find((key) => location.pathname.startsWith(key)) ?? '/dashboard', [location.pathname, navigationItems]);

  const accountMenu: MenuProps['items'] = [
    { key: 'store', icon: <ShopOutlined />, label: user?.storeName || user?.merchantName || '当前门店', disabled: true },
    { type: 'divider' },
    { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', danger: true },
  ];

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
          <img className="tanban-brand-image" src={tanbanLogo} alt="摊伴 TANBAN" />
        </button>
        <Menu
          className="merchant-menu"
          mode="inline"
          theme="dark"
          items={navigationItems}
          selectedKeys={[selectedKey]}
          defaultOpenKeys={['dine-in-domain', 'catalog-domain', 'customer-domain', 'marketing-domain']}
          onClick={({ key }) => navigate(key)}
        />
        {!collapsed && <div className="sider-store-card"><ShopOutlined /><div><small>当前门店</small><strong>{user?.storeName || user?.merchantName || '我的门店'}</strong></div></div>}
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
          <Tooltip title="订单消息">
            <Badge dot offset={[-3, 3]}><Button type="text" className="header-action" icon={<BellOutlined />} /></Badge>
          </Tooltip>
          <span className="header-divider" />
          <Dropdown
            menu={{
              items: accountMenu,
              onClick: ({ key }) => {
                if (key === 'logout') {
                  logout();
                  navigate('/login', { replace: true });
                }
              },
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
