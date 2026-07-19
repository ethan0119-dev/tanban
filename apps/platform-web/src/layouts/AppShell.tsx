import {
  ApartmentOutlined,
  AuditOutlined,
  BankOutlined,
  DashboardOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  ShopOutlined,
  TeamOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { Avatar, Button, Dropdown, Grid, Layout, Menu, Space, Tag, Typography } from 'antd';
import type { MenuProps } from 'antd';
import { useEffect, useState } from 'react';
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { Brand } from '../components/Brand';
import { useAuth } from '../context/AuthContext';
import { canManagePlatformUsers } from '../lib/permissions';

const { Header, Sider, Content } = Layout;

const allMenuItems: NonNullable<MenuProps['items']> = [
  { key: '/dashboard', label: <Link to="/dashboard">经营总览</Link>, icon: <DashboardOutlined /> },
  { key: '/users', label: <Link to="/users">管理员用户</Link>, icon: <TeamOutlined /> },
  { key: '/tenants', label: <Link to="/tenants">商户管理</Link>, icon: <ShopOutlined /> },
  { key: '/stores', label: <Link to="/stores">门店管理</Link>, icon: <ApartmentOutlined /> },
  {
    key: 'configuration',
    label: '平台配置',
    icon: <SettingOutlined />,
    children: [
      { key: '/settings/payment', label: <Link to="/settings/payment">支付服务商</Link>, icon: <BankOutlined /> },
      { key: '/settings/system', label: <Link to="/settings/system">系统设置</Link>, icon: <SafetyCertificateOutlined /> },
    ],
  },
  { key: '/audit-logs', label: <Link to="/audit-logs">审计日志</Link>, icon: <AuditOutlined /> },
];

export function AppShell() {
  const location = useLocation();
  const navigate = useNavigate();
  const screens = Grid.useBreakpoint();
  const mobile = !screens.lg;
  const [collapsed, setCollapsed] = useState(false);
  const { user, logout } = useAuth();
  const menuItems = canManagePlatformUsers(user)
    ? allMenuItems
    : allMenuItems.filter((item) => item?.key !== '/users');

  useEffect(() => {
    setCollapsed(mobile);
  }, [mobile]);

  const handleLogout = () => {
    logout();
    navigate('/login', { replace: true });
  };

  return (
    <Layout className={`admin-shell ${collapsed ? 'admin-shell--collapsed' : ''} ${mobile ? 'admin-shell--mobile' : ''}`}>
      <Sider
        className="admin-sider"
        width={232}
        collapsedWidth={mobile ? 0 : 76}
        collapsed={collapsed}
        trigger={null}
        theme="dark"
      >
        <Link to="/dashboard" className="admin-sider__brand"><Brand compact={collapsed && !mobile} /></Link>
        <Menu
          mode="inline"
          theme="dark"
          selectedKeys={[location.pathname]}
          defaultOpenKeys={['configuration']}
          items={menuItems}
          onClick={() => { if (mobile) setCollapsed(true); }}
        />
        {!collapsed && <div className="admin-sider__footer"><span>TB</span><div><strong>摊伴 SaaS</strong><small>让小生意更好经营</small></div></div>}
      </Sider>

      {mobile && !collapsed && <button type="button" aria-label="关闭导航" className="admin-sider-mask" onClick={() => setCollapsed(true)} />}

      <Layout className="admin-main">
        <Header className="admin-header">
          <Button
            type="text"
            className="admin-header__trigger"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={() => setCollapsed((value) => !value)}
            aria-label={collapsed ? '展开导航' : '收起导航'}
          />
          <div className="admin-header__title">
            <strong>{import.meta.env.VITE_APP_TITLE || '摊伴系统管理端'}</strong>
            <small>PLATFORM CONSOLE</small>
          </div>
          <Space className="admin-header__actions">
            <Tag color="orange" className="edition-tag">SaaS 管理中心</Tag>
            <Dropdown
              trigger={['click']}
              menu={{
                items: [
                  { key: 'profile', icon: <UserOutlined />, label: '账户信息', disabled: true },
                  { type: 'divider' },
                  { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', danger: true, onClick: handleLogout },
                ],
              }}
            >
              <Space className="account-entry">
                <Avatar size="small" className="account-entry__avatar">{user?.name?.slice(0, 1) || '管'}</Avatar>
                <span className="account-entry__text">
                  <Typography.Text>{user?.name || user?.username || '管理员'}</Typography.Text>
                  <small>{user?.role === 'PLATFORM_ADMIN' ? '超级管理员' : user?.role === 'PLATFORM_OPERATOR' ? '运营管理员' : user?.role || '平台管理员'}</small>
                </span>
              </Space>
            </Dropdown>
          </Space>
        </Header>
        <Content className="admin-content">
          <main className="app-content"><Outlet /></main>
        </Content>
      </Layout>
    </Layout>
  );
}
