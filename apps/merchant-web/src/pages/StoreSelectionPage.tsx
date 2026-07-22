/* eslint-disable @next/next/no-img-element -- merchant logos are uploaded runtime assets */
import { ArrowRightOutlined, LogoutOutlined, ShopOutlined } from '@ant-design/icons';
import { Alert, Avatar, Button, Card, Space, Typography } from 'antd';
import { useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import tanbanIcon from '../assets/brand/tanban-icon.png';
import { errorMessage } from '../api/client';
import { useAuth } from '../auth/AuthContext';

export function StoreSelectionPage() {
  const { user, pendingWorkspaces, selectionRequired, selectWorkspace, logout } = useAuth();
  const navigate = useNavigate();
  const [selecting, setSelecting] = useState<string | number | null>(null);
  const [error, setError] = useState('');

  if (user) return <Navigate to="/dashboard" replace />;
  if (!selectionRequired) return <Navigate to="/login" replace />;

  const choose = async (tenantId: string | number) => {
    setSelecting(tenantId);
    setError('');
    try {
      await selectWorkspace(tenantId);
      navigate('/dashboard', { replace: true });
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setSelecting(null);
    }
  };

  return (
    <main className="store-selection-page">
      <section className="store-selection-shell">
        <header className="store-selection-header">
          <div className="store-selection-brand"><img src={tanbanIcon} alt="" /><span><strong>摊伴</strong><small>TANBAN</small></span></div>
          <Button icon={<LogoutOutlined />} onClick={() => { logout(); navigate('/login', { replace: true }); }}>返回登录</Button>
        </header>
        <div className="store-selection-copy">
          <Typography.Title level={2}>选择要管理的店铺</Typography.Title>
          <Typography.Paragraph type="secondary">每家店铺的数据、商品、订单和员工相互独立，切换不会影响其他店铺。</Typography.Paragraph>
        </div>
        {error && <Alert type="error" showIcon message={error} closable onClose={() => setError('')} />}
        <div className="store-selection-grid">
          {pendingWorkspaces.map((workspace) => (
            <Card
              key={String(workspace.tenantId)}
              hoverable
              className="store-selection-card"
              onClick={() => void choose(workspace.tenantId)}
            >
              <Space align="center" size={16}>
                <Avatar shape="square" size={58} src={workspace.storeLogoUrl} icon={<ShopOutlined />} />
                <div className="store-selection-name">
                  <Typography.Text strong>{workspace.storeName || workspace.tenantName}</Typography.Text>
                  <Typography.Text type="secondary">老板账号 · 独立经营数据</Typography.Text>
                </div>
                <Button type="primary" shape="circle" icon={<ArrowRightOutlined />} loading={String(selecting) === String(workspace.tenantId)} aria-label={`进入${workspace.storeName}`} />
              </Space>
            </Card>
          ))}
        </div>
      </section>
    </main>
  );
}
